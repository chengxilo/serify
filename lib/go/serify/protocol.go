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
	"fmt"

	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/chengxilo/serify/internal/typekind"
)

// float32ByteLen is the wire size of a float32 in bytes (IEEE 754 single).
const float32ByteLen = 4

// float64ByteLen is the wire size of a float64 in bytes (IEEE 754 double).
const float64ByteLen = 8

// SchemaField describes one field from the bind message.
// Fields carries the nested schema for struct types.
type SchemaField struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Fields   []SchemaField     `json:"fields,omitempty"`   // nested schema for struct / list<struct> / optional<struct> / map<K,struct>
	KeyType  string            `json:"key_type,omitempty"` // map key type, e.g. "string"
	Variants []SchemaVariant   `json:"variants,omitempty"` // for oneof<...>
	Tags     map[string]string `json:"tags,omitempty"`
}

// SchemaVariant is one arm of a oneof: a tag and its payload schema (Payload is
// nil for a unit variant).
type SchemaVariant struct {
	Name    string       `json:"name"`
	Payload *SchemaField `json:"payload,omitempty"`
}

func findSchemaVariant(vs []SchemaVariant, tag string) *SchemaVariant {
	for i := range vs {
		if vs[i].Name == tag {
			return &vs[i]
		}
	}
	return nil
}

// DecodeFieldMap converts the "data" object from the protocol into a FieldMap.
func DecodeFieldMap(raw map[string]json.RawMessage, schema []SchemaField) (*FieldMap, error) {
	fm := NewFieldMap()
	for _, sf := range schema {
		r, ok := raw[sf.Name]
		if !ok {
			continue
		}
		if err := decodeField(fm, sf, r); err != nil {
			return nil, fmt.Errorf("field %s: %w", sf.Name, err)
		}
	}
	return fm, nil
}

func decodeField(fm *FieldMap, sf SchemaField, r json.RawMessage) error {
	typ := sf.Type
	switch {
	case strings.HasPrefix(typ, "list<"):
		return decodeList(fm, sf, typ[5:len(typ)-1], r)
	case strings.HasPrefix(typ, "optional<"):
		return decodeOptional(fm, sf, typ[9:len(typ)-1], r)
	case strings.HasPrefix(typ, "array<"):
		return decodeArray(fm, sf, r)
	case strings.HasPrefix(typ, "map<"):
		return decodeMap(fm, sf, r)
	case strings.HasPrefix(typ, "enum<"):
		// The variant name travels as a string. The variants themselves are in the
		// type (enum<a,b,c>), so a worker can derive an ordinal for its byte layout
		// with EnumVariants below.
		v, err := jsonValue[string](r)
		if err != nil {
			return err
		}
		fm.fields[sf.Name] = v
		return nil
	case strings.HasPrefix(typ, "oneof<"):
		return decodeOneOf(fm, sf, r)
	default:
		v, err := decodeScalar(typ, sf.Fields, r)
		if err != nil {
			return err
		}
		fm.fields[sf.Name] = v
		return nil
	}
}

// decodeOneOf decodes a oneof wire value {tag: payload} (payload null for a
// unit variant) into a *Variant, decoding the payload per the variant's schema.
func decodeOneOf(fm *FieldMap, sf SchemaField, r json.RawMessage) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(r, &obj); err != nil {
		return err
	}
	if len(obj) != 1 {
		return fmt.Errorf("oneof must name exactly one variant, got %d", len(obj))
	}
	for tag, payload := range obj {
		sv := findSchemaVariant(sf.Variants, tag)
		if sv == nil {
			return fmt.Errorf("unknown oneof variant %q", tag)
		}
		if sv.Payload == nil {
			fm.fields[sf.Name] = &Variant{Tag: tag}
			return nil
		}
		tmp := NewFieldMap()
		if err := decodeField(tmp, *sv.Payload, payload); err != nil {
			return fmt.Errorf("variant %q: %w", tag, err)
		}
		fm.fields[sf.Name] = &Variant{Tag: tag, Value: tmp.fields[sv.Payload.Name]}
		return nil
	}
	return nil
}

