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

package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// TestCLI_Run_FullMatrix exercises --full-matrix with CSV export.
func TestCLI_Run_FullMatrix(t *testing.T) {
	requireWorkers(t, happy.langs...)

	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, happy.runArgs("go", happy.CasePath(), "--full-matrix", "--csv", csv)...)
	require.Equal(t, 0, code, "exit code = %d, want 0\n%s", code, out)

	grid := readResultGrid(t, csv)

	// All four lang pairs on a known id.
	id := "all_types/json/basic"
	pairs := []string{"go", "rust"}
	for _, src := range pairs {
		for _, dst := range pairs {
			op := report.OpMatrix
			rec, ok := grid[id][src+"→"+dst][op]
			if !assert.True(t, ok, "[%s / %s→%s / %s] no row in CSV", id, src, dst, op) {
				continue
			}
			assert.Equal(t, string(report.StatusPass), rec.Status, "[%s / %s→%s] status = %s, want PASS", id, src, dst, rec.Status)
		}
	}

	// Serialize + deserialize rows still PASS.
	assertCell(t, grid, id, "go", report.OpSerialize, report.StatusPass)
	assertCell(t, grid, id, "rust", report.OpSerialize, report.StatusPass)
}

// TestCLI_Run_JSONOutput verifies --output json produces parseable JSON records.
func TestCLI_Run_JSONOutput(t *testing.T) {
	requireWorkers(t, happy.langs...)

	out, code := testutil.RunSerify(t, happy.runArgs("go", happy.CasePath(), "--output", "json")...)
	require.Equal(t, 0, code, "exit code = %d, want 0\n%s", code, out)

	// Slice from the first '[' — nothing prints after the JSON array when --csv is absent.
	idx := strings.Index(out, "[")
	require.GreaterOrEqual(t, idx, 0, "no JSON array found in output:\n%s", out)
	jsonStr := out[idx:]

	var records []report.Record
	err := json.Unmarshal([]byte(jsonStr), &records)
	require.NoError(t, err, "json.Unmarshal: %v\n%s", err, jsonStr)

	found := false
	for _, r := range records {
		if r.TestID == "all_types/json/basic" && r.Language == "go" && r.Operation == "serialize" &&
			r.Status == "PASS" {
			found = true
			break
		}
	}
	assert.True(t, found, "all_types/json/basic go serialize PASS not found in %d records", len(records))
}

// TestCLI_Run_BuildOnly exercises --build-only (first coverage of the real build path).
func TestCLI_Run_BuildOnly(t *testing.T) {
	requireWorkers(t, happy.langs...)

	// Use the happy go worker dir without --no-build.
	goDir := happy.WorkerPaths()[0]
	out, code := testutil.RunSerify(t, "run", "--ref", "go", "--cases", happy.CasePath(), "--build-only", goDir)
	require.Equal(t, 0, code, "exit code = %d, want 0\n%s", code, out)
	assert.Contains(t, out, "Build complete (--build-only).", "build-only message")
	assert.NotContains(t, out, "Suite:", "should not run tests")
}

// TestCLI_Run_FlagValidation checks CLI flag validation (no workers needed).
func TestCLI_Run_FlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{
			"bad output",
			[]string{"run", "--cases", happy.CasePath(), "--output", "yaml", happy.WorkerPaths()[0]},
			"--output must be table, json, or junit",
		},
		{
			"zero timeout",
			[]string{"run", "--cases", happy.CasePath(), "--timeout", "0", happy.WorkerPaths()[0]},
			"--timeout must be > 0",
		},
		{
			"build-only + no-build",
			[]string{"run", "--cases", happy.CasePath(), "--build-only", "--no-build", happy.WorkerPaths()[0]},
			"mutually exclusive",
		},
		{"missing ref", []string{"run", "--cases", happy.CasePath(), happy.WorkerPaths()[0]}, "--ref is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := testutil.RunSerify(t, tt.args...)
			require.NotEqual(t, 0, code, "expected non-zero exit, got 0\n%s", out)
			assert.Contains(t, out, tt.wantSub, tt.name)
		})
	}
}

