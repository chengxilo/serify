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

package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/typekind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func field(name, base string) config.Field {
	return config.Field{Name: name, Type: config.FieldType{Base: base}}
}

func TestEncodeData_Scalars(t *testing.T) {
	schema := []config.Field{
		field("name", typekind.String),
		field("age", typekind.Uint32),
		field("active", typekind.Bool),
	}
	data := map[string]any{"name": "alice", "age": 30, "active": true}

	out, err := EncodeData(data, schema)
	require.NoError(t, err)
	assert.Equal(t, "alice", out["name"], "name = %v", out["name"])
	assert.Equal(t, 30, out["age"], "age = %v", out["age"])
	assert.Equal(t, true, out["active"], "active = %v", out["active"])
}

func TestEncodeData_Float32(t *testing.T) {
	schema := []config.Field{field("val", typekind.Float32)}
	data := map[string]any{"val": 1.5}

	out, err := EncodeData(data, schema)
	require.NoError(t, err)

	var expected [4]byte
	binary.LittleEndian.PutUint32(expected[:], math.Float32bits(1.5))
	want := hex.EncodeToString(expected[:])

	assert.Equal(t, want, out["val"], "got %v, want %s", out["val"], want)
}

func TestEncodeData_Float64(t *testing.T) {
	schema := []config.Field{field("val", typekind.Float64)}
	data := map[string]any{"val": 3.14}

	out, err := EncodeData(data, schema)
	require.NoError(t, err)

	var expected [8]byte
	binary.LittleEndian.PutUint64(expected[:], math.Float64bits(3.14))
	want := hex.EncodeToString(expected[:])

	assert.Equal(t, want, out["val"], "got %v, want %s", out["val"], want)
}

func TestEncodeData_BigInts(t *testing.T) {
	for _, base := range []string{typekind.Uint64, typekind.Int64, typekind.Uint128, typekind.Int128} {
		t.Run(base, func(t *testing.T) {
			schema := []config.Field{field("n", base)}
			data := map[string]any{"n": 12345}

			out, err := EncodeData(data, schema)
			require.NoError(t, err)
			assert.Equal(t, "12345", out["n"], "got %v, want %q", out["n"], "12345")
		})
	}
}

func TestEncodeData_Bytes(t *testing.T) {
	schema := []config.Field{field("data", typekind.Bytes)}

	t.Run("array", func(t *testing.T) {
		data := map[string]any{"data": []any{0xde, 0xad}}
		out, err := EncodeData(data, schema)
		require.NoError(t, err)
		assert.Equal(t, "dead", out["data"], "got %v, want %q", out["data"], "dead")
	})

	t.Run("hex_string", func(t *testing.T) {
		data := map[string]any{"data": "beef"}
		out, err := EncodeData(data, schema)
		require.NoError(t, err)
		assert.Equal(t, "beef", out["data"], "got %v, want %q", out["data"], "beef")
	})
}

func TestEncodeData_Optional(t *testing.T) {
	schema := []config.Field{{
		Name: "opt",
		Type: config.FieldType{Base: typekind.Optional, Elem: &config.FieldType{Base: typekind.String}},
	}}

	t.Run("nil", func(t *testing.T) {
		out, err := EncodeData(map[string]any{"opt": nil}, schema)
		require.NoError(t, err)
		assert.Nil(t, out["opt"], "got %v, want nil", out["opt"])
	})

	t.Run("present", func(t *testing.T) {
		out, err := EncodeData(map[string]any{"opt": "hello"}, schema)
		require.NoError(t, err)
		assert.Equal(t, "hello", out["opt"], "got %v, want %q", out["opt"], "hello")
	})
}

func TestEncodeData_List(t *testing.T) {
	schema := []config.Field{{
		Name: "nums",
		Type: config.FieldType{Base: typekind.List, Elem: &config.FieldType{Base: typekind.Uint64}},
	}}
	data := map[string]any{"nums": []any{1, 2, 3}}

	out, err := EncodeData(data, schema)
	require.NoError(t, err)
	arr := out["nums"].([]any)
	assert.Equal(t, []any{"1", "2", "3"}, arr, "got %v", arr)
}

func TestEncodeData_Array(t *testing.T) {
	schema := []config.Field{{
		Name: "pair",
		Type: config.FieldType{Base: typekind.Array, Elem: &config.FieldType{Base: typekind.String}, ArrayN: 2},
	}}
	data := map[string]any{"pair": []any{"a", "b"}}

	out, err := EncodeData(data, schema)
	require.NoError(t, err)
	arr := out["pair"].([]any)
	assert.Equal(t, []any{"a", "b"}, arr, "got %v", arr)
}

func TestEncodeData_ArrayLengthMismatch(t *testing.T) {
	schema := []config.Field{{
		Name: "pair",
		Type: config.FieldType{Base: typekind.Array, Elem: &config.FieldType{Base: typekind.String}, ArrayN: 2},
	}}
	data := map[string]any{"pair": []any{"a"}}

	_, err := EncodeData(data, schema)
	require.Error(t, err, "expected error for array length mismatch")
}

func TestEncodeData_Map(t *testing.T) {
	schema := []config.Field{{
		Name: "kv",
		Type: config.FieldType{
			Base: typekind.Map,
			Key:  &config.FieldType{Base: typekind.String},
			Elem: &config.FieldType{Base: typekind.Uint64},
		},
	}}
	data := map[string]any{"kv": map[string]any{"x": 42}}

	out, err := EncodeData(data, schema)
	require.NoError(t, err)
	m := out["kv"].(map[string]any)
	assert.Equal(t, "42", m["x"], "got %v", m["x"])
}

func TestEncodeData_Struct(t *testing.T) {
	schema := []config.Field{{
		Name: "addr",
		Type: config.FieldType{
			Base: typekind.Struct,
			Fields: []config.Field{
				field("city", typekind.String),
				field("zip", typekind.Uint32),
			},
		},
	}}
	data := map[string]any{"addr": map[string]any{"city": "NYC", "zip": 10001}}

	out, err := EncodeData(data, schema)
	require.NoError(t, err)
	addr := out["addr"].(map[string]any)
	assert.Equal(t, "NYC", addr["city"], "got %v", addr)
	assert.Equal(t, 10001, addr["zip"], "got %v", addr)
}

func TestEncodeData_UnknownField(t *testing.T) {
	schema := []config.Field{field("name", typekind.String)}
	data := map[string]any{"unknown": "val"}

	_, err := EncodeData(data, schema)
	require.Error(t, err, "expected error for unknown field")
}
