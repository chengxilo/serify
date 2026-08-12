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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaGen_FileLevelStructure(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "person.yaml"), `
fields:
  - id: uint64
  - name: string
formats:
  - name: binary
    oracle: bytes
cases:
  - name: alice
    data:
      id: 1
      name: "Alice"
`)

	require.NoError(t, runSchemaGen(dir), "runSchemaGen")

	// Verify .schemas/ was created.
	sd := filepath.Join(dir, ".schemas")
	require.DirExists(t, sd, ".schemas/ not created")

	// Verify defs.schema.json exists and has required definitions.
	defsB, err := os.ReadFile(filepath.Join(sd, "defs.schema.json"))
	require.NoError(t, err, "defs.schema.json")
	var defs map[string]any
	require.NoError(t, json.Unmarshal(defsB, &defs), "unmarshal defs.schema.json")
	d := defs["definitions"].(map[string]any)
	for _, name := range []string{"uint8", "uint64", "int64", "uint128", "int128", "bytes", "fieldsSection"} {
		assert.Contains(t, d, name, "missing definition %q", name)
	}
	// formats is per-file (enum of the file's own declared formats), so it
	// must not be a shared definition.
	assert.NotContains(t, d, "formatsSection", "formatsSection must not be in defs: formats are per-file, generated from the user's formats list")

	// Verify person.schema.json is a file-level schema.
	personB, err := os.ReadFile(filepath.Join(sd, "person.schema.json"))
	require.NoError(t, err, "person.schema.json")
	var person map[string]any
	require.NoError(t, json.Unmarshal(personB, &person), "unmarshal person.schema.json")

	assert.Equal(t, "object", person["type"], "top-level schema must be object")
	props := person["properties"].(map[string]any)
	for _, k := range []string{"import", "fields", "formats", "cases"} {
		assert.Contains(t, props, k, "top-level properties missing %q", k)
	}

	// A formats entry is a {name, oracle} mapping, both required — the shape
	// the loader demands. The name enum is generated from the file's own
	// formats: list. This used to assert a bare string enum, which is the
	// pre-oracle syntax: the generated schema then rejected every real case
	// file's formats: block while the loader accepted it.
	fitems := props["formats"].(map[string]any)["items"].(map[string]any)
	assert.Equal(t, "object", fitems["type"], "a formats entry must be a mapping, got %v", fitems)
	assert.ElementsMatch(t, []any{"name", "oracle"}, fitems["required"],
		"both name and oracle must be required, got %v", fitems["required"])
	fprops := fitems["properties"].(map[string]any)
	assert.Equal(t, []any{"binary"}, fprops["name"].(map[string]any)["enum"],
		"formats name enum = %v, want [binary] from the declared formats", fprops["name"])
	assert.Equal(t, []any{"bytes", "semantic"}, fprops["oracle"].(map[string]any)["enum"],
		"oracle enum = %v, want the two comparison oracles", fprops["oracle"])
	req := person["required"].([]any)
	foundFormats, foundCases := false, false
	for _, r := range req {
		switch r.(string) {
		case "formats":
			foundFormats = true
		case "cases":
			foundCases = true
		}
	}
	assert.True(t, foundFormats, "top-level required missing formats")
	assert.True(t, foundCases, "top-level required missing cases")
	// schema/variants are required as a pair of alternatives, not outright: a
	// type file declares exactly one of the two.
	assert.Len(t, person["oneOf"], 2, "expected a two-branch oneOf for schema/variants, got %v", person["oneOf"])

	// Verify the data schema inside cases has the correct fields.
	casesProp := props["cases"].(map[string]any)
	casesItems := casesProp["items"].(map[string]any)
	casesProps := casesItems["properties"].(map[string]any)
	for _, k := range []string{"name", "description", "data"} {
		assert.Contains(t, casesProps, k, "case item missing %q", k)
	}
	dataSchema := casesProps["data"].(map[string]any)
	dataProps := dataSchema["properties"].(map[string]any)
	assert.Contains(t, dataProps, "id", "data missing 'id' field")
	assert.Contains(t, dataProps, "name", "data missing 'name' field")

	// Verify modeline in YAML.
	yamlB, err := os.ReadFile(filepath.Join(dir, "person.yaml"))
	require.NoError(t, err, "read person.yaml")
	assert.Contains(t, string(yamlB), "# yaml-language-server: $schema=.schemas/person.schema.json", "missing modeline for person.yaml")
}

