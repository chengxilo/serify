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
	"fmt"
	"maps"
	"math/big"
	"reflect"

	"github.com/iancoleman/strcase"
)

func reflectKey(f reflect.StructField) string {
	if tag := f.Tag.Get("serify"); tag != "" {
		return tag
	}
	return strcase.ToSnake(f.Name)
}

// reflectCodec bridges a struct and a FieldMap, optionally applying per-type
// Converters registered on the Suite. A zero reflectCodec (nil converters) is
// the plain built-in mapping.
type reflectCodec struct {
	converters map[reflect.Type]Converter
}

// reflectFill and reflectExtract are the no-converter entry points, kept for
// call sites and tests that do not customize any type.
func reflectFill(v reflect.Value, fm *FieldMap) error { return reflectCodec{}.fill(v, fm) }
func reflectExtract(v reflect.Value, fm *FieldMap)    { reflectCodec{}.extract(v, fm) }

// promotable reports whether f is an anonymous embedded struct field to flatten
// into the parent — the same promotion Go and encoding/json apply, so an SDK
// type that groups fields in an embedded struct maps to a flat schema without a
// shim. A tagged embed is treated as a named field, not promoted; a non-struct
// embed has nothing to promote; a type with its own whole-type converter is a
// value, not a group to flatten.
func (c reflectCodec) promotable(f reflect.StructField) bool {
	if !f.Anonymous || f.Tag.Get("serify") != "" {
		return false
	}
	ft := f.Type
	if ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	if ft.Kind() != reflect.Struct {
		return false
	}
	_, hasConv := c.converters[ft]
	return !hasConv
}

