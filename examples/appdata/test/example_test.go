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

package example

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/lang"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// TestExamples_Customer asserts that every available language agrees on
// `customer` — the only type with two formats (a hand-written `binary` layout
// and a `json` one through each language's own encoder) and the only one with
// nested structs. Both formats declare `oracle: semantic`, since a byte oracle
// would make every worker reproduce Go's HTML escaping.
func TestExamples_Customer(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, lang.Go)

	grid := allWorkersGrid(t)
	for _, lang := range availableLangs {
		for _, op := range []string{"serialize", "deserialize"} {
			for _, format := range []string{"binary", "json"} {
				testutil.AssertCell(t, grid, "customer/"+format+"/typical", lang, op, report.StatusPass, nil)
				testutil.AssertCell(t, grid, "customer/"+format+"/new_account", lang, op, report.StatusPass, nil)
				testutil.AssertCell(t, grid, "customer/"+format+"/unicode", lang, op, report.StatusPass, nil)
				testutil.AssertCell(t, grid, "customer/"+format+"/boundary", lang, op, report.StatusPass, nil)
				testutil.AssertCell(t, grid, "customer/"+format+"/text_escapes", lang, op, report.StatusPass, nil)
			}
		}
	}
}

// allWorkersGrid runs the full suite across every available worker exactly once
// and caches the result grid.
//
// Every TestExamples_* issues the identical command and differs only in which
// cells it asserts; a run each crosses go test's 10-minute limit.
var (
	sharedGridOnce sync.Once
	sharedGrid     testutil.ResultGrid
	sharedGridErr  string
)

func allWorkersGrid(t *testing.T) testutil.ResultGrid {
	t.Helper()
	sharedGridOnce.Do(func() {
		repoRoot := testutil.RepoRoot()
		casesDir := filepath.Join(repoRoot, "examples", "appdata", "cases")
		csv := filepath.Join(os.TempDir(), "serify-examples-shared.csv")

		// --expect-skips makes this a coverage assertion too: a SKIP is
		// exit-code-neutral, so any skip not declared in expected_skips/ fails
		// the run. Only languages actually in the run have their file read.
		args := []string{
			"run", "--ref", lang.Go, "--cases", casesDir, "--csv", csv,
			"--expect-skips", filepath.Join(casesDir, "expected_skips"),
		}
		for _, lang := range availableLangs {
			args = append(args, filepath.Join(repoRoot, "examples", "appdata", lang))
		}
		out, code := testutil.RunSerify(t, args...)
		if code != 0 {
			sharedGridErr = fmt.Sprintf("expected exit 0, got %d\n%s", code, out)
			return
		}
		sharedGrid = testutil.ReadResultGrid(t, csv)
	})
	if sharedGridErr != "" {
		require.Fail(t, sharedGridErr)
	}
	return sharedGrid
}

// TestExamples_Ledger asserts that every available language agrees byte-for-byte
// on `ledger`, whose i128_boundaries case carries ±2^127 and so only passes with
// true 128-bit support in the library.
func TestExamples_Ledger(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, lang.Go)

	grid := allWorkersGrid(t)
	for _, lang := range availableLangs {
		for _, op := range []string{"serialize", "deserialize"} {
			testutil.AssertCell(t, grid, "ledger/binary/i128_boundaries", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "ledger/binary/deposit", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "ledger/binary/genesis", lang, op, report.StatusPass, nil)
		}
	}
}

// TestExamples_Notification asserts that every available language agrees
// byte-for-byte on `notification`, the suite's `sum` type. Its four cases cover
// every payload arity: a unit variant, a scalar payload, a u64 payload that must
// survive as a decimal string, and a struct payload.
func TestExamples_Notification(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, lang.Go)

	grid := allWorkersGrid(t)
	for _, lang := range availableLangs {
		for _, op := range []string{"serialize", "deserialize"} {
			testutil.AssertCell(t, grid, "notification/binary/unit_variant", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "notification/binary/scalar_payload", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "notification/binary/u64_payload", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "notification/binary/struct_payload", lang, op, report.StatusPass, nil)
		}
	}
}

// TestExamples_Signals asserts that every available language agrees byte-for-byte
// on `signals`, whose sixteen fields between them use every scalar the schema
// allows as a list element. `serify validate` cannot catch a library that drops
// one, because the runner's encoder is generic — only a worker run can.
//
// empty_lists is not filler: a list's u32 count prefix must be written even when
// there are no elements. The type also carries the suite's only all-nine
// `optional<scalar>` (dropped_frames) and `enum` (mode).
func TestExamples_Signals(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, lang.Go)

	grid := allWorkersGrid(t)
	for _, lang := range availableLangs {
		for _, op := range []string{"serialize", "deserialize"} {
			testutil.AssertCell(t, grid, "signals/binary/typical", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "signals/binary/empty_lists", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "signals/binary/width_boundaries", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "signals/binary/float_extremes", lang, op, report.StatusPass, nil)
		}
	}
}

// TestExamples_Telemetry asserts agreement on `telemetry`, which carries the
// only NaN/Inf `optional<float32>` (`humidity_pct`) plus the suite's only
// `uint128`, two differently shaped fixed arrays and a map<string,uint64>.
//
// Elixir is excluded permanently: the BEAM has no NaN and no infinity, so
// float_nan and float_inf are unrepresentable there (see expected_skips/).
func TestExamples_Telemetry(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, lang.Go)

	grid := allWorkersGrid(t)
	for _, l := range availableLangs {
		if l == lang.Elixir {
			continue
		}
		for _, op := range []string{"serialize", "deserialize"} {
			// humidity_pct is present in nominal and null in zero, so both
			// sides of the optional are compared byte-for-byte.
			testutil.AssertCell(t, grid, "telemetry/binary/nominal", l, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/zero", l, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/float_nan", l, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/float_inf", l, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/float_neg_zero", l, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/int_boundaries", l, op, report.StatusPass, nil)
		}
	}
}

// TestExamples_Order asserts that every available language agrees on `order`,
// the only type combining an `enum`, a `list<struct>`, a `map<string,struct>`
// and an `optional<struct>` — and, through line_item's own `unit_price`, the
// only struct nested inside a struct.
func TestExamples_Order(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, lang.Go)

	grid := allWorkersGrid(t)
	for _, lang := range availableLangs {
		for _, op := range []string{"serialize", "deserialize"} {
			testutil.AssertCell(t, grid, "order/binary/paid", lang, op, report.StatusPass, nil)
			// empty_draft nulls both optionals and empties both collections, so
			// the absent side of each is compared as well as the present one.
			testutil.AssertCell(t, grid, "order/binary/empty_draft", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "order/binary/boundary", lang, op, report.StatusPass, nil)
		}
	}
}

// TestExamples_Audit runs --audit over the same worker set.
func TestExamples_Audit(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, lang.Go)

	repoRoot := testutil.RepoRoot()
	var workerDirs []string
	for _, lang := range availableLangs {
		if _, ok := missingLang[lang]; !ok {
			workerDirs = append(workerDirs, filepath.Join(repoRoot, "examples", "appdata", lang))
		}
	}

	casesDir := filepath.Join(repoRoot, "examples", "appdata", "cases")
	csv := filepath.Join(t.TempDir(), "out.csv")

	args := []string{"run", "--ref", lang.Go, "--cases", casesDir, "--csv", csv, "--audit"}
	args = append(args, workerDirs...)

	out, code := testutil.RunSerify(t, args...)
	_ = out
	t.Logf("audit exit code = %d", code)
}
