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
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func TestMapOf(t *testing.T) {
	p := strPtr("opt")
	fm := MapOf(
		"uint8", uint8(1),
		"uint16", uint16(2),
		"uint32", uint32(3),
		"uint64", uint64(4),
		"int8", int8(-1),
		"int16", int16(-2),
		"int32", int32(-3),
		"int64", int64(-4),
		"float32", float32(1.5),
		"float64", 2.5,
		"b", true,
		"s", "hello",
		"bs", []byte{0xAA},
		"ls", []string{"x"},
		"opt", p,
		"arr", [4]uint32{1, 2, 3, 4},
	)
	if v, _ := fm.GetU8("uint8"); v != 1 {
		t.Errorf("uint8: %d", v)
	}
	if v, _ := fm.GetU32("uint32"); v != 3 {
		t.Errorf("uint32: %d", v)
	}
	if v, _ := fm.GetI64("int64"); v != -4 {
		t.Errorf("int64: %d", v)
	}
	if v, _ := fm.GetF32("float32"); v != 1.5 {
		t.Errorf("float32: %v", v)
	}
	if v, _ := fm.GetBool("b"); !v {
		t.Error("bool: false")
	}
	if v, _ := fm.GetOptionalString("opt"); v == nil || *v != "opt" {
		t.Errorf("opt: %v", v)
	}
	if v, _ := fm.GetListU32("arr"); len(v) != 4 || v[0] != 1 || v[3] != 4 {
		t.Errorf("arr: %v", v)
	}
}

func TestMapOf_StructAndListStruct(t *testing.T) {
	inner := NewFieldMap()
	inner.SetU32("x", 7)
	b := NewFieldMap()
	b.SetString("k", "v")

	fm := MapOf(
		"s", inner,
		"ls", []*FieldMap{b},
	)
	if v, _ := fm.GetStruct("s"); v != inner {
		t.Error("struct not stored")
	}
	if v, _ := fm.GetListStruct("ls"); len(v) != 1 || v[0] != b {
		t.Error("list struct not stored")
	}
}

func TestMapOf_OddArgs(t *testing.T) {
	fm := MapOf("a", uint32(1), "orphan") // orphan has no value -> not stored
	if v, _ := fm.GetU32("a"); v != 1 {
		t.Errorf("a: %d", v)
	}
	if _, err := fm.GetString("orphan"); err == nil {
		t.Error("orphan key should not be stored")
	}
}

func makeTestRecordFM(profile *string) *FieldMap {
	fm := NewFieldMap()
	fm.SetU64("user_id", 42)
	fm.SetString("username", "Alice")
	fm.SetF32("score", 1.5)
	fm.SetBool("active", true)
	fm.SetBytes("metadata", []byte{0xCA, 0xFE})
	fm.SetListString("tags", []string{"admin", "user"})
	fm.SetOptionalString("profile", profile)
	fm.SetListU32("counts", []uint32{1, 2, 3, 4})
	addrFm := NewFieldMap()
	addrFm.SetString("street", "Main St")
	addrFm.SetU32("zip", 12345)
	fm.SetStruct("address", addrFm)
	fm.SetMap("scores", map[string]any{"math": uint32(95), "english": uint32(87)})
	labelFm := NewFieldMap()
	labelFm.SetString("value", "important")
	labelFm.SetU32("priority", 1)
	fm.SetMap("labels", map[string]any{"urgent": labelFm})
	return fm
}

