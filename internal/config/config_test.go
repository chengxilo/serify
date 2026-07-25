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
	"strings"
	"testing"

	"github.com/chengxilo/serify/internal/typekind"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}
	byName := map[string]FieldType{}
	for _, f := range cf.Schema {
		byName[f.Name] = f.Type
	}

	// tag <- imported registry (_labels.yaml)
	if tag := byName["tag"]; tag.Base != "struct" || len(tag.Fields) != 2 {
		t.Errorf("imported Label not resolved: %+v", tag)
	}
	// who <- imported per-type file (buyer); buyer.home <- auto-loaded Address
	who := byName["who"]
	if who.Base != "struct" || len(who.Fields) != 2 {
		t.Fatalf("imported buyer not resolved: %+v", who)
	}
	if who.Fields[1].Name != "home" || who.Fields[1].Type.Base != "struct" {
		t.Errorf("buyer.home (cross-ref to Address) not resolved: %+v", who.Fields[1])
	}
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
		if err != nil {
			t.Errorf("typeOf(%q): %v", in, err)
			continue
		}
		if got := ft.String(); got != want {
			t.Errorf("typeOf(%q) = %q, want %q", in, got, want)
		}
	}

	// The named type's fields are inlined.
	pt, _ := r.typeOf("Point")
	if len(pt.Fields) != 2 || pt.Fields[0].Name != "x" || pt.Fields[0].Type.Base != "int32" {
		t.Errorf("named type not resolved: %+v", pt.Fields)
	}

	if _, err := r.typeOf("nope"); err == nil {
		t.Error("expected error for unknown type")
	}
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
	if err != nil {
		t.Fatalf("typeOf(wire_name): %v", err)
	}
	if ft.Base != typekind.String {
		t.Errorf("transparent field ref = %q, want a bare string", ft.String())
	}
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
	if err != nil {
		t.Fatalf("typeOf(identifier): %v", err)
	}
	if ft.Base != typekind.Sum {
		t.Fatalf("sum field ref = %q, want a bare sum", ft.String())
	}
	if len(ft.Variants) != 3 {
		t.Fatalf("variants = %d, want 3", len(ft.Variants))
	}
	if ft.Variants[0].Name != "numeric" || ft.Variants[0].Type.Base != typekind.Uint32 {
		t.Errorf("variant[0] = %+v, want numeric: uint32", ft.Variants[0])
	}
	if ft.Variants[2].Name != "unset" || ft.Variants[2].Type != nil {
		t.Errorf("variant[2] = %+v, want the unit variant `unset`", ft.Variants[2])
	}

	// As the type under test: the sum carried under `value`.
	standalone, err := r.sumSchema(named["identifier"].fields)
	if err != nil {
		t.Fatalf("sumSchema: %v", err)
	}
	if len(standalone) != 1 || standalone[0].Name != "value" || standalone[0].Type.Base != typekind.Sum {
		t.Errorf("sum standalone = %+v, want a {value: sum} record", standalone)
	}
}

// The sum type expression is gone; the error must say what replaced it rather
// than reporting an unknown type.
func TestSchemaResolver_SumExpressionRejected(t *testing.T) {
	r := newSchemaResolver(map[string]rawType{})
	_, err := r.typeOf("sum<a: uint32, b>")
	if err == nil {
		t.Fatal("expected the sum type expression to be rejected")
	}
	if !strings.Contains(err.Error(), "variants:") {
		t.Errorf("error should point at `variants:`: %v", err)
	}
}

// A `fields:` entry with no type is a missing type; the same entry under
// `variants:` is a unit variant. The section it sits in decides.
func TestCheckSections_UntypedEntry(t *testing.T) {
	entries := []map[string]string{{"x": "uint32"}, {"oops": ""}}
	fs := []rawTypeField{{name: "x", typ: "uint32"}, {name: "oops"}}

	err := checkSections("t.yaml", entries, nil, fs, false)
	if err == nil {
		t.Fatal("expected an error for a schema field with no type")
	}
	if !strings.Contains(err.Error(), "variants:") {
		t.Errorf("error should point at `variants:`: %v", err)
	}
	if err := checkSections("t.yaml", nil, entries, fs, false); err != nil {
		t.Errorf("the same entries are legal under variants: %v", err)
	}
}

