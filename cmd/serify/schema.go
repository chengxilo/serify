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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/typekind"
)

// Case-file section names, as they appear as YAML/JSON Schema keys.
const (
	keyFields   = "fields"
	keyVariants = "variants"
	keyFormats  = "formats"
	keyCases    = "cases"
)

// oneSection requires exactly one of `fields:` (a record) or `variants:` (a sum),
// which is how a type file declares which of the two it is.
func oneSection() []any {
	return []any{
		obj("required", []any{keyFields}, "not", obj("required", []any{keyVariants})),
		obj("required", []any{keyVariants}, "not", obj("required", []any{keyFields})),
	}
}

// defaultCasesDir is where `serify schema` looks when given no arguments.
const defaultCasesDir = "cases"

// sharedSchemaCount is the number of always-written schemas (defs + reusable),
// on top of one per type.
const sharedSchemaCount = 2

// J is a JSON Schema fragment builder (map[string]any with chainable helpers).
type J map[string]any

func (j J) set(k string, v any) { j[k] = v }

func obj(pairs ...any) J {
	m := J{}
	for i := 0; i+1 < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			return J{}
		}
		m[k] = pairs[i+1]
	}
	return m
}

var (
	String  = obj("type", "string")
	Boolean = obj("type", "boolean")
	Null    = obj("type", "null")
)

func ref(path string) J  { return obj("$ref", path) }
func optional(inner J) J { return obj("anyOf", []any{inner, Null}) }
func array(inner J) J    { return obj("type", "array", "items", inner) }
func mapOf(inner J) J    { return obj("type", "object", "additionalProperties", inner) }

func fixedArray(n int, inner J) J {
	return obj("type", "array", "minItems", n, "maxItems", n, "items", inner)
}

func object(props J, required []string) J {
	return obj("type", "object", "additionalProperties", false,
		"required", required, "properties", props)
}

// --- CLI ---

func newSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [cases-dir...]",
		Short: "Generate JSON Schema files from case-file field/variant sections",
		Long: `Generate .schema.json files from the fields:/variants: sections of case YAML
files, so editors (VS Code, JetBrains) can give autocomplete and validation.

Writes schemas to <cases>/.schemas/ and inserts a # yaml-language-server modeline
into each case file. Re-run after changing fields:/variants: sections.

An optional <cases>/_config.yaml (formats: [name, ...]) declares the suite's
format universe: every case file's formats: is then validated against it, in
the generated schemas and by this command. Without it, each file's enum is
generated from its own formats: list.

Accepts multiple directories. If none given, defaults to "cases".

Examples:
  serify schema                          # generate for ./cases
  serify schema examples/cases           # single directory
  serify schema cases1 cases2 cases3     # multiple directories`,
		Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			dirs := args
			if len(dirs) == 0 {
				dirs = []string{defaultCasesDir}
			}
			for _, d := range dirs {
				if err := runSchemaGen(d); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}

