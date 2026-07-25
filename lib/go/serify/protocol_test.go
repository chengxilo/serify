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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, tc.want, enc["v"], "want %v (%T), got %v (%T)", tc.want, tc.want, enc["v"], enc["v"])
		})
	}
}

func TestProtocol_U64_Max(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "uint64"}}
	raw := mkRaw(t, "v", "18446744073709551615")
	fm := mustDecode(t, raw, sc)
	v, err := fm.GetU64("v")
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), v)
	enc := mustEncode(t, fm, sc)
	assert.Equal(t, "18446744073709551615", enc["v"], "encode: %v", enc["v"])
}

func TestProtocol_U128_As_U64(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "uint128"}}
	raw := mkRaw(t, "v", "12345678901234567890")
	fm := mustDecode(t, raw, sc)
	enc := mustEncode(t, fm, sc)
	assert.Equal(t, "12345678901234567890", enc["v"], "uint128 roundtrip: %v", enc["v"])
}

func TestProtocol_I64_Min(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "int64"}}
	raw := mkRaw(t, "v", "-9223372036854775808")
	fm := mustDecode(t, raw, sc)
	v, err := fm.GetI64("v")
	require.NoError(t, err)
	require.Equal(t, int64(math.MinInt64), v)
	enc := mustEncode(t, fm, sc)
	assert.Equal(t, "-9223372036854775808", enc["v"], "encode int64 min: %v", enc["v"])
}

func TestProtocol_F32(t *testing.T) {
	for _, want := range []float32{0, 1, -1, 3.14, float32(math.Inf(1)), float32(math.Inf(-1))} {
		t.Run(fmt.Sprintf("%v", want), func(t *testing.T) {
			sc := []SchemaField{{Name: "v", Type: "float32"}}
			h := f32hex(want)
			fm := mustDecode(t, mkRaw(t, "v", h), sc)
			got, err := fm.GetF32("v")
			require.NoError(t, err)
			switch {
			case math.IsInf(float64(want), 1):
				assert.True(t, math.IsInf(float64(got), 1), "+Inf: got %v", got)
			case math.IsInf(float64(want), -1):
				assert.True(t, math.IsInf(float64(got), -1), "-Inf: got %v", got)
			default:
				assert.InDelta(t, float64(want), float64(got), 1e-5, "want %.6g, got %.6g", want, got)
			}
			enc := mustEncode(t, fm, sc)
			assert.Equal(t, h, enc["v"], "encode: got %v want %v", enc["v"], h)
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
			require.NoError(t, err)
			switch {
			case math.IsInf(want, 1):
				assert.True(t, math.IsInf(got, 1), "+Inf: got %v", got)
			case math.IsInf(want, -1):
				assert.True(t, math.IsInf(got, -1), "-Inf: got %v", got)
			default:
				assert.InDelta(t, want, got, 1e-15, "want %.15g, got %.15g", want, got)
			}
			enc := mustEncode(t, fm, sc)
			assert.Equal(t, h, enc["v"], "encode: got %v want %v", enc["v"], h)
		})
	}
}

func TestProtocol_Bytes(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "bytes"}}
	raw := mkRaw(t, "v", "deadbeef")
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetBytes("v")
	require.NoError(t, err)
	require.Equal(t, "deadbeef", hex.EncodeToString(got))
	enc := mustEncode(t, fm, sc)
	assert.Equal(t, "deadbeef", enc["v"], "encode: %v", enc["v"])
}

func TestProtocol_Bytes_Empty(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "bytes"}}
	raw := mkRaw(t, "v", "")
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetBytes("v")
	if err != nil || len(got) != 0 {
		assert.NoError(t, err, "empty bytes: %v %v", got, err)
		assert.Empty(t, got, "empty bytes: %v %v", got, err)
	}
}

