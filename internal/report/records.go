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
	"cmp"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
)

// Record is one flat result row — the canonical data layer. The CSV is a direct
// dump of these, and the terminal table is rendered from them, so inspecting the
// CSV verifies the run and the table is just a readable view of the same data.
type Record struct {
	TestID    string `json:"test_id"`
	Type      string `json:"type"`
	Format    string `json:"format"`
	Case      string `json:"case"`
	Language  string `json:"language"`
	Operation string `json:"operation"`
	Status    Status `json:"status"`
	Detail    string `json:"detail"`
}

// Column names shared by the CSV dump and the terminal table.
const (
	colTestID    = "test_id"
	colType      = "type"
	colFormat    = "format"
	colCase      = "case"
	colLanguage  = "language"
	colOperation = "operation"
	colStatus    = "status"
	colDetail    = "detail"
)

// csvHeader is the column order for both WriteCSV and ReadCSV.
var csvHeader = []string{
	colTestID,
	colType,
	colFormat,
	colCase,
	colLanguage,
	colOperation,
	colStatus,
	colDetail,
}

// testIDParts is the number of "/"-separated fields in a test ID
// ("type/format/case").
const testIDParts = 3

// tableFixedCols is the number of leading, left-aligned columns in the rendered
// table ("case id", "format", "operation") before the per-language columns.
const tableFixedCols = 3

// splitTestID3 splits "type/format/case" into its three parts; any other shape
// is an error.
func splitTestID3(tid string) (string, string, string, error) {
	parts := strings.SplitN(tid, "/", testIDParts)
	if len(parts) != testIDParts {
		return "", "", "", fmt.Errorf("invalid test id: %v", tid)
	}
	return parts[0], parts[1], parts[2], nil
}

// Records flattens the report into the canonical row list, ordered by TestIDs
// (type → case → format), then language (reference-first, then any extras such
// as matrix "src→dst" pseudo-languages), then operation.
//
//nolint:gocognit // flattens a 3-level result map with a multi-key sort; the nesting is the data shape
func (r *Report) Records() ([]Record, error) {
	rank := make(map[string]int, len(r.Languages))
	for i, l := range r.Languages {
		rank[l] = i
	}
	// Operations sort in this display order: the round-trip ops first, then the
	// audit checks.
	opDisplayOrder := []string{
		OpSerialize, OpDeserialize, OpMatrix,
		OpAuditMutation, OpAuditStability, OpAuditOutputZeroCopy,
		OpAuditZeroCopy, OpAuditInputMut, OpAuditDeserStability,
	}
	opOrder := make(map[string]int, len(opDisplayOrder))
	for i, op := range opDisplayOrder {
		opOrder[op] = i
	}

	var recs []Record
	for _, tid := range r.TestIDs {
		typ, format, caseName, err := splitTestID3(tid)
		if err != nil {
			return nil, fmt.Errorf("failed to split test ID: %w", err)
		}
		byLang := r.Results[tid]

		langs := slices.Collect(maps.Keys(byLang))
		slices.SortFunc(langs, func(a, b string) int {
			ra, oka := rank[a]
			rb, okb := rank[b]
			if oka != okb {
				// Known (reference set) languages first.
				if oka {
					return -1
				}
				return 1
			}
			if oka && ra != rb {
				return cmp.Compare(ra, rb)
			}
			return cmp.Compare(a, b)
		})

		for _, lang := range langs {
			byOp := byLang[lang]
			ops := slices.Collect(maps.Keys(byOp))
			slices.SortFunc(ops, func(a, b string) int {
				oa, oka := opOrder[a]
				ob, okb := opOrder[b]
				if oka && okb {
					return cmp.Compare(oa, ob)
				}
				if oka != okb {
					// Known operations first.
					if oka {
						return -1
					}
					return 1
				}
				return cmp.Compare(a, b)
			})
			for _, op := range ops {
				res := byOp[op]
				recs = append(recs, Record{
					TestID:    tid,
					Type:      typ,
					Format:    format,
					Case:      caseName,
					Language:  lang,
					Operation: op,
					Status:    res.Status,
					Detail:    res.Detail,
				})
			}
		}
	}
	return recs, nil
}