// fill populates a struct from a FieldMap using `serify:` tags or snake_case
// field names. Nested structs are handled recursively; anonymous embedded
// structs are flattened into the same level (see promotable).
//
//nolint:gocognit,gocyclo,cyclop,funlen // one flat `case` per Go scalar/collection kind. The score counts cases, not tangle; splitting it would just relocate the switch.
func (c reflectCodec) fill(v reflect.Value, fm *FieldMap) error {
	t := v.Type()
	// A converter registered for the whole type (not just a field) turns the
	// entire FieldMap into the value — this is how a top-level sum type maps to
	// an SDK type built through a constructor, e.g. iggy's identifier.
	if conv, ok := c.converters[t]; ok {
		out, err := conv.from(fm)
		if err != nil {
			return fmt.Errorf("converter From for %s: %w", t, err)
		}
		ov := reflect.ValueOf(out)
		if !ov.IsValid() || !ov.Type().AssignableTo(t) {
			return fmt.Errorf("converter for %s returned %T, not assignable", t, out)
		}
		v.Set(ov)
		return nil
	}
	for i := range t.NumField() {
		f := t.Field(i)
		fv := v.Field(i)
		if c.promotable(f) {
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
			}
			if err := c.fill(fv, fm); err != nil {
				return err
			}
			continue
		}
		key := reflectKey(f)
		raw, exists := fm.fields[key]
		if !exists {
			continue
		}
		if conv, ok := c.converters[fv.Type()]; ok {
			out, err := conv.from(raw)
			if err != nil {
				return fmt.Errorf("field %q: converter From: %w", key, err)
			}
			ov := reflect.ValueOf(out)
			if !ov.IsValid() || !ov.Type().AssignableTo(fv.Type()) {
				return fmt.Errorf("field %q: converter returned %T, not assignable to %s", key, out, fv.Type())
			}
			fv.Set(ov)
			continue
		}
		switch fv.Interface().(type) {
		case uint8:
			n, err := fm.GetU8(key)
			if err != nil {
				return err
			}
			fv.SetUint(uint64(n))
		case uint16:
			n, err := fm.GetU16(key)
			if err != nil {
				return err
			}
			fv.SetUint(uint64(n))
		case uint32:
			n, err := fm.GetU32(key)
			if err != nil {
				return err
			}
			fv.SetUint(uint64(n))
		case uint64:
			n, err := fm.GetU64(key)
			if err != nil {
				return err
			}
			fv.SetUint(n)
		case int8:
			n, err := fm.GetI8(key)
			if err != nil {
				return err
			}
			fv.SetInt(int64(n))
		case int16:
			n, err := fm.GetI16(key)
			if err != nil {
				return err
			}
			fv.SetInt(int64(n))
		case int32:
			n, err := fm.GetI32(key)
			if err != nil {
				return err
			}
			fv.SetInt(int64(n))
		case int64:
			n, err := fm.GetI64(key)
			if err != nil {
				return err
			}
			fv.SetInt(n)
		case float32:
			n, err := fm.GetF32(key)
			if err != nil {
				return err
			}
			fv.SetFloat(float64(n))
		case float64:
			n, err := fm.GetF64(key)
			if err != nil {
				return err
			}
			fv.SetFloat(n)
		case bool:
			b, err := fm.GetBool(key)
			if err != nil {
				return err
			}
			fv.SetBool(b)
		case string:
			s, err := fm.GetString(key)
			if err != nil {
				return err
			}
			fv.SetString(s)
		case []byte:
			b, err := fm.GetBytes(key)
			if err != nil {
				return err
			}
			fv.SetBytes(b)
		case []string:
			ss, err := fm.GetListString(key)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(ss))
		case *string:
			sp, err := fm.GetOptionalString(key)
			if err != nil {
				return err
			}
			if sp == nil {
				fv.Set(reflect.Zero(fv.Type()))
			} else {
				fv.Set(reflect.ValueOf(sp))
			}
		case *big.Int:
			// uint128/int128: Go has no native 128-bit integer, so a worker
			// declares the field as *big.Int.
			n, err := fm.GetBig(key)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(n))
		case []*big.Int:
			ns, err := fm.GetListBig(key)
			if err != nil {
				return err
			}
			fv.Set(reflect.ValueOf(ns))
		default:
			switch {
			case fv.Kind() == reflect.Struct:
				nested, err := fm.GetStruct(key)
				if err != nil {
					return fmt.Errorf("field %q: declared %s but case data is not a struct: %w", key, fv.Type(), err)
				}
				if nested == nil {
					return fmt.Errorf("field %q: declared %s but case data is null", key, fv.Type())
				}
				if err := c.fill(fv, nested); err != nil {
					return fmt.Errorf("field %q: %w", key, err)
				}
			case fv.Kind() == reflect.Array:
				// An array<T,N> is stored as the []T a list<T> would be, so this
				// copies out of that slice into the fixed-size Go array.
				raw, ok := fm.fields[key]
				if !ok {
					return fmt.Errorf("field %q: declared %s but case data has no such field", key, fv.Type())
				}
				rv := reflect.ValueOf(raw)
				if !rv.IsValid() || rv.Kind() != reflect.Slice {
					return fmt.Errorf("field %q: declared %s but case data decoded as %T", key, fv.Type(), raw)
				}
				if rv.Len() != fv.Len() {
					return fmt.Errorf("field %q: declared %s but case data has %d elements", key, fv.Type(), rv.Len())
				}
				for i := range fv.Len() {
					if err := c.setArrayElem(fv.Index(i), rv.Index(i).Interface()); err != nil {
						return fmt.Errorf("field %q[%d]: %w", key, i, err)
					}
				}
			case fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.Struct:
				nested, err := fm.GetListStruct(key)
				if err != nil {
					return fmt.Errorf("field %q: %w", key, err)
				}
				elemType := fv.Type().Elem()
				// Build a non-nil slice even when empty: a nil []T marshals to
				// JSON null where an empty one marshals to [], and workers in
				// other languages emit [].
				out := reflect.MakeSlice(fv.Type(), len(nested), len(nested))
				for i, n := range nested {
					elem := reflect.New(elemType).Elem()
					if err := c.fill(elem, n); err != nil {
						return fmt.Errorf("field %q[%d]: %w", key, i, err)
					}
					out.Index(i).Set(elem)
				}
				fv.Set(out)
			case fv.Kind() == reflect.Map && fv.Type().Key().Kind() == reflect.String:
				m, ok := fm.GetMap(key)
				if !ok {
					return fmt.Errorf("field %q: declared %s but case data is not a map", key, fv.Type())
				}
				newMap := reflect.MakeMap(fv.Type())
				elemType := fv.Type().Elem()
				for k, v := range m {
					if elemType.Kind() == reflect.Struct {
						nested, ok := v.(*FieldMap)
						if !ok {
							continue
						}
						elem := reflect.New(elemType).Elem()
						if err := c.fill(elem, nested); err != nil {
							return fmt.Errorf("field %q map[%q]: %w", key, k, err)
						}
						newMap.SetMapIndex(reflect.ValueOf(k), elem)
					} else {
						rv := reflect.ValueOf(v)
						if rv.IsValid() && rv.Type().ConvertibleTo(elemType) {
							newMap.SetMapIndex(reflect.ValueOf(k), rv.Convert(elemType))
						}
					}
				}
				fv.Set(newMap)
			case fv.Kind() == reflect.Pointer:
				// optional<T>: decodeOptional stores either nil, the concrete
				// scalar, or a *FieldMap for a struct. Only *string and *big.Int
				// used to be handled here, which is why a model could express
				// neither optional<float32> nor optional<struct>.
				raw, ok := fm.fields[key]
				if !ok || raw == nil {
					fv.Set(reflect.Zero(fv.Type()))
					continue
				}
				elemType := fv.Type().Elem()
				p := reflect.New(elemType)
				if nested, isFM := raw.(*FieldMap); isFM && elemType.Kind() == reflect.Struct {
					if err := c.fill(p.Elem(), nested); err != nil {
						return fmt.Errorf("field %q: %w", key, err)
					}
					fv.Set(p)
					continue
				}
				// A *T defers to T's converter. How a type occupies a field is
				// T's decision; an enclosing pointer must not re-decide it, or a
				// converted type stops being converted the moment it is optional.
				if conv, ok := c.converters[elemType]; ok {
					out, err := conv.from(raw)
					if err != nil {
						return fmt.Errorf("field %q: converter From: %w", key, err)
					}
					ov := reflect.ValueOf(out)
					if !ov.IsValid() || !ov.Type().AssignableTo(elemType) {
						return fmt.Errorf("field %q: converter returned %T, not assignable to %s", key, out, elemType)
					}
					p.Elem().Set(ov)
					fv.Set(p)
					continue
				}
				rv := reflect.ValueOf(raw)
				if !rv.Type().AssignableTo(fv.Type().Elem()) {
					return fmt.Errorf("field %q: declared %s but case data decoded as %T", key, fv.Type(), raw)
				}
				p.Elem().Set(rv)
				fv.Set(p)
			case fv.Kind() == reflect.Slice:
				// Any remaining list: []bool, []uint16, []int8, [][]byte and so
				// on. decodeList stores exactly the []T the schema implies, so
				// the stored value is assignable as-is and this needs no arm per
				// element type — which is what kept several of them unreachable.
				raw, ok := fm.fields[key]
				if !ok {
					return fmt.Errorf("field %q: declared %s but case data has no such field", key, fv.Type())
				}
				rv := reflect.ValueOf(raw)
				if !rv.IsValid() || !rv.Type().AssignableTo(fv.Type()) {
					return fmt.Errorf("field %q: declared %s but case data decoded as %T", key, fv.Type(), raw)
				}
				fv.Set(rv)
			default:
				return fmt.Errorf("ReflectShim: unsupported field type %s for %q", fv.Type(), key)
			}
		}
	}
	return nil
}

