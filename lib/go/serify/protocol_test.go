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

package serify

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
)

func TestProtocol_Scalars(t *testing.T) {
	cases := []struct {
		typ  string
		send any // JSON value sent by runner
		want any // Go value after decode
	}{
		{"uint8", 200.0, uint8(200)},
		{"uint16", 60000.0, uint16(60000)},
		{"uint32", 4294967295.0, uint32(4294967295)},
		{"int8", -128.0, int8(-128)},
		{"int16", -32768.0, int16(-32768)},
		{"int32", float64(math.MinInt32), int32(math.MinInt32)},
		{"bool", true, true},
		{"bool", false, false},
		{"string", "hello", "hello"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s=%v", tc.typ, tc.send), func(t *testing.T) {
			sc := []SchemaField{{Name: "v", Type: tc.typ}}
			fm := mustDecode(t, mkRaw(t, "v", tc.send), sc)
			enc := mustEncode(t, fm, sc)
			if enc["v"] != tc.want {
				t.Errorf("want %v (%T), got %v (%T)", tc.want, tc.want, enc["v"], enc["v"])
			}
		})
	}
}

func TestProtocol_U64_Max(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "uint64"}}
	raw := mkRaw(t, "v", "18446744073709551615")
	fm := mustDecode(t, raw, sc)
	v, err := fm.GetU64("v")
	if err != nil || v != math.MaxUint64 {
		t.Fatalf("GetU64: %v %v", v, err)
	}
	if enc := mustEncode(t, fm, sc); enc["v"] != "18446744073709551615" {
		t.Errorf("encode: %v", enc["v"])
	}
}

func TestProtocol_U128_As_U64(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "uint128"}}
	raw := mkRaw(t, "v", "12345678901234567890")
	fm := mustDecode(t, raw, sc)
	if enc := mustEncode(t, fm, sc); enc["v"] != "12345678901234567890" {
		t.Errorf("uint128 roundtrip: %v", enc["v"])
	}
}

func TestProtocol_I64_Min(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "int64"}}
	raw := mkRaw(t, "v", "-9223372036854775808")
	fm := mustDecode(t, raw, sc)
	v, err := fm.GetI64("v")
	if err != nil || v != math.MinInt64 {
		t.Fatalf("GetI64: %v %v", v, err)
	}
	if enc := mustEncode(t, fm, sc); enc["v"] != "-9223372036854775808" {
		t.Errorf("encode int64 min: %v", enc["v"])
	}
}

func TestProtocol_F32(t *testing.T) {
	for _, want := range []float32{0, 1, -1, 3.14, float32(math.Inf(1)), float32(math.Inf(-1))} {
		t.Run(fmt.Sprintf("%v", want), func(t *testing.T) {
			sc := []SchemaField{{Name: "v", Type: "float32"}}
			h := f32hex(want)
			fm := mustDecode(t, mkRaw(t, "v", h), sc)
			got, err := fm.GetF32("v")
			if err != nil {
				t.Fatalf("GetF32: %v", err)
			}
			switch {
			case math.IsInf(float64(want), 1) && !math.IsInf(float64(got), 1):
				t.Errorf("+Inf: got %v", got)
			case math.IsInf(float64(want), -1) && !math.IsInf(float64(got), -1):
				t.Errorf("-Inf: got %v", got)
			case !math.IsInf(float64(want), 0) && math.Abs(float64(got-want)) > 1e-5:
				t.Errorf("want %.6g, got %.6g", want, got)
			}
			enc := mustEncode(t, fm, sc)
			if enc["v"] != h {
				t.Errorf("encode: got %v want %v", enc["v"], h)
			}
		})
	}
}

