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
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/chengxilo/serify/internal/typekind"
)

// A type is written in compact YAML. A record lists its fields under `fields:`,
// each a single `name: type` pair; a sum lists its arms under `variants:` (see
// rawType). The type's name is its filename; `import:` reuses other type files.
// Type expressions reuse the ParseType grammar (list<T>, array<T,N>,
// optional<T>, map<K,V>, scalars + aliases).
//
//	# address.yaml          (a reusable type: no cases)
//	fields:
//	  - street: string
//	  - zip:    uint32
//
//	# user.yaml
//	import:
//	  - address.yaml
//	fields:
//	  - user_id: uint64
//	  - counts:  array<uint32,4>
//	  - address: address          # type `address` (address.yaml) -> nested struct

// rawTypeField is one entry of a compact type file before type resolution.
type rawTypeField struct {
	name string
	typ  string
}

// rawType is a named type as loaded, before resolution: its ordered entries plus
// how to read them.
//
// A type declares either `fields:` (a record: each entry is a field, which needs
// a type) or `variants:` (a sum: each entry is `tag: payload`, and an entry with
// no type is a unit variant). The section name is the declaration — there is no
// separate flag, and the two are mutually exclusive by construction.
//
// A sum referenced from another type's field is that field's type directly —
// bare, at the referencing key — so `stream_id: identifier` is a oneof at
// `stream_id`. As the type under test it becomes the `{value: <variant>}` record
// a standalone sum needs (see sumSchema), because the top level of a schema is a
// field list and a bare sum is not one.
//
// `transparent: true` is the record-side equivalent for a newtype over a single
// field: referenced as a field it inlines that field's type instead of nesting as
// a one-field struct. A sum needs no such flag, so `transparent:` on a
// `variants:` type is an error.
type rawType struct {
	fields      []rawTypeField
	transparent bool
	sum         bool
}

// compactFields converts a list of single-key YAML maps (`- name: type`) into
// ordered raw fields.
func compactFields(items []map[string]string) ([]rawTypeField, error) {
	out := make([]rawTypeField, 0, len(items))
	for _, m := range items {
		if len(m) != 1 {
			return nil, fmt.Errorf("each entry must be one `name: type` pair, got %v", m)
		}
		for name, typ := range m {
			out = append(out, rawTypeField{name: name, typ: typ})
		}
	}
	return out, nil
}

// allowedKeys are the top-level sections a type file may declare. YAML decoding
// drops keys it does not recognise without a word, so a stale or misspelled one
// silently changes what the file means — `feilds:` would read as a type with no
// entries at all. Keys starting with "_" are scratch space (YAML anchors) and are
// deliberately left alone.
var allowedKeys = []string{"cases", "fields", "formats", "import", "transparent", "variants"}