// setArrayElem assigns a decoded array element v into the array slot dst,
// recursing for struct elements and converting between compatible numeric types.
func (c reflectCodec) setArrayElem(dst reflect.Value, v any) error {
	if dst.Kind() == reflect.Struct {
		nested, ok := v.(*FieldMap)
		if !ok || nested == nil {
			return nil
		}
		return c.fill(dst, nested)
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil
	}
	switch {
	case rv.Type().AssignableTo(dst.Type()):
		dst.Set(rv)
	case rv.Type().ConvertibleTo(dst.Type()):
		dst.Set(rv.Convert(dst.Type()))
	default:
		return fmt.Errorf("cannot assign %T to %s", v, dst.Type())
	}
	return nil
}

// extract reads a struct into a FieldMap using `serify:` tags or snake_case
// names. Nested structs are handled recursively.
//
//nolint:gocognit,cyclop,funlen // flat type switch, one case per Go scalar/collection kind. The score counts cases, not tangle.
func (c reflectCodec) extract(v reflect.Value, fm *FieldMap) {
	t := v.Type()
	// Whole-type converter (see fill): the value produces the entire FieldMap.
	if conv, ok := c.converters[t]; ok {
		enc := conv.to(v.Interface())
		encFM, ok := enc.(*FieldMap)
		if !ok {
			panic(fmt.Sprintf("serify: whole-type converter for %s must map To a *FieldMap, got %T", t, enc))
		}
		maps.Copy(fm.fields, encFM.fields)
		return
	}
	for i := range t.NumField() {
		f := t.Field(i)
		fv := v.Field(i)
		if c.promotable(f) {
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					continue
				}
				fv = fv.Elem()
			}
			c.extract(fv, fm)
			continue
		}
		key := reflectKey(f)
		if conv, ok := c.converters[fv.Type()]; ok {
			storeNative(fm, key, conv.to(fv.Interface()))
			continue
		}
		switch val := fv.Interface().(type) {
		case uint8:
			fm.SetU8(key, val)
		case uint16:
			fm.SetU16(key, val)
		case uint32:
			fm.SetU32(key, val)
		case uint64:
			fm.SetU64(key, val)
		case int8:
			fm.SetI8(key, val)
		case int16:
			fm.SetI16(key, val)
		case int32:
			fm.SetI32(key, val)
		case int64:
			fm.SetI64(key, val)
		case float32:
			fm.SetF32(key, val)
		case float64:
			fm.SetF64(key, val)
		case bool:
			fm.SetBool(key, val)
		case string:
			fm.SetString(key, val)
		case []byte:
			fm.SetBytes(key, val)
		case []string:
			fm.SetListString(key, val)
		case *string:
			fm.SetOptionalString(key, val)
		case *big.Int:
			fm.SetBig(key, val)
		case []*big.Int:
			fm.SetListBig(key, val)
		default:
			switch {
			case fv.Kind() == reflect.Struct:
				nested := NewFieldMap()
				c.extract(fv, nested)
				fm.SetStruct(key, nested)
			case fv.Kind() == reflect.Array:
				// Mirror of fill: emit the []T that encodeList expects, built
				// from the array's own element type.
				elemT := fv.Type().Elem()
				if elemT.Kind() == reflect.Struct {
					fm.fields[key] = c.extractElems(fv)
					continue
				}
				out := reflect.MakeSlice(reflect.SliceOf(elemT), fv.Len(), fv.Len())
				reflect.Copy(out, fv)
				fm.fields[key] = out.Interface()
			case fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.Struct:
				fm.SetListStruct(key, c.extractElems(fv))
			case fv.Kind() == reflect.Pointer:
				// Mirror of fill's optional<T> arm: nil, a nested FieldMap for a
				// struct (which is what encodeScalar expects), or the pointee.
				switch {
				case fv.IsNil():
					fm.fields[key] = nil
				case fv.Type().Elem().Kind() == reflect.Struct:
					nested := NewFieldMap()
					c.extract(fv.Elem(), nested)
					fm.fields[key] = nested
				default:
					// Mirror of fill: a *T is written through T's converter.
					if conv, ok := c.converters[fv.Type().Elem()]; ok {
						storeNative(fm, key, conv.to(fv.Elem().Interface()))
						continue
					}
					fm.fields[key] = fv.Elem().Interface()
				}
			case fv.Kind() == reflect.Slice:
				// Mirror of fill's generic slice arm: encodeList expects exactly
				// the []T the schema implies, which is what the field already is.
				fm.fields[key] = fv.Interface()
			case fv.Kind() == reflect.Map && fv.Type().Key().Kind() == reflect.String:
				out := make(map[string]any, fv.Len())
				for _, k := range fv.MapKeys() {
					mv := fv.MapIndex(k)
					if mv.Kind() == reflect.Struct {
						nested := NewFieldMap()
						c.extract(mv, nested)
						out[k.String()] = nested
					} else {
						out[k.String()] = mv.Interface()
					}
				}
				fm.SetMap(key, out)
			}
		}
	}
}

