// Copyright 2026 Chengxi Luo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chengxilo/serify/internal/typekind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestImport(t *testing.T) {
	dir := t.TempDir()
	// Every type is a `fields:` record. order imports a leaf type (label) and a
	// whole type (buyer), which itself imports address — so order picks up
	// address transitively without naming it directly.
	mustWrite(t, filepath.Join(dir, "address.yaml"), "fields:\n  - street: string\n  - zip: uint32\n")
	mustWrite(t, filepath.Join(dir, "label.yaml"), "fields:\n  - value: string\n  - priority: uint32\n")
	mustWrite(t, filepath.Join(dir, "buyer.yaml"),
		"import:\n  - address.yaml\nfields:\n  - id: uint64\n  - home: address\n")
	mustWrite(t, filepath.Join(dir, "order.yaml"),
		"import:\n  - label.yaml\n  - buyer.yaml\nfields:\n  - tag: label\n  - who: buyer\n")

	cf, err := LoadCases(filepath.Join(dir, "order.yaml"))
	require.NoError(t, err, "LoadCases: %v", err)
	byName := map[string]FieldType{}
	for _, f := range cf.Schema {
		byName[f.Name] = f.Type
	}

	// tag <- imported registry (_labels.yaml)
	tag := byName["tag"]
	assert.Equal(t, "struct", tag.Base, "imported Label not resolved: %+v", tag)
	assert.Len(t, tag.Fields, 2, "imported Label not resolved: %+v", tag)
	// who <- imported per-type file (buyer); buyer.home <- auto-loaded Address
	who := byName["who"]
	require.Equal(t, "struct", who.Base, "imported buyer not resolved: %+v", who)
	require.Len(t, who.Fields, 2, "imported buyer not resolved: %+v", who)
	assert.Equal(t, "home", who.Fields[1].Name, "buyer.home (cross-ref to Address) not resolved: %+v", who.Fields[1])
	assert.Equal(t, "struct", who.Fields[1].Type.Base, "buyer.home (cross-ref to Address) not resolved: %+v", who.Fields[1])
}

func TestSchemaResolver(t *testing.T) {
	named := map[string]rawType{
		"Point": {fields: []rawTypeField{{name: "x", typ: "int32"}, {name: "y", typ: "int32"}}},
	}
	r := newSchemaResolver(named)

	// Every type form resolves to the expected canonical String() (long form).
	cases := map[string]string{
		"uint64":             "uint64",
		"string":             "string",
		"array<uint32,4>":    "array<uint32,4>",
		"list<string>":       "list<string>",
		"optional<string>":   "optional<string>",
		"map<string,uint32>": "map<string,uint32>",
		"Point":              "struct", // named type -> nested struct
	}
	for in, want := range cases {
		ft, err := r.typeOf(in)
		if !assert.NoError(t, err, "typeOf(%q): %v", in, err) {
			continue
		}
		assert.Equal(t, want, ft.String(), "typeOf(%q) = %q, want %q", in, ft.String(), want)
	}

	// The named type's fields are inlined.
	pt, _ := r.typeOf("Point")
	if assert.Len(t, pt.Fields, 2, "named type not resolved: %+v", pt.Fields) {
		assert.Equal(t, "x", pt.Fields[0].Name, "named type not resolved: %+v", pt.Fields)
		assert.Equal(t, "int32", pt.Fields[0].Type.Base, "named type not resolved: %+v", pt.Fields)
	}

	_, err := r.typeOf("nope")
	assert.Error(t, err, "expected error for unknown type")
}

// A transparent named type is a newtype: referenced as a field it contributes
// its single inner field's type directly (bare), not a one-field struct.
func TestSchemaResolver_Transparent(t *testing.T) {
	r := newSchemaResolver(map[string]rawType{
		"wire_name": {
			fields: []rawTypeField{{
				name: "value",
				typ:  "string",
			},
			},
			transparent: true},
	})

	ft, err := r.typeOf("wire_name")
	require.NoError(t, err, "typeOf(wire_name): %v", err)
	assert.Equal(t, typekind.String, ft.Base, "transparent field ref = %q, want a bare string", ft.String())
}

