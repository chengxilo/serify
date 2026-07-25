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
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRecord mirrors the full schema defined in examples/cases.
type addrFields struct {
	Street string
	Zip    uint32
}

type labelFields struct {
	Value    string
	Priority uint32
}

type testRecord struct {
	UserID   uint64 `serify:"user_id"`
	Username string
	Score    float32
	Active   bool
	Metadata []byte
	Tags     []string
	Profile  *string
	Counts   [4]uint32
	Address  addrFields             `serify:"address"`
	Scores   map[string]uint32      `serify:"scores"`
	Labels   map[string]labelFields `serify:"labels"`
}

func strPtr(s string) *string { return &s }

func f32hex(v float32) string {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
	return hex.EncodeToString(b)
}

func f64hex(v float64) string {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(v))
	return hex.EncodeToString(b)
}

func mkRaw(t *testing.T, kv ...any) map[string]json.RawMessage {
	t.Helper()
	m := make(map[string]json.RawMessage)
	for i := 0; i+1 < len(kv); i += 2 {
		k := kv[i].(string)
		b, err := json.Marshal(kv[i+1])
		require.NoError(t, err, "mkRaw marshal %v: %v", kv[i+1], err)
		m[k] = b
	}
	return m
}

func mustDecode(t *testing.T, raw map[string]json.RawMessage, sc []SchemaField) *FieldMap {
	t.Helper()
	fm, err := DecodeFieldMap(raw, sc)
	require.NoError(t, err, "DecodeFieldMap: %v", err)
	return fm
}

func mustEncode(t *testing.T, fm *FieldMap, sc []SchemaField) map[string]any {
	t.Helper()
	out, err := EncodeFieldMap(fm, sc)
	require.NoError(t, err, "EncodeFieldMap: %v", err)
	return out
}

func TestFieldMap_ScalarRoundTrip(t *testing.T) {
	fm := NewFieldMap()
	fm.SetU8("uint8", 0xFF)
	fm.SetU16("uint16", 0xFFFF)
	fm.SetU32("uint32", 0xFFFFFFFF)
	fm.SetU64("uint64", 0xFFFFFFFFFFFFFFFF)
	fm.SetI8("int8", -128)
	fm.SetI16("int16", -32768)
	fm.SetI32("int32", math.MinInt32)
	fm.SetI64("int64", math.MinInt64)
	fm.SetF32("float32", 1.5)
	fm.SetF64("float64", math.Pi)
	fm.SetBool("bool", true)
	fm.SetString("str", "hello")
	fm.SetBytes("bytes", []byte{1, 2, 3})

	if v, err := fm.GetU8("uint8"); err != nil || v != 0xFF {
		assert.NoError(t, err, "U8: %v %v", v, err)
		assert.Equal(t, uint8(0xFF), v, "U8: %v %v", v, err)
	}
	if v, err := fm.GetU16("uint16"); err != nil || v != 0xFFFF {
		assert.NoError(t, err, "U16: %v %v", v, err)
		assert.Equal(t, uint16(0xFFFF), v, "U16: %v %v", v, err)
	}
	if v, err := fm.GetU32("uint32"); err != nil || v != 0xFFFFFFFF {
		assert.NoError(t, err, "U32: %v %v", v, err)
		assert.Equal(t, uint32(0xFFFFFFFF), v, "U32: %v %v", v, err)
	}
	if v, err := fm.GetU64("uint64"); err != nil || v != 0xFFFFFFFFFFFFFFFF {
		assert.NoError(t, err, "U64: %v %v", v, err)
		assert.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), v, "U64: %v %v", v, err)
	}
	if v, err := fm.GetI8("int8"); err != nil || v != -128 {
		assert.NoError(t, err, "I8: %v %v", v, err)
		assert.Equal(t, int8(-128), v, "I8: %v %v", v, err)
	}
	if v, err := fm.GetI16("int16"); err != nil || v != -32768 {
		assert.NoError(t, err, "I16: %v %v", v, err)
		assert.Equal(t, int16(-32768), v, "I16: %v %v", v, err)
	}
	if v, err := fm.GetI32("int32"); err != nil || v != math.MinInt32 {
		assert.NoError(t, err, "I32: %v %v", v, err)
		assert.Equal(t, int32(math.MinInt32), v, "I32: %v %v", v, err)
	}
	if v, err := fm.GetI64("int64"); err != nil || v != math.MinInt64 {
		assert.NoError(t, err, "I64: %v %v", v, err)
		assert.Equal(t, int64(math.MinInt64), v, "I64: %v %v", v, err)
	}
	if v, err := fm.GetF32("float32"); err != nil || v != 1.5 {
		assert.NoError(t, err, "F32: %v %v", v, err)
		assert.Equal(t, float32(1.5), v, "F32: %v %v", v, err)
	}
	if v, err := fm.GetF64("float64"); err != nil || v != math.Pi {
		assert.NoError(t, err, "F64: %v %v", v, err)
		assert.Equal(t, math.Pi, v, "F64: %v %v", v, err)
	}
	if v, err := fm.GetBool("bool"); err != nil || !v {
		assert.NoError(t, err, "Bool: %v %v", v, err)
		assert.True(t, v, "Bool: %v %v", v, err)
	}
	if v, err := fm.GetString("str"); err != nil || v != "hello" {
		assert.NoError(t, err, "String: %v %v", v, err)
		assert.Equal(t, "hello", v, "String: %v %v", v, err)
	}
	if v, err := fm.GetBytes("bytes"); err != nil || string(v) != "\x01\x02\x03" {
		assert.NoError(t, err, "Bytes: %v %v", v, err)
		assert.Equal(t, []byte{1, 2, 3}, v, "Bytes: %v %v", v, err)
	}
}

