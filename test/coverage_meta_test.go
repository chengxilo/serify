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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// The decision logic behind --expect-skips is unit-tested in
// internal/orchestrate/coverage_test.go. What that cannot reach is the half
// that lives in the CLI: finding the directory, loading <lang>.yaml, and
// deciding whether to enforce at all. Every one of those steps fails *open* —
// a missing directory, an unreadable file and an empty declaration all leave
// the run green — so a break in the wiring looks exactly like a passing run,
// which is the failure mode this flag exists to prevent in workers.
//
// The fixture is the audit suite driven by go and python. python's worker does
// not register the four formats whose faults need a mutable alias into a
// runtime-owned buffer, so it reports eight genuine SKIPs (four formats x two
// directions) while go reports none. They are coverage skips, not the cascade
// kind CheckExpectedSkips exempts, which makes this the smallest real subject
// in the repo: two workers, one type, one side skipping.

// auditCoverage is the (type, language) pair the assertions below hang on.
const (
	coverageType    = "audit"
	coverageSkipper = language.Python
)

// coverageSkipIDs are the eight rows python skips, as test ids.
var coverageSkipIDs = []string{
	"audit/value-mutating/basic",
	"audit/zero-copy/basic",
	"audit/list-zero-copy/basic",
	"audit/output-zero-copy/basic",
}

// writeExpectSkips creates an expect-skips directory holding one file per
// entry, and returns its path. An entry with no types writes no file, which is
// how "this worker is expected to cover everything" is spelled.
func writeExpectSkips(t *testing.T, decls map[string][]string) string {
	t.Helper()
	dir := t.TempDir()
	for lang, types := range decls {
		body := "types:\n"
		for _, ty := range types {
			body += "  - " + ty + "\n"
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, lang+".yaml"), []byte(body), 0o600),
			"write expect-skips for %s", lang)
	}
	return dir
}

// TestCLI_ExpectSkips covers the three outcomes the flag defines, plus the
// baseline it departs from: a skip is exit-code-neutral until a directory says
// otherwise.
func TestCLI_ExpectSkips(t *testing.T) {
	c := audit.With(language.Go, coverageSkipper)
	requireWorkers(t, c.langs...)

	run := func(t *testing.T, extra ...string) (string, int, resultGrid) {
		t.Helper()
		csv := filepath.Join(t.TempDir(), "out.csv")
		args := append([]string{"--csv", csv, "--audit"}, extra...)
		out, code := testutil.RunSerify(t, c.runArgs(language.Go, c.CasePath(), args...)...)
		return out, code, readResultGrid(t, csv)
	}

	// Baseline: with no directory the flag is inert and python's gaps are
	// invisible. This is the behaviour the other three cases depart from, and
	// the reason the flag has to be opt-in by directory rather than by default:
	// switching it on for every suite would fail every existing honest skip.
	t.Run("no directory leaves skips exit-code-neutral", func(t *testing.T) {
		out, code, grid := run(t)
		require.Equal(t, 0, code, "serify exit = %d, want 0 (a skip alone must not fail a run)\n%s", code, out)
		for _, id := range coverageSkipIDs {
			testutil.AssertCell(t, grid, id, coverageSkipper, report.OpSerialize, report.StatusSkip, nil)
			testutil.AssertCell(t, grid, id, coverageSkipper, report.OpDeserialize, report.StatusSkip, nil)
		}
	})

	// The regression this guards: a worker stops covering something and says
	// nothing. The directory exists, so enforcement is on; python has no file,
	// so it is expected to cover everything and each skip becomes a FAIL.
	t.Run("undeclared skip fails the run", func(t *testing.T) {
		dir := writeExpectSkips(t, nil)
		out, code, grid := run(t, "--expect-skips", dir)
		require.Equal(t, 1, code, "serify exit = %d, want 1 (undeclared skips are a coverage regression)\n%s", code, out)

		for _, id := range coverageSkipIDs {
			testutil.AssertCell(t, grid, id, coverageSkipper, report.OpSerialize, report.StatusFail, nil)
			testutil.AssertCell(t, grid, id, coverageSkipper, report.OpDeserialize, report.StatusFail, nil)
		}
		rec := grid[coverageSkipIDs[0]][coverageSkipper][report.OpSerialize]
		assert.Contains(t, rec.Detail, "undeclared skip",
			"detail = %q, want it to name the undeclared skip so the fix is obvious", rec.Detail)

		// go covers everything and must be untouched: enforcement is per
		// language, not a switch that reclassifies every skip in the run.
		testutil.AssertCell(t, grid, "audit/clean/basic", language.Go, report.OpSerialize, report.StatusPass, nil)
	})

	// A declared gap is the honest case and stays green.
	t.Run("declared skip stays green", func(t *testing.T) {
		dir := writeExpectSkips(t, map[string][]string{coverageSkipper: {coverageType}})
		out, code, grid := run(t, "--expect-skips", dir)
		require.Equal(t, 0, code, "serify exit = %d, want 0 (the gap is declared)\n%s", code, out)
		for _, id := range coverageSkipIDs {
			testutil.AssertCell(t, grid, id, coverageSkipper, report.OpSerialize, report.StatusSkip, nil)
			testutil.AssertCell(t, grid, id, coverageSkipper, report.OpDeserialize, report.StatusSkip, nil)
		}
	})

	// The other direction of drift: a worker gains coverage and its declaration
	// is never removed, so the file keeps permitting a gap that no longer
	// exists. That is a warning, not a failure — the run is honest, the file is
	// merely stale. go skips nothing here, so its entry is exactly that.
	t.Run("stale declaration warns without failing", func(t *testing.T) {
		dir := writeExpectSkips(t, map[string][]string{
			coverageSkipper: {coverageType},
			language.Go:     {coverageType},
		})
		out, code, _ := run(t, "--expect-skips", dir)
		require.Equal(t, 0, code, "serify exit = %d, want 0 (a stale entry is advisory)\n%s", code, out)
		assert.Contains(t, out, "stale expected-skip",
			"a declaration nothing used must be reported so it gets deleted\n%s", out)
	})
}