func TestProtocol_String_Unicode(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "string"}}
	for _, s := range []string{"", "hello", "ä¸­æ–‡ç”¨æˆ·ðŸ˜‰", "Ã±oÃ±o", "æ—¥æœ¬èªžãƒ†ã‚¹ãƒˆ"} {
		fm := mustDecode(t, mkRaw(t, "v", s), sc)
		got, err := fm.GetString("v")
		if err != nil || got != s {
			assert.NoError(t, err, "string %q: got %q %v", s, got, err)
			assert.Equal(t, s, got, "string %q: got %q %v", s, got, err)
		}
	}
}

func TestProtocol_Optional_Null(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "optional<string>"}}
	raw := map[string]json.RawMessage{"v": json.RawMessage("null")}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalString("v")
	if err != nil || got != nil {
		assert.NoError(t, err, "optional null: %v %v", got, err)
		assert.Nil(t, got, "optional null: %v %v", got, err)
	}
	enc := mustEncode(t, fm, sc)
	if enc["v"] != nil {
		assert.Nil(t, enc["v"], "encode null: %v", enc["v"])
	}
}

func TestProtocol_Optional_Present(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "optional<string>"}}
	raw := mkRaw(t, "v", "world")
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalString("v")
	if err != nil || got == nil || *got != "world" {
		assert.NoError(t, err, "optional present: %v %v", got, err)
		if err == nil {
			require.NotNil(t, got, "optional present: %v %v", got, err)
			assert.Equal(t, "world", *got, "optional present: %v %v", got, err)
		}
	}
	enc := mustEncode(t, fm, sc)
	assert.Equal(t, "world", enc["v"], "encode present: %v", enc["v"])
}

func TestProtocol_Array_U32_RoundTrip(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "array<uint32,4>"}}
	raw := map[string]json.RawMessage{
		"v": json.RawMessage(`[4294967295,0,1,2]`),
	}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListU32("v")
	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, uint32(0xFFFFFFFF), got[0], "got %v", got)
	assert.Equal(t, uint32(0), got[1], "got %v", got)
	assert.Equal(t, uint32(1), got[2], "got %v", got)
	assert.Equal(t, uint32(2), got[3], "got %v", got)
	enc := mustEncode(t, fm, sc)
	arr, ok := enc["v"].([]any)
	if !ok || len(arr) != 4 || arr[0] != uint32(0xFFFFFFFF) {
		assert.True(t, ok, "encode array: %#v", enc["v"])
		assert.Len(t, arr, 4, "encode array: %#v", enc["v"])
		assert.Equal(t, uint32(0xFFFFFFFF), arr[0], "encode array: %#v", enc["v"])
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
		assert.NoError(t, err, "numeric array: %v %v", got, err)
		assert.Len(t, got, 4, "numeric array: %v %v", got, err)
		if len(got) == 4 {
			assert.Equal(t, uint32(100), got[0], "numeric array: %v %v", got, err)
			assert.Equal(t, uint32(400), got[3], "numeric array: %v %v", got, err)
		}
	}
}

func TestProtocol_ListString(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<string>"}}
	raw := mkRaw(t, "v", []string{"alpha", "beta", "ÃŽÂ³ÃŽÂ´ÃŽÂµ"})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListString("v")
	if err != nil || len(got) != 3 || got[2] != "ÃŽÂ³ÃŽÂ´ÃŽÂµ" {
		assert.NoError(t, err, "ListString: %v %v", got, err)
		assert.Len(t, got, 3, "ListString: %v %v", got, err)
		if len(got) == 3 {
			assert.Equal(t, "ÃŽÂ³ÃŽÂ´ÃŽÂµ", got[2], "ListString: %v %v", got, err)
		}
	}
}

func TestProtocol_ListString_Empty(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<string>"}}
	raw := mkRaw(t, "v", []string{})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListString("v")
	if err != nil || len(got) != 0 {
		assert.NoError(t, err, "empty ListString: %v %v", got, err)
		assert.Empty(t, got, "empty ListString: %v %v", got, err)
	}
}