func TestFieldMap_Missing(t *testing.T) {
	fm := NewFieldMap()
	_, err := fm.GetU32("missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		if err == nil {
			assert.Error(t, err, "expected not-found error, got %v", err)
		} else {
			assert.Contains(t, err.Error(), "not found", "expected not-found error, got %v", err)
		}
	}
}

func TestFieldMap_TypeMismatch(t *testing.T) {
	fm := NewFieldMap()
	fm.SetU32("x", 42)
	_, err := fm.GetU64("x") // stored as uint32, reading as uint64
	if err == nil {
		assert.Error(t, err, "expected type-mismatch error")
	}
}

func TestFieldMap_Optional_Present(t *testing.T) {
	fm := NewFieldMap()
	fm.SetOptionalString("s", strPtr("hello"))
	v, err := fm.GetOptionalString("s")
	if err != nil || v == nil || *v != "hello" {
		assert.NoError(t, err, "present: %v %v", v, err)
		if err == nil {
			require.NotNil(t, v, "present: %v %v", v, err)
			assert.Equal(t, "hello", *v, "present: %v %v", v, err)
		}
	}
}

func TestFieldMap_Optional_Nil(t *testing.T) {
	fm := NewFieldMap()
	fm.SetOptionalString("s", nil)
	v, err := fm.GetOptionalString("s")
	if err != nil || v != nil {
		assert.NoError(t, err, "nil: %v %v", v, err)
		assert.Nil(t, v, "nil: %v %v", v, err)
	}
}

func TestFieldMap_Struct(t *testing.T) {
	inner := NewFieldMap()
	inner.SetU32("x", 99)
	outer := NewFieldMap()
	outer.SetStruct("inner", inner)

	got, err := outer.GetStruct("inner")
	require.NoError(t, err)
	require.NotNil(t, got)
	if v, _ := got.GetU32("x"); v != 99 {
		assert.Equal(t, uint32(99), v, "inner.x: %d", v)
	}
}

func TestFieldMap_Struct_Nil(t *testing.T) {
	fm := NewFieldMap()
	fm.SetStruct("s", nil)
	got, err := fm.GetStruct("s")
	if err != nil || got != nil {
		assert.NoError(t, err, "nil struct: %v %v", got, err)
		assert.Nil(t, got, "nil struct: %v %v", got, err)
	}
}

func TestFieldMap_OptionalStruct(t *testing.T) {
	inner := NewFieldMap()
	inner.SetBool("ok", true)

	fm := NewFieldMap()
	fm.SetOptionalStruct("s", inner)
	got, err := fm.GetOptionalStruct("s")
	require.NoError(t, err)
	require.NotNil(t, got)
	if v, _ := got.GetBool("ok"); !v {
		assert.True(t, v, "inner.ok: false")
	}

	fm.SetOptionalStruct("nil_s", nil)
	got2, err := fm.GetOptionalStruct("nil_s")
	assert.True(t, errors.Is(err, ErrNilField), "nil optional struct: %v %v", got2, err)
	assert.Nil(t, got2, "nil optional struct: %v %v", got2, err)
}

