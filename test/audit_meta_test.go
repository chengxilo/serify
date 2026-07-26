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

// Meta-test: drive deliberately-broken audit workers through the real `serify`
// CLI with --audit and assert it reports the expected warnings. Each format
// injects a different unsafe behaviour:
//
//	clean            – correct round-trip (no warnings)
//	mutating         – serializer mutates input struct → audit-mutation WARN
//	value-mutating   – value receiver mutates shared payload backing (Go only)
//	zero-copy        – deserializer aliases input buffer → audit-zero-copy WARN
//	list-zero-copy   – deserializer aliases tags via unsafe string
//	unstable         – serializer appends counter → audit-stability WARN
//	deser-unstable   – deserializer produces different result on repeat
//	input-mutating   – deserializer modifies input buffer
//	output-zero-copy – serializer returns buffer aliasing model fields
//
// Warnings do NOT cause a non-zero exit — the CLI must exit 0.
//
// Both the Go and Rust workers are driven; each language is asserted
// independently so a failure in one library does not mask the other.

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

func TestAuditWarningsAreReported(t *testing.T) {
	requireWorkers(t, audit.langs...)

	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, audit.runArgs(language.Go, audit.CasePath(), "--csv", csv, "--audit")...)

	require.Equal(t, 0, code, "serify exit = %d, want 0 (audit warnings are advisory)\n%s", code, out)

	grid := readResultGrid(t, csv)

	cleanID := "audit/clean/basic"
	mutID := "audit/mutating/basic"
	valMutID := "audit/value-mutating/basic"
	zcID := "audit/zero-copy/basic"
	listZcID := "audit/list-zero-copy/basic"
	unsID := "audit/unstable/basic"
	deserUnsID := "audit/deser-unstable/basic"
	inputMutID := "audit/input-mutating/basic"
	outZcID := "audit/output-zero-copy/basic"

	// --- Go worker -------------------------------------------------------

	// Clean format: no audit rows.
	assertNoAuditRow(t, grid, cleanID, language.Go)

	// Mutating format: mutation WARN on serialize.
	testutil.AssertCell(t, grid, mutID, language.Go, report.OpAuditMutation, report.StatusWarn, ptr("mutated fields: value"))

	// Value-mutating: mutation WARN (payload[0] was flipped) + stability WARN
	// (shared backing corruption affects the second serialize call).
	testutil.AssertCell(t, grid, valMutID, language.Go, report.OpAuditMutation, report.StatusWarn, ptr("mutated fields: payload"))
	testutil.AssertCell(
		t,
		grid,
		valMutID,
		language.Go,
		report.OpAuditStability,
		report.StatusWarn,
		ptr("serializer produced different output on repeat call"),
	)

	// Zero-copy format: zero-copy WARN on deserialize.
	testutil.AssertCell(t, grid, zcID, language.Go, report.OpAuditZeroCopy, report.StatusWarn, ptr("zero-copy fields: payload"))

	// List-zero-copy: zero-copy WARN for tags field.
	testutil.AssertCell(t, grid, listZcID, language.Go, report.OpAuditZeroCopy, report.StatusWarn, ptr("zero-copy fields: tags"))

	// Unstable format: stability WARN on serialize.
	testutil.AssertCell(
		t,
		grid,
		unsID,
		language.Go,
		report.OpAuditStability,
		report.StatusWarn,
		ptr("serializer produced different output on repeat call"),
	)

	// Deser-unstable: deser-stability WARN on deserialize.
	testutil.AssertCell(
		t,
		grid,
		deserUnsID,
		language.Go,
		report.OpAuditDeserStability,
		report.StatusWarn,
		ptr("deserializer produced different result on repeat call"),
	)

	// Input-mutating: input-mutation WARN on deserialize.
	testutil.AssertCell(t, grid, inputMutID, language.Go, report.OpAuditInputMut, report.StatusWarn, ptr("deserializer modified input buffer"))

	// Output-zero-copy: output-ZC WARN on serialize.
	testutil.AssertCell(t, grid, outZcID, language.Go, report.OpAuditOutputZeroCopy, report.StatusWarn, ptr("output aliases model fields: payload"))

	// --- Rust worker -----------------------------------------------------

	// Clean format: no audit rows.
	assertNoAuditRow(t, grid, cleanID, language.Rust)

	// Mutating format: mutation WARN + stability WARN.
	testutil.AssertCell(t, grid, mutID, language.Rust, report.OpAuditMutation, report.StatusWarn, ptr("mutated fields: value"))
	testutil.AssertCell(
		t,
		grid,
		mutID,
		language.Rust,
		report.OpAuditStability,
		report.StatusWarn,
		ptr("serializer produced different output on repeat call"),
	)

	// Value-mutating: Rust worker does NOT register this format → SKIP rows.
	testutil.AssertCell(t, grid, valMutID, language.Rust, report.OpSerialize, report.StatusSkip, nil)
	testutil.AssertCell(t, grid, valMutID, language.Rust, report.OpDeserialize, report.StatusSkip, nil)
	// No audit rows for SKIP format.
	assertNoAuditRow(t, grid, valMutID, language.Rust)

	// Zero-copy format: zero-copy WARN on deserialize.
	testutil.AssertCell(t, grid, zcID, language.Rust, report.OpAuditZeroCopy, report.StatusWarn, ptr("zero-copy fields: payload"))

	// List-zero-copy: zero-copy WARN for tags field.
	testutil.AssertCell(t, grid, listZcID, language.Rust, report.OpAuditZeroCopy, report.StatusWarn, ptr("zero-copy fields: tags"))

	// Unstable format: stability WARN on serialize.
	testutil.AssertCell(
		t,
		grid,
		unsID,
		language.Rust,
		report.OpAuditStability,
		report.StatusWarn,
		ptr("serializer produced different output on repeat call"),
	)

	// Deser-unstable: deser-stability WARN on deserialize.
	testutil.AssertCell(
		t,
		grid,
		deserUnsID,
		language.Rust,
		report.OpAuditDeserStability,
		report.StatusWarn,
		ptr("deserializer produced different result on repeat call"),
	)

	// Input-mutating: input-mutation WARN on deserialize.
	testutil.AssertCell(t, grid, inputMutID, language.Rust, report.OpAuditInputMut, report.StatusWarn, ptr("deserializer modified input buffer"))

	// Output-zero-copy: output-ZC WARN on serialize.
	testutil.AssertCell(
		t,
		grid,
		outZcID,
		language.Rust,
		report.OpAuditOutputZeroCopy,
		report.StatusWarn,
		ptr("output aliases model fields: payload"),
	)
}

func assertNoAuditRow(t *testing.T, grid resultGrid, id, lang string) {
	t.Helper()
	auditOps := []string{
		report.OpAuditMutation, report.OpAuditZeroCopy,
		report.OpAuditStability, report.OpAuditInputMut,
		report.OpAuditOutputZeroCopy, report.OpAuditDeserStability,
	}
	byOp, ok := grid[id][lang]
	if ok {
		for _, op := range auditOps {
			if rec, exists := byOp[op]; exists {
				assert.Fail(t, "unexpected audit row",
					"[%s / %s / %s] unexpected audit row: %s %s", id, lang, op, rec.Status, rec.Detail)
			}
		}
	}
}