// decodeScalar decodes a single non-container value (scalars + nested struct)
// into the concrete Go type the FieldMap stores for that type. It is the one
// place wire types are mapped to Go types; the field, list, optional and map
// paths all route element decoding through here.
func decodeScalar(typ string, fields []SchemaField, r json.RawMessage) (any, error) {
	switch typ {
	case typekind.Uint8:
		return numFromJSON[uint8](r)
	case typekind.Uint16:
		return numFromJSON[uint16](r)
	case typekind.Uint32:
		return numFromJSON[uint32](r)
	case typekind.Int8:
		return numFromJSON[int8](r)
	case typekind.Int16:
		return numFromJSON[int16](r)
	case typekind.Int32:
		return numFromJSON[int32](r)
	case typekind.Uint64:
		return uintFromString(r)
	case typekind.Int64:
		return intFromString(r)
	case typekind.Uint128:
		return bigFromString(r, false)
	case typekind.Int128:
		return bigFromString(r, true)
	case typekind.Float32:
		b, err := hexBytes(r, float32ByteLen)
		if err != nil {
			return nil, err
		}
		return math.Float32frombits(binary.LittleEndian.Uint32(b)), nil
	case typekind.Float64:
		b, err := hexBytes(r, float64ByteLen)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(b)), nil
	case typekind.Bool:
		return jsonValue[bool](r)
	case typekind.String:
		return jsonValue[string](r)
	case typekind.Bytes:
		var s string
		if err := json.Unmarshal(r, &s); err != nil {
			return nil, err
		}
		return hex.DecodeString(s)
	case typekind.Struct:
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(r, &obj); err != nil {
			return nil, err
		}
		return DecodeFieldMap(obj, fields)
	default:
		return nil, fmt.Errorf("unknown type %q", typ)
	}
}

// decodeList decodes every element through decodeScalar, so a list supports
// exactly the element types a bare field does. The switch below only names the
// Go slice each element type collects into — it holds no decoding logic, which
// is why a new scalar cannot be reachable as a field but not as a list element.
func decodeList(fm *FieldMap, sf SchemaField, elemType string, r json.RawMessage) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(r, &arr); err != nil {
		return err
	}
	var (
		out any
		err error
	)
	switch elemType {
	case typekind.Uint8:
		out, err = collectList[uint8](sf, elemType, arr)
	case typekind.Uint16:
		out, err = collectList[uint16](sf, elemType, arr)
	case typekind.Uint32:
		out, err = collectList[uint32](sf, elemType, arr)
	case typekind.Uint64:
		out, err = collectList[uint64](sf, elemType, arr)
	case typekind.Int8:
		out, err = collectList[int8](sf, elemType, arr)
	case typekind.Int16:
		out, err = collectList[int16](sf, elemType, arr)
	case typekind.Int32:
		out, err = collectList[int32](sf, elemType, arr)
	case typekind.Int64:
		out, err = collectList[int64](sf, elemType, arr)
	case typekind.Uint128, typekind.Int128:
		out, err = collectList[*big.Int](sf, elemType, arr)
	case typekind.Float32:
		out, err = collectList[float32](sf, elemType, arr)
	case typekind.Float64:
		out, err = collectList[float64](sf, elemType, arr)
	case typekind.Bool:
		out, err = collectList[bool](sf, elemType, arr)
	case typekind.String:
		out, err = collectList[string](sf, elemType, arr)
	case typekind.Bytes:
		out, err = collectList[[]byte](sf, elemType, arr)
	case typekind.Struct:
		out, err = collectList[*FieldMap](sf, elemType, arr)
	default:
		return fmt.Errorf("unsupported list element type %q", elemType)
	}
	if err != nil {
		return err
	}
	fm.fields[sf.Name] = out
	return nil
}