func TestProtocol_F64(t *testing.T) {
	for _, want := range []float64{0, 1, -1, math.Pi, math.Inf(1), math.Inf(-1)} {
		t.Run(fmt.Sprintf("%v", want), func(t *testing.T) {
			sc := []SchemaField{{Name: "v", Type: "float64"}}
			h := f64hex(want)
			fm := mustDecode(t, mkRaw(t, "v", h), sc)
			got, err := fm.GetF64("v")
			if err != nil {
				t.Fatalf("GetF64: %v", err)
			}
			switch {
			case math.IsInf(want, 1) && !math.IsInf(got, 1):
				t.Errorf("+Inf: got %v", got)
			case math.IsInf(want, -1) && !math.IsInf(got, -1):
				t.Errorf("-Inf: got %v", got)
			case !math.IsInf(want, 0) && math.Abs(got-want) > 1e-15:
				t.Errorf("want %.15g, got %.15g", want, got)
			}
			if enc := mustEncode(t, fm, sc); enc["v"] != h {
				t.Errorf("encode: got %v want %v", enc["v"], h)
			}
		})
	}
}

func TestProtocol_Bytes(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "bytes"}}
	raw := mkRaw(t, "v", "deadbeef")
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetBytes("v")
	if err != nil || hex.EncodeToString(got) != "deadbeef" {
		t.Fatalf("GetBytes: %v %v", got, err)
	}
	if enc := mustEncode(t, fm, sc); enc["v"] != "deadbeef" {
		t.Errorf("encode: %v", enc["v"])
	}
}

func TestProtocol_Bytes_Empty(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "bytes"}}
	raw := mkRaw(t, "v", "")
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetBytes("v")
	if err != nil || len(got) != 0 {
		t.Errorf("empty bytes: %v %v", got, err)
	}
}

func TestProtocol_String_Unicode(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "string"}}
	for _, s := range []string{"", "hello", "ä¸\u00adæ–‡ç”¨æˆ·ðŸ˜‰", "Ã±oÃ±o", "æ—¥æœ¬èªžãƒ†ã‚¹ãƒˆ"} {
		fm := mustDecode(t, mkRaw(t, "v", s), sc)
		if got, err := fm.GetString("v"); err != nil || got != s {
			t.Errorf("string %q: got %q %v", s, got, err)
		}
	}
}

func TestProtocol_Optional_Null(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "optional<string>"}}
	raw := map[string]json.RawMessage{"v": json.RawMessage("null")}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalString("v")
	if err != nil || got != nil {
		t.Errorf("optional null: %v %v", got, err)
	}
	enc := mustEncode(t, fm, sc)
	if enc["v"] != nil {
		t.Errorf("encode null: %v", enc["v"])
	}
}

func TestProtocol_Optional_Present(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "optional<string>"}}
	raw := mkRaw(t, "v", "world")
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalString("v")
	if err != nil || got == nil || *got != "world" {
		t.Errorf("optional present: %v %v", got, err)
	}
	if enc := mustEncode(t, fm, sc); enc["v"] != "world" {
		t.Errorf("encode present: %v", enc["v"])
	}
}

func TestProtocol_Array_U32_RoundTrip(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "array<uint32,4>"}}
	raw := map[string]json.RawMessage{
		"v": json.RawMessage(`[4294967295,0,1,2]`),
	}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListU32("v")
	if err != nil {
		t.Fatalf("GetListU32: %v", err)
	}
	if len(got) != 4 || got[0] != 0xFFFFFFFF || got[1] != 0 || got[2] != 1 || got[3] != 2 {
		t.Errorf("got %v", got)
	}
	enc := mustEncode(t, fm, sc)
	arr, ok := enc["v"].([]any)
	if !ok || len(arr) != 4 || arr[0] != uint32(0xFFFFFFFF) {
		t.Errorf("encode array: %#v", enc["v"])
	}
}

func TestProtocol_Array_U32_Numbers(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "array<uint32,4>"}}
	raw := map[string]json.RawMessage{
		"v": json.RawMessage(`[100, 200, 300, 400]`),
	}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListU32("v")
	if err != nil || len(got) != 4 || got[0] != 100 || got[3] != 400 {
		t.Errorf("numeric array: %v %v", got, err)
	}
}

func TestProtocol_ListString(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<string>"}}
	raw := mkRaw(t, "v", []string{"alpha", "beta", "ÃŽÂ³ÃŽÂ´ÃŽÂµ"})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListString("v")
	if err != nil || len(got) != 3 || got[2] != "ÃŽÂ³ÃŽÂ´ÃŽÂµ" {
		t.Errorf("ListString: %v %v", got, err)
	}
}