func TestReflectFill_AllFields(t *testing.T) {
	profile := "bio text"
	msg := &testRecord{}
	if err := reflectFill(reflect.ValueOf(msg).Elem(), makeTestRecordFM(&profile)); err != nil {
		t.Fatalf("reflectFill: %v", err)
	}
	if msg.UserID != 42 {
		t.Errorf("UserID: %d", msg.UserID)
	}
	if msg.Username != "Alice" {
		t.Errorf("Username: %q", msg.Username)
	}
	if math.Abs(float64(msg.Score-1.5)) > 1e-6 {
		t.Errorf("Score: %v", msg.Score)
	}
	if !msg.Active {
		t.Error("Active: false")
	}
	if len(msg.Tags) != 2 || msg.Tags[0] != "admin" {
		t.Errorf("Tags: %v", msg.Tags)
	}
	if msg.Profile == nil || *msg.Profile != "bio text" {
		t.Errorf("Profile: %v", msg.Profile)
	}
	if msg.Counts != [4]uint32{1, 2, 3, 4} {
		t.Errorf("Counts: %v", msg.Counts)
	}
	if msg.Address.Street != "Main St" {
		t.Errorf("Address.Street: %q", msg.Address.Street)
	}
	if msg.Address.Zip != 12345 {
		t.Errorf("Address.Zip: %d", msg.Address.Zip)
	}
	if msg.Scores["math"] != 95 {
		t.Errorf("Scores[math]: %d", msg.Scores["math"])
	}
}

func TestReflectExtract_AllFields(t *testing.T) {
	profile := "bio"
	msg := &testRecord{
		UserID:   99,
		Username: "Bob",
		Score:    2.5,
		Active:   false,
		Tags:     []string{"x"},
		Profile:  &profile,
		Counts:   [4]uint32{10, 20, 30, 40},
		Address:  addrFields{Street: "Oak Ave", Zip: 77777},
		Scores:   map[string]uint32{"a": 1},
	}
	fm := NewFieldMap()
	reflectExtract(reflect.ValueOf(msg).Elem(), fm)

	if v, _ := fm.GetU64("user_id"); v != 99 {
		t.Errorf("user_id: %d", v)
	}
	if v, _ := fm.GetString("username"); v != "Bob" {
		t.Errorf("username: %q", v)
	}
	if v, _ := fm.GetBool("active"); v {
		t.Error("active: true")
	}
	if v, _ := fm.GetOptionalString("profile"); v == nil || *v != "bio" {
		t.Errorf("profile: %v", v)
	}
	addr, _ := fm.GetStruct("address")
	if addr == nil {
		t.Fatal("address: nil")
	}
	if v, _ := addr.GetString("street"); v != "Oak Ave" {
		t.Errorf("address.street: %q", v)
	}
	if v, _ := addr.GetU32("zip"); v != 77777 {
		t.Errorf("address.zip: %d", v)
	}
	scores, ok := fm.GetMap("scores")
	if !ok || scores["a"] != uint32(1) {
		t.Errorf("scores: %v", scores)
	}
}

func TestReflectFillExtract_RoundTrip(t *testing.T) {
	profile := "text"
	fm1 := makeTestRecordFM(&profile)

	msg := &testRecord{}
	if err := reflectFill(reflect.ValueOf(msg).Elem(), fm1); err != nil {
		t.Fatalf("reflectFill: %v", err)
	}

	fm2 := NewFieldMap()
	reflectExtract(reflect.ValueOf(msg).Elem(), fm2)

	if v, _ := fm2.GetU64("user_id"); v != 42 {
		t.Errorf("user_id: %d", v)
	}
	if v, _ := fm2.GetString("username"); v != "Alice" {
		t.Errorf("username: %q", v)
	}
	if v, _ := fm2.GetBool("active"); !v {
		t.Error("active: false")
	}
	tags, _ := fm2.GetListString("tags")
	if len(tags) != 2 || tags[0] != "admin" {
		t.Errorf("tags: %v", tags)
	}
	if v, _ := fm2.GetOptionalString("profile"); v == nil || *v != "text" {
		t.Errorf("profile: %v", v)
	}
	addr, _ := fm2.GetStruct("address")
	if addr == nil {
		t.Fatal("address: nil")
	}
	if v, _ := addr.GetString("street"); v != "Main St" {
		t.Errorf("address.street: %q", v)
	}
}