// extractElems extracts every element of a struct-element slice or array into
// its own FieldMap. A list<struct> and an array<struct,N> are stored the same
// way, so both arms share this.
func (c *reflectCodec) extractElems(fv reflect.Value) []*FieldMap {
	out := make([]*FieldMap, fv.Len())
	for i := range fv.Len() {
		nested := NewFieldMap()
		c.extract(fv.Index(i), nested)
		out[i] = nested
	}
	return out
}

// storeNative writes a Converter's To output — a serify-native value — into the
// FieldMap under key. To is contracted to be total and to return one of these
// types; anything else is a converter bug, surfaced on the first serialize as a
// panic rather than a silently dropped field.
func storeNative(fm *FieldMap, key string, val any) {
	if setScalar(fm, key, val) {
		return
	}
	switch v := val.(type) {
	case *FieldMap:
		// A converter for a struct-shaped field (e.g. a nested identifier)
		// produces the field's own FieldMap.
		fm.SetStruct(key, v)
	case []*FieldMap:
		// list<struct>. A converter needs this whenever the SDK's own shape is
		// not a slice of structs — a map keyed by ID, say, which has to be put
		// in wire order before it can be compared.
		fm.SetListStruct(key, v)
	case *Variant:
		// Go has no sum type, so a oneof field is always a converter's job: the
		// model holds whatever shape the SDK uses and the converter names the
		// active variant.
		fm.fields[key] = v
	default:
		panic(fmt.Sprintf("serify: converter To for %q returned unsupported type %T", key, val))
	}
}
