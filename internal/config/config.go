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
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/chengxilo/serify/internal/typekind"
)

// ErrNoTypesFound is returned by LoadSuite when a directory has no tested types.
// Callers that are tolerant of reusable-only directories can check with errors.Is.
var ErrNoTypesFound = errors.New("no type files found")

const (
	extYAML     = ".yaml"
	errWrapFmt  = "%s: %w"
	errParseFmt = "parse %s: %w"
)

// FieldType represents a parsed type from the schema.
type FieldType struct {
	Base     string     // uint8/16/32/64, int8/16/32/64, float32, float64, bool, string, bytes, optional, list, array, struct, map, enum, sum
	Elem     *FieldType // for optional<T>, list<T>, array<T,N>, map value type
	Key      *FieldType // for map<K,V> key type
	ArrayN   int        // for array<T,N>
	Fields   []Field    // for struct (and list<struct> / optional<struct> / map<K,struct> via Elem)
	Values   []string   // for enum
	Variants []Variant  // for sum
}

// Variant is one arm of a sum<...>: a tag name and its payload type.
// A sum is a sum-of-products — each variant is a product of 0..N fields:
//   - Type == nil            → a unit variant (no payload), e.g. `balanced`
//   - Type is a scalar/etc.  → a single-payload variant, e.g. `numeric: uint32`
//   - Type is a struct       → a multi-field variant (the struct holds the N args)
type Variant struct {
	Name string
	Type *FieldType
}

func (ft FieldType) String() string {
	switch ft.Base {
	case typekind.Optional:
		return fmt.Sprintf("%s<%s>", typekind.Optional, ft.Elem)
	case typekind.List:
		return fmt.Sprintf("%s<%s>", typekind.List, ft.Elem)
	case typekind.Array:
		return fmt.Sprintf("%s<%s,%d>", typekind.Array, ft.Elem, ft.ArrayN)
	case typekind.Map:
		return fmt.Sprintf("%s<%s,%s>", typekind.Map, ft.Key, ft.Elem)
	case typekind.Enum:
		// Self-describing, like list<T> and map<K,V>: the worker gets the variants
		// so it can derive an ordinal for its byte layout. The *value* still travels
		// as the variant name.
		return fmt.Sprintf("%s<%s>", typekind.Enum, strings.Join(ft.Values, ","))
	case typekind.Sum:
		parts := make([]string, len(ft.Variants))
		for i, v := range ft.Variants {
			if v.Type == nil {
				parts[i] = v.Name // unit variant
			} else {
				parts[i] = fmt.Sprintf("%s: %s", v.Name, v.Type.String())
			}
		}
		return fmt.Sprintf("%s<%s>", typekind.Sum, strings.Join(parts, ", "))
	default:
		return ft.Base
	}
}

// Field is a named, typed schema field with optional metadata tags.
type Field struct {
	Name string
	Type FieldType
	Tags map[string]string // arbitrary metadata forwarded to workers via bind
}

// CasesFile describes one data type: its schema plus the test cases for it.
// A single-file load yields one of these; a directory load yields several.
type CasesFile struct {
	Name              string   // type name (the worker bind "type"); the filename without .yaml
	Formats           []string // serialization formats to test this type with (yaml `formats:`)
	ReferenceLanguage string   // set by the caller (from --ref), not read from the file
	Schema            []Field
	Cases             []TestCase // yaml `cases:`
}

// CasesSet is a collection of types tested together in one run, sharing a
// reference language. Each Type is loaded from its own file (one type per file).
type CasesSet struct {
	ReferenceLanguage string
	Types             []*CasesFile
}

// TestIDFmt returns the id for a case within a (type, format) ("type/format/case").
func TestIDFmt(typeName, format, caseName string) string {
	return typeName + "/" + format + "/" + caseName
}