func TestProtocol_ListString_Empty(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<string>"}}
	raw := mkRaw(t, "v", []string{})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListString("v")
	if err != nil || len(got) != 0 {
		t.Errorf("empty ListString: %v %v", got, err)
	}
}

func TestProtocol_ListU32(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<uint32>"}}
	raw := mkRaw(t, "v", []float64{0, 1, 4294967295})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListU32("v")
	if err != nil || len(got) != 3 || got[2] != 0xFFFFFFFF {
		t.Errorf("ListU32: %v %v", got, err)
	}
}

func TestProtocol_ListU64(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<uint64>"}}
	raw := map[string]json.RawMessage{
		"v": json.RawMessage(`["18446744073709551615","0","1"]`),
	}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListU64("v")
	if err != nil || len(got) != 3 || got[0] != math.MaxUint64 {
		t.Errorf("ListU64: %v %v", got, err)
	}
}

func TestProtocol_ListI64(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<int64>"}}
	raw := map[string]json.RawMessage{
		"v": json.RawMessage(`["-9223372036854775808","0","9223372036854775807"]`),
	}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListI64("v")
	if err != nil || len(got) != 3 || got[0] != math.MinInt64 || got[2] != math.MaxInt64 {
		t.Errorf("ListI64: %v %v", got, err)
	}
}

func TestProtocol_ListF32(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<float32>"}}
	hexes := []string{f32hex(1.5), f32hex(-2.0), f32hex(0)}
	raw := map[string]json.RawMessage{
		"v": func() json.RawMessage { b, _ := json.Marshal(hexes); return b }(),
	}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListF32("v")
	if err != nil || len(got) != 3 || math.Abs(float64(got[0]-1.5)) > 1e-6 || math.Abs(float64(got[1]+2.0)) > 1e-6 {
		t.Errorf("ListF32: %v %v", got, err)
	}
}

func TestProtocol_ListBool(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<bool>"}}
	raw := mkRaw(t, "v", []bool{true, false, true})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListBool("v")
	if err != nil || len(got) != 3 || !got[0] || got[1] || !got[2] {
		t.Errorf("ListBool: %v %v", got, err)
	}
}

func TestProtocol_Struct(t *testing.T) {
	addrFields := []SchemaField{
		{Name: "street", Type: "string"},
		{Name: "zip", Type: "uint32"},
	}
	sc := []SchemaField{{Name: "addr", Type: "struct", Fields: addrFields}}
	raw := map[string]json.RawMessage{
		"addr": json.RawMessage(`{"street":"Main St","zip":90210}`),
	}
	fm := mustDecode(t, raw, sc)
	addr, err := fm.GetStruct("addr")
	if err != nil || addr == nil {
		t.Fatalf("GetStruct: %v %v", addr, err)
	}
	if s, _ := addr.GetString("street"); s != "Main St" {
		t.Errorf("street: %q", s)
	}
	if z, _ := addr.GetU32("zip"); z != 90210 {
		t.Errorf("zip: %d", z)
	}

	enc := mustEncode(t, fm, sc)
	addrMap, ok := enc["addr"].(map[string]any)
	if !ok {
		t.Fatalf("addr type: %T", enc["addr"])
	}
	if addrMap["street"] != "Main St" {
		t.Errorf("encoded street: %v", addrMap["street"])
	}
	if addrMap["zip"] != uint32(90210) {
		t.Errorf("encoded zip: %v (%T)", addrMap["zip"], addrMap["zip"])
	}
}

