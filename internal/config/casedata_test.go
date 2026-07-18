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

package config

import (
	"math/big"
	"path/filepath"
	"strings"
	"testing"
)

// loadOneCase writes a single-type file and returns its first case's data.
func loadOneCase(t *testing.T, schema, data string) (map[string]any, error) {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "t.yaml"),
		"formats: [binary]\nfields:\n"+schema+"cases:\n  - name: c\n    data:\n"+data)
	cf, err := LoadCases(filepath.Join(dir, "t.yaml"))
	if err != nil {
		return nil, err
	}
	return cf.Cases[0].Data, nil
}

func TestCaseData_BareBigIntsDecodeExactly(t *testing.T) {
	// Unquoted 128-bit literals used to degrade to float64 via yaml.v3's
	// type guessing; schema-directed decoding must keep them exact.
	data, err := loadOneCase(t,
		"  - amount: uint128\n  - debit: int128\n  - count: uint64\n  - ts: int64\n",
		"      amount: 340282366920938463463374607431768211455\n"+
			"      debit: -170141183460469231731687303715884105728\n"+
			"      count: 18446744073709551615\n"+
			"      ts: -9223372036854775808\n")
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}

	if got := data["amount"].(*big.Int).String(); got != "340282366920938463463374607431768211455" {
		t.Errorf("uint128 = %s, precision lost", got)
	}
	if got := data["debit"].(*big.Int).String(); got != "-170141183460469231731687303715884105728" {
		t.Errorf("int128 = %s, precision lost", got)
	}
	if got := data["count"].(uint64); got != 18446744073709551615 {
		t.Errorf("uint64 = %d", got)
	}
	if got := data["ts"].(int64); got != -9223372036854775808 {
		t.Errorf("int64 = %d", got)
	}
}

func TestCaseData_QuotedStringsStillAccepted(t *testing.T) {
	data, err := loadOneCase(t,
		"  - amount: uint128\n  - count: uint64\n",
		"      amount: \"1180591620717411303424\"\n      count: \"42\"\n")
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}
	if got := data["amount"].(*big.Int).String(); got != "1180591620717411303424" {
		t.Errorf("uint128 = %s", got)
	}
	if got := data["count"].(uint64); got != 42 {
		t.Errorf("uint64 = %d", got)
	}
}

func TestCaseData_FloatFormsRejectedForIntFields(t *testing.T) {
	for _, bad := range []string{"1e3", "1.5"} {
		_, err := loadOneCase(t, "  - count: uint64\n", "      count: "+bad+"\n")
		if err == nil {
			t.Errorf("%s into uint64 should be rejected", bad)
		} else if !strings.Contains(err.Error(), "expected an integer") {
			t.Errorf("%s: unexpected error: %v", bad, err)
		}
	}
}

func TestCaseData_OutOfRangeRejected(t *testing.T) {
	// 2^64 into uint64: yaml.v3's own decode silently produces a wrapped
	// float-derived value here — the schema-directed path must error.
	tests := []struct{ schema, value string }{
		{"  - count: uint64\n", "      count: 18446744073709551616\n"},
		{"  - ts: int64\n", "      ts: 9223372036854775808\n"},
		{"  - amount: uint128\n", "      amount: -1\n"},
		{"  - amount: uint128\n", "      amount: 340282366920938463463374607431768211456\n"},
		{"  - debit: int128\n", "      debit: 170141183460469231731687303715884105728\n"},
	}
	for _, tc := range tests {
		_, err := loadOneCase(t, tc.schema, tc.value)
		if err == nil || !strings.Contains(err.Error(), "out of range") {
			t.Errorf("schema %q value %q: want out-of-range error, got %v",
				strings.TrimSpace(tc.schema), strings.TrimSpace(tc.value), err)
		}
	}
}

func TestCaseData_ContainersRecurse(t *testing.T) {
	data, err := loadOneCase(t,
		"  - ids: list<uint128>\n  - balances: map<string,int128>\n  - memo: optional<uint64>\n",
		"      ids: [1180591620717411303424, \"7\"]\n"+
			"      balances: { a: -1180591620717411303424 }\n"+
			"      memo: 18446744073709551615\n")
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}

	ids := data["ids"].([]any)
	if got := ids[0].(*big.Int).String(); got != "1180591620717411303424" {
		t.Errorf("list<uint128>[0] = %s", got)
	}
	if got := ids[1].(*big.Int).String(); got != "7" {
		t.Errorf("list<uint128>[1] = %s", got)
	}
	if got := data["balances"].(map[string]any)["a"].(*big.Int).String(); got != "-1180591620717411303424" {
		t.Errorf("map value = %s", got)
	}
	if got := data["memo"].(uint64); got != 18446744073709551615 {
		t.Errorf("optional<uint64> = %d", got)
	}
}

func TestCaseData_StructFieldsRecurse(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "wallet.yaml"), "fields:\n  - balance: int128\n")
	mustWrite(t, filepath.Join(dir, "t.yaml"),
		"import:\n  - wallet.yaml\nformats: [binary]\n"+
			"fields:\n  - w: wallet\n"+
			"cases:\n  - name: c\n    data:\n      w: { balance: 1180591620717411303424 }\n")
	cf, err := LoadCases(filepath.Join(dir, "t.yaml"))
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}
	w := cf.Cases[0].Data["w"].(map[string]any)
	if got := w["balance"].(*big.Int).String(); got != "1180591620717411303424" {
		t.Errorf("nested struct int128 = %s", got)
	}
}

func TestCaseData_NullAndOtherKindsUnchanged(t *testing.T) {
	data, err := loadOneCase(t,
		"  - memo: optional<uint64>\n  - score: float64\n  - raw: bytes\n  - name: string\n  - pin: array<uint8,2>\n",
		"      memo: null\n      score: .nan\n      raw: [0xde, 0xad]\n      name: \"x\"\n      pin: [1, 2]\n")
	if err != nil {
		t.Fatalf("LoadCases: %v", err)
	}
	if v, present := data["memo"]; !present || v != nil {
		t.Errorf("optional null: present=%v v=%v", present, v)
	}
	if f, ok := data["score"].(float64); !ok || f == f { // NaN != NaN
		t.Errorf("float64 .nan lost: %v", data["score"])
	}
	if _, ok := data["raw"].([]any); !ok {
		t.Errorf("bytes array form changed type: %T", data["raw"])
	}
	if data["name"] != "x" {
		t.Errorf("string = %v", data["name"])
	}
}
