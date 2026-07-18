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

package report

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// sampleReport builds a small report: two languages, one case in two formats,
// serialize + deserialize each.
func sampleReport() *Report {
	langs := []string{"go", "rust"}
	ids := []string{"user/binary/basic", "user/json/basic"}
	r := New(langs, ids, "table")
	for _, id := range ids {
		for _, lang := range langs {
			for _, op := range []string{OpSerialize, OpDeserialize} {
				r.Add(Result{TestID: id, Language: lang, Operation: op, Status: StatusPass})
			}
		}
	}
	return r
}

// TestRecordsCSVRoundTrip verifies the canonical records survive a CSV
// write/read unchanged — i.e. the CSV is a faithful dump of the run data.
func TestRecordsCSVRoundTrip(t *testing.T) {
	recs, err := sampleReport().Records()
	if err != nil {
		t.Fatalf("failed to get records from report: %v", err)
	}
	if len(recs) != 8 { // 2 formats × 2 langs × 2 ops
		t.Fatalf("expected 8 records, got %d", len(recs))
	}

	var buf bytes.Buffer
	if err := WriteCSV(&buf, recs); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	got, err := ReadCSV(&buf)
	if err != nil {
		t.Fatalf("ReadCSV: %v", err)
	}
	if !reflect.DeepEqual(recs, got) {
		t.Errorf("round-trip mismatch:\n want %+v\n got  %+v", recs, got)
	}
}

// TestRecordsSplitFields checks a record is split into type/format/case.
func TestRecordsSplitFields(t *testing.T) {
	recs, _ := sampleReport().Records()
	r0 := recs[0]
	if r0.Type != "user" || r0.Format != "binary" || r0.Case != "basic" {
		t.Errorf("bad split: %+v", r0)
	}
}

// TestRenderTableFromRecords confirms the table renders from records and shows
// the case id once with both formats.
func TestRenderTableFromRecords(t *testing.T) {
	records, _ := sampleReport().Records()
	var buf bytes.Buffer
	if err := RenderTable(&buf, records); err != nil {
		t.Fatalf("RenderTable: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"case id", "format", "operation", "user/basic", "binary", "json"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q\n%s", want, out)
		}
	}
}

// TestUnimplementedTypes pins the detection of a type that no worker
// implements. Such a type produces a column of SKIP, which in the grid reads
// exactly like a healthy run: the suite reports exit 0 having compared nothing
// about it at all.
func TestUnimplementedTypes(t *testing.T) {
	r := New([]string{"go", "rust"}, nil, "table")

	// ledger: implemented by go, skipped by rust — a normal partial suite.
	r.Add(Result{TestID: "ledger/binary/deposit", Language: "go", Operation: OpSerialize, Status: StatusPass})
	r.Add(Result{TestID: "ledger/binary/deposit", Language: "rust", Operation: OpSerialize, Status: StatusSkip})

	// telemetry: skipped by everyone.
	r.Add(Result{TestID: "telemetry/binary/nominal", Language: "go", Operation: OpSerialize, Status: StatusSkip})
	r.Add(Result{TestID: "telemetry/binary/nominal", Language: "rust", Operation: OpSerialize, Status: StatusSkip})

	// order: skipped by everyone too, so the result must be name-ordered.
	r.Add(Result{TestID: "order/binary/paid", Language: "go", Operation: OpSerialize, Status: StatusSkip})

	got := r.unimplementedTypes()
	want := []string{"order", "telemetry"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// A type with a FAIL is implemented — it ran and disagreed, which is the
// opposite of untested.
func TestUnimplementedTypes_FailureCounts(t *testing.T) {
	r := New([]string{"go"}, nil, "table")
	r.Add(Result{TestID: "wrong/binary/x", Language: "go", Operation: OpSerialize, Status: StatusFail})
	if got := r.unimplementedTypes(); len(got) != 0 {
		t.Errorf("a failing type is implemented, got %v", got)
	}
}