func decodeOptional(fm *FieldMap, sf SchemaField, elemType string, r json.RawMessage) error {
	if string(r) == "null" {
		switch elemType {
		case typekind.String:
			fm.SetOptionalString(sf.Name, nil)
		case typekind.Struct:
			fm.SetOptionalStruct(sf.Name, nil)
		default:
			fm.fields[sf.Name] = nil
		}
		return nil
	}
	switch elemType {
	case typekind.String:
		s, err := jsonValue[string](r)
		if err != nil {
			return err
		}
		fm.SetOptionalString(sf.Name, &s)
	default:
		// struct and scalar optionals share the same decode path; structs are
		// stored as *FieldMap, scalars as their concrete type.
		v, err := decodeScalar(elemType, sf.Fields, r)
		if err != nil {
			return err
		}
		fm.fields[sf.Name] = v
	}
	return nil
}

// arrayElemType extracts the element type from an array type string.
// e.g. "array<uint32,4>" -> "uint32", "array<struct,2>" -> "struct".
func arrayElemType(typ string) string {
	inner := typ[6 : len(typ)-1] // strip "array<" and ">"
	if comma := strings.LastIndex(inner, ","); comma >= 0 {
		return strings.TrimSpace(inner[:comma])
	}
	return strings.TrimSpace(inner)
}

// decodeArray decodes an array<T,N>. An array is a list whose length the schema
// fixes, so it shares decodeList's storage and element decoding outright; the
// only thing this adds is the length check. Keeping a second representation for
// the same data is what let array<T,N> support exactly uint32 while list<T>
// supported eleven other element types.
func decodeArray(fm *FieldMap, sf SchemaField, r json.RawMessage) error {
	elemType := arrayElemType(sf.Type)
	if err := decodeList(fm, sf, elemType, r); err != nil {
		return err
	}
	want, ok := arrayLen(sf.Type)
	if !ok {
		return fmt.Errorf("array type %q has no length", sf.Type)
	}
	if got := reflect.ValueOf(fm.fields[sf.Name]).Len(); got != want {
		return fmt.Errorf("array %s: expected %d elements, got %d", sf.Name, want, got)
	}
	return nil
}

// arrayLen returns N from "array<T,N>".
func arrayLen(typ string) (int, bool) {
	inner := typ[6 : len(typ)-1]
	comma := strings.LastIndex(inner, ",")
	if comma < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(inner[comma+1:]))
	if err != nil {
		return 0, false
	}
	return n, true
}

// mapValueType extracts the value type from a map type string.
// e.g. "map<string,uint32>" -> "uint32", "map<string,struct>" -> "struct".
func mapValueType(typ string) string {
	inner := typ[4 : len(typ)-1] // strip "map<" and ">"
	kv := typekind.SplitTopLevel(inner)
	if len(kv) != 2 {
		return ""
	}
	return strings.TrimSpace(kv[1])
}

func decodeMap(fm *FieldMap, sf SchemaField, r json.RawMessage) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(r, &obj); err != nil {
		return err
	}
	valType := mapValueType(sf.Type)
	out := make(map[string]any, len(obj))
	for k, raw := range obj {
		v, err := decodeScalar(valType, sf.Fields, raw)
		if err != nil {
			return fmt.Errorf("map[%q]: %w", k, err)
		}
		out[k] = v
	}
	fm.SetMap(sf.Name, out)
	return nil
}

// EncodeFieldMap converts a FieldMap back to the protocol JSON representation.
func EncodeFieldMap(fm *FieldMap, schema []SchemaField) (map[string]any, error) {
	out := make(map[string]any, len(schema))
	for _, sf := range schema {
		v, ok := fm.fields[sf.Name]
		if !ok {
			continue
		}
		enc, err := encodeField(sf, v)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", sf.Name, err)
		}
		out[sf.Name] = enc
	}
	return out, nil
}

