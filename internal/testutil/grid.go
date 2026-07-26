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

package testutil

import (
	"os"
	"testing"

	"github.com/chengxilo/serify/internal/report"
)

// ResultGrid indexes CSV records as testID -> language -> operation -> record.
type ResultGrid map[string]map[string]map[string]report.Record

// ReadResultGrid reads a CSV exported by `serify run --csv` and indexes it.
func ReadResultGrid(t *testing.T, csvPath string) ResultGrid {
	t.Helper()
	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer func() { _ = f.Close() }()
	recs, err := report.ReadCSV(f)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	grid := ResultGrid{}
	for _, r := range recs {
		if grid[r.TestID] == nil {
			grid[r.TestID] = map[string]map[string]report.Record{}
		}
		if grid[r.TestID][r.Language] == nil {
			grid[r.TestID][r.Language] = map[string]report.Record{}
		}
		grid[r.TestID][r.Language][r.Operation] = r
	}
	return grid
}

// AssertCell checks one cell of a ResultGrid. A FAIL must carry non-empty detail.
// If wantDetail is non-nil, the cell's detail must match exactly.
func AssertCell(t *testing.T, grid ResultGrid, id, lang, op string, wantStatus report.Status, wantDetail *string) {
	t.Helper()
	rec, ok := grid[id][lang][op]
	if !ok {
		t.Errorf("[%s / %s / %s] no row in CSV", id, lang, op)
		return
	}
	if rec.Status != wantStatus {
		t.Errorf("[%s / %s / %s] status = %s, want %s (detail: %s)", id, lang, op, rec.Status, wantStatus, rec.Detail)
		return
	}
	if wantStatus == report.StatusFail && rec.Detail == "" {
		t.Errorf("[%s / %s / %s] FAIL carries no diagnostic detail", id, lang, op)
	}
	if wantDetail != nil && rec.Detail != *wantDetail {
		t.Errorf("[%s / %s / %s] detail = %q, want %q", id, lang, op, rec.Detail, *wantDetail)
	}
}