func TestFieldMap_Collections(t *testing.T) {
	fm := NewFieldMap()
	fm.SetListString("ls", []string{"a", "b", "c"})
	fm.SetListU32("lu32", []uint32{0, 1, 0xFFFFFFFF})
	fm.SetListU64("lu64", []uint64{0, math.MaxUint64})
	fm.SetListI32("li32", []int32{math.MinInt32, 0, math.MaxInt32})
	fm.SetListI64("li64", []int64{math.MinInt64, math.MaxInt64})
	fm.SetListF32("lf32", []float32{1.5, -2.5})
	fm.SetListBool("lbool", []bool{true, false, true})
	fm.SetListU32("arr", []uint32{10, 20, 30, 40})

	if v, err := fm.GetListString("ls"); err != nil || len(v) != 3 || v[1] != "b" {
		assert.NoError(t, err, "ListString: %v %v", v, err)
		assert.Len(t, v, 3, "ListString: %v %v", v, err)
		if len(v) >= 2 {
			assert.Equal(t, "b", v[1], "ListString: %v %v", v, err)
		}
	}
	if v, err := fm.GetListU32("lu32"); err != nil || v[2] != 0xFFFFFFFF {
		assert.NoError(t, err, "ListU32: %v %v", v, err)
		if len(v) >= 3 {
			assert.Equal(t, uint32(0xFFFFFFFF), v[2], "ListU32: %v %v", v, err)
		}
	}
	if v, err := fm.GetListU64("lu64"); err != nil || v[1] != math.MaxUint64 {
		assert.NoError(t, err, "ListU64: %v %v", v, err)
		if len(v) >= 2 {
			assert.Equal(t, uint64(math.MaxUint64), v[1], "ListU64: %v %v", v, err)
		}
	}
	if v, err := fm.GetListI32("li32"); err != nil || v[0] != math.MinInt32 {
		assert.NoError(t, err, "ListI32: %v %v", v, err)
		if len(v) >= 1 {
			assert.Equal(t, int32(math.MinInt32), v[0], "ListI32: %v %v", v, err)
		}
	}
	if v, err := fm.GetListI64("li64"); err != nil || v[0] != math.MinInt64 {
		assert.NoError(t, err, "ListI64: %v %v", v, err)
		if len(v) >= 1 {
			assert.Equal(t, int64(math.MinInt64), v[0], "ListI64: %v %v", v, err)
		}
	}
	if v, err := fm.GetListF32("lf32"); err != nil || v[1] != -2.5 {
		assert.NoError(t, err, "ListF32: %v %v", v, err)
		if len(v) >= 2 {
			assert.Equal(t, float32(-2.5), v[1], "ListF32: %v %v", v, err)
		}
	}
	if v, err := fm.GetListBool("lbool"); err != nil || !v[0] || v[1] || !v[2] {
		assert.NoError(t, err, "ListBool: %v %v", v, err)
		if len(v) >= 3 {
			assert.True(t, v[0], "ListBool: %v %v", v, err)
			assert.False(t, v[1], "ListBool: %v %v", v, err)
			assert.True(t, v[2], "ListBool: %v %v", v, err)
		}
	}
	if v, err := fm.GetListU32("arr"); err != nil || len(v) != 4 || v[0] != 10 || v[3] != 40 {
		assert.NoError(t, err, "ArrayU32: %v %v", v, err)
		assert.Len(t, v, 4, "ArrayU32: %v %v", v, err)
		if len(v) >= 4 {
			assert.Equal(t, uint32(10), v[0], "ArrayU32: %v %v", v, err)
			assert.Equal(t, uint32(40), v[3], "ArrayU32: %v %v", v, err)
		}
	}
}

func TestFieldMap_ListStruct(t *testing.T) {
	a := NewFieldMap()
	a.SetString("k", "one")
	b := NewFieldMap()
	b.SetString("k", "two")

	fm := NewFieldMap()
	fm.SetListStruct("items", []*FieldMap{a, b})

	got, err := fm.GetListStruct("items")
	require.NoError(t, err)
	require.Len(t, got, 2)
	if v, _ := got[0].GetString("k"); v != "one" {
		assert.Equal(t, "one", v, "items[0]: %q", v)
	}
	if v, _ := got[1].GetString("k"); v != "two" {
		assert.Equal(t, "two", v, "items[1]: %q", v)
	}
}