func encodeField(sf SchemaField, v any) (any, error) {
	typ := sf.Type
	switch {
	case strings.HasPrefix(typ, "list<"):
		return encodeList(sf, typ[5:len(typ)-1], v)
	case strings.HasPrefix(typ, "optional<"):
		return encodeOptional(sf, typ[9:len(typ)-1], v)
	case strings.HasPrefix(typ, "enum<"):
		sv, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected a string enum variant, got %T", v)
		}
		return sv, nil
	case strings.HasPrefix(typ, "oneof<"):
		return encodeOneOf(sf, v)
	case strings.HasPrefix(typ, "array<"):
		// An array encodes exactly like the list it is.
		return encodeList(sf, arrayElemType(typ), v)
	case strings.HasPrefix(typ, "map<"):
		m, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected map[string]any for map, got %T", v)
		}
		valType := mapValueType(typ)
		out := make(map[string]any, len(m))
		for k, mv := range m {
			enc, err := encodeScalar(valType, sf.Fields, mv)
			if err != nil {
				return nil, fmt.Errorf("map[%q]: %w", k, err)
			}
			out[k] = enc
		}
		return out, nil
	default:
		return encodeScalar(typ, sf.Fields, v)
	}
}

// encodeOneOf is the inverse of decodeOneOf: a *Variant becomes {tag: payload}.
func encodeOneOf(sf SchemaField, v any) (any, error) {
	variant, ok := v.(*Variant)
	if !ok {
		return nil, fmt.Errorf("expected *Variant for oneof, got %T", v)
	}
	sv := findSchemaVariant(sf.Variants, variant.Tag)
	if sv == nil {
		return nil, fmt.Errorf("unknown oneof variant %q", variant.Tag)
	}
	if sv.Payload == nil {
		return map[string]any{variant.Tag: nil}, nil
	}
	tmp := NewFieldMap()
	tmp.fields[sv.Payload.Name] = variant.Value
	enc, err := EncodeFieldMap(tmp, []SchemaField{*sv.Payload})
	if err != nil {
		return nil, fmt.Errorf("variant %q: %w", variant.Tag, err)
	}
	return map[string]any{variant.Tag: enc[sv.Payload.Name]}, nil
}

// encodeScalar is the inverse of decodeScalar: it turns a stored Go value back
// into its wire JSON representation.
func encodeScalar(typ string, fields []SchemaField, v any) (any, error) {
	switch typ {
	case typekind.Uint8,
		typekind.Uint16,
		typekind.Uint32,
		typekind.Int8,
		typekind.Int16,
		typekind.Int32,
		typekind.Bool,
		typekind.String:
		return v, nil
	case typekind.Uint128, typekind.Int128:
		return bigToString(v)
	case typekind.Uint64:
		n, ok := v.(uint64)
		if !ok {
			return nil, fmt.Errorf("expected uint64, got %T", v)
		}
		return strconv.FormatUint(n, 10), nil
	case typekind.Int64:
		n, ok := v.(int64)
		if !ok {
			return nil, fmt.Errorf("expected int64, got %T", v)
		}
		return strconv.FormatInt(n, 10), nil
	case typekind.Float32:
		f, ok := v.(float32)
		if !ok {
			return nil, fmt.Errorf("expected float32, got %T", v)
		}
		return floatHex(uint64(math.Float32bits(f)), float32ByteLen), nil
	case typekind.Float64:
		f, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("expected float64, got %T", v)
		}
		return floatHex(math.Float64bits(f), float64ByteLen), nil
	case typekind.Bytes:
		b, ok := v.([]byte)
		if !ok {
			return nil, fmt.Errorf("expected []byte, got %T", v)
		}
		return hex.EncodeToString(b), nil
	case typekind.Struct:
		nested, ok := v.(*FieldMap)
		if !ok {
			return nil, fmt.Errorf("expected *FieldMap for struct, got %T", v)
		}
		return EncodeFieldMap(nested, fields)
	default:
		return nil, fmt.Errorf("unknown type %q", typ)
	}
}