func TestProtocol_Struct_U32Boundary(t *testing.T) {
	addrFields := []SchemaField{{Name: "zip", Type: "uint32"}}
	sc := []SchemaField{{Name: "addr", Type: "struct", Fields: addrFields}}
	raw := map[string]json.RawMessage{
		"addr": json.RawMessage(`{"zip":4294967295}`),
	}
	fm := mustDecode(t, raw, sc)
	addr, _ := fm.GetStruct("addr")
	if z, _ := addr.GetU32("zip"); z != 0xFFFFFFFF {
		t.Errorf("uint32 max in struct: %d", z)
	}
	enc := mustEncode(t, fm, sc)
	addrMap := enc["addr"].(map[string]any)
	if addrMap["zip"] != uint32(0xFFFFFFFF) {
		t.Errorf("encoded uint32 max: %v (%T)", addrMap["zip"], addrMap["zip"])
	}
}

func TestProtocol_ListStruct(t *testing.T) {
	inner := []SchemaField{{Name: "n", Type: "uint32"}}
	sc := []SchemaField{{Name: "items", Type: "list<struct>", Fields: inner}}
	raw := map[string]json.RawMessage{
		"items": json.RawMessage(`[{"n":1},{"n":2},{"n":3}]`),
	}
	fm := mustDecode(t, raw, sc)
	items, err := fm.GetListStruct("items")
	if err != nil || len(items) != 3 {
		t.Fatalf("ListStruct decode: %v %v", items, err)
	}
	if v, _ := items[2].GetU32("n"); v != 3 {
		t.Errorf("items[2].n: %d", v)
	}

	enc := mustEncode(t, fm, sc)
	list, ok := enc["items"].([]any)
	if !ok || len(list) != 3 {
		t.Fatalf("ListStruct encode: %v", enc["items"])
	}
	m := list[0].(map[string]any)
	if m["n"] != uint32(1) {
		t.Errorf("items[0].n: %v (%T)", m["n"], m["n"])
	}
}

func TestProtocol_OptionalStruct_Null(t *testing.T) {
	inner := []SchemaField{{Name: "x", Type: "uint32"}}
	sc := []SchemaField{{Name: "s", Type: "optional<struct>", Fields: inner}}
	raw := map[string]json.RawMessage{"s": json.RawMessage("null")}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalStruct("s")
	if !errors.Is(err, ErrNilField) || got != nil {
		t.Errorf("optional[struct] null: %v %v", got, err)
	}
	enc := mustEncode(t, fm, sc)
	if enc["s"] != nil {
		t.Errorf("encode null struct: %v", enc["s"])
	}
}

func TestProtocol_OptionalStruct_Present(t *testing.T) {
	inner := []SchemaField{{Name: "x", Type: "uint32"}}
	sc := []SchemaField{{Name: "s", Type: "optional<struct>", Fields: inner}}
	raw := map[string]json.RawMessage{"s": json.RawMessage(`{"x":42}`)}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalStruct("s")
	if err != nil || got == nil {
		t.Fatalf("optional[struct] present: %v %v", got, err)
	}
	if v, _ := got.GetU32("x"); v != 42 {
		t.Errorf("s.x: %d", v)
	}
}

func TestProtocol_MissingField_Skipped(t *testing.T) {
	sc := []SchemaField{
		{Name: "a", Type: "uint32"},
		{Name: "b", Type: "string"},
	}
	raw := mkRaw(t, "a", 5.0) // b is absent
	fm := mustDecode(t, raw, sc)
	if v, _ := fm.GetU32("a"); v != 5 {
		t.Errorf("a: %d", v)
	}
	if _, err := fm.GetString("b"); err == nil {
		t.Error("expected not-found error for absent b")
	}
}

func TestProtocol_UnknownType_Error(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "uuid"}}
	raw := mkRaw(t, "v", "something")
	_, err := DecodeFieldMap(raw, sc)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestProtocol_NestedStruct_MultiLevel(t *testing.T) {
	innerFields := []SchemaField{{Name: "value", Type: "uint64"}}
	outerFields := []SchemaField{{Name: "inner", Type: "struct", Fields: innerFields}}
	sc := []SchemaField{{Name: "outer", Type: "struct", Fields: outerFields}}

	raw := map[string]json.RawMessage{
		"outer": json.RawMessage(`{"inner":{"value":"9999999999999999999"}}`),
	}
	fm := mustDecode(t, raw, sc)
	outer, _ := fm.GetStruct("outer")
	inner, _ := outer.GetStruct("inner")
	v, err := inner.GetU64("value")
	if err != nil || v != 9999999999999999999 {
		t.Errorf("deep nested uint64: %v %v", v, err)
	}
}

