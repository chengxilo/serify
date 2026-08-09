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
	"io"
	"reflect"
)

// SerializeFunc is any function with one of these signatures:
//
//	func(*T) ([]byte, error)
//	func(T)  ([]byte, error)
//	func(*T) []byte
//	func(T)  []byte
//	func(*T) string    (string is converted to []byte)
//	func(T)  string
type SerializeFunc = any

// DeserializeFunc is any function with one of these signatures:
//
//	func(*T, []byte) error    - in-place
//	func([]byte) (*T, error)  - factory
//	func(*T, string) error    - in-place, string input
//	func(string) (*T, error)  - factory, string input
type DeserializeFunc = any

// Format groups a serializer and deserializer under one format name.
// Either field may be nil to indicate that direction is not supported.
type Format struct {
	Serializer   SerializeFunc
	Deserializer DeserializeFunc
}

// Type describes one data type: Model is used for reflection-based field
// mapping (FieldMap <-> struct); Formats holds named encode/decode pairs.
//
// Model is a pointer to the worker's struct, and the format functions then take
// and return that struct — the FieldMap never reaches the worker. Leaving Model
// nil is the other path, for a type with no natural struct (the audit fixtures
// mutate a FieldMap on purpose): the functions must then be exactly
//
//	func(*FieldMap) ([]byte, error)   // serializer
//	func([]byte) (*FieldMap, error)   // deserializer
//
// Audit works either way, measured against whichever of the two the worker
// actually touches.
type Type struct {
	Model   any
	Formats map[string]Format
}

// Converter customizes how one Go type maps to and from a FieldMap, for model
// fields whose type serify cannot bridge on its own — typically an SDK type that
// stores a value differently from its schema shape (a 128-bit id held as a
// [16]byte where the schema declares uint128, a UUID, and so on).
//
// A Converter is registered per Go type on the Suite and applies to every model
// field of that exact type, at any nesting depth. Build one with NewConverter;
// the zero value is not usable.
type Converter struct {
	from func(any) (any, error)
	to   func(any) any
}

// NewConverter builds a Converter between a Go type T and its Carrier — the
// serify representation of the field, i.e. the value a FieldMap holds for the
// field's schema kind:
//
//	uint128 / int128  -> *big.Int
//	bytes             -> []byte
//	string            -> string
//	uint32, bool, …   -> that Go scalar
//	a struct / a whole flattened type -> *FieldMap
//
// Both T and Carrier are inferred from the function signatures, so the caller
// writes fully typed functions and never asserts an internal type:
//
//	serify.NewConverter(
//	    func(v *big.Int) (MessageID, error) { … },  // from: rebuild T (may fail)
//	    func(id MessageID) *big.Int { … },           // to:   render T (total)
//	)
//
// from runs on data crossing into the worker and may fail; to must be total.
func NewConverter[T, Carrier any](from func(Carrier) (T, error), to func(T) Carrier) Converter {
	return Converter{
		from: func(v any) (any, error) {
			c, ok := v.(Carrier)
			if !ok {
				var zero Carrier
				return nil, fmt.Errorf("serify: converter carrier is %T, want %T", v, zero)
			}
			return from(c)
		},
		to: func(field any) any { return to(field.(T)) },
	}
}

// Suite is the top-level worker configuration.
// In and Out default to os.Stdin / os.Stdout when nil.
type Suite struct {
	In    io.Reader
	Out   io.Writer
	Types map[string]Type
	// Converters maps a model field's Go type to a custom FieldMap translation.
	// Optional: a nil map leaves the built-in mapping in force for every field.
	Converters map[reflect.Type]Converter
}

// Accepted arities for the user-supplied serializer/deserializer signatures we
// validate by reflection.
const (
	// serOutAndError: func(T) ([]byte, error), as opposed to a bare func(T) []byte.
	serOutAndError = 2
	// deserInPlaceParams: func(*T, data) error, as opposed to a factory func(data) (*T, error).
	deserInPlaceParams = 2
	// deserFactoryOuts: the (*T, error) a factory deserializer returns.
	deserFactoryOuts = 2
)

