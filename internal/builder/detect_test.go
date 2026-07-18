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

package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		wantLang string
	}{
		{"*.go", []string{"main.go"}, "go"},
		{"go", []string{"go.mod"}, "go"},
		{"rust", []string{"Cargo.toml"}, "rust"},
		{"java", []string{"pom.xml"}, "java"},
		{"elixir", []string{"mix.exs"}, "elixir"},
		{"csharp", []string{"Worker.csproj"}, "csharp"},
		{"node", []string{"package.json"}, "node"},
		{"python", []string{"worker.py"}, "python"},
		{"python_any", []string{"app.py"}, "python"},
		{"*.cpp", []string{"worker.cpp"}, "cpp"},
		{"cpp_any", []string{"main.cpp"}, "cpp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				path := filepath.Join(dir, f)
				if err := os.WriteFile(path, nil, 0644); err != nil {
					t.Fatal(err)
				}
			}

			lang, err := DetectLanguage(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lang != tt.wantLang {
				t.Errorf("got %q, want %q", lang, tt.wantLang)
			}
		})
	}
}

func TestDetectLanguage_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := DetectLanguage(dir)
	if err == nil {
		t.Fatal("expected error for ambiguous markers")
	}
	if got := err.Error(); !strings.Contains(got, "ambiguous") {
		t.Errorf("error should mention 'ambiguous', got: %s", got)
	}
}

func TestDetectLanguage_NoMarker(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectLanguage(dir)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
	if got := err.Error(); !strings.Contains(got, "cannot detect") {
		t.Errorf("error should mention 'cannot detect', got: %s", got)
	}
}

func TestDetect_NoYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Language != "go" {
		t.Errorf("language = %q, want %q", info.Language, "go")
	}
	if info.Manifest.Build == nil || *info.Manifest.Build != Defaults["go"].Build {
		t.Errorf("build = %v, want default %q", info.Manifest.Build, Defaults["go"].Build)
	}
	if info.Manifest.Run != Defaults["go"].Run {
		t.Errorf("run = %q, want default %q", info.Manifest.Run, Defaults["go"].Run)
	}
}

func TestDetect_YAMLOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "worker.yaml"),
		[]byte("build: custom build\nrun: custom run\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Language != "go" {
		t.Errorf("language = %q, want %q", info.Language, "go")
	}
	if info.Manifest.Build == nil || *info.Manifest.Build != "custom build" {
		t.Errorf("build = %v, want %q", info.Manifest.Build, "custom build")
	}
	if info.Manifest.Run != "custom run" {
		t.Errorf("run = %q, want %q", info.Manifest.Run, "custom run")
	}
}

func TestDetect_PartialYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "worker.yaml"),
		[]byte("build: custom cargo build\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := Detect(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Language != "rust" {
		t.Errorf("language = %q, want %q", info.Language, "rust")
	}
	if info.Manifest.Build == nil || *info.Manifest.Build != "custom cargo build" {
		t.Errorf("build = %v, want %q", info.Manifest.Build, "custom cargo build")
	}
	if info.Manifest.Run != Defaults["rust"].Run {
		t.Errorf("run = %q, want default %q", info.Manifest.Run, Defaults["rust"].Run)
	}
}