func TestSchemaGen_FormatsRegistry(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "_config.yaml"), `
formats:
  - binary
  - json
`)
	mustWrite(t, filepath.Join(dir, "person.yaml"), `
fields:
  - id: uint64
formats:
  - name: binary
    oracle: bytes
cases:
  - name: alice
    data:
      id: 1
`)

	require.NoError(t, runSchemaGen(dir), "runSchemaGen")

	// The registry becomes the shared formatsSection enum in defs.
	defsB, err := os.ReadFile(filepath.Join(dir, ".schemas", "defs.schema.json"))
	require.NoError(t, err, "defs.schema.json")
	var defs map[string]any
	require.NoError(t, json.Unmarshal(defsB, &defs), "unmarshal defs.schema.json")
	fs, ok := defs["definitions"].(map[string]any)["formatsSection"].(map[string]any)
	require.True(t, ok, "defs missing formatsSection despite _config.yaml registry")
	enum := fs["items"].(map[string]any)["properties"].(map[string]any)["name"].(map[string]any)["enum"].([]any)
	assert.Equal(t, []any{"binary", "json"}, enum, "registry enum = %v, want [binary json]", enum)

	// The per-file schema refs the shared definition instead of self-enumerating.
	personB, err := os.ReadFile(filepath.Join(dir, ".schemas", "person.schema.json"))
	require.NoError(t, err, "person.schema.json")
	var person map[string]any
	require.NoError(t, json.Unmarshal(personB, &person), "unmarshal person.schema.json")
	f := person["properties"].(map[string]any)["formats"].(map[string]any)
	assert.Equal(t, "defs.schema.json#/definitions/formatsSection", f["$ref"], "formats = %v, want $ref to defs formatsSection", f)
}

func TestSchemaGen_FormatsRegistryViolation(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "_config.yaml"), `
formats:
  - binary
`)
	mustWrite(t, filepath.Join(dir, "person.yaml"), `
fields:
  - id: uint64
formats:
  - name: xml
    oracle: bytes
cases:
  - name: alice
    data:
      id: 1
`)

	err := runSchemaGen(dir)
	require.Error(t, err, "expected error for format not in _config.yaml")
	assert.Contains(t, err.Error(), `"xml"`, "error should name the format and the registry, got: %v", err)
	assert.Contains(t, err.Error(), "_config.yaml", "error should name the format and the registry, got: %v", err)
}

func TestSchemaGen_ReusableType(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "addr.yaml"), `
fields:
  - street: string
  - zip: uint32
`)

	require.NoError(t, runSchemaGen(dir), "runSchemaGen")

	// Reusable type must point to reusable.schema.json.
	yamlB, err := os.ReadFile(filepath.Join(dir, "addr.yaml"))
	require.NoError(t, err, "read addr.yaml")
	assert.Contains(t, string(yamlB), "# yaml-language-server: $schema=.schemas/reusable.schema.json", "reusable type must use reusable.schema.json")

	// reusable.schema.json must exist and NOT require formats/cases.
	rb, err := os.ReadFile(filepath.Join(dir, ".schemas", "reusable.schema.json"))
	require.NoError(t, err, "reusable.schema.json")
	var reusable map[string]any
	require.NoError(t, json.Unmarshal(rb, &reusable), "unmarshal reusable.schema.json")
	if req, ok := reusable["required"].([]any); ok {
		assert.NotContains(t, req, "formats", "reusable schema must not require formats")
		assert.NotContains(t, req, "cases", "reusable schema must not require cases")
	}
	assert.Len(t, reusable["oneOf"], 2, "expected a two-branch oneOf for schema/variants, got %v", reusable["oneOf"])
}

func TestSchemaGen_ModelineUpdate(t *testing.T) {
	dir := t.TempDir()

	// First run: write modeline pointing to old schema.
	mustWrite(t, filepath.Join(dir, "rec.yaml"), `
# yaml-language-server: $schema=.schemas/old.schema.json
fields:
  - id: uint64
formats:
  - name: binary
    oracle: bytes
cases:
  - name: one
    data:
      id: 1
`)
	require.NoError(t, runSchemaGen(dir), "first run")

	yamlB, _ := os.ReadFile(filepath.Join(dir, "rec.yaml"))
	assert.NotContains(t, string(yamlB), "old.schema.json", "stale modeline must be replaced")
	assert.Contains(t, string(yamlB), "rec.schema.json", "current modeline must be present")

	// Second run with correct modeline already present — must be idempotent.
	require.NoError(t, runSchemaGen(dir), "second run")
	yamlB2, _ := os.ReadFile(filepath.Join(dir, "rec.yaml"))
	assert.Equal(t, 1, strings.Count(string(yamlB2), "# yaml-language-server:"), "modeline must not be duplicated")
}