// buildSerializer wraps a SerializeFunc into a typed func(*FieldMap)([]byte,error).
// model must be *T. Returns an error on signature mismatch.
// Returns a *serializeAuditHolder that the NDJSON loop can use to detect struct
// mutation during serialization when audit mode is enabled.
//
//nolint:gocognit,funlen // reflective signature validation: every accepted param/return shape needs its own check
func buildSerializer(
	model any, fn any, converters map[reflect.Type]Converter,
) (func(*FieldMap) ([]byte, error), *serializeAuditHolder, error) {
	if fn == nil {
		return nil, nil, errors.New("nil function")
	}
	codec := reflectCodec{converters: converters}
	fnVal := reflect.ValueOf(fn)
	if fnVal.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("expected function, got %T", fn)
	}
	fnType := fnVal.Type()

	if model == nil {
		return buildFieldMapSerializer(fn)
	}

	modelVal := reflect.ValueOf(model)
	if modelVal.Kind() != reflect.Pointer {
		return nil, nil, fmt.Errorf("Type.Model must be a pointer, got %T", model)
	}
	ptrType := modelVal.Type()
	elemType := ptrType.Elem()

	if fnType.NumIn() != 1 {
		return nil, nil, fmt.Errorf("serializer must have exactly 1 parameter, got %d", fnType.NumIn())
	}
	var ptrReceiver bool
	switch fnType.In(0) {
	case ptrType:
		ptrReceiver = true
	case elemType:
		ptrReceiver = false
	default:
		return nil, nil, fmt.Errorf("serializer parameter must be %v or %v, got %v", ptrType, elemType, fnType.In(0))
	}

	byteSliceType := reflect.TypeFor[[]byte]()
	stringType := reflect.TypeFor[string]()
	errorType := reflect.TypeFor[error]()
	var returnsString, returnsError bool

	switch fnType.NumOut() {
	case 1:
		switch fnType.Out(0) {
		case byteSliceType:
		case stringType:
			returnsString = true
		default:
			return nil, nil, fmt.Errorf("serializer must return []byte or string, got %v", fnType.Out(0))
		}
	case serOutAndError:
		if !fnType.Out(1).Implements(errorType) {
			return nil, nil, fmt.Errorf("serializer second return must implement error, got %v", fnType.Out(1))
		}
		returnsError = true
		switch fnType.Out(0) {
		case byteSliceType:
		case stringType:
			returnsString = true
		default:
			return nil, nil, fmt.Errorf("serializer first return must be []byte or string, got %v", fnType.Out(0))
		}
	default:
		return nil, nil, fmt.Errorf("serializer must return 1 or 2 values, got %d", fnType.NumOut())
	}

	holder := &serializeAuditHolder{}

	serFunc := func(fm *FieldMap) ([]byte, error) {
		msgPtr := reflect.New(elemType)
		if err := codec.fill(msgPtr.Elem(), fm); err != nil {
			return nil, err
		}
		var arg reflect.Value
		if ptrReceiver {
			arg = msgPtr
		} else {
			arg = msgPtr.Elem()
		}

		// Audit: snapshot struct before serialization (always, including value
		// receivers — their slice/map backing is shared with the model).
		var before *FieldMap
		if holder.Enabled {
			beforeFM := NewFieldMap()
			codec.extract(msgPtr.Elem(), beforeFM)
			before = SnapshotFieldMap(beforeFM)
		}

		results := fnVal.Call([]reflect.Value{arg})
		if returnsError && !results[1].IsNil() {
			err, _ := results[1].Interface().(error)
			return nil, err
		}

		var b []byte
		if returnsString {
			b = []byte(results[0].String())
		} else {
			b, _ = results[0].Interface().([]byte)
		}

		// Audit: mutation + output-ZC detection
		if holder.Enabled {
			// Mutation: compare struct after serialization
			afterFM := NewFieldMap()
			codec.extract(msgPtr.Elem(), afterFM)
			holder.LastMutations = CompareFieldMaps(before, afterFM)

			// Output zero-copy: does returned []byte alias model fields?
			holder.LastOutputZC = nil
			if !returnsString && len(b) > 0 {
				afterClone := SnapshotFieldMap(afterFM)
				xorFlip(b)
				flippedFM := NewFieldMap()
				codec.extract(msgPtr.Elem(), flippedFM)
				holder.LastOutputZC = CompareFieldMaps(afterClone, flippedFM)
				xorFlip(b) // restore (also restores any aliased model memory)
			}
		}

		return b, nil
	}
	return serFunc, holder, nil
}

// buildFieldMapSerializer handles a Type with no Model: the worker's function
// takes the FieldMap itself. Audit works one layer down from the model path —
// there is no struct to compare, so mutation and output aliasing are measured
// against the FieldMap the worker was handed, which is what it can reach.
func buildFieldMapSerializer(fn any) (func(*FieldMap) ([]byte, error), *serializeAuditHolder, error) {
	inner, ok := fn.(func(*FieldMap) ([]byte, error))
	if !ok {
		return nil, nil, fmt.Errorf(
			"with no Type.Model the serializer must be func(*FieldMap) ([]byte, error), got %T", fn)
	}

	holder := &serializeAuditHolder{}

	serFunc := func(fm *FieldMap) ([]byte, error) {
		var before *FieldMap
		if holder.Enabled {
			before = SnapshotFieldMap(fm)
		}

		b, err := inner(fm)
		if err != nil {
			return nil, err
		}

		if holder.Enabled {
			holder.LastMutations = CompareFieldMaps(before, fm)

			holder.LastOutputZC = nil
			if len(b) > 0 {
				afterClone := SnapshotFieldMap(fm)
				xorFlip(b)
				holder.LastOutputZC = CompareFieldMaps(afterClone, fm)
				xorFlip(b) // restore (also restores any aliased FieldMap memory)
			}
		}

		return b, nil
	}
	return serFunc, holder, nil
}