// checkKeys rejects top-level keys outside allowedKeys.
func checkKeys(path string, raw []byte) error {
	var top map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &top); err != nil {
		return nil // the typed decode reports the real parse error
	}
	var unknown []string
	for k := range top {
		if !strings.HasPrefix(k, "_") && !slices.Contains(allowedKeys, k) {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	return fmt.Errorf("%s: unknown key %s (allowed: %s)",
		path, strings.Join(unknown, ", "), strings.Join(allowedKeys, ", "))
}

// checkSections validates the entry list against the section it came from. A
// `fields:` entry is a field and needs a type; a `variants:` entry is a variant
// and may omit one, which makes it a unit variant.
func checkSections(path string, fields, variants []map[string]string, fs []rawTypeField, transparent bool) error {
	switch {
	case len(fields) > 0 && len(variants) > 0:
		return fmt.Errorf("%s: a type is either a record (`fields:`) or a sum (`variants:`), not both", path)
	case len(fields) == 0 && len(variants) == 0:
		return fmt.Errorf("%s: file has no `fields:` or `variants:`", path)
	}
	if len(variants) > 0 {
		if transparent {
			return fmt.Errorf("%s: `transparent:` applies to a `fields:` record; "+
				"a sum is already inlined at the referencing field", path)
		}
		return nil
	}
	if transparent && len(fs) != 1 {
		return fmt.Errorf("%s: a transparent type must have exactly one field, got %d", path, len(fs))
	}
	for _, f := range fs {
		if strings.TrimSpace(f.typ) == "" {
			return fmt.Errorf("%s: field %q has no type "+
				"(an entry with no type is a unit variant, which belongs under `variants:`)", path, f.name)
		}
	}
	return nil
}

// loadImportable returns the named type a file contributes when `import`ed: the
// filename -> its `fields:`/`variants:` (cases, if any, are ignored here). It follows
// the file's own `import:` list transitively; `seen` breaks cycles and avoids
// re-loading.
func loadImportable(path string, seen map[string]bool) (map[string]rawType, error) {
	abs, _ := filepath.Abs(path)
	if seen[abs] {
		return map[string]rawType{}, nil
	}
	seen[abs] = true

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("import %s: %w", path, err)
	}
	if err := checkKeys(path, raw); err != nil {
		return nil, err
	}
	var hdr struct {
		Fields      []map[string]string `yaml:"fields"`
		Variants    []map[string]string `yaml:"variants"`
		Import      []string            `yaml:"import"`
		Transparent bool                `yaml:"transparent"`
	}
	if err := yaml.Unmarshal(raw, &hdr); err != nil {
		return nil, fmt.Errorf(errParseFmt, path, err)
	}

	// The imported type's name is its filename (without .yaml).
	name := strings.TrimSuffix(filepath.Base(path), extYAML)
	entries := hdr.Fields
	if len(hdr.Variants) > 0 {
		entries = hdr.Variants
	}
	fs, err := compactFields(entries)
	if err != nil {
		return nil, fmt.Errorf(errWrapFmt, path, err)
	}
	if err := checkSections(path, hdr.Fields, hdr.Variants, fs, hdr.Transparent); err != nil {
		return nil, err
	}
	out := map[string]rawType{name: {fields: fs, transparent: hdr.Transparent, sum: len(hdr.Variants) > 0}}

	for _, imp := range hdr.Import {
		sub, err := loadImportable(filepath.Join(filepath.Dir(path), imp), seen)
		if err != nil {
			return nil, err
		}
		maps.Copy(out, sub)
	}
	return out, nil
}

// schemaResolver resolves compact type expressions into FieldTypes, inlining
// references to named struct types recursively (memoized, cycle-checked).
type schemaResolver struct {
	named     map[string]rawType
	resolved  map[string][]Field
	resolving map[string]bool
}

func newSchemaResolver(named map[string]rawType) *schemaResolver {
	return &schemaResolver{
		named:     named,
		resolved:  map[string][]Field{},
		resolving: map[string]bool{},
	}
}

func (r *schemaResolver) fields(raw []rawTypeField) ([]Field, error) {
	out := make([]Field, 0, len(raw))
	for _, rf := range raw {
		ft, err := r.typeOf(rf.typ)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", rf.name, err)
		}
		out = append(out, Field{Name: rf.name, Type: ft})
	}
	return out, nil
}

func (r *schemaResolver) namedType(name string) ([]Field, error) {
	if f, ok := r.resolved[name]; ok {
		return f, nil
	}
	if r.resolving[name] {
		return nil, fmt.Errorf("recursive type %q", name)
	}
	r.resolving[name] = true
	defer delete(r.resolving, name)

	f, err := r.fields(r.named[name].fields)
	if err != nil {
		return nil, fmt.Errorf("type %q: %w", name, err)
	}
	r.resolved[name] = f
	return f, nil
}

