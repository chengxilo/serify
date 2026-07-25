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
	"errors"
	"fmt"
	"math/big"
	"reflect"
)

// ErrNilField is returned by GetStruct-like methods when a field exists but is nil.
var ErrNilField = errors.New("field is nil")

const errFieldNotFound = "field %q not found"

// FieldMap is a typed key-value store used by both serialize and deserialize paths.
type FieldMap struct {
	fields map[string]any
}

func NewFieldMap() *FieldMap { return &FieldMap{fields: make(map[string]any)} }

func (f *FieldMap) GetU8(key string) (uint8, error)    { return getTyped[uint8](f, key) }
func (f *FieldMap) GetU16(key string) (uint16, error)  { return getTyped[uint16](f, key) }
func (f *FieldMap) GetU32(key string) (uint32, error)  { return getTyped[uint32](f, key) }
func (f *FieldMap) GetU64(key string) (uint64, error)  { return getTyped[uint64](f, key) }
func (f *FieldMap) GetI8(key string) (int8, error)     { return getTyped[int8](f, key) }
func (f *FieldMap) GetI16(key string) (int16, error)   { return getTyped[int16](f, key) }
func (f *FieldMap) GetI32(key string) (int32, error)   { return getTyped[int32](f, key) }
func (f *FieldMap) GetI64(key string) (int64, error)   { return getTyped[int64](f, key) }
func (f *FieldMap) GetF32(key string) (float32, error) { return getTyped[float32](f, key) }
func (f *FieldMap) GetF64(key string) (float64, error) { return getTyped[float64](f, key) }
func (f *FieldMap) GetBool(key string) (bool, error)   { return getTyped[bool](f, key) }

// GetBig returns a uint128/int128 field. Go has no native 128-bit integer, so
// both kinds are carried as *big.Int; the protocol layer has already range-checked
// the value against the declared kind.
func (f *FieldMap) GetBig(key string) (*big.Int, error) { return getTyped[*big.Int](f, key) }

func (f *FieldMap) GetListBig(key string) ([]*big.Int, error) {
	return getTyped[[]*big.Int](f, key)
}

func (f *FieldMap) GetString(key string) (string, error) { return getTyped[string](f, key) }
func (f *FieldMap) GetBytes(key string) ([]byte, error)  { return getTyped[[]byte](f, key) }

func (f *FieldMap) GetListString(key string) ([]string, error) {
	return getTyped[[]string](f, key)
}
func (f *FieldMap) GetListU8(key string) ([]uint8, error)     { return getTyped[[]uint8](f, key) }
func (f *FieldMap) GetListU16(key string) ([]uint16, error)   { return getTyped[[]uint16](f, key) }
func (f *FieldMap) GetListU32(key string) ([]uint32, error)   { return getTyped[[]uint32](f, key) }
func (f *FieldMap) GetListU64(key string) ([]uint64, error)   { return getTyped[[]uint64](f, key) }
func (f *FieldMap) GetListI8(key string) ([]int8, error)      { return getTyped[[]int8](f, key) }
func (f *FieldMap) GetListI16(key string) ([]int16, error)    { return getTyped[[]int16](f, key) }
func (f *FieldMap) GetListI32(key string) ([]int32, error)    { return getTyped[[]int32](f, key) }
func (f *FieldMap) GetListI64(key string) ([]int64, error)    { return getTyped[[]int64](f, key) }
func (f *FieldMap) GetListF32(key string) ([]float32, error)  { return getTyped[[]float32](f, key) }
func (f *FieldMap) GetListF64(key string) ([]float64, error)  { return getTyped[[]float64](f, key) }
func (f *FieldMap) GetListBool(key string) ([]bool, error)    { return getTyped[[]bool](f, key) }
func (f *FieldMap) GetListBytes(key string) ([][]byte, error) { return getTyped[[][]byte](f, key) }
func (f *FieldMap) GetListStruct(key string) ([]*FieldMap, error) {
	return getTyped[[]*FieldMap](f, key)
}

