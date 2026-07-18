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
	"strings"
	"testing"

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
	if code != 1 {
		t.Fatalf("serify exit = %d, want 1 (errors without known-failures must fail)\n%s", code, out)
	}

	grid1 := readResultGrid(t, csv1)

	// err_ser/boom: serialize FAIL both langs, deserialize SKIP.
	assertCell(t, grid1, "wrong/err_ser/boom", language.Go, report.OpSerialize, report.StatusFail)
	assertCell(t, grid1, "wrong/err_ser/boom", language.Rust, report.OpSerialize, report.StatusFail)
	assertCell(t, grid1, "wrong/err_ser/boom", language.Go, report.OpDeserialize, report.StatusSkip)
	assertCell(t, grid1, "wrong/err_ser/boom", language.Rust, report.OpDeserialize, report.StatusSkip)
	// Verify detail contains the injected error.
	rec := grid1["wrong/err_ser/boom"][language.Go][report.OpSerialize]
	if !strings.Contains(rec.Detail, "injected serialize error") {
		t.Errorf("err_ser detail = %q, want injected serialize error", rec.Detail)
	}

	// err_deser/boom: serialize PASS, deserialize FAIL.
	assertCell(t, grid1, "wrong/err_deser/boom", language.Go, report.OpSerialize, report.StatusPass)
	assertCell(t, grid1, "wrong/err_deser/boom", language.Rust, report.OpSerialize, report.StatusPass)
	assertCell(t, grid1, "wrong/err_deser/boom", language.Go, report.OpDeserialize, report.StatusFail)
	assertCell(t, grid1, "wrong/err_deser/boom", language.Rust, report.OpDeserialize, report.StatusFail)

	// Run 2: with --known-failures — must exit 0 with XFAILs.
	kfDir := wrong.CasePathNamed("known_failures")
	csv2 := filepath.Join(t.TempDir(), "out2.csv")
	out2, code2 := testutil.RunSerify(t, wrong.runArgs(language.Go, errCases,
		"--csv", csv2, "--known-failures", kfDir)...)
	if code2 != 0 {
		t.Fatalf("serify exit = %d, want 0 (all failures are known)\n%s", code2, out2)
	}
	if !strings.Contains(out2, "XFAIL:") {
		t.Errorf("known-failures summary missing XFAIL:\n%s", out2)
	}

	grid2 := readResultGrid(t, csv2)
	assertCell(t, grid2, "wrong/err_ser/boom", language.Go, report.OpSerialize, report.StatusXFail)
	assertCell(t, grid2, "wrong/err_ser/boom", language.Rust, report.OpSerialize, report.StatusXFail)
	assertCell(t, grid2, "wrong/err_deser/boom", language.Go, report.OpDeserialize, report.StatusXFail)
	assertCell(t, grid2, "wrong/err_deser/boom", language.Rust, report.OpDeserialize, report.StatusXFail)

	// err_deser's serialize op succeeds despite its known-failures entry: that
	// must surface as XPASS (a warning), not blend into PASS.
	assertCell(t, grid2, "wrong/err_deser/boom", language.Go, report.OpSerialize, report.StatusXPass)
	assertCell(t, grid2, "wrong/err_deser/boom", language.Rust, report.OpSerialize, report.StatusXPass)
	rec2 := grid2["wrong/err_deser/boom"][language.Go][report.OpSerialize]
	if !strings.Contains(rec2.Detail, "expected to fail") {
		t.Errorf("XPASS detail = %q, want mention of expected-to-fail reason", rec2.Detail)
	}

	// err_ser's deserialize op is skipped (nothing was serialized); a
	// known-failures entry must not convert SKIP into anything else.
	assertCell(t, grid2, "wrong/err_ser/boom", language.Go, report.OpDeserialize, report.StatusSkip)
	assertCell(t, grid2, "wrong/err_ser/boom", language.Rust, report.OpDeserialize, report.StatusSkip)
}

// TestCLI_Run_TimeoutOnHungWorker verifies that --timeout kills a hung worker.
// The hang format sleeps 3 s; --timeout 1 ensures the race is always won by timeout.
func TestCLI_Run_TimeoutOnHungWorker(t *testing.T) {
	requireWorkers(t, wrong.langs...)

	hangCases := wrong.CasePathNamed("cases_hang")
	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, wrong.runArgs(language.Go, hangCases,
		"--csv", csv, "--timeout", "1")...)
	if code != 1 {
		t.Fatalf("serify exit = %d, want 1 (timeout must fail)\n%s", code, out)
	}

	grid := readResultGrid(t, csv)

	// Both languages ERROR on serialize; deserialize SKIP (reference serialize failed).
	assertCell(t, grid, "wrong/hang/sleeper", language.Go, report.OpSerialize, report.StatusError)
	assertCell(t, grid, "wrong/hang/sleeper", language.Rust, report.OpSerialize, report.StatusError)
	rec := grid["wrong/hang/sleeper"][language.Go][report.OpSerialize]
	if !strings.Contains(rec.Detail, "timeout after 1s") {
		t.Errorf("hang detail = %q, want timeout after 1s", rec.Detail)
	}
	assertCell(t, grid, "wrong/hang/sleeper", language.Go, report.OpDeserialize, report.StatusSkip)
	assertCell(t, grid, "wrong/hang/sleeper", language.Rust, report.OpDeserialize, report.StatusSkip)
}

// TestCLI_Run_WorkerCrash verifies that a crashing worker produces ERROR results.
func TestCLI_Run_WorkerCrash(t *testing.T) {
	requireWorkers(t, wrong.langs...)

	crashCases := wrong.CasePathNamed("cases_crash")
	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, wrong.runArgs(language.Go, crashCases, "--csv", csv)...)
	if code != 1 {
		t.Fatalf("serify exit = %d, want 1 (crash must fail)\n%s", code, out)
	}

	grid := readResultGrid(t, csv)

	// Both languages ERROR on serialize with non-empty detail; deserialize SKIP.
	assertCell(t, grid, "wrong/crash/abort", language.Go, report.OpSerialize, report.StatusError)
	assertCell(t, grid, "wrong/crash/abort", language.Rust, report.OpSerialize, report.StatusError)
	// Detail is platform-dependent (EOF/pipe/exit), just check non-empty.
	rec := grid["wrong/crash/abort"][language.Go][report.OpSerialize]
	if rec.Detail == "" {
		t.Error("crash detail is empty, want non-empty transport error")
	}
	assertCell(t, grid, "wrong/crash/abort", language.Go, report.OpDeserialize, report.StatusSkip)
	assertCell(t, grid, "wrong/crash/abort", language.Rust, report.OpDeserialize, report.StatusSkip)
}