// typeOf resolves a type expression: scalars (with aliases), named struct types,
// and the parameterized forms list<T>/array<T,N>/optional<T>/map<K,V>/enum<...>.
// It is the single implementation of the type grammar; ParseType is the
// no-named-types entry point.
//
//nolint:gocognit,funlen // type-expression parser: one branch per type constructor
func (r *schemaResolver) typeOf(expr string) (FieldType, error) {
	expr = strings.TrimSpace(expr)
	if canon, ok := scalarAliases[expr]; ok {
		expr = canon
	}
	if slices.Contains(typekind.Scalars, expr) {
		return FieldType{Base: expr}, nil
	}
	if expr == typekind.Struct {
		return FieldType{Base: typekind.Struct}, nil
	}
	if nt, ok := r.named[expr]; ok {
		// A sum is its variants: as a field it *is* the sum, so
		// `stream_id: identifier` is a oneof at `stream_id` rather than a struct
		// wrapping one. (It becomes a `{value: <variant>}` record only when it is
		// itself the type under test — see sumSchema.)
		if nt.sum {
			if r.resolving[expr] {
				return FieldType{}, fmt.Errorf("recursive type %q", expr)
			}
			r.resolving[expr] = true
			defer delete(r.resolving, expr)
			return r.sumType(nt.fields)
		}
		// A transparent type is a newtype: as a field it contributes its single
		// inner field's type directly. (The type resolves as an ordinary struct
		// only when it is itself the type under test.)
		if nt.transparent {
			return r.typeOf(nt.fields[0].typ)
		}
		f, err := r.namedType(expr)
		if err != nil {
			return FieldType{}, err
		}
		return FieldType{Base: typekind.Struct, Fields: f}, nil
	}

	m := typeParamRe.FindStringSubmatch(expr)
	if m == nil {
		return FieldType{}, fmt.Errorf("unknown type %q", expr)
	}
	outer, inner := m[1], m[2]
	switch outer {
	case typekind.Enum:
		var vals []string
		for v := range strings.SplitSeq(inner, ",") {
			if v = strings.TrimSpace(v); v != "" {
				vals = append(vals, v)
			}
		}
		if len(vals) == 0 {
			return FieldType{}, fmt.Errorf("enum requires at least one value: %q", expr)
		}
		return FieldType{Base: typekind.Enum, Values: vals}, nil
	case typekind.Optional, typekind.List:
		elem, err := r.typeOf(inner)
		if err != nil {
			return FieldType{}, err
		}
		return FieldType{Base: outer, Elem: &elem}, nil
	case typekind.Array:
		comma := strings.LastIndex(inner, ",")
		if comma < 0 {
			return FieldType{}, fmt.Errorf("array type requires size: array<T,N>, got %q", expr)
		}
		elem, err := r.typeOf(inner[:comma])
		if err != nil {
			return FieldType{}, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(inner[comma+1:]))
		if err != nil {
			return FieldType{}, fmt.Errorf("array size must be an integer in %q", expr)
		}
		return FieldType{Base: typekind.Array, Elem: &elem, ArrayN: n}, nil
	case typekind.Map:
		kv := typekind.SplitTopLevel(inner)
		if len(kv) != 2 {
			return FieldType{}, fmt.Errorf("map type requires key and value: map<K,V>, got %q", expr)
		}
		key, err := r.typeOf(kv[0])
		if err != nil {
			return FieldType{}, err
		}
		val, err := r.typeOf(strings.TrimSpace(kv[1]))
		if err != nil {
			return FieldType{}, err
		}
		return FieldType{Base: typekind.Map, Key: &key, Elem: &val}, nil
	case typekind.OneOf:
		return FieldType{}, fmt.Errorf(
			"%q: a sum is declared with a `variants:` section on its own type file, not "+
				"as a type expression — put the arms there (an entry with no type is a unit "+
				"variant) and reference the type by name", expr)
	default:
		return FieldType{}, fmt.Errorf("unknown parameterized type %q", outer)
	}
}

// sumType builds the oneof for a `sum: true` type from its entries: each entry is
// one variant, `tag: payload-type`, and an entry with no type is a unit variant.
func (r *schemaResolver) sumType(raw []rawTypeField) (FieldType, error) {
	if len(raw) == 0 {
		return FieldType{}, fmt.Errorf("a sum requires at least one variant")
	}
	variants := make([]Variant, 0, len(raw))
	for _, rf := range raw {
		v := Variant{Name: rf.name}
		if typeExpr := strings.TrimSpace(rf.typ); typeExpr != "" {
			pt, err := r.typeOf(typeExpr)
			if err != nil {
				return FieldType{}, fmt.Errorf("variant %q: %w", rf.name, err)
			}
			v.Type = &pt
		}
		variants = append(variants, v)
	}
	return FieldType{Base: typekind.OneOf, Variants: variants}, nil
}

// sumSchema is the schema of a sum type when it is the type under test. The top
// level of a schema is a list of named fields, so a bare sum cannot sit there;
// it is carried as the single field `value`, which is also where a referencing
// field's key would put it.
func (r *schemaResolver) sumSchema(raw []rawTypeField) ([]Field, error) {
	ft, err := r.sumType(raw)
	if err != nil {
		return nil, err
	}
	return []Field{{Name: sumValueKey, Type: ft}}, nil
}

// sumValueKey is the field a standalone sum is carried under. The worker
// libraries' model bindings hardcode the same name.
const sumValueKey = "value"