func runSchemaGen(casesDir string) error {
	set, err := config.LoadSuite(casesDir)
	if err != nil {
		if !errors.Is(err, config.ErrNoTypesFound) {
			return fmt.Errorf("load cases: %w", err)
		}
		set = &config.CasesSet{}
	}
	reusable, err := loadReusable(casesDir)
	if err != nil {
		return fmt.Errorf("load reusable: %w", err)
	}
	if len(set.Types) == 0 && len(reusable) == 0 {
		return fmt.Errorf("%s: no .yaml files found", casesDir)
	}
	allTypes := map[string][]config.Field{}
	for _, ty := range set.Types {
		allTypes[ty.Name] = ty.Schema
	}
	maps.Copy(allTypes, reusable)

	registry, err := config.LoadFormatsRegistry(casesDir)
	if err != nil {
		return err
	}
	if registry != nil {
		allowed := make(map[string]bool, len(registry))
		for _, f := range registry {
			allowed[f] = true
		}
		for _, ty := range set.Types {
			for _, f := range ty.Formats {
				if !allowed[f] {
					return fmt.Errorf("%s/%s.yaml: format %q is not declared in %s (has: %s)",
						casesDir, ty.Name, f, config.SuiteConfigFile, strings.Join(registry, ", "))
				}
			}
		}
	}

	schemasDir := filepath.Join(casesDir, ".schemas")
	_ = os.MkdirAll(schemasDir, 0750)

	if err := writeDefs(schemasDir, registry); err != nil {
		return err
	}

	for _, ty := range set.Types {
		if err := writeTypeSchema(
			casesDir,
			schemasDir,
			ty.Name,
			ty.Name+".schema.json",
			ty.Schema,
			ty.Formats,
			registry,
			allTypes,
		); err != nil {
			return err
		}
	}
	for name, fields := range reusable {
		if err := writeTypeSchema(casesDir, schemasDir, name, "reusable.schema.json", fields, nil, registry, allTypes); err != nil {
			return err
		}
	}

	// Always-updated shared schemas.
	reusableSchema := obj(
		"$schema", "http://json-schema.org/draft-07/schema#",
		"$comment", "AUTO-GENERATED by `serify schema`. Do not edit manually.",
		"title", "serify reusable type file",
		"description", "For types with no cases of their own (pulled in via import:).",
		"type", "object", "additionalProperties", false,
		// Keys starting with "_" are scratch space (YAML anchors etc.); the
		// loader ignores unknown keys, so the schema must not flag them.
		"patternProperties", obj("^_", obj()),
		"oneOf", oneSection(),
		"properties", obj(
			"import", array(obj("type", "string", "pattern", "\\.yaml$")),
			keyFields, ref("defs.schema.json#/definitions/fieldsSection"),
			keyVariants, ref("defs.schema.json#/definitions/variantsSection"),
			"transparent", obj("type", "boolean"),
		),
	)
	_ = writeJSON(filepath.Join(schemasDir, "reusable.schema.json"), reusableSchema)
	fmt.Println("  reusable     → .schemas/reusable.schema.json")
	fmt.Println("  defs         → .schemas/defs.schema.json")
	fmt.Printf("\nWrote %d schemas to %s\n",
		len(set.Types)+len(reusable)+sharedSchemaCount, schemasDir)
	return nil
}

func writeTypeSchema(
	casesDir, schemasDir, name, modelineFile string,
	fields []config.Field,
	formats []string,
	registry []string,
	allTypes map[string][]config.Field,
) error {
	// With a registry the formats enum is shared (defs.schema.json); without
	// one it is generated from the file's own declared formats.
	formatsJ := ref("defs.schema.json#/definitions/formatsSection")
	if registry == nil {
		formatsJ = formatsSchema(formats)
	}
	schema := fileSchema(allTypes, fields, formatsJ)
	if err := writeJSON(filepath.Join(schemasDir, name+".schema.json"), schema); err != nil {
		return err
	}
	_ = writeModeline(filepath.Join(casesDir, name+".yaml"), modelineFile)
	fmt.Printf("  %-12s → .schemas/%s.schema.json\n", name, name)
	return nil
}

// formatsSchema validates a formats: list against the given names — the suite
// registry (_config.yaml) when one exists, else the file's own declared
// formats. Format names are worker-defined, so nothing is hard-coded; with no
// names at all (a reusable type, no registry) any non-empty string is allowed.
func formatsSchema(formats []string) J {
	items := obj("type", "string", "minLength", 1)
	if len(formats) > 0 {
		vals := make([]any, len(formats))
		for i, f := range formats {
			vals[i] = f
		}
		items = obj("enum", vals)
	}
	return obj("type", "array", "minItems", 1, "items", items)
}

