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

	"github.com/chengxilo/serify/internal/testutil"
)

// TestExamples_GoRust asserts real conformance for the workers that have been
// migrated to the current case suite: go and rust implement `customer` (binary +
// json) and `ledger` (binary). Unlike TestExamples_AllLanguages below, this one
// fails the build if the workers disagree — it is the test that actually guards
// the suite.
//
// `order` and `telemetry` are not implemented by either worker yet and are
// reported as SKIPPED, so the run still exits 0.
func TestExamples_GoRust(t *testing.T) {
	requireLang(t, "go")
	// Skipped rather than failed when rust is absent: this test is meaningful
	// only in an environment that has rust, and which languages an environment
	// must have is now declared by SERIFY_REQUIRE (see TestMain) rather than
	// hardcoded here. That lets CI run this package with no -run filter at all.
	skipUnless(t, "rust")

	repoRoot := testutil.RepoRoot()
	casesDir := filepath.Join(repoRoot, "examples", "cases")
	csv := filepath.Join(t.TempDir(), "out.csv")

	out, code := testutil.RunSerify(t, "run",
		"--ref", "go",
		"--cases", casesDir,
		"--csv", csv,
		filepath.Join(repoRoot, "examples", "go"),
		filepath.Join(repoRoot, "examples", "rust"),
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\n%s", code, out)
	}

	grid := testutil.ReadResultGrid(t, csv)
	for _, lang := range []string{"go", "rust"} {
		for _, op := range []string{"serialize", "deserialize"} {
			// customer: the json format is where encoder quirks (HTML escaping,
			// float formatting, base64) diverge between languages.
			testutil.AssertCell(t, grid, "customer/json/text_escapes", lang, op, "PASS")
			testutil.AssertCell(t, grid, "customer/json/boundary", lang, op, "PASS")
			testutil.AssertCell(t, grid, "customer/binary/typical", lang, op, "PASS")
		}
	}
}

// allWorkersGrid runs the full suite across every available worker exactly once
// and caches the result grid.
//
// TestExamples_Ledger, _Notification and _Signals issue the identical command
// and differ only in which cells they assert, so they used to pay for three
// complete 9-worker runs. With `telemetry` and `order` implemented the package
// crossed go test's 10-minute limit and the whole thing timed out; sharing one
// run is both the fix and what the tests always meant.
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

		args := []string{"run", "--ref", "go", "--cases", casesDir, "--csv", csv}
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
		t.Fatal(sharedGridErr)
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
			testutil.AssertCell(t, grid, "ledger/binary/i128_boundaries", lang, op, "PASS")
			testutil.AssertCell(t, grid, "ledger/binary/deposit", lang, op, "PASS")
			testutil.AssertCell(t, grid, "ledger/binary/genesis", lang, op, "PASS")
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
			testutil.AssertCell(t, grid, "notification/binary/unit_variant", lang, op, "PASS")
			testutil.AssertCell(t, grid, "notification/binary/scalar_payload", lang, op, "PASS")
			testutil.AssertCell(t, grid, "notification/binary/u64_payload", lang, op, "PASS")
			testutil.AssertCell(t, grid, "notification/binary/struct_payload", lang, op, "PASS")
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
			testutil.AssertCell(t, grid, "signals/binary/typical", lang, op, "PASS")
			testutil.AssertCell(t, grid, "signals/binary/empty_lists", lang, op, "PASS")
			testutil.AssertCell(t, grid, "signals/binary/width_boundaries", lang, op, "PASS")
			testutil.AssertCell(t, grid, "signals/binary/float_extremes", lang, op, "PASS")
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
func TestExamples_Telemetry(t *testing.T) {
	if len(availableLangs) == 0 {
		t.Skip("no example worker toolchains available")
	}
	requireLang(t, "go")

	grid := allWorkersGrid(t)
	for _, lang := range []string{"go", "rust"} {
		if _, missing := missingLang[lang]; missing {
			continue
		}
		for _, op := range []string{"serialize", "deserialize"} {
			// humidity_pct is present in nominal and null in zero, so both
			// sides of the optional are compared byte-for-byte.
			testutil.AssertCell(t, grid, "telemetry/binary/nominal", lang, op, "PASS")
			testutil.AssertCell(t, grid, "telemetry/binary/zero", lang, op, "PASS")
			testutil.AssertCell(t, grid, "telemetry/binary/float_nan", lang, op, "PASS")
			testutil.AssertCell(t, grid, "telemetry/binary/int_boundaries", lang, op, "PASS")
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
