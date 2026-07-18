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

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

type Point struct {
	X    int32  `json:"x"    serify:"x"`
	Y    int32  `json:"y"    serify:"y"`
	Z    int32  `json:"z"    serify:"z"`
	Name string `json:"name"`
}

type Tag struct {
	Name   string `json:"name"`
	Weight uint32 `json:"weight"`
}

type AllTypes struct {
	Uint8     uint8             `json:"uint8"      serify:"uint8"`
	Uint16    uint16            `json:"uint16"     serify:"uint16"`
	Uint32    uint32            `json:"uint32"     serify:"uint32"`
	Uint64    uint64            `json:"uint64"     serify:"uint64"`
	Int8      int8              `json:"int8"       serify:"int8"`
	Int16     int16             `json:"int16"      serify:"int16"`
	Int32     int32             `json:"int32"      serify:"int32"`
	Int64     int64             `json:"int64"      serify:"int64"`
	Float32   float32           `json:"float32"    serify:"float32"`
	Float64   float64           `json:"float64"    serify:"float64"`
	Bool      bool              `json:"bool"       serify:"bool"`
	String    string            `json:"string"     serify:"string"`
	Bytes     []byte            `json:"bytes"      serify:"bytes"`
	List      []string          `json:"list"       serify:"list"`
	Optional  *string           `json:"optional"   serify:"optional"`
	Array     [4]uint32         `json:"array"      serify:"array"`
	Struct    Point             `json:"struct"     serify:"struct"`
	Map       map[string]uint32 `json:"map"        serify:"map"`
	MapStruct map[string]Tag    `json:"map_struct" serify:"map_struct"`
	Status    string            `json:"status"     serify:"status"`
}

// statusVariants is the declaration order of the enum in all_types.yaml. The
// binary layout stores the ordinal, not the name, which is the usual reason a
// worker cares that a field is an enum at all.
var statusVariants = []string{"pending", "paid", "shipped", "delivered", "cancelled"}

func statusOrdinal(s string) (uint8, error) {
	for i, v := range statusVariants {
		if v == s {
			return uint8(i), nil
		}
	}
	return 0, fmt.Errorf("unknown status %q", s)
}

func statusName(i uint8) (string, error) {
	if int(i) >= len(statusVariants) {
		return "", fmt.Errorf("status ordinal %d out of range", i)
	}
	return statusVariants[i], nil
}

var errTruncated = errors.New("truncated")

func appendLenStr(buf []byte, s string) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
	return append(buf, s...)
}

func readLenStr(b []byte) (string, []byte, error) {
	if len(b) < 4 {
		return "", b, errTruncated
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < n {
		return "", b, errTruncated
	}
	return string(b[:n]), b[n:], nil
}

func (a *AllTypes) MarshalBinary() ([]byte, error) {
	var buf []byte

	buf = append(buf, a.Uint8)
	buf = binary.LittleEndian.AppendUint16(buf, a.Uint16)
	buf = binary.LittleEndian.AppendUint32(buf, a.Uint32)
	buf = binary.LittleEndian.AppendUint64(buf, a.Uint64)
	buf = append(buf, byte(a.Int8))
	buf = binary.LittleEndian.AppendUint16(buf, uint16(a.Int16))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(a.Int32))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(a.Int64))
	buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(a.Float32))
	buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(a.Float64))

	if a.Bool {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}

	buf = appendLenStr(buf, a.String)

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(a.Bytes)))
	buf = append(buf, a.Bytes...)

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(a.List)))
	for _, s := range a.List {
		buf = appendLenStr(buf, s)
	}

	if a.Optional == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = appendLenStr(buf, *a.Optional)
	}

	for _, n := range a.Array {
		buf = binary.LittleEndian.AppendUint32(buf, n)
	}

	buf = binary.LittleEndian.AppendUint32(buf, uint32(a.Struct.X))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(a.Struct.Y))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(a.Struct.Z))
	buf = appendLenStr(buf, a.Struct.Name)

	keys := sortedKeys(a.Map)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(keys)))
	for _, k := range keys {
		buf = appendLenStr(buf, k)
		buf = binary.LittleEndian.AppendUint32(buf, a.Map[k])
	}

	tkeys := sortedTagKeys(a.MapStruct)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(tkeys)))
	for _, k := range tkeys {
		buf = appendLenStr(buf, k)
		buf = appendLenStr(buf, a.MapStruct[k].Name)
		buf = binary.LittleEndian.AppendUint32(buf, a.MapStruct[k].Weight)
	}

	ord, err := statusOrdinal(a.Status)
	if err != nil {
		return nil, err
	}
	buf = append(buf, ord)

	return buf, nil
}