// parseDeserializer wraps a DeserializeFunc into a typed func([]byte)(*FieldMap,error).
//
//nolint:gocognit // reflective signature validation: every accepted param/return shape needs its own check
func parseDeserializer(
	model any, fn any, converters map[reflect.Type]Converter,
) (func([]byte) (*FieldMap, error), error) {
	if fn == nil {
		return nil, errors.New("nil function")
	}
	codec := reflectCodec{converters: converters}
	fnVal := reflect.ValueOf(fn)
	if fnVal.Kind() != reflect.Func {
		return nil, fmt.Errorf("expected function, got %T", fn)
	}
	fnType := fnVal.Type()

	if model == nil {
		inner, ok := fn.(func([]byte) (*FieldMap, error))
		if !ok {
			return nil, fmt.Errorf(
				"with no Type.Model the deserializer must be func([]byte) (*FieldMap, error), got %T", fn)
		}
		return inner, nil
	}

	modelVal := reflect.ValueOf(model)
	if modelVal.Kind() != reflect.Pointer {
		return nil, fmt.Errorf("Type.Model must be a pointer, got %T", model)
	}
	ptrType := modelVal.Type()
	elemType := ptrType.Elem()

	byteSliceType := reflect.TypeFor[[]byte]()
	stringType := reflect.TypeFor[string]()
	errorType := reflect.TypeFor[error]()

	switch fnType.NumIn() {
	case deserInPlaceParams:
		// in-place: func(*T, []byte) error  or  func(*T, string) error
		if fnType.In(0) != ptrType {
			return nil, fmt.Errorf("in-place deserializer first param must be %v, got %v", ptrType, fnType.In(0))
		}
		var stringInput bool
		switch fnType.In(1) {
		case byteSliceType:
		case stringType:
			stringInput = true
		default:
			return nil, fmt.Errorf("in-place deserializer second param must be []byte or string, got %v", fnType.In(1))
		}
		if fnType.NumOut() != 1 || !fnType.Out(0).Implements(errorType) {
			return nil, errors.New("in-place deserializer must return error")
		}
		return func(b []byte) (*FieldMap, error) {
			msgPtr := reflect.New(elemType)
			var arg1 reflect.Value
			if stringInput {
				arg1 = reflect.ValueOf(string(b))
			} else {
				arg1 = reflect.ValueOf(b)
			}
			results := fnVal.Call([]reflect.Value{msgPtr, arg1})
			if !results[0].IsNil() {
				err, _ := results[0].Interface().(error)
				return nil, err
			}
			fm := NewFieldMap()
			codec.extract(msgPtr.Elem(), fm)
			return fm, nil
		}, nil

	case 1:
		// factory: func([]byte) (*T, error)  or  func(string) (*T, error)
		var stringInput bool
		switch fnType.In(0) {
		case byteSliceType:
		case stringType:
			stringInput = true
		default:
			return nil, fmt.Errorf("factory deserializer param must be []byte or string, got %v", fnType.In(0))
		}
		if fnType.NumOut() != deserFactoryOuts {
			return nil, fmt.Errorf("factory deserializer must return (*T, error), got %d returns", fnType.NumOut())
		}
		if fnType.Out(0) != ptrType {
			return nil, fmt.Errorf("factory deserializer first return must be %v, got %v", ptrType, fnType.Out(0))
		}
		if !fnType.Out(1).Implements(errorType) {
			return nil, fmt.Errorf("factory deserializer second return must implement error, got %v", fnType.Out(1))
		}
		return func(b []byte) (*FieldMap, error) {
			var arg reflect.Value
			if stringInput {
				arg = reflect.ValueOf(string(b))
			} else {
				arg = reflect.ValueOf(b)
			}
			results := fnVal.Call([]reflect.Value{arg})
			if !results[1].IsNil() {
				err, _ := results[1].Interface().(error)
				return nil, err
			}
			fm := NewFieldMap()
			codec.extract(results[0].Elem(), fm)
			return fm, nil
		}, nil

	default:
		return nil, fmt.Errorf("deserializer must have 1 or 2 inputs, got %d", fnType.NumIn())
	}
}
