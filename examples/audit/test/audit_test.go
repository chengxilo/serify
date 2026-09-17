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

// Package audit tests the audit example: that three codecs sharing one byte
// layout are indistinguishable to a conformance run, and that --audit tells
// them apart.
package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/lang"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// The cases, split by what there is to find in them. An audit finding depends
// on the data as much as on the code: there is nothing to alias in an empty
// string and nothing to release in an empty buffer.
var (
	loudCases  = []string{"typical", "unicode"} // strings and a payload
	quietCases = []string{"empty"}              // neither
	// no_payload sits between the two: strings to alias, no buffer to release.
	partialCase = "no_payload"
)

func exampleDir() string {
	return filepath.Join(testutil.RepoRoot(), "examples", "audit")
}

// runSuite drives the example once and returns the result grid. `audit` picks
// whether --audit is passed; everything else about the two runs is identical,
// which is the point being asserted.
func runSuite(t *testing.T, audit bool) testutil.ResultGrid {
	t.Helper()

	if reason := testutil.MissingToolchain(lang.Rust); reason != "" {
		t.Skip("rust toolchain unavailable: " + reason)
	}

	dir := exampleDir()
	name := "serify-audit-plain.csv"
	if audit {
		name = "serify-audit-audit.csv"
	}
	csv := filepath.Join(os.TempDir(), name)

	// No --ref: cases/_config.yaml names go the leader.
	args := []string{"run", "--cases", filepath.Join(dir, "cases"), "--csv", csv}
	if audit {
		args = append(args, "--audit")
	}
	args = append(args, filepath.Join(dir, lang.Go), filepath.Join(dir, lang.Rust))

	out, code := testutil.RunSerify(t, args...)
	// Warnings are warnings: --audit must not change the exit code.
	require.Equal(t, 0, code, "expected a clean run (audit=%v):\n%s", audit, out)
	return testutil.ReadResultGrid(t, csv)
}

// TestAudit_ConformanceIsBlindToBoth is the premise of the whole example: all
// three formats encode the same layout, so a conformance run cannot tell the
// safe codec from the unsafe ones.
func TestAudit_ConformanceIsBlindToBoth(t *testing.T) {
	grid := runSuite(t, false)

	for _, format := range []string{"safe", "fast", "handoff"} {
		for _, c := range append(append([]string{}, loudCases...), quietCases...) {
			id := "frame/" + format + "/" + c
			for _, op := range []string{"serialize", "deserialize"} {
				testutil.AssertCell(t, grid, id, lang.Go, op, report.StatusPass, nil)

				// Rust declines `handoff`: its serializer takes &FieldMap, so
				// the mutation that format is built around cannot be written
				// without unsafe. A skip is a declaration, not a failure.
				want := report.StatusPass
				if format == "handoff" {
					want = report.StatusSkip
				}
				testutil.AssertCell(t, grid, id, lang.Rust, op, want, nil)
			}
		}
	}

	// And nothing was reported. Every audit operation is absent from the grid,
	// because the checks did not run.
	for _, op := range auditOps {
		requireNoCell(t, grid, "frame/fast/typical", lang.Go, op)
		requireNoCell(t, grid, "frame/handoff/typical", lang.Go, op)
	}
}

var auditOps = []string{
	report.OpAuditZeroCopy,
	report.OpAuditMutation,
	report.OpAuditStability,
	report.OpAuditInputMut,
	report.OpAuditOutputZeroCopy,
	report.OpAuditDeserStability,
}

// TestAudit_FindsWhatConformanceCannot asserts the two findings, in both
// directions: they appear where the unsafe code is, and nowhere else.
func TestAudit_FindsWhatConformanceCannot(t *testing.T) {
	grid := runSuite(t, true)

	// `fast` aliases the input buffer. Both languages do it, and both are
	// caught — this is the finding that is not a bug.
	for _, c := range loudCases {
		for _, lang := range []string{lang.Go, lang.Rust} {
			testutil.AssertCell(t, grid,
				"frame/fast/"+c, lang, report.OpAuditZeroCopy, report.StatusWarn, nil)
		}
	}
	// Strings but no blob: still aliasing, so still caught.
	testutil.AssertCell(t, grid,
		"frame/fast/"+partialCase, lang.Go, report.OpAuditZeroCopy, report.StatusWarn, nil)

	// `handoff` empties the caller's FieldMap. Go only — Rust does not
	// implement the format at all.
	for _, c := range loudCases {
		testutil.AssertCell(t, grid,
			"frame/handoff/"+c, lang.Go, report.OpAuditMutation, report.StatusWarn, nil)

		// The second finding is the same bug seen from the other end: serify
		// serializes twice to check stability, and the second call reads a
		// payload the first one already emptied. A serializer that mutates its
		// input cannot be a stable one.
		testutil.AssertCell(t, grid,
			"frame/handoff/"+c, lang.Go, report.OpAuditStability, report.StatusWarn, nil)
	}

	// `safe` is the control group: audited, and silent.
	for _, c := range append(append([]string{}, loudCases...), quietCases...) {
		for _, op := range auditOps {
			requireNoCell(t, grid, "frame/safe/"+c, lang.Go, op)
			requireNoCell(t, grid, "frame/safe/"+c, lang.Rust, op)
		}
	}

	// So are the cases with nothing to find, under the very codecs that were
	// unsafe a case ago: a finding is about this format on this data.
	for _, c := range quietCases {
		for _, op := range auditOps {
			requireNoCell(t, grid, "frame/fast/"+c, lang.Go, op)
			requireNoCell(t, grid, "frame/handoff/"+c, lang.Go, op)
		}
	}
	// no_payload has no buffer to release, so handoff is clean there too, even
	// though the same code warns on `typical`.
	requireNoCell(t, grid, "frame/handoff/"+partialCase, lang.Go, report.OpAuditMutation)
}

// requireNoCell fails if the grid holds a result for this (id, lang, op).
func requireNoCell(t *testing.T, grid testutil.ResultGrid, id, lang, op string) {
	t.Helper()
	byLang, ok := grid[id]
	if !ok {
		return
	}
	byOp, ok := byLang[lang]
	if !ok {
		return
	}
	res, ok := byOp[op]
	require.False(t, ok, "unexpected %s result for %s / %s: %s", op, id, lang, res.Status)
}