// TestIDs returns every namespaced test id across all types in the set. Ordered
// type → case → format so each case's formats sit adjacently (this drives the
// report's row grouping); authored case order and declared format order are kept.
func (s *CasesSet) TestIDs() []string {
	var ids []string
	for _, ty := range s.Types {
		for _, tc := range ty.Cases {
			for _, f := range ty.Formats {
				ids = append(ids, TestIDFmt(ty.Name, f, tc.Name))
			}
		}
	}
	return ids
}

type TestCase struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Data        map[string]any `yaml:"data"`
}

// WorkerManifest is worker.yaml (optional). If present it may override the
// default build/run commands for the auto-detected language. The language
// itself is always detected from marker files in the worker directory.
//
// Build is a pointer so that an explicit `build: ""` (this worker needs no build
// step) is distinguishable from an absent key (use the language default). A
// plain string cannot express the difference, and the absent case wins.
type WorkerManifest struct {
	Build *string `yaml:"build"`
	Run   string  `yaml:"run"`
}

// KnownFailure is one entry in known_failures/<lang>.yaml.
type KnownFailure struct {
	TestID string `yaml:"test_id"`
	Reason string `yaml:"reason"`
	Issue  string `yaml:"issue"`
}

type KnownFailuresFile struct {
	KnownFailures []KnownFailure `yaml:"known_failures"`
}

// LoadCases parses a single type file (e.g. cases/user.yaml). Named struct
// types referenced by its schema must be pulled in via `import:`.
func LoadCases(path string) (*CasesFile, error) {
	return loadTypeFile(path)
}

// loadTypeFile parses one type file: a `formats:` list, an `import:` list, a
// `fields:` (record) or `variants:` (sum) section, and a `cases:` list. The type
// name is the filename (without .yaml). Named types resolve against imports
// (no implicit registry).
func loadTypeFile(path string) (*CasesFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if err := checkKeys(path, raw); err != nil {
		return nil, err
	}
	var tmp struct {
		Formats     []string            `yaml:"formats"`
		Import      []string            `yaml:"import"`
		Fields      []map[string]string `yaml:"fields"`
		Variants    []map[string]string `yaml:"variants"`
		Cases       []rawTestCase       `yaml:"cases"`
		Transparent bool                `yaml:"transparent"`
	}
	if err := yaml.Unmarshal(raw, &tmp); err != nil {
		return nil, fmt.Errorf(errParseFmt, path, err)
	}

	name := strings.TrimSuffix(filepath.Base(path), extYAML)

	// Build the named-type registry from imports (transitively).
	registry := map[string]rawType{}
	seen := map[string]bool{}
	for _, imp := range tmp.Import {
		sub, err := loadImportable(filepath.Join(filepath.Dir(path), imp), seen)
		if err != nil {
			return nil, fmt.Errorf(errWrapFmt, name, err)
		}
		maps.Copy(registry, sub)
	}

	isSum := len(tmp.Variants) > 0
	entries := tmp.Fields
	if isSum {
		entries = tmp.Variants
	}
	rawFields, err := compactFields(entries)
	if err != nil {
		return nil, fmt.Errorf("%s schema: %w", name, err)
	}
	if err := checkSections(path, tmp.Fields, tmp.Variants, rawFields, tmp.Transparent); err != nil {
		return nil, err
	}
	// A sum under test is carried as the single field `value`; anything else is
	// an ordinary field list.
	resolver := newSchemaResolver(registry)
	var schema []Field
	if isSum {
		schema, err = resolver.sumSchema(rawFields)
	} else {
		schema, err = resolver.fields(rawFields)
	}
	if err != nil {
		return nil, fmt.Errorf("%s schema: %w", name, err)
	}

	// Decode each case's data schema-directed (see casedata.go): 64/128-bit
	// integer fields are parsed exactly from their literal text instead of
	// letting yaml.v3 guess a Go type and degrade big values to float64.
	cases := make([]TestCase, len(tmp.Cases))
	for i, rc := range tmp.Cases {
		data, err := decodeCaseData(schema, rc.Data)
		if err != nil {
			return nil, fmt.Errorf("%s case %q: %w", name, rc.Name, err)
		}
		cases[i] = TestCase{Name: rc.Name, Description: rc.Description, Data: data}
	}

	cf := &CasesFile{
		Name:    name,
		Formats: tmp.Formats,
		Schema:  schema,
		Cases:   cases,
	}
	if err := cf.validate(); err != nil {
		return nil, err
	}
	return cf, nil
}

