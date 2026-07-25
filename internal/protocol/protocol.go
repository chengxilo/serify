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

// Package protocol defines the NDJSON wire format between runner and workers.
package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"io"
	"math"
	"strconv"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/typekind"
)

// scannerBufSize is the initial and max token buffer for the NDJSON scanner (4 MB).
const scannerBufSize = 4 * 1024 * 1024

// Op is the wire-format operation name used in the "op" field.
type Op string

const (
	OpPing        Op = "ping"
	OpBind        Op = "bind"
	OpSerialize   Op = "serialize"
	OpDeserialize Op = "deserialize"
	OpExit        Op = "exit"
)

// Status is the wire-format status code returned in worker responses.
type Status string

const (
	StatusOK      Status = "OK"
	StatusError   Status = "ERROR"
	StatusSkipped Status = "SKIPPED"
)

// ProtocolVersion is the wire revision this runner speaks. Workers report theirs
// in the ping response and the two must match exactly. The runner and every
// worker library ship from this repository together, so a mismatch always means
// one side was built from different sources — there is no version range to
// negotiate. Bump this on any breaking change to the messages below.
const ProtocolVersion = 2

// PingRequest is the startup health check. It names no type and carries no
// schema: it only asks whether the worker is up and which protocol it speaks.
type PingRequest struct {
	Op string `json:"op"` // "ping"
}

// BindRequest points the worker at one (type, format) and its schema. The
// runner sends one per (type, format) group; it is not a once-per-process
// initialisation, which is why it is not called init.
type BindRequest struct {
	Op     string        `json:"op"`
	Schema []SchemaField `json:"schema"`
	Type   string        `json:"type,omitempty"`
	Format string        `json:"format,omitempty"`
	Audit  bool          `json:"audit,omitempty"`
}

// SchemaField is the wire representation of one schema field.
// Nested struct fields are carried in Fields; Tags holds arbitrary metadata.
type SchemaField struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"`
	Fields   []SchemaField     `json:"fields,omitempty"`   // nested schema for struct / map<K,struct>
	KeyType  string            `json:"key_type,omitempty"` // map key type, e.g. "string"
	Variants []SchemaVariant   `json:"variants,omitempty"` // for sum<...>
	Tags     map[string]string `json:"tags,omitempty"`     // metadata forwarded from cases.yaml
}

// SchemaVariant is one arm of a sum on the wire: a tag and its payload schema
// (Payload is nil for a unit variant).
type SchemaVariant struct {
	Name    string       `json:"name"`
	Payload *SchemaField `json:"payload,omitempty"`
}

type SerializeRequest struct {
	ID   string         `json:"id"`
	Op   string         `json:"op"` // "serialize"
	Data map[string]any `json:"data"`
}

type DeserializeRequest struct {
	ID  string `json:"id"`
	Op  string `json:"op"` // "deserialize"
	Hex string `json:"hex"`
}

type ExitRequest struct {
	Op string `json:"op"` // "exit"
}

type Response struct {
	ID     string `json:"id"`
	Op     string `json:"op"`
	Status Status `json:"status"` // OK, ERROR, SKIPPED

	Hex  string         `json:"hex,omitempty"`
	Data map[string]any `json:"data,omitempty"`

	// ProtocolVersion is set only on ping responses. It is a pointer so an
	// absent field is distinguishable from a worker that reports version 0.
	ProtocolVersion *int `json:"protocol_version,omitempty"`

	Error  string `json:"error,omitempty"`
	Reason string `json:"reason,omitempty"`

	Audit *AuditReport `json:"audit,omitempty"`
}

// AuditReport carries unsafe-behaviour findings detected by the worker during
// audit mode. It is only populated when the runner passes --audit.
type AuditReport struct {
	Mutations            []string `json:"mutations,omitempty"`
	Stable               *bool    `json:"stable,omitempty"`
	OutputZeroCopyFields []string `json:"output_zero_copy_fields,omitempty"`
	ZeroCopyFields       []string `json:"zero_copy_fields,omitempty"`
	InputMutated         bool     `json:"input_mutated,omitempty"`
	DeserStable          *bool    `json:"deser_stable,omitempty"`
}

// Writer writes NDJSON messages to a writer (worker stdin).
type Writer struct{ w io.Writer }