func TestProtocol_ListU32(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<uint32>"}}
	raw := mkRaw(t, "v", []float64{0, 1, 4294967295})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListU32("v")
	if err != nil || len(got) != 3 || got[2] != 0xFFFFFFFF {
		assert.NoError(t, err, "ListU32: %v %v", got, err)
		assert.Len(t, got, 3, "ListU32: %v %v", got, err)
		if len(got) == 3 {
			assert.Equal(t, uint32(0xFFFFFFFF), got[2], "ListU32: %v %v", got, err)
		}
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
		assert.NoError(t, err, "ListU64: %v %v", got, err)
		assert.Len(t, got, 3, "ListU64: %v %v", got, err)
		if len(got) == 3 {
			assert.Equal(t, uint64(math.MaxUint64), got[0], "ListU64: %v %v", got, err)
		}
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
		assert.NoError(t, err, "ListI64: %v %v", got, err)
		assert.Len(t, got, 3, "ListI64: %v %v", got, err)
		if len(got) == 3 {
			assert.Equal(t, int64(math.MinInt64), got[0], "ListI64: %v %v", got, err)
			assert.Equal(t, int64(math.MaxInt64), got[2], "ListI64: %v %v", got, err)
		}
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
		assert.NoError(t, err, "ListF32: %v %v", got, err)
		assert.Len(t, got, 3, "ListF32: %v %v", got, err)
		if len(got) >= 2 {
			assert.InDelta(t, 1.5, got[0], 1e-6, "ListF32: %v %v", got, err)
			assert.InDelta(t, -2.0, got[1], 1e-6, "ListF32: %v %v", got, err)
		}
	}
}