// LoadSuite loads a set of types from a directory, one type per file. Every non
// _-prefixed *.yaml is one type (tested if it has cases, otherwise reusable via
// import). The reference language is supplied by the caller (the --ref flag),
// not stored in the directory, so ReferenceLanguage is left empty here.
func LoadSuite(dir string) (*CasesSet, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: cases path must be a directory of per-type files", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var types []*CasesFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, "_") || !strings.HasSuffix(name, extYAML) {
			continue
		}
		cf, err := loadTypeFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		if len(cf.Cases) == 0 {
			continue // a reusable type (no cases); only used when imported
		}
		types = append(types, cf)
	}
	if len(types) == 0 {
		return nil, fmt.Errorf(errWrapFmt, dir, ErrNoTypesFound)
	}
	slices.SortFunc(types, func(a, b *CasesFile) int { return cmp.Compare(a.Name, b.Name) })
	return &CasesSet{Types: types}, nil
}

// scalarAliases maps alternative spellings to their canonical name. Canonical
// names are the long form (uint8, int64, float32, …); these aliases are
// normalized at parse time so everything downstream (workers, the wire format,
// comparison) only ever sees the canonical form.
var scalarAliases = map[string]string{
	"float":   typekind.Float32,
	"double":  typekind.Float64,
	"boolean": typekind.Bool,
}

var typeParamRe = regexp.MustCompile(`^(\w+)<(.+)>$`)

// ParseType parses a type string like "uint64", "struct", "optional<string>", "array<uint32,4>", "list<struct>", "map<string,uint32>".
// Aliases (e.g. "float" for "float32") are normalized to their canonical form.
// It is the schemaResolver grammar with no named types in scope.
func ParseType(s string) (FieldType, error) {
	return newSchemaResolver(nil).typeOf(s)
}

// validate checks one type's schema and cases. reference_language is a
// suite-level concern (checked by the caller), not required per type file.
func (cf *CasesFile) validate() error {
	if len(cf.Schema) == 0 {
		return fmt.Errorf("type %q: schema must have at least one field", cf.Name)
	}
	// A tested type (one with cases) must declare its serialization formats
	// explicitly; there is no implicit default. Reusable-only types (no cases,
	// imported by others) are exempt.
	if len(cf.Cases) > 0 && len(cf.Formats) == 0 {
		return fmt.Errorf(
			"type %q: must declare at least one format (add a `formats:` list, e.g. `formats: [binary]`)",
			cf.Name,
		)
	}
	fieldNames := make(map[string]bool, len(cf.Schema))
	for _, f := range cf.Schema {
		fieldNames[f.Name] = true
	}
	fieldTypes := make(map[string]FieldType, len(cf.Schema))
	for _, f := range cf.Schema {
		fieldTypes[f.Name] = f.Type
	}
	for _, tc := range cf.Cases {
		if tc.Name == "" {
			return errors.New("test case missing name")
		}
		for k, v := range tc.Data {
			if !fieldNames[k] {
				return fmt.Errorf("test case %q: field %q not in schema", tc.Name, k)
			}
			if err := validateEnum(fieldTypes[k], v); err != nil {
				return fmt.Errorf("test case %q: field %q: %w", tc.Name, k, err)
			}
		}
	}
	return nil
}

