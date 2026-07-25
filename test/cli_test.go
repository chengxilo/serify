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
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// --- tests -------------------------------------------------------------------

// TestCLI_Run drives `serify run` against the Go example (reference = itself):
// the whole CLI path — load cases, start worker, bind, serialize/deserialize,
// report, exit 0.
func TestCLI_Run(t *testing.T) {
	requireWorkers(t, happy.langs...)
	out, code := testutil.RunSerify(t, happy.runArgs("go", happy.CasePath())...)
	require.Equal(t, 0, code, "exit code = %d, want 0\n%s", code, out)
	t.Log(out)
	assert.Contains(t, out, "Suite:", "run")
	assert.Contains(t, out, "case id", "run") // table header
	assert.Contains(t, out, "PASSED:", "run")
	assert.Contains(t, out, "FAIL: 0", "run")
}

// TestCLI_Run_AllLanguages runs the happy suite over every language whose
// toolchain is installed and requires a full-board PASS: every case, both
// formats, both operations, byte-parity with the go reference. Languages with
// missing toolchains are left out of the run (CI has all of them).
func TestCLI_Run_AllLanguages(t *testing.T) {
	requireWorkers(t, language.Go)

	var paths, langs []string
	for _, w := range happy.Workers() {
		if _, missing := missingLang[w.Language]; missing {
			t.Logf("skipping %s: %s", w.Language, missingLang[w.Language])
			continue
		}
		paths = append(paths, w.Path)
		langs = append(langs, w.Language)
	}

	csv := filepath.Join(t.TempDir(), "out.csv")
	args := []string{"run", "--ref", "go", "--cases", happy.CasePath(), "--csv", csv, "--no-build"}
	args = append(args, paths...)
	out, code := testutil.RunSerify(t, args...)
	require.Equal(t, 0, code, "exit code = %d, want 0 (langs: %s)\n%s", code, strings.Join(langs, ", "), out)

	cases := []string{
		"basic", "zero", "boundary", "large_collections", "signed_max",
		"escapes", "html_escapes", "unicode", "bytes_all", "empty_vs_null",
	}
	grid := readResultGrid(t, csv)
	for _, lang := range langs {
		for _, format := range []string{"binary", "json"} {
			for _, name := range cases {
				id := "all_types/" + format + "/" + name
				assertCell(t, grid, id, lang, report.OpSerialize, report.StatusPass)
				assertCell(t, grid, id, lang, report.OpDeserialize, report.StatusPass)
			}
		}
	}
}

// TestCLI_RunCSVAndTable verifies `serify run --csv` writes the canonical CSV and
// that `serify table` re-renders it (the data layer / view split, via the CLI).
func TestCLI_RunCSVAndTable(t *testing.T) {
	requireWorkers(t, happy.langs...)

	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, happy.runArgs("rust", happy.CasePath(), "--csv", csv)...)
	require.Equal(t, 0, code, "exit code = %d, want 0\n%s", code, out)
	assert.Contains(t, out, "Wrote results to", "run --csv")

	// Verify CSV via the grid helpers (structured, not raw substring checks).
	grid := readResultGrid(t, csv)
	assertCell(t, grid, "all_types/json/basic", "go", "serialize", "PASS")
	assertCell(t, grid, "all_types/json/basic", "rust", "serialize", "PASS")

	tableOut, code := testutil.RunSerify(t, "table", csv)
	require.Equal(t, 0, code, "serify table exit = %d, want 0\n%s", code, tableOut)
	assert.Contains(t, tableOut, "case id", "table")
	assert.Contains(t, tableOut, "all_types/basic", "table")
}

// TestCLI_RunWithoutRef: `run` without --ref exits non-zero with a clear message
// that --ref is required. (The old late-check message "reference language \"\""
// not among provided workers" has been replaced by an early validate() check.)
func TestCLI_RunWithoutRef(t *testing.T) {
	requireWorkers(t, happy.langs...)
	out, code := testutil.RunSerify(t, slices.Concat(
		[]string{
			"run",
			"--cases", happy.CasePath(),
			"--no-build",
		},
		happy.WorkerPaths())...)
	require.NotEqual(t, 0, code, "expected non-zero exit, got 0\n%s", out)
	assert.Contains(t, out, "--ref is required and must name one of the worker languages", "ref-missing")
}

// TestCLI_CasesMustDeclareFormats: a tested type with no formats is rejected.
func TestCLI_CasesMustDeclareFormats(t *testing.T) {
	out, code := testutil.RunSerify(t, slices.Concat(
		[]string{
			"run",
			"--ref", "go",
			"--cases", invalidSchema.CasePath(),
			"--no-build",
		},
		invalidSchema.WorkerPaths())...)
	require.NotEqual(t, 0, code, "expected non-zero exit, got 0\n%s", out)
	assert.Contains(t, out, "must declare at least one format", "no-formats")
}

// TestCLI_TableMissingFile: `table` on a missing CSV fails cleanly.
func TestCLI_TableMissingFile(t *testing.T) {
	out, code := testutil.RunSerify(t,
		"table", filepath.Join(t.TempDir(), "nope.csv"))
	require.NotEqual(t, 0, code, "expected non-zero exit, got 0\n%s", out)
	assert.Contains(t, out, "open", "table-missing")
}

// TestCLI_Run_JUnit checks the `--output junit` path emits JUnit XML.
func TestCLI_Run_JUnit(t *testing.T) {
	requireWorkers(t, happy.langs...)
	out, code := testutil.RunSerify(t, happy.runArgs("go", happy.CasePath(), "--output", "junit")...)
	require.Equal(t, 0, code, "exit code = %d, want 0\n%s", code, out)
	assert.Contains(t, out, "<testsuites>", "junit")
	assert.Contains(t, out, "<testcase", "junit")
}