// A `variants:` type's entries are variants, and an entry with no type is a unit
// variant. Referenced as a field the type *is* the sum; as the type under test it
// becomes the `{value: <variant>}` record the top level of a schema requires.
func TestSchemaResolver_Sum(t *testing.T) {
	named := map[string]rawType{
		"identifier": {
			fields: []rawTypeField{
				{name: "numeric", typ: "uint32"},
				{name: "string", typ: "string"},
				{name: "unset"}, // no type: a unit variant
			},
			sum: true,
		},
	}
	r := newSchemaResolver(named)

	// Referenced as a field: the sum itself, no wrapping struct.
	ft, err := r.typeOf("identifier")
	require.NoError(t, err, "typeOf(identifier): %v", err)
	require.Equal(t, typekind.Sum, ft.Base, "sum field ref = %q, want a bare sum", ft.String())
	require.Len(t, ft.Variants, 3, "variants = %d, want 3", len(ft.Variants))
	assert.Equal(t, "numeric", ft.Variants[0].Name, "variant[0] = %+v, want numeric: uint32", ft.Variants[0])
	assert.Equal(t, typekind.Uint32, ft.Variants[0].Type.Base, "variant[0] = %+v, want numeric: uint32", ft.Variants[0])
	assert.Equal(t, "unset", ft.Variants[2].Name, "variant[2] = %+v, want the unit variant `unset`", ft.Variants[2])
	assert.Nil(t, ft.Variants[2].Type, "variant[2] = %+v, want the unit variant `unset`", ft.Variants[2])

	// As the type under test: the sum carried under `value`.
	standalone, err := r.sumSchema(named["identifier"].fields)
	require.NoError(t, err, "sumSchema: %v", err)
	if assert.Len(t, standalone, 1, "sum standalone = %+v, want a {value: sum} record", standalone) {
		assert.Equal(t, "value", standalone[0].Name, "sum standalone = %+v, want a {value: sum} record", standalone)
		assert.Equal(t, typekind.Sum, standalone[0].Type.Base, "sum standalone = %+v, want a {value: sum} record", standalone)
	}
}

// The sum type expression is gone; the error must say what replaced it rather
// than reporting an unknown type.
func TestSchemaResolver_SumExpressionRejected(t *testing.T) {
	r := newSchemaResolver(map[string]rawType{})
	_, err := r.typeOf("sum<a: uint32, b>")
	require.Error(t, err, "expected the sum type expression to be rejected")
	assert.Contains(t, err.Error(), "variants:", "error should point at `variants:`: %v", err)
}

// A `fields:` entry with no type is a missing type; the same entry under
// `variants:` is a unit variant. The section it sits in decides.
func TestCheckSections_UntypedEntry(t *testing.T) {
	entries := []map[string]string{{"x": "uint32"}, {"oops": ""}}
	fs := []rawTypeField{{name: "x", typ: "uint32"}, {name: "oops"}}

	err := checkSections("t.yaml", entries, nil, fs, false)
	require.Error(t, err, "expected an error for a schema field with no type")
	assert.Contains(t, err.Error(), "variants:", "error should point at `variants:`: %v", err)
	assert.NoError(t, checkSections("t.yaml", nil, entries, fs, false), "the same entries are legal under variants")
}

// A type is a record or a sum, never both and never neither.
func TestCheckSections_ExactlyOneSection(t *testing.T) {
	entries := []map[string]string{{"a": "uint32"}}
	fs := []rawTypeField{{name: "a", typ: "uint32"}}

	assert.Error(t, checkSections("t.yaml", entries, entries, fs, false), "expected both sections at once to be rejected")
	assert.Error(t, checkSections("t.yaml", nil, nil, nil, false), "expected a file with neither section to be rejected")
	// transparent modifies a record; a sum is already inlined at the reference.
	assert.Error(t, checkSections("t.yaml", nil, entries, fs, true), "expected transparent on a sum to be rejected")
}

// An unknown top-level key is an error: YAML drops what it does not recognise, so
// a misspelled section would silently read as an empty type.
func TestCheckKeys(t *testing.T) {
	assert.NoError(t, checkKeys("t.yaml", []byte("variants:\n  - a: uint32\n")), "known keys must pass")
	// The keys this replaced (schema:, and the earlier sum:), and a plain typo.
	for _, src := range []string{"schema:\n  - a: uint32\n", "sum: true\nfields:\n  - a: uint32\n", "feilds:\n  - a: uint32\n"} {
		err := checkKeys("t.yaml", []byte(src))
		if !assert.Error(t, err, "expected an unknown-key error for %q", src) {
			continue
		}
		assert.Contains(t, err.Error(), "unknown key", "error should name the problem: %v", err)
	}
	// Underscore keys are scratch space (YAML anchors) and stay legal.
	assert.NoError(t, checkKeys("t.yaml", []byte("_anchors: x\nvariants:\n  - a: uint32\n")), "underscore keys must pass")
}

func TestLoadSuite_Directory(t *testing.T) {
	set, err := LoadSuite(filepath.Join("..", "..", "examples", "cases"))
	require.NoError(t, err, "LoadSuite: %v", err)
	byName := map[string]*CasesFile{}
	for _, ty := range set.Types {
		byName[ty.Name] = ty
	}
	require.Contains(t, byName, "customer", "missing type customer")
	require.Contains(t, byName, "order", "missing type order")
	order := byName["order"]

	// The shared `address` struct must be resolved into both types via `ref:`.
	for _, ty := range []string{"customer", "order"} {
		var addr *FieldType
		for _, f := range byName[ty].Schema {
			if f.Type.Base == "struct" && len(f.Type.Fields) >= 2 {
				ft := f.Type
				addr = &ft
			}
		}
		require.NotNil(t, addr, "type %q: no struct field using shared address", ty)
		assert.GreaterOrEqual(t, len(addr.Fields), 2, "type %q: shared address not resolved (got %d fields): %+v", ty, len(addr.Fields), addr.Fields)
	}

	assert.NotEmpty(t, order.Cases, "order has no cases")
}