func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// Write emits one NDJSON message. It deliberately avoids json.Marshal, which
// HTML-escapes <, > and & by default: that would send a type like
// optional<string> as "optional<string>". Standard JSON libraries decode
// that back, but a worker with a hand-rolled JSON parser (several worker libs have one)
// sees a mangled type name and silently fails to match any schema type. Nothing
// here is ever embedded in HTML, so the escaping buys nothing.
func (w *Writer) Write(v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // Encode already appends the newline NDJSON needs
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := w.w.Write(buf.Bytes())
	return err
}

// Reader reads NDJSON responses from a reader (worker stdout).
type Reader struct{ s *bufio.Scanner }

func NewReader(r io.Reader) *Reader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, scannerBufSize), scannerBufSize)
	return &Reader{s: s}
}

func (r *Reader) Read() (*Response, error) {
	if !r.s.Scan() {
		if err := r.s.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	var resp Response
	if err := json.Unmarshal(r.s.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("malformed worker response: %w (line: %s)", err, r.s.Text())
	}
	return &resp, nil
}

func findVariant(vs []config.Variant, tag string) *config.Variant {
	for i := range vs {
		if vs[i].Name == tag {
			return &vs[i]
		}
	}
	return nil
}

// EncodeData converts raw test-case data values into wire-safe representations
// (u64/u128 → decimal string, f32/f64 → IEEE 754 LE hex, struct → nested map).
func EncodeData(data map[string]any, schema []config.Field) (map[string]any, error) {
	out := make(map[string]any, len(data))
	fieldTypes := make(map[string]config.Field, len(schema))
	for _, f := range schema {
		fieldTypes[f.Name] = f
	}
	for k, v := range data {
		f, ok := fieldTypes[k]
		if !ok {
			return nil, fmt.Errorf("field %q not in schema", k)
		}
		enc, err := encodeValue(v, f.Type)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		out[k] = enc
	}
	return out, nil
}

//nolint:gocognit,funlen // one encode branch per schema type
func encodeValue(v any, ft config.FieldType) (any, error) {
	switch ft.Base {
	case typekind.Float32:
		f, err := toFloat64(v)
		if err != nil {
			return nil, err
		}
		bits := math.Float32bits(float32(f))
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], bits)
		return hex.EncodeToString(b[:]), nil

	case typekind.Float64:
		f, err := toFloat64(v)
		if err != nil {
			return nil, err
		}
		bits := math.Float64bits(f)
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], bits)
		return hex.EncodeToString(b[:]), nil

	case typekind.Uint64, typekind.Int64, typekind.Uint128, typekind.Int128:
		// 64/128-bit integers travel as decimal strings to survive JSON.
		return fmt.Sprintf("%v", v), nil

	case typekind.Bytes:
		// Authored as a byte array ([0xde, 0xad, ...] or [222, 173, ...]); a hex
		// string is also accepted. Travels to workers as hex either way.
		return encodeBytes(v)

	case typekind.Struct:
		obj, ok := toStringMap(v)
		if !ok {
			return nil, fmt.Errorf("expected object for struct, got %T", v)
		}
		out := make(map[string]any, len(ft.Fields))
		for _, f := range ft.Fields {
			fv, exists := obj[f.Name]
			if !exists {
				continue
			}
			enc, err := encodeValue(fv, f.Type)
			if err != nil {
				return nil, fmt.Errorf(".%s: %w", f.Name, err)
			}
			out[f.Name] = enc
		}
		return out, nil

	case typekind.Optional:
		if v == nil {
			return nil, nil //nolint:nilnil // sentinel error not appropriate: EncodeData callers check err != nil
		}
		return encodeValue(v, *ft.Elem)

	case typekind.List, typekind.Array:
		arr, ok := toSlice(v)
		if !ok {
			return nil, fmt.Errorf("expected %s, got %T", ft.Base, v)
		}
		if ft.Base == typekind.Array && len(arr) != ft.ArrayN {
			return nil, fmt.Errorf("array length mismatch: expected %d, got %d", ft.ArrayN, len(arr))
		}
		out := make([]any, len(arr))
		for i, elem := range arr {
			enc, err := encodeValue(elem, *ft.Elem)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out[i] = enc
		}
		return out, nil

	case typekind.Map:
		obj, ok := toStringMap(v)
		if !ok {
			return nil, fmt.Errorf("expected object for map, got %T", v)
		}
		out := make(map[string]any, len(obj))
		for k, mv := range obj {
			enc, err := encodeValue(mv, *ft.Elem)
			if err != nil {
				return nil, fmt.Errorf("[%q]: %w", k, err)
			}
			out[k] = enc
		}
		return out, nil

	case typekind.Enum:
		// The variant name travels as a string; the schema carries the variant list
		// so the worker can map it onto whatever its byte layout uses (typically an
		// ordinal). config.validate has already checked the name is a declared one.
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected a string enum variant, got %T", v)
		}
		return s, nil

	case typekind.Sum:
		// A variant travels as a single-key map {tag: encoded_payload}; the
		// payload is encoded per that variant's type (nil for a unit variant).
		obj, ok := toStringMap(v)
		if !ok || len(obj) != 1 {
			return nil, fmt.Errorf("expected a single-key object for a sum, got %T", v)
		}
		for tag, payload := range obj {
			variant := findVariant(ft.Variants, tag)
			if variant == nil {
				return nil, fmt.Errorf("unknown variant %q", tag)
			}
			if variant.Type == nil {
				return map[string]any{tag: nil}, nil
			}
			enc, err := encodeValue(payload, *variant.Type)
			if err != nil {
				return nil, fmt.Errorf("variant %q: %w", tag, err)
			}
			return map[string]any{tag: enc}, nil
		}
		return nil, fmt.Errorf("empty sum value")

	default:
		// u8, u16, u32, i8..i32, bool, string, bytes - pass through
		return v, nil
	}
}