// TestCLI_Run_MalformedCases exercises error paths for bad case files.
func TestCLI_Run_MalformedCases(t *testing.T) {
	// Invalid YAML.
	badDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(badDir, "bad.yaml"), []byte("{not yaml!!"), 0644))
	out, code := testutil.RunSerify(t, slices.Concat(
		[]string{"run", "--ref", "go", "--cases", badDir, "--no-build"},
		happy.WorkerPaths())...)
	require.NotEqual(t, 0, code, "expected non-zero exit for bad yaml, got 0\n%s", out)
	assert.Contains(t, out, "load cases:", "bad yaml error")

	// Nonexistent directory.
	out2, code2 := testutil.RunSerify(t, slices.Concat(
		[]string{"run", "--ref", "go", "--cases", "/nonexistent", "--no-build"},
		happy.WorkerPaths())...)
	require.NotEqual(t, 0, code2, "expected non-zero exit for missing dir, got 0\n%s", out2)
	assert.Contains(t, out2, "stat", "nonexistent dir")
}

// TestCLI_Run_BadRunCommand exercises the error path when a worker's run command fails.
func TestCLI_Run_BadRunCommand(t *testing.T) {
	tmp := t.TempDir()
	// Create a minimal Go marker file so the builder detects "go".
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\nfunc main() {}"), 0644))
	// worker.yaml with a nonexistent run command.
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "worker.yaml"), []byte("run: ./no-such-worker"), 0644))
	out, code := testutil.RunSerify(t, "run", "--ref", "go", "--cases", happy.CasePath(), "--no-build", tmp)
	require.NotEqual(t, 0, code, "expected non-zero exit, got 0\n%s", out)
	assert.Contains(t, out, "start go worker", "bad run command")
}

// TestCLI_Run_DuplicateLanguage pins the rejection of two workers of the same
// language. Results are keyed by language everywhere downstream — report, CSV
// and table all have one column per language — so the second worker used to
// silently overwrite the first: the run reported a single column and a single
// worker's pass count, with nothing to say a worker's results were discarded.
func TestCLI_Run_DuplicateLanguage(t *testing.T) {
	goWorker := filepath.Join(happy.path, "go")
	out, code := testutil.RunSerify(t,
		"run", "--ref", "go", "--cases", happy.CasePath(), "--no-build",
		goWorker, goWorker)
	require.NotEqual(t, 0, code, "expected non-zero exit for two workers of the same language\n%s", out)
	assert.Contains(t, out, "two go workers", "error should name the duplicated language")
}

// TestCLI_Validate tests the `serify validate` subcommand — case file loading
// and worker detection without running tests.
func TestCLI_Validate(t *testing.T) {
	// Validate cases only (no workers).
	out, code := testutil.RunSerify(t, "validate", "--cases", happy.CasePath())
	require.Equal(t, 0, code, "validate exit = %d, want 0\n%s", code, out)
	assert.Contains(t, out, "Suite:", "validate should show suite")

	// Validate with a worker directory.
	out2, code2 := testutil.RunSerify(t, "validate", "--cases", happy.CasePath(), happy.WorkerPaths()[0])
	require.Equal(t, 0, code2, "validate exit = %d, want 0\n%s", code2, out2)
	assert.Contains(t, out2, "Worker go", "validate should detect worker")

	// Bad cases directory.
	out3, code3 := testutil.RunSerify(t, "validate", "--cases", "/nonexistent")
	require.NotEqual(t, 0, code3, "expected non-zero exit for bad cases, got 0\n%s", out3)
	assert.Contains(t, out3, "load cases:", "bad cases error")
}

// TestCLI_Validate_CasesDirPassedPositionally pins the fix for a silent
// wrong-answer bug: positional arguments are worker directories, so
// `serify validate my-cases` used to validate whatever --cases defaulted to
// (./cases), print a full successful suite report for it, and only then fail
// with an unrelated-looking "cannot detect language" error. A user with a
// ./cases directory therefore saw a green report for a directory they never
// named.
func TestCLI_Validate_CasesDirPassedPositionally(t *testing.T) {
	out, code := testutil.RunSerify(t, "validate", happy.CasePath())
	require.NotEqual(t, 0, code, "expected non-zero exit when a cases dir is passed positionally\n%s", out)
	assert.Contains(t, out, "--cases", "error should name the flag to use instead")
	// The suite report must not appear: nothing was successfully validated.
	assert.NotContains(t, out, "Total:", "validate printed a suite report before failing:\n%s", out)
}