func TestSchemaGen_AllTypeForms(t *testing.T) {
	dir := t.TempDir()

	// One tested type exercising every serify type form.
	mustWrite(t, filepath.Join(dir, "all.yaml"), `
formats:
  - name: binary
    oracle: bytes
fields:
  - u8: uint8
  - u16: uint16
  - u32: uint32
  - u64: uint64
  - i8: int8
  - i16: int16
  - i32: int32
  - i64: int64
  - u128: uint128
  - i128: int128
  - f32: float32
  - f64: float64
  - b: bool
  - s: string
  - raw: bytes
  - opt: optional<string>
  - lst: list<uint32>
  - arr: array<uint8,4>
  - mp: map<string,uint32>
cases:
  - name: basic
    data:
      u8: 42
      u16: 1024
      u32: 100000
      u64: 1
      i8: -10
      i16: -500
      i32: -100000
      i64: -1
      u128: "0"
      i128: "0"
      f32: 1.0
      f64: 2.0
      b: true
      s: "hello"
      raw: "deadbeef"
      opt: null
      lst: [1, 2]
      arr: [1, 2, 3, 4]
      mp: {}
`)
	require.NoError(t, runSchemaGen(dir), "runSchemaGen")

	schemaB, err := os.ReadFile(filepath.Join(dir, ".schemas", "all.schema.json"))
	require.NoError(t, err, "all.schema.json")
	var schema map[string]any
	require.NoError(t, json.Unmarshal(schemaB, &schema), "unmarshal all.schema.json")

	// Drill into the data schema props.
	c := schema["properties"].(map[string]any)["cases"].(map[string]any)
	i := c["items"].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)
	props := i["properties"].(map[string]any)

	checks := map[string]struct{ hasRef, hasAnyOf bool }{
		"u8":   {hasRef: true},
		"u16":  {hasRef: true},
		"u128": {hasRef: true},
		"i64":  {hasRef: true},
		"f32":  {},
		"b":    {},
		"s":    {},
		"raw":  {hasRef: true},
		"opt":  {hasAnyOf: true},
		"lst":  {},
		"arr":  {},
		"mp":   {},
	}
	for field, expect := range checks {
		if !assert.Contains(t, props, field, "field %q missing from data schema", field) {
			continue
		}
		f := props[field]
		fm := f.(map[string]any)
		if expect.hasRef {
			assert.NotNil(t, fm["$ref"], "field %q: expected $ref, got %v", field, fm)
		}
		if expect.hasAnyOf {
			assert.NotNil(t, fm["anyOf"], "field %q: expected anyOf, got %v", field, fm)
		}
	}

	// arr must have minItems/maxItems.
	arrF := props["arr"].(map[string]any)
	assert.Equal(t, float64(4), arrF["minItems"], "array: expected minItems=4, maxItems=4, got %v", arrF)
	assert.Equal(t, float64(4), arrF["maxItems"], "array: expected minItems=4, maxItems=4, got %v", arrF)
}

// A sum field has two spellings in case data and the loader distinguishes them:
// a unit variant is written bare, a payload variant as a single-key mapping.
// Neither was expressible before — a sum fell through to the scalar map, which
// does not know the base and answers String, so every {tag: payload} in a case
// file failed against its own generated schema while loading fine. The point of
// generating these is save-time validation, and it was inverted for sums.
func TestSchemaGen_SumField(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "channel.yaml"), `
variants:
  - silent:
  - sms: string
  - push: uint64
`)
	mustWrite(t, filepath.Join(dir, "note.yaml"), `
import:
  - channel.yaml
fields:
  - channel: channel
formats:
  - name: binary
    oracle: bytes
cases:
  - name: quiet
    data:
      channel: silent
`)

	require.NoError(t, runSchemaGen(dir), "runSchemaGen")

	noteB, err := os.ReadFile(filepath.Join(dir, ".schemas", "note.schema.json"))
	require.NoError(t, err, "note.schema.json")
	var note map[string]any
	require.NoError(t, json.Unmarshal(noteB, &note), "unmarshal note.schema.json")

	data := note["properties"].(map[string]any)["cases"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)
	ch := data["properties"].(map[string]any)["channel"].(map[string]any)

	branches, ok := ch["oneOf"].([]any)
	require.True(t, ok, "a sum with unit variants needs both spellings, got %v", ch)
	require.Len(t, branches, 2, "expected the bare-tag and single-key-mapping branches, got %v", branches)

	// Only the unit variant may be written bare: `channel: sms` is a load error
	// ("variant %q needs a payload"), so it must not validate either.
	bare := branches[0].(map[string]any)
	assert.Equal(t, []any{"silent"}, bare["enum"],
		"the bare form must list unit variants only, got %v", bare["enum"])

	mapping := branches[1].(map[string]any)
	assert.Equal(t, float64(1), mapping["maxProperties"],
		"a sum value names exactly one variant, got %v", mapping)
	assert.Equal(t, false, mapping["additionalProperties"],
		"an unknown tag is a load error and must not validate, got %v", mapping)

	tags := mapping["properties"].(map[string]any)
	for _, tag := range []string{"silent", "sms", "push"} {
		assert.Contains(t, tags, tag, "mapping form missing variant %q", tag)
	}
	assert.Equal(t, "null", tags["silent"].(map[string]any)["type"],
		"a unit variant's payload in the mapping form is null, got %v", tags["silent"])
	assert.Equal(t, "string", tags["sms"].(map[string]any)["type"],
		"sms carries a string payload, got %v", tags["sms"])
}

func TestSchemaGen_RejectsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	err := runSchemaGen(dir)
	assert.Error(t, err, "expected error for empty directory")
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0644))
}
