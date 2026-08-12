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

	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// TestExamples_Customer asserts that every available language agrees on
// `customer`, the only type in the suite carrying two formats. That is what
// makes it worth the size: `binary` is a layout each worker writes by hand,
// `json` goes through whatever encoder the language ships, and the two fail in
// completely different ways. Both declare `oracle: semantic`, so what has to
// match is the decoded value — under a byte oracle every worker would have to
// reproduce Go's HTML escaping to stay green.
//
// It is also the only type with nested structs, and porting it is what found
// the same dead nested-struct branch in four bindings at once: node, csharp,
// java and php each recognised a nested model by a duck-type no model has ever
// satisfied, so every one of those branches was unreachable and a nested struct
// went out stringified. text_escapes and boundary are the two cases that pin
// the encoders: the first carries the characters JSON encoders disagree about,
// the second max uint64 and min int64, which no JSON double can hold.
func TestExamples_Customer(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, "go")

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
// Every TestExamples_* above and below issues the identical command and differs
// only in which cells it asserts, so they used to pay for a complete 9-worker
// run each. The package crossed go test's 10-minute limit and the whole thing
// timed out; sharing one run is both the fix and what the tests always meant.
var (
	sharedGridOnce sync.Once
	sharedGrid     testutil.ResultGrid
	sharedGridErr  string
)

func allWorkersGrid(t *testing.T) testutil.ResultGrid {
	t.Helper()
	sharedGridOnce.Do(func() {
		repoRoot := testutil.RepoRoot()
		casesDir := filepath.Join(repoRoot, "examples", "cases")
		csv := filepath.Join(os.TempDir(), "serify-examples-shared.csv")

		// --expect-skips turns this run into a coverage assertion as well as a
		// parity one. A SKIP is exit-code-neutral, so a worker that quietly
		// stops registering a type reads exactly like one that never did; the
		// declarations in expected_skips/ say which gaps are known, and any
		// other skip now fails the run. Only languages actually in the run have
		// their file read, so a partial local run raises no stale warnings.
		args := []string{
			"run", "--ref", "go", "--cases", casesDir, "--csv", csv,
			"--expect-skips", filepath.Join(casesDir, "expected_skips"),
		}
		for _, lang := range availableLangs {
			args = append(args, filepath.Join(repoRoot, "examples", lang))
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
// on `ledger`, whose i128_boundaries case carries ±2^127. That case only passes
// with true 128-bit support in the library, and each language reaches it differently:
// native i128 (Rust, C#, C++), an arbitrary-precision integer (Python, Node,
// Elixir), a big-integer library (Go's math/big, Java's BigInteger), or decimal
// strings (PHP, whose int is only 64-bit).
func TestExamples_Ledger(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, "go")

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
// byte-for-byte on `notification`, the suite's `sum` type. Its four cases are
// the three payload arities the type system allows — a unit variant, a scalar
// payload, a u64 payload that must survive as a decimal string, and a struct
// payload — and each language reaches them through its own sum type: an enum
// (Rust), a sealed interface (Java), a union of dataclasses (Python), a property
// union (PHP), an abstract record hierarchy (C#), a tagged tuple (Elixir), a
// std::variant (C++), a declared arm list (Node), or a Converter (Go).
func TestExamples_Notification(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, "go")

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
// on `signals`, whose sixteen fields between them use **every scalar the schema
// allows as a list element**.
//
// This is the regression guard for a gap that was invisible for a long time:
// `list<T>` accepted only a subset of `T`, the subset differed per language, and
// `serify validate` accepted all of them because the runner's encoder is generic.
// So a case file could declare `list<float64>` or `list<int8>`, pass validation,
// and then fail — or, in Python, Java and C++, silently produce wrong or missing
// data — once a worker actually ran. Every element type is exercised here so a
// library that drops one fails in all nine languages at once.
//
// empty_lists is not filler: a list's u32 count prefix must be written even when
// there are no elements, and Java's model binding used to drop an empty list from
// the FieldMap entirely.
//
// It also carries the suite's only all-nine `optional<scalar>` (dropped_frames,
// an optional<uint32> — present in typical/width_boundaries/float_extremes, null
// in empty_lists). In the statically-typed bindings that is a distinct code path
// from optional<string> — a Go pointer arm, a Rust OptionScalar, an Elixir
// {:optional,_} clause, a C++ SERIFY_FIELD_OPTIONAL macro — and before this field
// no all-nine type exercised it, so Elixir's and C++'s were verified by nothing.
//
// It also carries the suite's only all-nine `enum` (mode, four variants). An enum
// binds as a plain string in every language — the variant name on the wire, the
// string↔ordinal mapping in the worker — so this needs no library support, but it
// was previously exercised only by `order`, which is Go-only.
func TestExamples_Signals(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, "go")

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
// only NaN/Inf `optional<float32>` (`humidity_pct`) plus its
// only `uint128`, two differently shaped fixed arrays, a map<string,uint64>, and
// float cases covering NaN, ±Inf and negative zero.
//
// It went unimplemented for a long time not through neglect but because no model
// binding could express an optional<scalar>: Go's reflection shim knew only
// *string and *big.Int, and Rust's derive only Option<String>. Both are generic
// now, and this is what keeps them that way.
//
// Elixir is the one language excluded, and permanently: the BEAM has no NaN and
// no infinity, so float_nan and float_inf are unrepresentable there. That gap is
// declared in expected_skips/elixir.yaml rather than hidden.
func TestExamples_Telemetry(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, "go")

	grid := allWorkersGrid(t)
	for _, lang := range availableLangs {
		if lang == "elixir" {
			continue
		}
		for _, op := range []string{"serialize", "deserialize"} {
			// humidity_pct is present in nominal and null in zero, so both
			// sides of the optional are compared byte-for-byte.
			testutil.AssertCell(t, grid, "telemetry/binary/nominal", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/zero", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/float_nan", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/float_inf", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/float_neg_zero", lang, op, report.StatusPass, nil)
			testutil.AssertCell(t, grid, "telemetry/binary/int_boundaries", lang, op, report.StatusPass, nil)
		}
	}
}

// TestExamples_Order asserts that every available language agrees on `order`,
// the only type combining an `enum`, a `list<struct>`, a `map<string,struct>`
// and an `optional<struct>` — and, through line_item's own `unit_price`, the
// only struct nested inside a struct.
//
// `billing_address` is the suite's only `optional<struct>`, and porting order is
// what exercised it for the first time in seven bindings: the node, csharp,
// java and php ones had to hand back a null rather than a model, and the C++
// one had no macro for the shape at all — SERIFY_FIELD_OPTIONAL covers an
// optional scalar and SERIFY_FIELD_STRUCT a required struct, and the FieldMap
// slot here is neither.
//
// order was the last gap: with this type in every worker, the only skip left in
// the suite is elixir/telemetry, which cannot exist.
func TestExamples_Order(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, "go")

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
	requireLang(t, "go")

	repoRoot := testutil.RepoRoot()
	var workerDirs []string
	for _, lang := range availableLangs {
		if _, ok := missingLang[lang]; !ok {
			workerDirs = append(workerDirs, filepath.Join(repoRoot, "examples", lang))
		}
	}

	casesDir := filepath.Join(repoRoot, "examples", "cases")
	csv := filepath.Join(t.TempDir(), "out.csv")

	args := []string{"run", "--ref", "go", "--cases", casesDir, "--csv", csv, "--audit"}
	args = append(args, workerDirs...)

	out, code := testutil.RunSerify(t, args...)
	_ = out
	t.Logf("audit exit code = %d", code)
}