// validateEnum rejects an enum value that is not one of the declared variants.
// Enums travel as plain strings, so without this a typo'd variant reaches the
// workers and is only caught (if at all) as a byte mismatch far downstream.
func validateEnum(ft FieldType, v any) error {
	if ft.Base != typekind.Enum {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("expected one of [%s], got %T", strings.Join(ft.Values, ", "), v)
	}
	if slices.Contains(ft.Values, s) {
		return nil
	}
	return fmt.Errorf("%q is not one of [%s]", s, strings.Join(ft.Values, ", "))
}

// LoadWorkerManifest reads worker.yaml from a directory. Returns nil (no error)
// if the file does not exist; returns an error only on parse failures.
func LoadWorkerManifest(dir string) (*WorkerManifest, error) {
	path := filepath.Join(dir, "worker.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // sentinel error not appropriate: builder.go caller checks err != nil
		}
		return nil, err
	}
	var m WorkerManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf(errParseFmt, path, err)
	}
	return &m, nil
}

// FormatsRegistryFile is the optional per-suite format registry. It is
// "_"-prefixed so LoadSuite and schema generation skip it as a case file.
const FormatsRegistryFile = "_formats.yaml"

type formatsRegistryFile struct {
	Formats []string `yaml:"formats"`
}

// loadOptionalYAML unmarshals <path> into a T. A missing file is not an error —
// it means the suite did not declare this at all — and is reported by the bool
// so a caller can still tell it apart from a file that exists but is empty.
func loadOptionalYAML[T any](path string) (T, bool, error) {
	var v T
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return v, false, nil
	}
	if err != nil {
		return v, false, err
	}
	if err := yaml.Unmarshal(raw, &v); err != nil {
		return v, false, fmt.Errorf(errParseFmt, path, err)
	}
	return v, true, nil
}

// LoadFormatsRegistry reads <dir>/_formats.yaml — the suite's declared format
// universe. Returns nil (no restriction) if the file does not exist.
func LoadFormatsRegistry(dir string) ([]string, error) {
	path := filepath.Join(dir, FormatsRegistryFile)
	fr, found, err := loadOptionalYAML[formatsRegistryFile](path)
	if err != nil || !found {
		return nil, err
	}
	if len(fr.Formats) == 0 {
		return nil, fmt.Errorf("%s: formats list is empty", path)
	}
	return fr.Formats, nil
}

// ExpectedSkips declares the coverage a worker is *allowed* to be missing.
// Without it a SKIP is invisible: a renamed type or a dropped registration
// silently turns green instead of failing, so lost coverage never surfaces.
//
//	types:      whole conformance types this SDK has no code for (both directions)
//	operations: per-direction type lists, e.g. deserialize: [a, b]
//
// Deliberately no wildcards: declaring a whole direction would re-hide exactly
// the regressions this guards against (a type that *does* have a decoder losing
// its registration would silently skip under a blanket "deserialize").
type ExpectedSkips struct {
	Types      []string            `yaml:"types"`
	Operations map[string][]string `yaml:"operations"`
}

// Covers reports whether a SKIP of typeName/op was declared.
func (e ExpectedSkips) Covers(typeName, op string) bool {
	return slices.Contains(e.Types, typeName) || slices.Contains(e.Operations[op], typeName)
}

// LoadExpectedSkips reads <dir>/<lang>.yaml. A missing file means "this worker
// is expected to cover everything", so any skip it reports is unexpected.
func LoadExpectedSkips(dir, lang string) (ExpectedSkips, error) {
	es, _, err := loadOptionalYAML[ExpectedSkips](filepath.Join(dir, lang+extYAML))
	return es, err
}

// LoadKnownFailures reads known_failures/<lang>.yaml, returns empty slice if not found.
func LoadKnownFailures(dir, lang string) ([]KnownFailure, error) {
	kf, _, err := loadOptionalYAML[KnownFailuresFile](filepath.Join(dir, lang+extYAML))
	if err != nil {
		return nil, err
	}
	return kf.KnownFailures, nil
}