// TestProtocol_ListEveryScalarElem pins the invariant that a list supports every
// element type a bare field does. Before decodeList routed through decodeScalar
// it carried its own switch, and uint16/int8/int16/float64/bytes were missing
// from it — declarable in a case file, accepted by `serify validate`, and only
// failing once a worker actually ran. Adding a scalar to typekind without
// teaching decodeList about it used to be silently possible; now it cannot be.
func TestProtocol_ListEveryScalarElem(t *testing.T) {
	cases := []struct {
		elem string
		send any // the JSON array the runner sends
		want any // the Go slice the FieldMap should hold
	}{
		{"uint8", []any{0, 255}, []uint8{0, 255}},
		{"uint16", []any{0, 65535}, []uint16{0, 65535}},
		{"uint32", []any{0, 4294967295}, []uint32{0, 4294967295}},
		{"uint64", []any{"0", "18446744073709551615"}, []uint64{0, math.MaxUint64}},
		{"int8", []any{-128, 127}, []int8{-128, 127}},
		{"int16", []any{-32768, 32767}, []int16{-32768, 32767}},
		{"int32", []any{-2147483648, 2147483647}, []int32{math.MinInt32, math.MaxInt32}},
		{"int64", []any{"-9223372036854775808", "0"}, []int64{math.MinInt64, 0}},
		{"float32", []any{f32hex(1.5), f32hex(-2)}, []float32{1.5, -2}},
		{"float64", []any{f64hex(1.5), f64hex(-2)}, []float64{1.5, -2}},
		{"bool", []any{true, false}, []bool{true, false}},
		{"string", []any{"a", ""}, []string{"a", ""}},
		{"bytes", []any{"dead", ""}, [][]byte{{0xde, 0xad}, {}}},
	}

	for _, c := range cases {
		t.Run(c.elem, func(t *testing.T) {
			sc := []SchemaField{{Name: "v", Type: "list<" + c.elem + ">"}}
			raw := mkRaw(t, "v", c.send)

			fm := mustDecode(t, raw, sc)
			got, ok := fm.fields["v"]
			if !ok {
				t.Fatal("field not decoded")
			}
			if fmt.Sprintf("%v (%T)", got, got) != fmt.Sprintf("%v (%T)", c.want, c.want) {
				t.Fatalf("decode: got %v (%T), want %v (%T)", got, got, c.want, c.want)
			}

			// Re-encoding must reproduce the wire form the runner sent, so the
			// two directions cannot drift apart for any element type.
			enc := mustEncode(t, fm, sc)
			gotJSON, err := json.Marshal(enc["v"])
			if err != nil {
				t.Fatalf("marshal encoded: %v", err)
			}
			wantJSON, err := json.Marshal(c.send)
			if err != nil {
				t.Fatalf("marshal sent: %v", err)
			}
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("re-encode: got %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

// list<uint128>/list<int128> carry *big.Int, so they are checked apart from the
// table above rather than compared by formatted value.
func TestProtocol_ListBigElems(t *testing.T) {
	for _, elem := range []string{"uint128", "int128"} {
		t.Run(elem, func(t *testing.T) {
			sc := []SchemaField{{Name: "v", Type: "list<" + elem + ">"}}
			raw := map[string]json.RawMessage{"v": json.RawMessage(`["170141183460469231731687303715884105727","0"]`)}

			fm := mustDecode(t, raw, sc)
			got, err := fm.GetListBig("v")
			if err != nil || len(got) != 2 {
				t.Fatalf("GetListBig: %v %v", got, err)
			}
			if got[0].String() != "170141183460469231731687303715884105727" || got[1].Sign() != 0 {
				t.Fatalf("decode: %v", got)
			}

			enc := mustEncode(t, fm, sc)
			b, _ := json.Marshal(enc["v"])
			if string(b) != `["170141183460469231731687303715884105727","0"]` {
				t.Errorf("re-encode: %s", b)
			}
		})
	}
}
