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

// Meta-test: drive deliberately-faulty workers (the `wrong` type) through the
// real `serify` CLI and assert it reports the resulting disagreements correctly.
// Each worker drops its OWN language from `langs` when the active format's fault
// directive is disabled, so its output diverges from the reference; serify must
// flag exactly the operations the case's flags predict, and exit non-zero.
//
// This goes through the actual CLI binary (not orchestrate in-process): it
// exercises arg parsing, --ref, the process exit code, and the canonical CSV
// export. Because the CSV is the single source of truth for results, asserting
// against it is just as precise as inspecting the in-memory report.

package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/testutil"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/report"
)

func loadWrongType(t *testing.T) *config.CasesFile {
	t.Helper()
	cf, err := config.LoadCases(
		filepath.Join(
			testutil.RepoRoot(), "test", "cases", "wrong", "cases", "wrong.yaml"))
	require.NoError(t, err, "load wrong.yaml")
	return cf
}

// caseFlag reads a required bool fault directive from a case's data.
func caseFlag(t *testing.T, data map[string]any, key string) bool {
	t.Helper()
	v, ok := data[key]
	require.True(t, ok, "case data missing %q", key)
	b, ok := v.(bool)
	require.True(t, ok, "case field %q is %T, want bool", key, v)
	return b
}

// formatFlags returns the (serialize, deserialize) directives for one format.
func formatFlags(t *testing.T, data map[string]any, format string) (bool, bool) {
	t.Helper()
	switch format {
	case "binary":
		return caseFlag(t, data, "binary_serialize"), caseFlag(t, data, "binary_deserialize")
	case "json":
		return caseFlag(t, data, "json_serialize"), caseFlag(t, data, "json_deserialize")
	default:
		require.Fail(t, "unexpected format", "format %q", format)
		return false, false
	}
}

// TestWrongWorkerErrorsAreReported builds the faulty go (and, if cargo is
// available, rust) workers, runs them through the `serify` CLI over
// test/worker/cases/wrong.yaml, and asserts the exported CSV cell-by-cell against
// the outcome each case's flags predict:
//
//	serialize:   reference always PASS; other workers FAIL iff <fmt>_serialize is off
//	             (their bytes diverge from the reference).
//	deserialize: every worker FAIL unless BOTH <fmt>_serialize and <fmt>_deserialize
//	             are on (a disabled direction corrupts the round-tripped data, which
//	             DataDiff catches against the original). Corruption never errors, so
//	             nothing is SKIP.
//
// Because faults are injected, the CLI itself must exit non-zero.
func TestWrongWorkerErrorsAreReported(t *testing.T) {
	requireWorkers(t, wrong.langs...)

	cf := loadWrongType(t)

	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, wrong.runArgs(language.Go, wrong.CasePath(), "--csv", csv)...)
	// Injected faults must surface as a non-zero CLI exit.
	require.Equal(t, 1, code, "serify exit = %d, want 1 (injected faults must fail the run)\n%s", code, out)

	grid := readResultGrid(t, csv)

	var expectedFails int
	for _, tc := range cf.Cases {
		for _, format := range cf.Formats {
			fs, fd := formatFlags(t, tc.Data, format)
			id := config.TestIDFmt(cf.Name, format, tc.Name)
			for _, lang := range []string{
				language.Go,
				language.Rust,
			} {
				wantSer := report.StatusPass
				if lang != language.Go && !fs {
					wantSer = report.StatusFail
				}
				wantDeser := report.StatusPass
				if !fs || !fd {
					wantDeser = report.StatusFail
				}
				if wantSer == report.StatusFail {
					expectedFails++
				}
				if wantDeser == report.StatusFail {
					expectedFails++
				}
				assertCell(t, grid, id, lang, report.OpSerialize, wantSer)
				assertCell(t, grid, id, lang, report.OpDeserialize, wantDeser)
			}
		}
	}

	// Non-vacuity guard: the matrix must actually exercise failures, otherwise a
	// worker that silently passed everything would masquerade as correct. This
	// holds only because every case's langs contains both go and rust (each worker
	// drops its own name, so the output diverges).
	require.Greater(t, expectedFails, 0, "vacuous test: no failures expected (does every case's langs include go and rust?)")
}