// WriteCSV writes records as CSV (header + one row each). encoding/csv quotes
// any commas/quotes/newlines in details (diffs).
func WriteCSV(w io.Writer, recs []Record) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, rec := range recs {
		if err := cw.Write([]string{
			rec.TestID, rec.Type, rec.Format, rec.Case,
			rec.Language, rec.Operation, string(rec.Status), rec.Detail,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ReadCSV parses a CSV previously written by WriteCSV (validates the header).
func ReadCSV(r io.Reader) ([]Record, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = len(csvHeader)
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("empty csv")
	}
	for i, h := range csvHeader {
		if rows[0][i] != h {
			return nil, fmt.Errorf("unexpected csv header: got %v, want %v", rows[0], csvHeader)
		}
	}
	recs := make([]Record, 0, len(rows)-1)
	for _, row := range rows[1:] {
		recs = append(recs, Record{
			TestID: row[0], Type: row[1], Format: row[2], Case: row[3],
			Language: row[4], Operation: row[5], Status: Status(row[6]), Detail: row[7],
		})
	}
	return recs, nil
}

// RenderTable renders the results grid from records: columns
// case id | format | operation | <language…>. Only serialize/deserialize rows
// form the grid (matrix rows live in the CSV only). Languages and groups are
// taken in first-appearance order, so a live run and a re-read CSV produce the
// same table. Rows are grouped by case (formats adjacent); the case id is shown
// once per case and the format once per (case, format).
//
//nolint:gocognit // grid layout: row/column grouping with per-cell fallbacks
func RenderTable(w io.Writer, recs []Record) error {
	type gkey struct{ caseID, format string }

	var langs []string
	seenLang := map[string]bool{}
	var order []gkey
	seenGroup := map[gkey]bool{}
	data := map[gkey]map[string]map[string]Status{} // group → op → lang → status

	for _, rec := range recs {
		if rec.Operation != OpSerialize && rec.Operation != OpDeserialize {
			continue
		}
		if !seenLang[rec.Language] {
			seenLang[rec.Language] = true
			langs = append(langs, rec.Language)
		}
		caseID := rec.Case
		if rec.Type != "" {
			caseID = rec.Type + "/" + rec.Case
		}
		k := gkey{caseID, rec.Format}
		if !seenGroup[k] {
			seenGroup[k] = true
			order = append(order, k)
			data[k] = map[string]map[string]Status{}
		}
		if data[k][rec.Operation] == nil {
			data[k][rec.Operation] = map[string]Status{}
		}
		data[k][rec.Operation][rec.Language] = rec.Status
	}

	headers := append([]string{"case id", colFormat, colOperation}, langs...)
	ops := []string{OpSerialize, OpDeserialize}
	var rows [][]string
	prevCase := ""
	for gi, k := range order {
		newCase := gi == 0 || k.caseID != prevCase
		prevCase = k.caseID
		for ri, op := range ops {
			row := make([]string, 0, tableFixedCols+len(langs))
			if ri == 0 && newCase {
				row = append(row, k.caseID)
			} else {
				row = append(row, "")
			}
			if ri == 0 {
				row = append(row, k.format)
			} else {
				row = append(row, "")
			}
			row = append(row, op)
			for _, lang := range langs {
				if st, ok := data[k][op][lang]; ok {
					row = append(row, statusColored(st))
				} else {
					row = append(row, "-")
				}
			}
			rows = append(rows, row)
		}
	}

	_, err := fmt.Fprintln(w, renderGrid(headers, rows, tableFixedCols))
	return err
}

// renderGrid renders a static lipgloss table. The first leftCols columns are
// left-aligned; the rest are centered.
func renderGrid(headers []string, rows [][]string, leftCols int) string {
	return lgtable.New().
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(_, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if col < leftCols {
				return s.Align(lipgloss.Left)
			}
			return s.Align(lipgloss.Center)
		}).
		Render()
}