// encodeBytes turns an authored bytes value into the wire hex string. The
// primary form is a byte array (`[0xde, 0xad, ...]` or `[222, 173, ...]`); a hex
// string is also accepted.
func encodeBytes(v any) (any, error) {
	switch x := v.(type) {
	case string:
		return x, nil // already hex
	case []any:
		b := make([]byte, len(x))
		for i, e := range x {
			n, ok := toInt(e)
			if !ok || n < 0 || n > 255 {
				return nil, fmt.Errorf("byte[%d]: %v is not an integer in 0..255", i, e)
			}
			b[i] = byte(n)
		}
		return hex.EncodeToString(b), nil
	default:
		return nil, fmt.Errorf("bytes must be a byte array or a hex string, got %T", v)
	}
}

// toInt converts a YAML-decoded integer to int.
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case uint64:
		if x > math.MaxInt {
			return 0, false
		}
		return int(x), true
	case float64:
		if x == float64(int64(x)) {
			return int(x), true
		}
	}
	return 0, false
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case string:
		return strconv.ParseFloat(x, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", v)
	}
}

func toSlice(v any) ([]any, bool) {
	x, ok := v.([]any)
	return x, ok
}

func toStringMap(v any) (map[string]any, bool) {
	x, ok := v.(map[string]any)
	return x, ok
}

// SchemaFields converts a config.Field slice to wire SchemaFields, recursing
// into struct/sum types.
func SchemaFields(fields []config.Field) []SchemaField {
	out := make([]SchemaField, len(fields))
	for i, f := range fields {
		sf := schemaFieldOfType(f.Name, f.Type)
		sf.Tags = f.Tags
		out[i] = sf
	}
	return out
}

// schemaFieldOfType builds a SchemaField for a named field of the given type,
// carrying the structured shape (nested struct fields, map key, variants).
func schemaFieldOfType(name string, ft config.FieldType) SchemaField {
	sf := SchemaField{Name: name, Type: ft.String()}
	switch ft.Base {
	case typekind.Struct:
		sf.Fields = SchemaFields(ft.Fields)
	case typekind.List, typekind.Optional:
		if ft.Elem != nil && ft.Elem.Base == typekind.Struct {
			sf.Fields = SchemaFields(ft.Elem.Fields)
		}
	case typekind.Map:
		sf.KeyType = ft.Key.String()
		if ft.Elem != nil && ft.Elem.Base == typekind.Struct {
			sf.Fields = SchemaFields(ft.Elem.Fields)
		}
	case typekind.Sum:
		sf.Variants = make([]SchemaVariant, len(ft.Variants))
		for j, v := range ft.Variants {
			sv := SchemaVariant{Name: v.Name}
			if v.Type != nil {
				p := schemaFieldOfType(v.Name, *v.Type)
				sv.Payload = &p
			}
			sf.Variants[j] = sv
		}
	}
	return sf
}