func TestParseType_Aliases(t *testing.T) {
	// Canonical names are the long form; a few aliases normalize to them.
	cases := map[string]string{
		"float":   "float32",
		"double":  "float64",
		"boolean": "bool",
		// canonical long forms pass through unchanged
		"uint32":  "uint32",
		"uint128": "uint128",
		"int64":   "int64",
		"string":  "string",
		// aliases inside parameterized types are normalized recursively
		"list<float>":            "list<float32>",
		"optional<double>":       "optional<float64>",
		"array<uint32,4>":        "array<uint32,4>",
		"map<string,uint64>":     "map<string,uint64>",
		"list<map<int32,uint8>>": "list<map<int32,uint8>>",
	}
	for in, want := range cases {
		ft, err := ParseType(in)
		if !assert.NoError(t, err, "ParseType(%q): unexpected error %v", in, err) {
			continue
		}
		assert.Equal(t, want, ft.String(), "ParseType(%q).String() = %q, want %q", in, ft.String(), want)
	}
}

func TestParseType_UnknownStillErrors(t *testing.T) {
	// The short forms (u8, f32, …) were removed and must now error.
	for _, in := range []string{"uint", "u33", "nope", "u8", "u64", "i32", "f32", "f64"} {
		_, err := ParseType(in)
		assert.Error(t, err, "ParseType(%q): expected error, got nil", in)
	}
}

// The oracle is per (type, format), not per format. This is the case that
// forced the move: two types share the format name "binary" and want opposite
// verdicts — the one holding a map<K,V> compares by value, while the map-free
// one keeps byte parity, which is the only place cross-language byte agreement
// is actually checked. Resolving per format could not express this.
func TestOracleIsPerTypeNotPerFormat(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "_formats.yaml"), "formats:\n  - binary\n")
	mustWrite(t, filepath.Join(dir, "withmap.yaml"),
		"fields:\n  - m: map<string,uint32>\nformats:\n  - name: binary\n    oracle: semantic\n"+
			"cases:\n  - name: c1\n    data:\n      m: {a: 1}\n")
	mustWrite(t, filepath.Join(dir, "nomap.yaml"),
		"fields:\n  - x: uint32\nformats:\n  - name: binary\n    oracle: bytes\n"+
			"cases:\n  - name: c1\n    data:\n      x: 1\n")

	set, err := LoadSuite(dir)
	require.NoError(t, err, "LoadSuite: %v", err)

	byName := map[string]*CasesFile{}
	for _, ty := range set.Types {
		byName[ty.Name] = ty
	}
	require.Contains(t, byName, "withmap", "withmap type not loaded")
	require.Contains(t, byName, "nomap", "nomap type not loaded")

	assert.Equal(t, OracleSemantic, byName["withmap"].OracleFor("binary"),
		"the map-bearing type must resolve binary to semantic")
	assert.Equal(t, OracleBytes, byName["nomap"].OracleFor("binary"),
		"the map-free type must keep binary on bytes, sharing the name changes nothing")
}

// Declaring the oracle is mandatory. A bare name used to mean "bytes", which
// chose the verdict for the author silently; it is now a load error naming the
// format and showing the fix.
func TestOracleMustBeDeclared(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "t.yaml"),
		"fields:\n  - x: uint32\nformats:\n  - binary\ncases:\n  - name: c1\n    data:\n      x: 1\n")
	_, err := LoadSuite(dir)
	require.Error(t, err, "a bare format name must not load")
	assert.Contains(t, err.Error(), "must declare an oracle", "error = %v", err)
	assert.Contains(t, err.Error(), "binary", "the error must name the offending format; got %v", err)
}

func TestOracleUnknownValue(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "t.yaml"),
		"fields:\n  - x: uint32\nformats:\n  - name: binary\n    oracle: bogus\n"+
			"cases:\n  - name: c1\n    data:\n      x: 1\n")
	_, err := LoadSuite(dir)
	require.Error(t, err, "LoadSuite error = %v, want unknown oracle", err)
	assert.Contains(t, err.Error(), "unknown oracle", "LoadSuite error = %v, want unknown oracle", err)
}

// The registry declares the format-name universe only. An oracle there is an
// author reaching for the old per-format spelling, and is rejected rather than
// silently ignored — ignoring it would leave them believing they had set it.
func TestFormatsRegistryRejectsOracle(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "_formats.yaml"),
		"formats:\n  - name: binary\n    oracle: semantic\n")
	mustWrite(t, filepath.Join(dir, "t.yaml"),
		"fields:\n  - x: uint32\nformats:\n  - name: binary\n    oracle: bytes\n"+
			"cases:\n  - name: c1\n    data:\n      x: 1\n")
	_, err := LoadSuite(dir)
	require.Error(t, err, "an oracle in the registry must be rejected")
	assert.Contains(t, err.Error(), "oracles belong in each type file", "error = %v", err)
}