func (f *FieldMap) GetOptionalString(key string) (*string, error) {
	v, ok := f.fields[key]
	if !ok {
		return nil, fmt.Errorf(errFieldNotFound, key)
	}
	if v == nil {
		return nil, nil //nolint:nilnil // sentinel error not appropriate: reflect.go caller checks err != nil
	}
	switch x := v.(type) {
	case string:
		s := x
		return &s, nil
	case *string:
		return x, nil
	}
	return nil, fmt.Errorf("field %q: expected optional string, got %T", key, v)
}

func (f *FieldMap) GetStruct(key string) (*FieldMap, error) {
	v, ok := f.fields[key]
	if !ok {
		return nil, fmt.Errorf(errFieldNotFound, key)
	}
	if v == nil {
		return nil, ErrNilField
	}
	fm, ok := v.(*FieldMap)
	if !ok {
		return nil, fmt.Errorf("field %q: expected *FieldMap, got %T", key, v)
	}
	return fm, nil
}

// GetOptionalStruct is identical to GetStruct: both a present struct and a null
// optional<struct> are stored as a *FieldMap (or nil), so the same lookup works.
func (f *FieldMap) GetOptionalStruct(key string) (*FieldMap, error) {
	return f.GetStruct(key)
}

// An array<T,N> is stored as the same []T a list<T> is, so it is read and
// written through the GetList*/SetList* accessors above — there is no separate
// array accessor family. The dedicated ones that used to live here only ever
// spoke [4]uint32, which is why array<T,N> supported exactly that one shape.

func (f *FieldMap) SetU8(key string, v uint8)      { f.fields[key] = v }
func (f *FieldMap) SetU16(key string, v uint16)    { f.fields[key] = v }
func (f *FieldMap) SetU32(key string, v uint32)    { f.fields[key] = v }
func (f *FieldMap) SetU64(key string, v uint64)    { f.fields[key] = v }
func (f *FieldMap) SetI8(key string, v int8)       { f.fields[key] = v }
func (f *FieldMap) SetI16(key string, v int16)     { f.fields[key] = v }
func (f *FieldMap) SetI32(key string, v int32)     { f.fields[key] = v }
func (f *FieldMap) SetI64(key string, v int64)     { f.fields[key] = v }
func (f *FieldMap) SetF32(key string, v float32)   { f.fields[key] = v }
func (f *FieldMap) SetF64(key string, v float64)   { f.fields[key] = v }
func (f *FieldMap) SetBool(key string, v bool)     { f.fields[key] = v }
func (f *FieldMap) SetBig(key string, v *big.Int)  { f.fields[key] = v }
func (f *FieldMap) SetString(key string, v string) { f.fields[key] = v }
func (f *FieldMap) SetBytes(key string, v []byte)  { f.fields[key] = v }

func (f *FieldMap) SetListString(key string, v []string)    { f.fields[key] = v }
func (f *FieldMap) SetListU8(key string, v []uint8)         { f.fields[key] = v }
func (f *FieldMap) SetListU16(key string, v []uint16)       { f.fields[key] = v }
func (f *FieldMap) SetListU32(key string, v []uint32)       { f.fields[key] = v }
func (f *FieldMap) SetListU64(key string, v []uint64)       { f.fields[key] = v }
func (f *FieldMap) SetListI8(key string, v []int8)          { f.fields[key] = v }
func (f *FieldMap) SetListI16(key string, v []int16)        { f.fields[key] = v }
func (f *FieldMap) SetListI32(key string, v []int32)        { f.fields[key] = v }
func (f *FieldMap) SetListI64(key string, v []int64)        { f.fields[key] = v }
func (f *FieldMap) SetListF32(key string, v []float32)      { f.fields[key] = v }
func (f *FieldMap) SetListF64(key string, v []float64)      { f.fields[key] = v }
func (f *FieldMap) SetListBool(key string, v []bool)        { f.fields[key] = v }
func (f *FieldMap) SetListBytes(key string, v [][]byte)     { f.fields[key] = v }
func (f *FieldMap) SetListBig(key string, v []*big.Int)     { f.fields[key] = v }
func (f *FieldMap) SetListStruct(key string, v []*FieldMap) { f.fields[key] = v }
func (f *FieldMap) SetStruct(key string, v *FieldMap)       { f.fields[key] = v }

