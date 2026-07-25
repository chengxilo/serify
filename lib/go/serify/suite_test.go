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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type simpleMsg struct {
	X uint32
	Y string
}

func encodeSimple(m *simpleMsg) []byte {
	buf := binary.LittleEndian.AppendUint32(nil, m.X)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(m.Y)))
	return append(buf, m.Y...)
}

func decodeSimple(m *simpleMsg, b []byte) error {
	if len(b) < 8 {
		return errors.New("too short")
	}
	m.X = binary.LittleEndian.Uint32(b[:4])
	n := int(binary.LittleEndian.Uint32(b[4:8]))
	if len(b) < 8+n {
		return errors.New("truncated")
	}
	m.Y = string(b[8 : 8+n])
	return nil
}

func TestBuildSerializer_SimpleStruct(t *testing.T) {
	ser, _, err := buildSerializer(&simpleMsg{}, func(m *simpleMsg) ([]byte, error) {
		return encodeSimple(m), nil
	}, nil)
	require.NoError(t, err, "buildSerializer: %v", err)
	fm := NewFieldMap()
	fm.SetU32("x", 0xDEAD)
	fm.SetString("y", "hello")

	b, err := ser(fm)
	require.NoError(t, err, "serialize: %v", err)
	if len(b) < 4 || binary.LittleEndian.Uint32(b[:4]) != 0xDEAD {
		assert.Equal(t, uint32(0xDEAD), binary.LittleEndian.Uint32(b[:4]), "x: got %v", b)
	}
}

func TestBuildDeserializer_InPlace(t *testing.T) {
	des, err := parseDeserializer(&simpleMsg{}, decodeSimple, nil)
	require.NoError(t, err, "buildDeserializer: %v", err)

	b := encodeSimple(&simpleMsg{X: 0xBEEF, Y: "world"})
	fm, err := des(b)
	require.NoError(t, err, "deserialize: %v", err)
	if v, _ := fm.GetU32("x"); v != 0xBEEF {
		assert.Equal(t, uint32(0xBEEF), v, "x: 0x%X", v)
	}
	if v, _ := fm.GetString("y"); v != "world" {
		assert.Equal(t, "world", v, "y: %q", v)
	}
}

func TestBuildDeserializer_Factory(t *testing.T) {
	fromB := func(b []byte) (*simpleMsg, error) {
		m := &simpleMsg{}
		return m, decodeSimple(m, b)
	}
	des, err := parseDeserializer(&simpleMsg{}, fromB, nil)
	require.NoError(t, err, "buildDeserializer: %v", err)

	b := encodeSimple(&simpleMsg{X: 55, Y: "factory"})
	fm, err := des(b)
	require.NoError(t, err, "deserialize: %v", err)
	if v, _ := fm.GetU32("x"); v != 55 {
		assert.Equal(t, uint32(55), v, "x: %d", v)
	}
	if v, _ := fm.GetString("y"); v != "factory" {
		assert.Equal(t, "factory", v, "y: %q", v)
	}
}

func TestBuildSerializer_BadSignature(t *testing.T) {
	cases := []struct {
		name string
		fn   any
	}{
		{"nil", nil},
		{"not func", "hello"},
		{"wrong param type", func(_ string) ([]byte, error) { return nil, nil }},
		{"two params", func(_ *simpleMsg, _ int) ([]byte, error) { return nil, nil }},
		{"bad return", func(_ *simpleMsg) int { return 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := buildSerializer(&simpleMsg{}, tc.fn, nil)
			if err == nil {
				assert.Error(t, err, "expected error for %q", tc.name)
			}
		})
	}
}

func TestBuildDeserializer_BadSignature(t *testing.T) {
	cases := []struct {
		name string
		fn   any
	}{
		{"nil", nil},
		{"not func", 42},
		{"no inputs", func() error { return nil }},
		{"wrong first param", func(_ string, _ []byte) error { return nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDeserializer(&simpleMsg{}, tc.fn, nil)
			if err == nil {
				assert.Error(t, err, "expected error for %q", tc.name)
			}
		})
	}
}