func TestReflectFill_OptionalNil(t *testing.T) {
	fm := NewFieldMap()
	fm.SetU64("user_id", 0)
	fm.SetString("username", "")
	fm.SetF32("score", 0)
	fm.SetBool("active", false)
	fm.SetListString("tags", []string{})
	fm.SetOptionalString("profile", nil)
	fm.SetListU32("counts", []uint32{0, 0, 0, 0})
	fm.SetStruct("address", NewFieldMap())

	msg := &testRecord{}
	if err := reflectFill(reflect.ValueOf(msg).Elem(), fm); err != nil {
		t.Fatalf("reflectFill: %v", err)
	}
	if msg.Profile != nil {
		t.Errorf("profile should be nil, got %v", msg.Profile)
	}
}

// TestReflect_GenericArray proves arbitrary array<T,N> element types (not just
// [4]uint32) round-trip through the reflection bridge.
func TestReflect_GenericArray(t *testing.T) {
	type box struct {
		Vals  [3]uint64
		Names [2]string
	}
	src := &box{Vals: [3]uint64{1, math.MaxUint64, 3}, Names: [2]string{"a", "b"}}

	fm := NewFieldMap()
	reflectExtract(reflect.ValueOf(src).Elem(), fm)

	dst := &box{}
	if err := reflectFill(reflect.ValueOf(dst).Elem(), fm); err != nil {
		t.Fatalf("reflectFill: %v", err)
	}
	if *dst != *src {
		t.Errorf("round-trip mismatch: got %#v want %#v", dst, src)
	}
}