func fileSchema(allTypes map[string][]config.Field, fields []config.Field, formatsJ J) J {
	data := dataSchema(allTypes, fields)
	return obj(
		"$schema", "http://json-schema.org/draft-07/schema#",
		"$comment", "AUTO-GENERATED by `serify schema`. Do not edit manually.",
		"title", "serify case file",
		"type", "object", "additionalProperties", false,
		// Keys starting with "_" are scratch space (YAML anchors etc.); the
		// loader ignores unknown keys, so the schema must not flag them.
		"patternProperties", obj("^_", obj()),
		"required", []any{keyFormats, keyCases},
		"oneOf", oneSection(),
		"properties", obj(
			"import", array(obj("type", "string", "pattern", "\\.yaml$")),
			keyFields, ref("defs.schema.json#/definitions/fieldsSection"),
			keyVariants, ref("defs.schema.json#/definitions/variantsSection"),
			"transparent", obj("type", "boolean"),
			keyFormats, formatsJ,
			keyCases, array(obj(
				"type", "object", "additionalProperties", false,
				"required", []any{"name", "data"},
				"properties", obj("name", String, "description", String, "data", data),
			)),
		),
	)
}

func dataSchema(allTypes map[string][]config.Field, fields []config.Field) J {
	props := J{}
	var req []string
	for _, f := range fields {
		props[f.Name] = fieldToSchema(allTypes, f.Type)
		req = append(req, f.Name)
	}
	return object(props, req)
}

func fieldToSchema(allTypes map[string][]config.Field, ft config.FieldType) J {
	switch ft.Base {
	case typekind.Optional:
		return optional(fieldToSchema(allTypes, *ft.Elem))
	case typekind.List:
		return array(fieldToSchema(allTypes, *ft.Elem))
	case typekind.Array:
		return fixedArray(ft.ArrayN, fieldToSchema(allTypes, *ft.Elem))
	case typekind.Map:
		return mapOf(fieldToSchema(allTypes, *ft.Elem))
	case typekind.Struct:
		return dataSchema(allTypes, ft.Fields)
	case typekind.Enum:
		vals := make([]any, len(ft.Values))
		for i, v := range ft.Values {
			vals[i] = v
		}
		return obj("enum", vals)
	case typekind.Bytes:
		return ref("defs.schema.json#/definitions/bytes")
	default:
		return scalarSchema(ft.Base)
	}
}

// scalarSchema maps a serify scalar type name to its JSON Schema form.
// The mapping derives from typekind.Scalars — every integer scalar gets a $ref,
// floats get a number with description, string/bool get their canonical forms.
var scalarSchema = func() func(string) J {
	m := make(map[string]J, len(typekind.Scalars))
	for _, s := range typekind.Scalars {
		switch s {
		case typekind.Bytes:
			// handled before scalarSchema is called
		case typekind.Bool:
			m[s] = Boolean
		case typekind.String:
			m[s] = String
		case typekind.Float32, typekind.Float64:
			m[s] = obj("type", "number",
				"description", "YAML float; .nan/.inf/-.inf are accepted for binary-only types.")
		default:
			m[s] = ref("defs.schema.json#/definitions/" + s)
		}
	}
	return func(base string) J {
		if j, ok := m[base]; ok {
			return j
		}
		return String
	}
}()

func loadReusable(dir string) (map[string][]config.Field, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]config.Field{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".yaml") || strings.HasPrefix(n, "_") {
			continue
		}
		name := strings.TrimSuffix(n, ".yaml")
		cf, err := config.LoadCases(filepath.Join(dir, n))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		if len(cf.Cases) > 0 {
			continue
		}
		out[name] = cf.Schema
	}
	return out, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	// Skip write if unchanged — avoid editor reload churn.
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, b) {
		return nil
	}
	return os.WriteFile(path, b, 0600)
}