// encodeList is the inverse of decodeList and mirrors its shape: every element
// goes back out through encodeScalar, and the switch only names the Go slice the
// value is expected to be stored as.
func encodeList(sf SchemaField, elemType string, v any) (any, error) {
	switch elemType {
	case typekind.Uint8:
		return encodeEachList[uint8](sf, elemType, v)
	case typekind.Uint16:
		return encodeEachList[uint16](sf, elemType, v)
	case typekind.Uint32:
		return encodeEachList[uint32](sf, elemType, v)
	case typekind.Uint64:
		return encodeEachList[uint64](sf, elemType, v)
	case typekind.Int8:
		return encodeEachList[int8](sf, elemType, v)
	case typekind.Int16:
		return encodeEachList[int16](sf, elemType, v)
	case typekind.Int32:
		return encodeEachList[int32](sf, elemType, v)
	case typekind.Int64:
		return encodeEachList[int64](sf, elemType, v)
	case typekind.Uint128, typekind.Int128:
		return encodeEachList[*big.Int](sf, elemType, v)
	case typekind.Float32:
		return encodeEachList[float32](sf, elemType, v)
	case typekind.Float64:
		return encodeEachList[float64](sf, elemType, v)
	case typekind.Bool:
		return encodeEachList[bool](sf, elemType, v)
	case typekind.String:
		return encodeEachList[string](sf, elemType, v)
	case typekind.Bytes:
		return encodeEachList[[]byte](sf, elemType, v)
	case typekind.Struct:
		return encodeEachList[*FieldMap](sf, elemType, v)
	default:
		return nil, fmt.Errorf("unsupported list element type %q", elemType)
	}
}

func encodeOptional(sf SchemaField, elemType string, v any) (any, error) {
	if v == nil {
		return nil, nil //nolint:nilnil // sentinel error not appropriate: EncodeFieldMap callers check err != nil
	}
	switch elemType {
	case typekind.String:
		switch x := v.(type) {
		case string:
			return x, nil
		case *string:
			if x == nil {
				return nil, nil //nolint:nilnil // sentinel error not appropriate: EncodeFieldMap callers check err != nil
			}
			return *x, nil
		}
		return nil, fmt.Errorf("optional<string>: unexpected type %T", v)
	default:
		// struct and scalar optionals both round-trip through encodeScalar.
		return encodeScalar(elemType, sf.Fields, v)
	}
}

// --- shared value converters -------------------------------------------------

// realNumber is the set of fixed-width integer types decoded from a JSON number.
// 64/128-bit integers are sent as strings, so they are not included here.
type realNumber interface {
	~int8 | ~int16 | ~int32 | ~uint8 | ~uint16 | ~uint32
}

// numFromJSON unmarshals a JSON number into T (small ints encoded as numbers).
func numFromJSON[T realNumber](r json.RawMessage) (T, error) {
	var n float64
	if err := json.Unmarshal(r, &n); err != nil {
		return 0, err
	}
	return T(n), nil
}

// jsonValue unmarshals a JSON value directly into T (used for bool/string).
func jsonValue[T any](r json.RawMessage) (T, error) {
	var v T
	err := json.Unmarshal(r, &v)
	return v, err
}

// uintFromString decodes a u64/u128 value sent as a decimal string.
func uintFromString(r json.RawMessage) (uint64, error) {
	var s string
	if err := json.Unmarshal(r, &s); err != nil {
		return 0, err
	}
	return strconv.ParseUint(s, 10, 64)
}

// intFromString decodes an i64 value sent as a decimal string.
func intFromString(r json.RawMessage) (int64, error) {
	var s string
	if err := json.Unmarshal(r, &s); err != nil {
		return 0, err
	}
	return strconv.ParseInt(s, 10, 64)
}

