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

package orchestrate

import (
	"testing"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/report"
)

func skipReport(t *testing.T, entries ...report.Result) *report.Report {
	t.Helper()
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.TestID)
	}
	rep := report.New([]string{"go"}, ids, "table")
	for _, e := range entries {
		rep.Add(e)
	}
	return rep
}

func skipAt(testID, op string) report.Result {
	return report.Result{TestID: testID, Language: "go", Operation: op, Status: report.StatusSkip}
}

// A skip that is declared stays a skip and keeps the run green.
func TestCheckExpectedSkips_DeclaredStaysGreen(t *testing.T) {
	rep := skipReport(t,
		skipAt("wire_name/binary/short", report.OpSerialize),
		skipAt("get_stream/binary/numeric_1", report.OpDeserialize),
	)
	CheckExpectedSkips(rep, map[string]config.ExpectedSkips{
		"go": {
			Types:      []string{"wire_name"},
			Operations: map[string][]string{report.OpDeserialize: {"get_stream"}},
		},
	})
	if got := rep.ExitCode(); got != 0 {
		t.Errorf("ExitCode = %d, want 0: declared skips must not fail the run", got)
	}
}

// An undeclared skip is a coverage regression and must fail — this is the whole
// point: without it, a dropped registration silently turns green.
func TestCheckExpectedSkips_UndeclaredFails(t *testing.T) {
	rep := skipReport(t, skipAt("get_client/binary/client_1", report.OpSerialize))
	CheckExpectedSkips(rep, map[string]config.ExpectedSkips{
		"go": {Types: []string{"wire_name"}},
	})
	if got := rep.ExitCode(); got == 0 {
		t.Error("ExitCode = 0, want non-zero: an undeclared skip must fail")
	}
}

// A blanket direction must not be expressible: declaring deserialize for one
// type must not cover another type's deserialize skip.
func TestCheckExpectedSkips_OperationIsPerType(t *testing.T) {
	rep := skipReport(t, skipAt("message_header/binary/simple", report.OpDeserialize))
	CheckExpectedSkips(rep, map[string]config.ExpectedSkips{
		"go": {Operations: map[string][]string{report.OpDeserialize: {"get_stream"}}},
	})
	if got := rep.ExitCode(); got == 0 {
		t.Error("ExitCode = 0, want non-zero: deserialize declared for get_stream must not cover message_header")
	}
}

// A cascade skip (the reference itself could not produce bytes) is fallout from
// a failure already reported elsewhere, not a coverage statement — declaring it
// must not be required, or a hung/crashed reference would bury every worker in
// bogus "undeclared skip" failures.
func TestCheckExpectedSkips_CascadeSkipsExempt(t *testing.T) {
	for _, detail := range []string{SkipRefSerializeFailed, SkipRefUnsupported} {
		rep := skipReport(t, report.Result{
			TestID: "wrong/binary/hang", Language: "go", Operation: report.OpDeserialize,
			Status: report.StatusSkip, Detail: detail,
		})
		CheckExpectedSkips(rep, map[string]config.ExpectedSkips{})
		if got := rep.ExitCode(); got != 0 {
			t.Errorf("detail %q: ExitCode = %d, want 0 (cascade skips are not coverage gaps)", detail, got)
		}
	}
}

// A declaration that no longer matches anything is reported so it gets removed.
func TestCheckExpectedSkips_StaleDeclarationWarns(t *testing.T) {
	rep := skipReport(t, report.Result{
		TestID: "get_client/binary/client_1", Language: "go",
		Operation: report.OpSerialize, Status: report.StatusPass,
	})
	CheckExpectedSkips(rep, map[string]config.ExpectedSkips{
		"go": {Types: []string{"wire_name"}},
	})
	if len(rep.Warnings) == 0 {
		t.Error("want a warning for the stale expected-skip entry")
	}
	if got := rep.ExitCode(); got != 0 {
		t.Errorf("ExitCode = %d, want 0: a stale entry warns, it does not fail", got)
	}
}