// A type is a record or a sum, never both and never neither.
func TestCheckSections_ExactlyOneSection(t *testing.T) {
	entries := []map[string]string{{"a": "uint32"}}
	fs := []rawTypeField{{name: "a", typ: "uint32"}}

	if err := checkSections("t.yaml", entries, entries, fs, false); err == nil {
		t.Error("expected both sections at once to be rejected")
	}
	if err := checkSections("t.yaml", nil, nil, nil, false); err == nil {
		t.Error("expected a file with neither section to be rejected")
	}
	// transparent modifies a record; a sum is already inlined at the reference.
	if err := checkSections("t.yaml", nil, entries, fs, true); err == nil {
		t.Error("expected transparent on a sum to be rejected")
	}
}

// An unknown top-level key is an error: YAML drops what it does not recognise, so
// a misspelled section would silently read as an empty type.
func TestCheckKeys(t *testing.T) {
	if err := checkKeys("t.yaml", []byte("variants:\n  - a: uint32\n")); err != nil {
		t.Errorf("known keys must pass: %v", err)
	}
	// The keys this replaced (schema:, and the earlier sum:), and a plain typo.
	for _, src := range []string{"schema:\n  - a: uint32\n", "sum: true\nfields:\n  - a: uint32\n", "feilds:\n  - a: uint32\n"} {
		err := checkKeys("t.yaml", []byte(src))
		if err == nil {
			t.Errorf("expected an unknown-key error for %q", src)
			continue
		}
		if !strings.Contains(err.Error(), "unknown key") {
			t.Errorf("error should name the problem: %v", err)
		}
	}
	// Underscore keys are scratch space (YAML anchors) and stay legal.
	if err := checkKeys("t.yaml", []byte("_anchors: x\nvariants:\n  - a: uint32\n")); err != nil {
		t.Errorf("underscore keys must pass: %v", err)
	}
}

func TestLoadSuite_Directory(t *testing.T) {
	set, err := LoadSuite(filepath.Join("..", "..", "examples", "cases"))
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	byName := map[string]*CasesFile{}
	for _, ty := range set.Types {
		byName[ty.Name] = ty
	}
	if _, ok := byName["customer"]; !ok {
		t.Fatal("missing type customer")
	}
	order, ok := byName["order"]
	if !ok {
		t.Fatal("missing type order")
	}

	// The shared `address` struct must be resolved into both types via `ref:`.
	for _, ty := range []string{"customer", "order"} {
		var addr *FieldType
		for _, f := range byName[ty].Schema {
			if f.Type.Base == "struct" && len(f.Type.Fields) >= 2 {
				ft := f.Type
				addr = &ft
			}
		}
		if addr == nil {
			t.Fatalf("type %q: no struct field using shared address", ty)
		}
		if len(addr.Fields) < 2 {
			t.Errorf("type %q: shared address not resolved (got %d fields): %+v", ty, len(addr.Fields), addr.Fields)
		}
	}

	if len(order.Cases) == 0 {
		t.Error("order has no cases")
	}
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
		if err != nil {
			t.Errorf("ParseType(%q): unexpected error %v", in, err)
			continue
		}
		if got := ft.String(); got != want {
			t.Errorf("ParseType(%q).String() = %q, want %q", in, got, want)
		}
	}
}

func TestParseType_UnknownStillErrors(t *testing.T) {
	// The short forms (u8, f32, …) were removed and must now error.
	for _, in := range []string{"uint", "u33", "nope", "u8", "u64", "i32", "f32", "f64"} {
		if _, err := ParseType(in); err == nil {
			t.Errorf("ParseType(%q): expected error, got nil", in)
		}
	}
}