func TestProtocol_ListBool(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "list<bool>"}}
	raw := mkRaw(t, "v", []bool{true, false, true})
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetListBool("v")
	if err != nil || len(got) != 3 || !got[0] || got[1] || !got[2] {
		assert.NoError(t, err, "ListBool: %v %v", got, err)
		assert.Len(t, got, 3, "ListBool: %v %v", got, err)
		if len(got) == 3 {
			assert.True(t, got[0], "ListBool: %v %v", got, err)
			assert.False(t, got[1], "ListBool: %v %v", got, err)
			assert.True(t, got[2], "ListBool: %v %v", got, err)
		}
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
	require.NoError(t, err)
	require.NotNil(t, addr)
	if s, _ := addr.GetString("street"); s != "Main St" {
		assert.Equal(t, "Main St", s, "street: %q", s)
	}
	if z, _ := addr.GetU32("zip"); z != 90210 {
		assert.Equal(t, uint32(90210), z, "zip: %d", z)
	}

	enc := mustEncode(t, fm, sc)
	addrMap, ok := enc["addr"].(map[string]any)
	require.True(t, ok, "addr type: %T", enc["addr"])
	if addrMap["street"] != "Main St" {
		assert.Equal(t, "Main St", addrMap["street"], "encoded street: %v", addrMap["street"])
	}
	if addrMap["zip"] != uint32(90210) {
		assert.Equal(t, uint32(90210), addrMap["zip"], "encoded zip: %v (%T)", addrMap["zip"], addrMap["zip"])
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
		assert.Equal(t, uint32(0xFFFFFFFF), z, "uint32 max in struct: %d", z)
	}
	enc := mustEncode(t, fm, sc)
	addrMap := enc["addr"].(map[string]any)
	if addrMap["zip"] != uint32(0xFFFFFFFF) {
		assert.Equal(t, uint32(0xFFFFFFFF), addrMap["zip"], "encoded uint32 max: %v (%T)", addrMap["zip"], addrMap["zip"])
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
	require.NoError(t, err)
	require.Len(t, items, 3)
	if v, _ := items[2].GetU32("n"); v != 3 {
		assert.Equal(t, uint32(3), v, "items[2].n: %d", v)
	}

	enc := mustEncode(t, fm, sc)
	list, ok := enc["items"].([]any)
	require.True(t, ok, "ListStruct encode: %v", enc["items"])
	require.Len(t, list, 3)
	m := list[0].(map[string]any)
	if m["n"] != uint32(1) {
		assert.Equal(t, uint32(1), m["n"], "items[0].n: %v (%T)", m["n"], m["n"])
	}
}

func TestProtocol_OptionalStruct_Null(t *testing.T) {
	inner := []SchemaField{{Name: "x", Type: "uint32"}}
	sc := []SchemaField{{Name: "s", Type: "optional<struct>", Fields: inner}}
	raw := map[string]json.RawMessage{"s": json.RawMessage("null")}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalStruct("s")
	if !errors.Is(err, ErrNilField) || got != nil {
		assert.True(t, errors.Is(err, ErrNilField), "optional[struct] null: %v %v", got, err)
		assert.Nil(t, got, "optional[struct] null: %v %v", got, err)
	}
	enc := mustEncode(t, fm, sc)
	if enc["s"] != nil {
		assert.Nil(t, enc["s"], "encode null struct: %v", enc["s"])
	}
}

func TestProtocol_OptionalStruct_Present(t *testing.T) {
	inner := []SchemaField{{Name: "x", Type: "uint32"}}
	sc := []SchemaField{{Name: "s", Type: "optional<struct>", Fields: inner}}
	raw := map[string]json.RawMessage{"s": json.RawMessage(`{"x":42}`)}
	fm := mustDecode(t, raw, sc)
	got, err := fm.GetOptionalStruct("s")
	require.NoError(t, err)
	require.NotNil(t, got)
	if v, _ := got.GetU32("x"); v != 42 {
		assert.Equal(t, uint32(42), v, "s.x: %d", v)
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
		assert.Equal(t, uint32(5), v, "a: %d", v)
	}
	_, err := fm.GetString("b")
	assert.Error(t, err, "expected not-found error for absent b")
}

func TestProtocol_UnknownType_Error(t *testing.T) {
	sc := []SchemaField{{Name: "v", Type: "uuid"}}
	raw := mkRaw(t, "v", "something")
	_, err := DecodeFieldMap(raw, sc)
	assert.Error(t, err, "expected error for unknown type")
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
		assert.NoError(t, err, "deep nested uint64: %v %v", v, err)
		assert.Equal(t, uint64(9999999999999999999), v, "deep nested uint64: %v %v", v, err)
	}
}

// TestProtocol_ListEveryScalarElem pins the invariant that a list supports every
// element type a bare field does. Before decodeList routed through decodeScalar
// it carried its own switch, and uint16/int8/int16/float64/bytes were missing
// from it -- declarable in a case file, accepted by `serify validate`, and only
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
			require.True(t, ok, "field not decoded")
			require.Equal(t, fmt.Sprintf("%v (%T)", c.want, c.want), fmt.Sprintf("%v (%T)", got, got), "decode: got %v (%T), want %v (%T)", got, got, c.want, c.want)

			// Re-encoding must reproduce the wire form the runner sent, so the
			// two directions cannot drift apart for any element type.
			enc := mustEncode(t, fm, sc)
			gotJSON, err := json.Marshal(enc["v"])
			require.NoError(t, err, "marshal encoded: %v", err)
			wantJSON, err := json.Marshal(c.send)
			require.NoError(t, err, "marshal sent: %v", err)
			assert.Equal(t, string(wantJSON), string(gotJSON), "re-encode: got %s, want %s", gotJSON, wantJSON)
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
			require.NoError(t, err)
			require.Len(t, got, 2)
			require.Equal(t, "170141183460469231731687303715884105727", got[0].String())
			require.Equal(t, 0, got[1].Sign())

			enc := mustEncode(t, fm, sc)
			b, _ := json.Marshal(enc["v"])
			assert.Equal(t, `["170141183460469231731687303715884105727","0"]`, string(b), "re-encode: %s", b)
		})
	}
}