// Variant is one arm of a sum: a tag and its decoded payload (nil for a unit
// variant). A sum field stores a *Variant.
type Variant struct {
	Tag   string
	Value any
}

// SetVariant stores a sum value: the active variant's tag and payload (pass a
// nil value for a unit variant).
func (f *FieldMap) SetVariant(key, tag string, value any) {
	f.fields[key] = &Variant{Tag: tag, Value: value}
}

// GetVariant returns the sum value stored at key.
func (f *FieldMap) GetVariant(key string) (*Variant, error) {
	v, ok := f.fields[key]
	if !ok {
		return nil, fmt.Errorf(errFieldNotFound, key)
	}
	vr, ok := v.(*Variant)
	if !ok {
		return nil, fmt.Errorf("field %q is not a variant (got %T)", key, v)
	}
	return vr, nil
}
func (f *FieldMap) SetOptionalString(key string, v *string) {
	if v == nil {
		f.fields[key] = nil
	} else {
		f.fields[key] = *v
	}
}
func (f *FieldMap) SetOptionalStruct(key string, v *FieldMap) {
	if v == nil {
		f.fields[key] = nil // store untyped nil so encodeOptional sees v==nil
	} else {
		f.fields[key] = v
	}
}

func (f *FieldMap) SetMap(key string, v map[string]any) { f.fields[key] = v }
func (f *FieldMap) GetMap(key string) (map[string]any, bool) {
	v, ok := f.fields[key]
	if !ok {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

func getTyped[T any](f *FieldMap, key string) (T, error) {
	var zero T
	v, ok := f.fields[key]
	if !ok {
		return zero, fmt.Errorf(errFieldNotFound, key)
	}
	t, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("field %q: expected %T, got %T", key, zero, v)
	}
	return t, nil
}

// setScalar stores val under key if it is one of the Go types that maps directly
// onto a serify scalar, and reports whether it did. Every caller that accepts a
// value of unknown type starts here and then handles whatever else it allows, so
// the scalars are enumerated once.
func setScalar(fm *FieldMap, key string, val any) bool {
	switch v := val.(type) {
	case uint8:
		fm.SetU8(key, v)
	case uint16:
		fm.SetU16(key, v)
	case uint32:
		fm.SetU32(key, v)
	case uint64:
		fm.SetU64(key, v)
	case int8:
		fm.SetI8(key, v)
	case int16:
		fm.SetI16(key, v)
	case int32:
		fm.SetI32(key, v)
	case int64:
		fm.SetI64(key, v)
	case float32:
		fm.SetF32(key, v)
	case float64:
		fm.SetF64(key, v)
	case bool:
		fm.SetBool(key, v)
	case string:
		fm.SetString(key, v)
	case []byte:
		fm.SetBytes(key, v)
	case *big.Int:
		// uint128/int128; the only scalar with no fixed-width Go type.
		fm.SetBig(key, v)
	default:
		return false
	}
	return true
}

// MapOf builds a FieldMap from alternating key/value pairs for simple cases.
func MapOf(kvs ...any) *FieldMap {
	fm := NewFieldMap()
	for i := 0; i+1 < len(kvs); i += 2 {
		key, _ := kvs[i].(string)
		val := kvs[i+1]
		if setScalar(fm, key, val) {
			continue
		}
		switch v := val.(type) {
		case []string:
			fm.SetListString(key, v)
		case *string:
			fm.SetOptionalString(key, v)
		case *FieldMap:
			fm.SetStruct(key, v)
		case []*FieldMap:
			fm.SetListStruct(key, v)
		default:
			// A Go array is how a worker spells an array<T,N> field; it is stored
			// as the []T a list<T> would be. Anything else is stored as-is rather
			// than dropped — silently ignoring an unrecognised value is how a
			// field goes missing with nothing reported.
			rv := reflect.ValueOf(v)
			if rv.IsValid() && rv.Kind() == reflect.Array {
				out := reflect.MakeSlice(reflect.SliceOf(rv.Type().Elem()), rv.Len(), rv.Len())
				reflect.Copy(out, rv)
				fm.fields[key] = out.Interface()
				continue
			}
			fm.fields[key] = v
		}
	}
	return fm
}