// A field whose Go type cannot hold the case data must be a hard error, not a
// silent zero fill. The real case that motivated this: a uint128 schema field
// bound to a [16]byte model field (an SDK that stores an id as opaque bytes).
// Zero-filling produced wrong bytes with no diagnostic, and a false pass on any
// case whose expected value happened to be zero.
func TestReflectFill_ShapeMismatchIsAnError(t *testing.T) {
	t.Run("array field, scalar data", func(t *testing.T) {
		type box struct {
			ID [16]byte
		}
		fm := NewFieldMap()
		fm.SetBig("id", big.NewInt(42))

		dst := &box{}
		err := reflectFill(reflect.ValueOf(dst).Elem(), fm)
		if err == nil {
			t.Fatalf("want error, got nil (ID silently left %v)", dst.ID)
		}
		if !strings.Contains(err.Error(), "id") {
			t.Errorf("error should name the field: %v", err)
		}
	})

	t.Run("struct field, scalar data", func(t *testing.T) {
		type inner struct{ A uint32 }
		type box struct {
			Nested inner
		}
		fm := NewFieldMap()
		fm.SetString("nested", "not a struct")

		dst := &box{}
		if err := reflectFill(reflect.ValueOf(dst).Elem(), fm); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("map field, scalar data", func(t *testing.T) {
		type box struct {
			Labels map[string]uint32
		}
		fm := NewFieldMap()
		fm.SetString("labels", "not a map")

		dst := &box{}
		if err := reflectFill(reflect.ValueOf(dst).Elem(), fm); err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

// A model field the schema does not mention stays untouched — that is not a
// mismatch, and must keep working.
func TestReflectFill_UnmentionedFieldIsSkipped(t *testing.T) {
	type box struct {
		A uint32
		B uint32
	}
	fm := NewFieldMap()
	fm.SetU32("a", 7)

	dst := &box{B: 99}
	if err := reflectFill(reflect.ValueOf(dst).Elem(), fm); err != nil {
		t.Fatalf("reflectFill: %v", err)
	}
	if dst.A != 7 || dst.B != 99 {
		t.Errorf("got %+v, want {A:7 B:99}", dst)
	}
}

// A Converter lets a model field of an otherwise-unmappable Go type ride a
// serify-native representation, with the worker stating the mapping. This mirrors
// iggy's MessageID: a defined [16]byte the schema declares as uint128, where the
// byte order is the worker's to fix (little-endian here).
func TestConverter_RoundTrip(t *testing.T) {
	type msgID [16]byte
	type header struct {
		ID msgID `serify:"id"`
	}

	convs := map[reflect.Type]Converter{
		reflect.TypeOf(msgID{}): NewConverter(
			func(v *big.Int) (msgID, error) {
				var id msgID
				v.FillBytes(id[:])
				for i, j := 0, len(id)-1; i < j; i, j = i+1, j-1 {
					id[i], id[j] = id[j], id[i] // -> little-endian
				}
				return id, nil
			},
			func(be msgID) *big.Int {
				for i, j := 0, len(be)-1; i < j; i, j = i+1, j-1 {
					be[i], be[j] = be[j], be[i]
				}
				return new(big.Int).SetBytes(be[:])
			},
		),
	}
	codec := reflectCodec{converters: convs}

	fm := NewFieldMap()
	fm.SetBig("id", big.NewInt(0x0102))

	var h header
	if err := codec.fill(reflect.ValueOf(&h).Elem(), fm); err != nil {
		t.Fatalf("fill: %v", err)
	}
	// 0x0102 little-endian in 16 bytes: 0x02 at index 0, 0x01 at index 1.
	if h.ID[0] != 0x02 || h.ID[1] != 0x01 {
		t.Fatalf("Decode byte order wrong: %v", h.ID)
	}

	out := NewFieldMap()
	codec.extract(reflect.ValueOf(&h).Elem(), out)
	got, err := out.GetBig("id")
	if err != nil {
		t.Fatalf("extract stored wrong type: %v", err)
	}
	if got.Cmp(big.NewInt(0x0102)) != 0 {
		t.Errorf("round-trip: got %s, want 258", got)
	}
}

// A whole-type converter folds a flattened sum type (kind + payload fields) into
// a single value, the way iggy's identifier maps kind/numeric_value/string_value
// onto one constructor-built Identifier. The same converter serves the type at
// the top level (the model IS the sum type) and nested inside another model.
func TestConverter_WholeType(t *testing.T) {
	// A stand-in "sum type" with an unexported field: only constructible here,
	// like an SDK type behind a constructor.
	type ident struct{ tag string }

	convs := map[reflect.Type]Converter{
		reflect.TypeOf(ident{}): NewConverter(
			func(fm *FieldMap) (ident, error) {
				kind, _ := fm.GetString("kind")
				switch kind {
				case "num":
					n, _ := fm.GetU32("num_value")
					return ident{tag: fmt.Sprintf("num:%d", n)}, nil
				case "str":
					s, _ := fm.GetString("str_value")
					return ident{tag: "str:" + s}, nil
				default:
					return ident{}, fmt.Errorf("bad kind %q", kind)
				}
			},
			func(id ident) *FieldMap {
				fm := NewFieldMap()
				fm.SetString("kind", "raw")
				fm.SetString("str_value", id.tag)
				fm.SetU32("num_value", 0)
				return fm
			},
		),
	}
	codec := reflectCodec{converters: convs}

	t.Run("top-level model", func(t *testing.T) {
		fm := NewFieldMap()
		fm.SetString("kind", "num")
		fm.SetU32("num_value", 7)

		var id ident
		if err := codec.fill(reflect.ValueOf(&id).Elem(), fm); err != nil {
			t.Fatalf("fill: %v", err)
		}
		if id.tag != "num:7" {
			t.Fatalf("top-level decode: got %q", id.tag)
		}

		out := NewFieldMap()
		codec.extract(reflect.ValueOf(&id).Elem(), out)
		if s, _ := out.GetString("str_value"); s != "num:7" {
			t.Errorf("top-level extract: got %q", s)
		}
	})

	t.Run("nested field", func(t *testing.T) {
		type request struct {
			StreamID ident `serify:"stream_id"`
		}
		inner := NewFieldMap()
		inner.SetString("kind", "str")
		inner.SetString("str_value", "orders")
		fm := NewFieldMap()
		fm.SetStruct("stream_id", inner)

		var req request
		if err := codec.fill(reflect.ValueOf(&req).Elem(), fm); err != nil {
			t.Fatalf("fill: %v", err)
		}
		if req.StreamID.tag != "str:orders" {
			t.Errorf("nested decode: got %q", req.StreamID.tag)
		}
	})
}

// An anonymous embedded struct is flattened into the parent's level, matching Go
// promotion and encoding/json — so an SDK type that groups fields in an embedded
// struct (iggy's CreateConsumerGroup embeds TopicPath) maps to a flat schema
// with no shim. A whole-type converter on a field inside the embed still fires.
func TestReflect_EmbeddedFlattening(t *testing.T) {
	type ident struct{ tag string }
	type topicPath struct {
		StreamID ident `serify:"stream_id"`
		TopicID  ident `serify:"topic_id"`
	}
	type request struct {
		topicPath        // embedded, no tag -> flattened
		Name      string `serify:"name"`
	}

	convs := map[reflect.Type]Converter{
		reflect.TypeOf(ident{}): NewConverter(
			func(fm *FieldMap) (ident, error) {
				s, _ := fm.GetString("v")
				return ident{tag: s}, nil
			},
			func(id ident) *FieldMap {
				fm := NewFieldMap()
				fm.SetString("v", id.tag)
				return fm
			},
		),
	}
	codec := reflectCodec{converters: convs}

	sid := NewFieldMap()
	sid.SetString("v", "s1")
	tid := NewFieldMap()
	tid.SetString("v", "t1")
	fm := NewFieldMap()
	fm.SetStruct("stream_id", sid) // flat keys, no "topic_path" wrapper
	fm.SetStruct("topic_id", tid)
	fm.SetString("name", "grp")

	var req request
	if err := codec.fill(reflect.ValueOf(&req).Elem(), fm); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if req.StreamID.tag != "s1" || req.TopicID.tag != "t1" || req.Name != "grp" {
		t.Fatalf("flatten decode: %+v", req)
	}

	out := NewFieldMap()
	codec.extract(reflect.ValueOf(&req).Elem(), out)
	// The embedded fields must appear at the top level, not under "topic_path".
	if _, ok := out.fields["topic_path"]; ok {
		t.Error("embedded struct leaked a topic_path key instead of flattening")
	}
	if _, ok := out.fields["stream_id"]; !ok {
		t.Error("embedded stream_id not promoted on extract")
	}
	if n, _ := out.GetString("name"); n != "grp" {
		t.Errorf("name: %q", n)
	}
}

// Without the converter, that same [16]byte field is a hard error (the array
// branch), not a silent zero — proving the converter is what enables it, not a
// pre-existing lax path.
func TestConverter_AbsentIsStillAnError(t *testing.T) {
	type msgID [16]byte
	type header struct {
		ID msgID `serify:"id"`
	}
	fm := NewFieldMap()
	fm.SetBig("id", big.NewInt(1))

	var h header
	if err := reflectFill(reflect.ValueOf(&h).Elem(), fm); err == nil {
		t.Fatal("want error without a converter, got nil")
	}
}

func TestReflectFill_SnakeCaseMapping(t *testing.T) {
	fm := NewFieldMap()
	fm.SetU64("user_id", 99) // explicit serify: tag
	fm.SetString("username", "Bob")

	msg := &testRecord{}
	if err := reflectFill(reflect.ValueOf(msg).Elem(), fm); err != nil {
		t.Fatalf("reflectFill: %v", err)
	}
	if msg.UserID != 99 {
		t.Errorf("UserID via serify tag: %d", msg.UserID)
	}
	if msg.Username != "Bob" {
		t.Errorf("Username via snake_case: %q", msg.Username)
	}
}
