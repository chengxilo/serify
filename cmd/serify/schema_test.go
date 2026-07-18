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
)

func TestSchemaGen_FileLevelStructure(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "person.yaml"), `
fields:
  - id: uint64
  - name: string
formats:
  - binary
cases:
  - name: alice
    data:
      id: 1
      name: "Alice"
`)

	if err := runSchemaGen(dir); err != nil {
		t.Fatalf("runSchemaGen: %v", err)
	}

	// Verify .schemas/ was created.
	sd := filepath.Join(dir, ".schemas")
	if _, err := os.Stat(sd); os.IsNotExist(err) {
		t.Fatalf(".schemas/ not created")
	}

	// Verify defs.schema.json exists and has required definitions.
	defsB, err := os.ReadFile(filepath.Join(sd, "defs.schema.json"))
	if err != nil {
		t.Fatalf("defs.schema.json: %v", err)
	}
	var defs map[string]any
	if err := json.Unmarshal(defsB, &defs); err != nil {
		t.Fatalf("unmarshal defs.schema.json: %v", err)
	}
	d := defs["definitions"].(map[string]any)
	for _, name := range []string{"uint8", "uint64", "int64", "uint128", "int128", "bytes", "fieldsSection"} {
		if _, ok := d[name]; !ok {
			t.Errorf("missing definition %q", name)
		}
	}
	// formats is per-file (enum of the file's own declared formats), so it
	// must not be a shared definition.
	if _, ok := d["formatsSection"]; ok {
		t.Error("formatsSection must not be in defs: formats are per-file, generated from the user's formats list")
	}

	// Verify person.schema.json is a file-level schema.
	personB, err := os.ReadFile(filepath.Join(sd, "person.schema.json"))
	if err != nil {
		t.Fatalf("person.schema.json: %v", err)
	}
	var person map[string]any
	if err := json.Unmarshal(personB, &person); err != nil {
		t.Fatalf("unmarshal person.schema.json: %v", err)
	}

	if person["type"] != "object" {
		t.Error("top-level schema must be object")
	}
	props := person["properties"].(map[string]any)
	for _, k := range []string{"import", "fields", "formats", "cases"} {
		if _, ok := props[k]; !ok {
			t.Errorf("top-level properties missing %q", k)
		}
	}

	// The formats enum is generated from the file's own formats: list.
	fitems := props["formats"].(map[string]any)["items"].(map[string]any)
	if enum, ok := fitems["enum"].([]any); !ok || len(enum) != 1 || enum[0] != "binary" {
		t.Errorf("formats items = %v, want enum [binary] from the declared formats", fitems)
	}
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
	if !foundFormats || !foundCases {
		t.Error("top-level required missing formats/cases")
	}
	// schema/variants are required as a pair of alternatives, not outright: a
	// type file declares exactly one of the two.
	if oneOf, ok := person["oneOf"].([]any); !ok || len(oneOf) != 2 {
		t.Errorf("expected a two-branch oneOf for schema/variants, got %v", person["oneOf"])
	}

	// Verify the data schema inside cases has the correct fields.
	casesProp := props["cases"].(map[string]any)
	casesItems := casesProp["items"].(map[string]any)
	casesProps := casesItems["properties"].(map[string]any)
	for _, k := range []string{"name", "description", "data"} {
		if _, ok := casesProps[k]; !ok {
			t.Errorf("case item missing %q", k)
		}
	}
	dataSchema := casesProps["data"].(map[string]any)
	dataProps := dataSchema["properties"].(map[string]any)
	if _, ok := dataProps["id"]; !ok {
		t.Error("data missing 'id' field")
	}
	if _, ok := dataProps["name"]; !ok {
		t.Error("data missing 'name' field")
	}

	// Verify modeline in YAML.
	yamlB, err := os.ReadFile(filepath.Join(dir, "person.yaml"))
	if err != nil {
		t.Fatalf("read person.yaml: %v", err)
	}
	if !strings.Contains(string(yamlB), "# yaml-language-server: $schema=.schemas/person.schema.json") {
		t.Error("missing modeline for person.yaml")
	}
}

func TestSchemaGen_FormatsRegistry(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "_formats.yaml"), `
formats:
  - binary
  - json
`)
	mustWrite(t, filepath.Join(dir, "person.yaml"), `
fields:
  - id: uint64
formats:
  - binary
cases:
  - name: alice
    data:
      id: 1
`)

	if err := runSchemaGen(dir); err != nil {
		t.Fatalf("runSchemaGen: %v", err)
	}

	// The registry becomes the shared formatsSection enum in defs.
	defsB, err := os.ReadFile(filepath.Join(dir, ".schemas", "defs.schema.json"))
	if err != nil {
		t.Fatalf("defs.schema.json: %v", err)
	}
	var defs map[string]any
	if err := json.Unmarshal(defsB, &defs); err != nil {
		t.Fatalf("unmarshal defs.schema.json: %v", err)
	}
	fs, ok := defs["definitions"].(map[string]any)["formatsSection"].(map[string]any)
	if !ok {
		t.Fatal("defs missing formatsSection despite _formats.yaml registry")
	}
	enum := fs["items"].(map[string]any)["enum"].([]any)
	if len(enum) != 2 || enum[0] != "binary" || enum[1] != "json" {
		t.Errorf("registry enum = %v, want [binary json]", enum)
	}

	// The per-file schema refs the shared definition instead of self-enumerating.
	personB, err := os.ReadFile(filepath.Join(dir, ".schemas", "person.schema.json"))
	if err != nil {
		t.Fatalf("person.schema.json: %v", err)
	}
	var person map[string]any
	if err := json.Unmarshal(personB, &person); err != nil {
		t.Fatalf("unmarshal person.schema.json: %v", err)
	}
	f := person["properties"].(map[string]any)["formats"].(map[string]any)
	if f["$ref"] != "defs.schema.json#/definitions/formatsSection" {
		t.Errorf("formats = %v, want $ref to defs formatsSection", f)
	}
}

