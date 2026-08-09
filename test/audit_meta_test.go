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
// All nine workers are driven, and every language is asserted independently
// so a failure in one library cannot mask another.

package test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// auditSkipped lists, per format, the languages whose worker does not register
// it: the faults that need a mutable alias into a buffer the runtime handed
// out, or a value receiver. A managed language has nothing to inject, so the
// format is reported SKIPPED.
var auditSkipped = map[string][]string{
	"value-mutating":   {language.Cpp, language.CSharp, language.Elixir, language.Java, language.Node, language.PHP, language.Python, language.Rust},
	"zero-copy":        {language.Cpp, language.CSharp, language.Elixir, language.Java, language.Node, language.PHP, language.Python},
	"list-zero-copy":   {language.Cpp, language.CSharp, language.Elixir, language.Java, language.Node, language.PHP, language.Python},
	"output-zero-copy": {language.Cpp, language.CSharp, language.Elixir, language.Java, language.Node, language.PHP, language.Python},
}

// auditWarnings is the expected warning grid: per (format, audit op), the
// languages whose worker must raise it. A language that registers the format
// but is absent from warn must produce no row for that op — its runtime forbids
// the unsafe behaviour outright, so the check correctly stays silent. Those
// carry a `why`, because a silent check and a broken check look identical from
// the outside and the difference is worth writing down.
var auditWarnings = []struct {
	format string
	op     string
	detail string
	warn   []string
	why    string
}{
	{
		format: "mutating",
		op:     report.OpAuditMutation,
		detail: "mutated fields: value",
		warn:   []string{language.Go, language.Cpp, language.CSharp, language.Java, language.Node, language.PHP, language.Python, language.Rust},
		why:    "elixir: every BEAM term is immutable, so a serializer cannot mutate the model it was handed",
	},
	{
		// Mutating the model also changes what the second serialize call sees.
		// Go's worker marshals before mutating, so its repeat call is unaffected.
		format: "mutating",
		op:     report.OpAuditStability,
		detail: "serializer produced different output on repeat call",
		warn:   []string{language.Cpp, language.CSharp, language.Java, language.Node, language.PHP, language.Python, language.Rust},
		why:    "go: marshals before mutating; elixir: cannot mutate at all",
	},
	{
		format: "value-mutating",
		op:     report.OpAuditMutation,
		detail: "mutated fields: payload",
		warn:   []string{language.Go},
	},
	{
		format: "value-mutating",
		op:     report.OpAuditStability,
		detail: "serializer produced different output on repeat call",
		warn:   []string{language.Go},
	},
	{
		format: "zero-copy",
		op:     report.OpAuditZeroCopy,
		detail: "zero-copy fields: payload",
		warn:   []string{language.Go, language.Rust},
	},
	{
		format: "list-zero-copy",
		op:     report.OpAuditZeroCopy,
		detail: "zero-copy fields: tags",
		warn:   []string{language.Go, language.Rust},
	},
	{
		format: "unstable",
		op:     report.OpAuditStability,
		detail: "serializer produced different output on repeat call",
		warn:   language.All,
	},
	{
		format: "input-mutating",
		op:     report.OpAuditInputMut,
		detail: "deserializer modified input buffer",
		warn:   []string{language.Go, language.Rust, language.Cpp, language.CSharp, language.Java, language.Node},
		why: "elixir (immutable binaries), php (copy-on-write strings) and python (immutable bytes) " +
			"hand the deserializer a value it cannot write through, so it mutates a private copy",
	},
	{
		format: "output-zero-copy",
		op:     report.OpAuditOutputZeroCopy,
		detail: "output aliases model fields: payload",
		warn:   []string{language.Go, language.Rust},
	},
	{
		format: "deser-unstable",
		op:     report.OpAuditDeserStability,
		detail: "deserializer produced different result on repeat call",
		warn:   language.All,
	},
}

func TestAuditWarningsAreReported(t *testing.T) {
	requireWorkers(t, audit.langs...)

	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, audit.runArgs(language.Go, audit.CasePath(), "--csv", csv, "--audit")...)

	require.Equal(t, 0, code, "serify exit = %d, want 0 (audit warnings are advisory)\n%s", code, out)

	grid := readResultGrid(t, csv)

	// Control group: the clean format must be silent in every language.
	for _, lang := range audit.langs {
		assertNoAuditRow(t, grid, "audit/clean/basic", lang)
	}

	// A format a worker does not register is SKIPPED, and a skipped op carries
	// no audit rows.
	for format, langs := range auditSkipped {
		id := "audit/" + format + "/basic"
		for _, lang := range langs {
			testutil.AssertCell(t, grid, id, lang, report.OpSerialize, report.StatusSkip, nil)
			testutil.AssertCell(t, grid, id, lang, report.OpDeserialize, report.StatusSkip, nil)
			assertNoAuditRow(t, grid, id, lang)
		}
	}

	for _, exp := range auditWarnings {
		id := "audit/" + exp.format + "/basic"
		for _, lang := range audit.langs {
			switch {
			case slices.Contains(exp.warn, lang):
				testutil.AssertCell(t, grid, id, lang, exp.op, report.StatusWarn, ptr(exp.detail))
			case slices.Contains(auditSkipped[exp.format], lang):
				// Already asserted SKIP above.
			default:
				assertNoAuditOp(t, grid, id, lang, exp.op, exp.why)
			}
		}
	}
}

// assertNoAuditOp checks that one audit op did not fire, quoting why the
// language cannot exhibit the fault so a future failure reads as a change in
// behaviour rather than an unexplained gap.
func assertNoAuditOp(t *testing.T, grid resultGrid, id, lang, op, why string) {
	t.Helper()
	if rec, ok := grid[id][lang][op]; ok {
		assert.Fail(t, "unexpected audit row",
			"[%s / %s / %s] unexpected audit row: %s %s (expected silence — %s)",
			id, lang, op, rec.Status, rec.Detail, why)
	}
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