func (a *AllTypes) UnmarshalBinary(data []byte) error {
	b := data
	var err error

	if len(b) < 1 {
		return errTruncated
	}
	a.Uint8 = b[0]
	b = b[1:]
	if len(b) < 2 {
		return errTruncated
	}
	a.Uint16 = binary.LittleEndian.Uint16(b[:2])
	b = b[2:]
	if len(b) < 4 {
		return errTruncated
	}
	a.Uint32 = binary.LittleEndian.Uint32(b[:4])
	b = b[4:]
	if len(b) < 8 {
		return errTruncated
	}
	a.Uint64 = binary.LittleEndian.Uint64(b[:8])
	b = b[8:]
	if len(b) < 1 {
		return errTruncated
	}
	a.Int8 = int8(b[0])
	b = b[1:]
	if len(b) < 2 {
		return errTruncated
	}
	a.Int16 = int16(binary.LittleEndian.Uint16(b[:2]))
	b = b[2:]
	if len(b) < 4 {
		return errTruncated
	}
	a.Int32 = int32(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < 8 {
		return errTruncated
	}
	a.Int64 = int64(binary.LittleEndian.Uint64(b[:8]))
	b = b[8:]
	if len(b) < 4 {
		return errTruncated
	}
	a.Float32 = math.Float32frombits(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < 8 {
		return errTruncated
	}
	a.Float64 = math.Float64frombits(binary.LittleEndian.Uint64(b[:8]))
	b = b[8:]
	if len(b) < 1 {
		return errTruncated
	}
	a.Bool = b[0] != 0
	b = b[1:]

	if a.String, b, err = readLenStr(b); err != nil {
		return err
	}

	if len(b) < 4 {
		return errTruncated
	}
	metaLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < metaLen {
		return errTruncated
	}
	a.Bytes = make([]byte, metaLen)
	copy(a.Bytes, b[:metaLen])
	b = b[metaLen:]

	if len(b) < 4 {
		return errTruncated
	}
	listLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	a.List = make([]string, listLen)
	for i := range a.List {
		if a.List[i], b, err = readLenStr(b); err != nil {
			return err
		}
	}

	if len(b) < 1 {
		return errTruncated
	}
	hasOpt := b[0] != 0
	b = b[1:]
	if hasOpt {
		var s string
		if s, b, err = readLenStr(b); err != nil {
			return err
		}
		a.Optional = &s
	}

	for i := range a.Array {
		if len(b) < 4 {
			return errTruncated
		}
		a.Array[i] = binary.LittleEndian.Uint32(b[:4])
		b = b[4:]
	}

	if len(b) < 12 {
		return errTruncated
	}
	a.Struct.X = int32(binary.LittleEndian.Uint32(b[:4]))
	a.Struct.Y = int32(binary.LittleEndian.Uint32(b[4:8]))
	a.Struct.Z = int32(binary.LittleEndian.Uint32(b[8:12]))
	b = b[12:]
	if a.Struct.Name, b, err = readLenStr(b); err != nil {
		return err
	}

	if len(b) < 4 {
		return errTruncated
	}
	mapLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	a.Map = make(map[string]uint32, mapLen)
	for range mapLen {
		var k string
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		if len(b) < 4 {
			return errTruncated
		}
		a.Map[k] = binary.LittleEndian.Uint32(b[:4])
		b = b[4:]
	}

	if len(b) < 4 {
		return errTruncated
	}
	tagLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	a.MapStruct = make(map[string]Tag, tagLen)
	for range tagLen {
		var k string
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		var t Tag
		if t.Name, b, err = readLenStr(b); err != nil {
			return err
		}
		if len(b) < 4 {
			return errTruncated
		}
		t.Weight = binary.LittleEndian.Uint32(b[:4])
		b = b[4:]
		a.MapStruct[k] = t
	}

	if len(b) < 1 {
		return errTruncated
	}
	if a.Status, err = statusName(b[0]); err != nil {
		return err
	}

	return nil
}

// marshalJSON deliberately avoids json.Marshal: it would HTML-escape <, >, &
// and U+2028/U+2029, a Go-only quirk no other language's encoder reproduces.
// Encoder.Encode appends a newline, so trim it.
func marshalJSON(a *AllTypes) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(a); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

func unmarshalJSON(a *AllTypes, b []byte) error { return json.Unmarshal(b, a) }

func sortedKeys(m map[string]uint32) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedTagKeys(m map[string]Tag) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