func TestSchemaGen_FormatsRegistryViolation(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "_formats.yaml"), `
formats:
  - binary
`)
	mustWrite(t, filepath.Join(dir, "person.yaml"), `
fields:
  - id: uint64
formats:
  - xml
cases:
  - name: alice
    data:
      id: 1
`)

	err := runSchemaGen(dir)
	if err == nil {
		t.Fatal("expected error for format not in _formats.yaml")
	}
	if !strings.Contains(err.Error(), `"xml"`) || !strings.Contains(err.Error(), "_formats.yaml") {
		t.Errorf("error should name the format and the registry, got: %v", err)
	}
}

func TestSchemaGen_ReusableType(t *testing.T) {
	dir := t.TempDir()

	mustWrite(t, filepath.Join(dir, "addr.yaml"), `
fields:
  - street: string
  - zip: uint32
`)

	if err := runSchemaGen(dir); err != nil {
		t.Fatalf("runSchemaGen: %v", err)
	}

	// Reusable type must point to reusable.schema.json.
	yamlB, err := os.ReadFile(filepath.Join(dir, "addr.yaml"))
	if err != nil {
		t.Fatalf("read addr.yaml: %v", err)
	}
	if !strings.Contains(string(yamlB), "# yaml-language-server: $schema=.schemas/reusable.schema.json") {
		t.Error("reusable type must use reusable.schema.json")
	}

	// reusable.schema.json must exist and NOT require formats/cases.
	rb, err := os.ReadFile(filepath.Join(dir, ".schemas", "reusable.schema.json"))
	if err != nil {
		t.Fatalf("reusable.schema.json: %v", err)
	}
	var reusable map[string]any
	if err := json.Unmarshal(rb, &reusable); err != nil {
		t.Fatalf("unmarshal reusable.schema.json: %v", err)
	}
	if req, ok := reusable["required"].([]any); ok {
		for _, r := range req {
			if r.(string) == "formats" || r.(string) == "cases" {
				t.Error("reusable schema must not require formats or cases")
			}
		}
	}
	if oneOf, ok := reusable["oneOf"].([]any); !ok || len(oneOf) != 2 {
		t.Errorf("expected a two-branch oneOf for schema/variants, got %v", reusable["oneOf"])
	}
}

func TestSchemaGen_ModelineUpdate(t *testing.T) {
	dir := t.TempDir()

	// First run: write modeline pointing to old schema.
	mustWrite(t, filepath.Join(dir, "rec.yaml"), `
# yaml-language-server: $schema=.schemas/old.schema.json
fields:
  - id: uint64
formats:
  - binary
cases:
  - name: one
    data:
      id: 1
`)
	if err := runSchemaGen(dir); err != nil {
		t.Fatalf("first run: %v", err)
	}

	yamlB, _ := os.ReadFile(filepath.Join(dir, "rec.yaml"))
	if strings.Contains(string(yamlB), "old.schema.json") {
		t.Error("stale modeline must be replaced")
	}
	if !strings.Contains(string(yamlB), "rec.schema.json") {
		t.Error("current modeline must be present")
	}

	// Second run with correct modeline already present — must be idempotent.
	if err := runSchemaGen(dir); err != nil {
		t.Fatalf("second run: %v", err)
	}
	yamlB2, _ := os.ReadFile(filepath.Join(dir, "rec.yaml"))
	if strings.Count(string(yamlB2), "# yaml-language-server:") != 1 {
		t.Error("modeline must not be duplicated")
	}
}

func TestSchemaGen_AllTypeForms(t *testing.T) {
	dir := t.TempDir()

	// One tested type exercising every serify type form.
	mustWrite(t, filepath.Join(dir, "all.yaml"), `
formats:
  - binary
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
	if err := runSchemaGen(dir); err != nil {
		t.Fatalf("runSchemaGen: %v", err)
	}

	schemaB, err := os.ReadFile(filepath.Join(dir, ".schemas", "all.schema.json"))
	if err != nil {
		t.Fatalf("all.schema.json: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaB, &schema); err != nil {
		t.Fatalf("unmarshal all.schema.json: %v", err)
	}

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
		f, ok := props[field]
		if !ok {
			t.Errorf("field %q missing from data schema", field)
			continue
		}
		fm := f.(map[string]any)
		if expect.hasRef && fm["$ref"] == nil {
			t.Errorf("field %q: expected $ref, got %v", field, fm)
		}
		if expect.hasAnyOf && fm["anyOf"] == nil {
			t.Errorf("field %q: expected anyOf, got %v", field, fm)
		}
	}

	// arr must have minItems/maxItems.
	arrF := props["arr"].(map[string]any)
	if arrF["minItems"] != 4.0 || arrF["maxItems"] != 4.0 {
		t.Errorf("array: expected minItems=4, maxItems=4, got %v", arrF)
	}
}

func TestSchemaGen_RejectsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	err := runSchemaGen(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
}