// Bounds for the 128-bit kinds. Go has no native 128-bit integer, so u128/i128
// values are carried as *big.Int — the only lossless option. They arrive as
// decimal strings, and parsing them into uint64/int64 (as this library used to)
// silently truncates every value above 2^64.
const (
	// int128Bits is the width of the 128-bit kinds; int128SignBits is the width of
	// their magnitude, i.e. one bit less for the sign.
	int128Bits     = 128
	int128SignBits = 127
	// decimalBase: u128/i128 travel as base-10 strings.
	decimalBase = 10
)

var (
	maxU128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), int128Bits), big.NewInt(1))
	maxI128 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), int128SignBits), big.NewInt(1))
	minI128 = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), int128SignBits))
)

// bigFromString decodes a u128/i128 value sent as a decimal string, rejecting
// anything outside the range of the declared kind rather than wrapping it.
func bigFromString(r json.RawMessage, signed bool) (*big.Int, error) {
	var s string
	if err := json.Unmarshal(r, &s); err != nil {
		return nil, err
	}
	n, ok := new(big.Int).SetString(s, decimalBase)
	if !ok {
		return nil, fmt.Errorf("invalid 128-bit integer %q", s)
	}
	if signed {
		if n.Cmp(minI128) < 0 || n.Cmp(maxI128) > 0 {
			return nil, fmt.Errorf("int128 out of range: %s", s)
		}
		return n, nil
	}
	if n.Sign() < 0 || n.Cmp(maxU128) > 0 {
		return nil, fmt.Errorf("uint128 out of range: %s", s)
	}
	return n, nil
}

// bigToString renders a *big.Int back onto the wire as a decimal string.
func bigToString(v any) (string, error) {
	n, ok := v.(*big.Int)
	if !ok {
		return "", fmt.Errorf("expected *big.Int for a 128-bit field, got %T", v)
	}
	if n == nil {
		return "0", nil
	}
	return n.String(), nil
}

// hexBytes decodes a hex string of exactly n bytes (floats are sent as LE hex).
func hexBytes(r json.RawMessage, n int) ([]byte, error) {
	var s string
	if err := json.Unmarshal(r, &s); err != nil {
		return nil, err
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != n {
		return nil, fmt.Errorf("expected %d hex bytes, got %d", n, len(b))
	}
	return b, nil
}

// floatHex renders float bits as an n-byte little-endian hex string.
func floatHex(bits uint64, n int) string {
	b := make([]byte, float64ByteLen)
	binary.LittleEndian.PutUint64(b, bits)
	return hex.EncodeToString(b[:n])
}

// collectList decodes every element of arr through decodeScalar and gathers the
// results into []T, the slice type a FieldMap stores for that element type.
func collectList[T any](sf SchemaField, elemType string, arr []json.RawMessage) ([]T, error) {
	out := make([]T, len(arr))
	for i, item := range arr {
		v, err := decodeScalar(elemType, sf.Fields, item)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		t, ok := v.(T)
		if !ok {
			return nil, fmt.Errorf("[%d]: expected %T, got %T", i, *new(T), v)
		}
		out[i] = t
	}
	return out, nil
}

// encodeEachList is the inverse of collectList: it asserts the stored value is
// []T and sends every element back out through encodeScalar.
func encodeEachList[T any](sf SchemaField, elemType string, v any) ([]any, error) {
	arr, ok := v.([]T)
	if !ok {
		return nil, fmt.Errorf("expected []%T, got %T", *new(T), v)
	}
	out := make([]any, len(arr))
	for i, x := range arr {
		enc, err := encodeScalar(elemType, sf.Fields, x)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out[i] = enc
	}
	return out, nil
}

// EnumVariants returns the variants of an enum type string ("enum<a,b,c>"), in
// declaration order, so a worker can map a variant name onto an ordinal for its
// byte layout. It returns nil for any other type.
func EnumVariants(typ string) []string {
	if !strings.HasPrefix(typ, "enum<") || !strings.HasSuffix(typ, ">") {
		return nil
	}
	inner := typ[len("enum<") : len(typ)-1]
	if inner == "" {
		return nil
	}
	parts := strings.Split(inner, ",")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}
