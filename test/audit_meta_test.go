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
//
// The `audit_model` type is the same idea one layer up: the same faults over
// the same byte layout, registered through each binding's **model** path rather
// than at the FieldMap boundary. See auditModelWarnings below and "Audit on the
// model path" in AGENTS.md.

package test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/lang"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// auditSkipped lists, per format, the languages whose worker does not register
// it: the faults that need a mutable alias into a buffer the runtime handed
// out, or a value receiver. A managed language has nothing to inject, so the
// format is reported SKIPPED.
var auditSkipped = map[string][]string{
	"value-mutating":   {lang.Cpp, lang.CSharp, lang.Elixir, lang.Java, lang.Node, lang.PHP, lang.Python, lang.Rust},
	"zero-copy":        {lang.Cpp, lang.CSharp, lang.Elixir, lang.Java, lang.Node, lang.PHP, lang.Python},
	"list-zero-copy":   {lang.Cpp, lang.CSharp, lang.Elixir, lang.Java, lang.Node, lang.PHP, lang.Python},
	"output-zero-copy": {lang.Cpp, lang.CSharp, lang.Elixir, lang.Java, lang.Node, lang.PHP, lang.Python},
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
		warn:   []string{lang.Go, lang.Cpp, lang.CSharp, lang.Java, lang.Node, lang.PHP, lang.Python, lang.Rust},
		why:    "elixir: every BEAM term is immutable, so a serializer cannot mutate the model it was handed",
	},
	{
		// Mutating the model also changes what the second serialize call sees.
		// Go's worker marshals before mutating, so its repeat call is unaffected.
		format: "mutating",
		op:     report.OpAuditStability,
		detail: "serializer produced different output on repeat call",
		warn:   []string{lang.Cpp, lang.CSharp, lang.Java, lang.Node, lang.PHP, lang.Python, lang.Rust},
		why:    "go: marshals before mutating; elixir: cannot mutate at all",
	},
	{
		format: "value-mutating",
		op:     report.OpAuditMutation,
		detail: "mutated fields: payload",
		warn:   []string{lang.Go},
	},
	{
		format: "value-mutating",
		op:     report.OpAuditStability,
		detail: "serializer produced different output on repeat call",
		warn:   []string{lang.Go},
	},
	{
		format: "zero-copy",
		op:     report.OpAuditZeroCopy,
		detail: "zero-copy fields: payload",
		warn:   []string{lang.Go, lang.Rust},
	},
	{
		format: "list-zero-copy",
		op:     report.OpAuditZeroCopy,
		detail: "zero-copy fields: tags",
		warn:   []string{lang.Go, lang.Rust},
	},
	{
		format: "unstable",
		op:     report.OpAuditStability,
		detail: "serializer produced different output on repeat call",
		warn:   lang.All,
	},
	{
		format: "input-mutating",
		op:     report.OpAuditInputMut,
		detail: "deserializer modified input buffer",
		warn:   []string{lang.Go, lang.Rust, lang.Cpp, lang.CSharp, lang.Java, lang.Node},
		why: "elixir (immutable binaries), php (copy-on-write strings) and python (immutable bytes) " +
			"hand the deserializer a value it cannot write through, so it mutates a private copy",
	},
	{
		format: "output-zero-copy",
		op:     report.OpAuditOutputZeroCopy,
		detail: "output aliases model fields: payload",
		warn:   []string{lang.Go, lang.Rust},
	},
	{
		format: "deser-unstable",
		op:     report.OpAuditDeserStability,
		detail: "deserializer produced different result on repeat call",
		warn:   lang.All,
	},
}

// auditModelSkipped is auditSkipped's twin for the `audit_model` type: only Go
// and Rust can hand back a model that views the input buffer instead of copying
// it, so everyone else declines `zero-copy`.
var auditModelSkipped = map[string][]string{
	"zero-copy": {lang.Cpp, lang.CSharp, lang.Elixir, lang.Java, lang.Node, lang.PHP, lang.Python},
	// Rust declines `mutating`: a serify serializer there receives `&M`, so
	// mutating it is UB and a release build discards the write outright. Not a
	// fault an honest Rust worker can commit through a model — the same reason
	// it declines `value-mutating` above and `handoff` in examples/audit.
	"mutating": {lang.Rust},
}

// auditModelWarnings is the expected grid for the model path. Every binding
// retains the model instance a call used and re-derives the FieldMap at each
// probe point; one that regresses to a pure conversion goes silent here and the
// `warn` assertion fails. `why` covers a language whose silence is legitimate,
// exactly as it does for auditWarnings.
var auditModelWarnings = []struct {
	format string
	op     string
	detail string
	warn   []string
	why    string
}{
	{
		// The subject: a serializer that scribbles on the object it was handed.
		format: "mutating",
		op:     report.OpAuditMutation,
		detail: "mutated fields: value",
		warn: []string{
			lang.Cpp, lang.CSharp, lang.Go, lang.Java,
			lang.Node, lang.PHP, lang.Python,
		},
		why: "elixir: every BEAM term is immutable, so a serializer cannot mutate the struct it was handed — its silence is about the runtime, not about audit",
	},
	{
		// The positive control: this fault is visible in the returned bytes, so
		// it does not depend on the model surviving the call. Every language
		// reporting it is what proves each model worker really ran under
		// --audit, which is what makes a silence elsewhere meaningful.
		format: "unstable",
		op:     report.OpAuditStability,
		detail: "serializer produced different output on repeat call",
		warn:   lang.All,
	},
	{
		// The deserialize half: a model that views the input buffer rather than
		// copying it. Only Go and Rust can express it at all — a managed
		// runtime's string is a copy by construction.
		format: "zero-copy",
		op:     report.OpAuditZeroCopy,
		detail: "zero-copy fields: payload",
		warn:   []string{lang.Go, lang.Rust},
	},
}

func TestAuditWarningsAreReported(t *testing.T) {
	requireWorkers(t, audit.langs...)

	csv := filepath.Join(t.TempDir(), "out.csv")
	out, code := testutil.RunSerify(t, audit.runArgs(lang.Go, audit.CasePath(), "--csv", csv, "--audit")...)

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

	// The same grid, one layer up: the model path. Same CLI run, so this costs
	// nothing beyond the assertions.
	t.Run("model path", func(t *testing.T) {
		// Control group: a model worker that does nothing wrong is silent.
		for _, lang := range audit.langs {
			assertNoAuditRow(t, grid, "audit_model/clean/basic", lang)
		}

		for format, langs := range auditModelSkipped {
			id := "audit_model/" + format + "/basic"
			for _, lang := range langs {
				testutil.AssertCell(t, grid, id, lang, report.OpSerialize, report.StatusSkip, nil)
				testutil.AssertCell(t, grid, id, lang, report.OpDeserialize, report.StatusSkip, nil)
				assertNoAuditRow(t, grid, id, lang)
			}
		}

		for _, exp := range auditModelWarnings {
			id := "audit_model/" + exp.format + "/basic"
			for _, lang := range audit.langs {
				switch {
				case slices.Contains(exp.warn, lang):
					testutil.AssertCell(t, grid, id, lang, exp.op, report.StatusWarn, ptr(exp.detail))
				case slices.Contains(auditModelSkipped[exp.format], lang):
					// Already asserted SKIP above.
				default:
					assertNoAuditOp(t, grid, id, lang, exp.op, exp.why)
				}
			}
		}
	})
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