func writeModeline(yamlPath, schemaFile string) error {
	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}
	want := fmt.Sprintf("# yaml-language-server: $schema=.schemas/%s", schemaFile)
	content := string(raw)
	if strings.Contains(content, want) {
		return nil
	}
	// Strip existing modeline lines.
	var filtered []string
	for line := range strings.SplitSeq(content, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "# yaml-language-server:") {
			filtered = append(filtered, line)
		}
	}
	// Insert after the last leading #-comment line (license header).
	insertAt := 0
	for i, line := range filtered {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		insertAt = i + 1
	}
	out := slices.Insert(filtered, insertAt, want)
	// #nosec G703 -- yamlPath is the case file we just read above, under a cases
	// dir the developer named on their own command line. There is no privilege
	// boundary here to traverse across.
	return os.WriteFile(yamlPath, []byte(strings.Join(out, "\n")), 0600)
}

// writeDefs writes the shared scalar-bounds and helper definitions.
func writeDefs(dir string, registry []string) error {
	defs := J{}

	// With a _config.yaml registry, the format universe is suite-level and
	// shared: every case file's formats: refs this definition.
	if registry != nil {
		defs.set("formatsSection", formatsSchema(registry))
	}

	// Plain integer scalars.
	for _, s := range []struct {
		name     string
		min, max int64
	}{
		{"int8", -128, 127}, {"int16", -32768, 32767}, {"int32", -2147483648, 2147483647},
		{"uint8", 0, 255}, {"uint16", 0, 65535}, {"uint32", 0, 4294967295},
	} {
		defs.set(s.name, obj("type", "integer", "minimum", s.min, "maximum", s.max))
	}
	// 64/128-bit integers: the loader decodes these schema-directed from the
	// literal text (exact at any size), so both a bare integer literal and a
	// quoted decimal string are valid. Bounds apply to the integer form,
	// the pattern to the string form.
	defs.set("int64", J{
		"type":    []any{"integer", "string"},
		"minimum": json.Number("-9223372036854775808"),
		"maximum": json.Number("9223372036854775807"),
		"pattern": "^-?[0-9]{1,19}$",
	})
	defs.set("uint64", J{
		"type":    []any{"integer", "string"},
		"minimum": json.Number("0"),
		"maximum": json.Number("18446744073709551615"),
		"pattern": "^[0-9]{1,20}$",
	})
	defs.set("int128", J{
		"type":    []any{"integer", "string"},
		"minimum": json.Number("-170141183460469231731687303715884105728"),
		"maximum": json.Number("170141183460469231731687303715884105727"),
		"pattern": "^-?[0-9]{1,39}$",
	})
	defs.set("uint128", J{
		"type":    []any{"integer", "string"},
		"minimum": json.Number("0"),
		"maximum": json.Number("340282366920938463463374607431768211455"),
		"pattern": "^[0-9]{1,39}$",
	})

	defs.set("fieldsSection", obj(
		"description", "Record type: one `name: type` pair per field.",
		"type", "array", "minItems", 1,
		"items", obj("type", "object", "minProperties", 1, "maxProperties", 1,
			"additionalProperties", obj("type", "string")),
	))
	defs.set("variantsSection", obj(
		"description", "Sum type: one `tag: payload-type` pair per entry. "+
			"An entry with no type is a unit variant.",
		"type", "array", "minItems", 1,
		"items", obj("type", "object", "minProperties", 1, "maxProperties", 1,
			"additionalProperties", obj("type", []any{"string", "null"})),
	))
	defs.set("bytes", obj("anyOf", []any{
		obj("type", "string", "pattern", "^([0-9a-fA-F]{2})*$", "description", "hex string"),
		obj("type", "array", "items", ref("#/definitions/uint8"), "description", "byte array, e.g. [0xde, 0xad]"),
	}))

	schema := obj(
		"$schema", "http://json-schema.org/draft-07/schema#",
		"$comment", "AUTO-GENERATED by `serify schema`. Do not edit manually.",
		"title", "serify shared definitions",
		"description", "Scalar bounds, file-level helpers, and shared structs. Regenerate with `serify schema`.",
		"definitions", defs,
	)
	return writeJSON(filepath.Join(dir, "defs.schema.json"), schema)
}
