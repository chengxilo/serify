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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// TestCLI_KnownFailures verifies that --known-failures turns FAIL into XFAIL
// for worker-reported protocol errors, and that a fully-known-failing run exits 0.
// Known-failures apply only to worker-reported protocol errors (StatusError),
// never to byte-diff or transport failures. An op that succeeds despite its
// known-failures entry must be reported XPASS, and SKIPs stay SKIPs.
// The entries live in the checked-in fixture test/cases/wrong/known_failures/.
func TestCLI_KnownFailures(t *testing.T) {
	requireWorkers(t, wrong.langs...)

	errCases := wrong.CasePathNamed("cases_error")

	// Run 1: without --known-failures — must exit 1 with FAILs.
	csv1 := filepath.Join(t.TempDir(), "out1.csv")
	out, code := testutil.RunSerify(t, wrong.runArgs(language.Go, errCases, "--csv", csv1)...)
	require.Equal(t, 1, code, "serify exit = %d, want 1 (errors without known-failures must fail)\n%s", code, out)

	grid1 := readResultGrid(t, csv1)

	// err_ser/boom: serialize FAIL both langs, deserialize SKIP.
	for _, lang := range wrong.langs {
		testutil.AssertCell(t, grid1, "wrong/err_ser/boom", lang, report.OpSerialize, report.StatusFail, nil)
		testutil.AssertCell(t, grid1, "wrong/err_ser/boom", lang, report.OpDeserialize, report.StatusSkip, nil)
	}
	// Verify detail contains the injected error.
	rec := grid1["wrong/err_ser/boom"][language.Go][report.OpSerialize]
	assert.Contains(t, rec.Detail, "injected serialize error",
		"err_ser detail = %q, want injected serialize error", rec.Detail)

	// err_deser/boom: serialize PASS, deserialize FAIL.
	for _, lang := range wrong.langs {
		testutil.AssertCell(t, grid1, "wrong/err_deser/boom", lang, report.OpSerialize, report.StatusPass, nil)
		testutil.AssertCell(t, grid1, "wrong/err_deser/boom", lang, report.OpDeserialize, report.StatusFail, nil)
	}

	// Run 2: with --known-failures — must exit 0 with XFAILs.
	kfDir := wrong.CasePathNamed("known_failures")
	csv2 := filepath.Join(t.TempDir(), "out2.csv")
	out2, code2 := testutil.RunSerify(t, wrong.runArgs(language.Go, errCases,
		"--csv", csv2, "--known-failures", kfDir)...)
	require.Equal(t, 0, code2, "serify exit = %d, want 0 (all failures are known)\n%s", code2, out2)
	assert.Contains(t, out2, "XFAIL:", "known-failures summary missing XFAIL:\n%s", out2)

	grid2 := readResultGrid(t, csv2)
	for _, lang := range wrong.langs {
		testutil.AssertCell(t, grid2, "wrong/err_ser/boom", lang, report.OpSerialize, report.StatusXFail, nil)
		testutil.AssertCell(t, grid2, "wrong/err_deser/boom", lang, report.OpDeserialize, report.StatusXFail, nil)
	}

	// err_deser's serialize op succeeds despite its known-failures entry: that
	// must surface as XPASS (a warning), not blend into PASS.
	for _, lang := range wrong.langs {
		testutil.AssertCell(t, grid2, "wrong/err_deser/boom", lang, report.OpSerialize, report.StatusXPass, nil)
	}
	rec2 := grid2["wrong/err_deser/boom"][language.Go][report.OpSerialize]
	assert.Contains(t, rec2.Detail, "expected to fail",
		"XPASS detail = %q, want mention of expected-to-fail reason", rec2.Detail)

	// err_ser's deserialize op is skipped (nothing was serialized); a
	// known-failures entry must not convert SKIP into anything else.
	for _, lang := range wrong.langs {
		testutil.AssertCell(t, grid2, "wrong/err_ser/boom", lang, report.OpDeserialize, report.StatusSkip, nil)
	}
}

// TestCLI_Run_TimeoutOnHungWorker verifies that --timeout kills a hung worker.
// The hang format sleeps 3 s; --timeout 1 ensures the race is always won by timeout.
//
// Go and Rust only, on purpose. --timeout also bounds the start-up ping, and a
// second is not enough for the JVM, the CLR or the BEAM to answer it: the run
// would die with "failed to ping csharp worker" before writing a CSV. What is
// under test is the runner's timeout handling, and one hung worker proves that.
var hang = wrong.With(language.Go, language.Rust)

func TestCLI_Run_TimeoutOnHungWorker(t *testing.T) {
	requireWorkers(t, hang.langs...)

	hangCases := hang.CasePathNamed("cases_hang")
	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, hang.runArgs(language.Go, hangCases,
		"--csv", csv, "--timeout", "1")...)
	require.Equal(t, 1, code, "serify exit = %d, want 1 (timeout must fail)\n%s", code, out)

	grid := readResultGrid(t, csv)

	// Both languages ERROR on serialize; deserialize SKIP (reference serialize failed).
	for _, lang := range hang.langs {
		testutil.AssertCell(t, grid, "wrong/hang/sleeper", lang, report.OpSerialize, report.StatusError, nil)
	}
	rec := grid["wrong/hang/sleeper"][language.Go][report.OpSerialize]
	assert.Contains(t, rec.Detail, "timeout after 1s",
		"hang detail = %q, want timeout after 1s", rec.Detail)
	for _, lang := range hang.langs {
		testutil.AssertCell(t, grid, "wrong/hang/sleeper", lang, report.OpDeserialize, report.StatusSkip, nil)
	}
}

// TestCLI_Run_WorkerCrash verifies that a crashing worker produces ERROR results.
func TestCLI_Run_WorkerCrash(t *testing.T) {
	requireWorkers(t, wrong.langs...)

	crashCases := wrong.CasePathNamed("cases_crash")
	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, wrong.runArgs(language.Go, crashCases, "--csv", csv)...)
	require.Equal(t, 1, code, "serify exit = %d, want 1 (crash must fail)\n%s", code, out)

	grid := readResultGrid(t, csv)

	// Both languages ERROR on serialize with non-empty detail; deserialize SKIP.
	for _, lang := range wrong.langs {
		testutil.AssertCell(t, grid, "wrong/crash/abort", lang, report.OpSerialize, report.StatusError, nil)
	}
	// Detail is platform-dependent (EOF/pipe/exit), just check non-empty.
	rec := grid["wrong/crash/abort"][language.Go][report.OpSerialize]
	assert.NotEmpty(t, rec.Detail, "crash detail is empty, want non-empty transport error")
	for _, lang := range wrong.langs {
		testutil.AssertCell(t, grid, "wrong/crash/abort", lang, report.OpDeserialize, report.StatusSkip, nil)
	}
}
