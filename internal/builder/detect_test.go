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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				require.NoError(t, os.WriteFile(path, nil, 0644))
			}

			lang, err := DetectLanguage(dir)
			require.NoError(t, err, "unexpected error: %v", err)
			assert.Equal(t, tt.wantLang, lang, "got %q, want %q", lang, tt.wantLang)
		})
	}
}

func TestDetectLanguage_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Cargo.toml"), nil, 0644))

	_, err := DetectLanguage(dir)
	require.Error(t, err, "expected error for ambiguous markers")
	assert.Contains(t, err.Error(), "ambiguous", "error should mention 'ambiguous', got: %s", err.Error())
}

func TestDetectLanguage_NoMarker(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectLanguage(dir)
	require.Error(t, err, "expected error for empty directory")
	assert.Contains(t, err.Error(), "cannot detect", "error should mention 'cannot detect', got: %s", err.Error())
}

func TestDetect_NoYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0644))

	info, err := Detect(dir)
	require.NoError(t, err, "unexpected error: %v", err)
	assert.Equal(t, "go", info.Language, "language = %q, want %q", info.Language, "go")
	if assert.NotNil(t, info.Manifest.Build, "build = %v, want default %q", info.Manifest.Build, Defaults["go"].Build) {
		assert.Equal(t, Defaults["go"].Build, *info.Manifest.Build, "build = %v, want default %q", info.Manifest.Build, Defaults["go"].Build)
	}
	assert.Equal(t, Defaults["go"].Run, info.Manifest.Run, "run = %q, want default %q", info.Manifest.Run, Defaults["go"].Run)
}

func TestDetect_YAMLOverride(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0644))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "worker.yaml"),
		[]byte("build: custom build\nrun: custom run\n"), 0644))

	info, err := Detect(dir)
	require.NoError(t, err, "unexpected error: %v", err)
	assert.Equal(t, "go", info.Language, "language = %q, want %q", info.Language, "go")
	if assert.NotNil(t, info.Manifest.Build, "build = %v, want %q", info.Manifest.Build, "custom build") {
		assert.Equal(t, "custom build", *info.Manifest.Build, "build = %v, want %q", info.Manifest.Build, "custom build")
	}
	assert.Equal(t, "custom run", info.Manifest.Run, "run = %q, want %q", info.Manifest.Run, "custom run")
}

func TestDetect_PartialYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Cargo.toml"), nil, 0644))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "worker.yaml"),
		[]byte("build: custom cargo build\n"), 0644))

	info, err := Detect(dir)
	require.NoError(t, err, "unexpected error: %v", err)
	assert.Equal(t, "rust", info.Language, "language = %q, want %q", info.Language, "rust")
	if assert.NotNil(t, info.Manifest.Build, "build = %v, want %q", info.Manifest.Build, "custom cargo build") {
		assert.Equal(t, "custom cargo build", *info.Manifest.Build, "build = %v, want %q", info.Manifest.Build, "custom cargo build")
	}
	assert.Equal(t, Defaults["rust"].Run, info.Manifest.Run, "run = %q, want default %q", info.Manifest.Run, Defaults["rust"].Run)
}
