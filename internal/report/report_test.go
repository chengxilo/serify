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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "failed to get records from report: %v", err)
	require.Len(t, recs, 8, "expected 8 records, got %d", len(recs))

	var buf bytes.Buffer
	err = WriteCSV(&buf, recs)
	require.NoError(t, err, "WriteCSV: %v", err)
	got, err := ReadCSV(&buf)
	require.NoError(t, err, "ReadCSV: %v", err)
	assert.Equal(t, recs, got, "round-trip mismatch")
}

// TestRecordsSplitFields checks a record is split into type/format/case.
func TestRecordsSplitFields(t *testing.T) {
	recs, _ := sampleReport().Records()
	r0 := recs[0]
	assert.Equal(t, "user", r0.Type, "bad split: %+v", r0)
	assert.Equal(t, "binary", r0.Format, "bad split: %+v", r0)
	assert.Equal(t, "basic", r0.Case, "bad split: %+v", r0)
}

// TestRenderTableFromRecords confirms the table renders from records and shows
// the case id once with both formats.
func TestRenderTableFromRecords(t *testing.T) {
	records, _ := sampleReport().Records()
	var buf bytes.Buffer
	err := RenderTable(&buf, records)
	require.NoError(t, err, "RenderTable: %v", err)
	out := buf.String()
	for _, want := range []string{"case id", "format", "operation", "user/basic", "binary", "json"} {
		assert.Contains(t, out, want, "table missing %q\n%s", want, out)
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
	require.Equal(t, want, got, "got %v, want %v", got, want)
}

// A type with a FAIL is implemented — it ran and disagreed, which is the
// opposite of untested.
func TestUnimplementedTypes_FailureCounts(t *testing.T) {
	r := New([]string{"go"}, nil, "table")
	r.Add(Result{TestID: "wrong/binary/x", Language: "go", Operation: OpSerialize, Status: StatusFail})
	got := r.unimplementedTypes()
	assert.Empty(t, got, "a failing type is implemented, got %v", got)
}
