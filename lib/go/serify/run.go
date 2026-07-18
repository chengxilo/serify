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

// Package serify is the worker-side library for the serify conformance framework.
// A worker imports it, supplies serialize/deserialize functions in a Suite, and
// Run() handles the full NDJSON protocol loop.
package serify

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/chengxilo/serify/internal/protocol"
)

const (
	wireStatus          = "status"
	wireProtocolVersion = "protocol_version"
)

// maxLineBytes caps a single NDJSON request/response line. Payloads with large
// byte fields or deep collections routinely exceed bufio.Scanner's 64 KiB default.
const maxLineBytes = 4 * 1024 * 1024

type parsedFormat struct {
	serializer   func(*FieldMap) ([]byte, error) // nil if not provided
	deserializer func([]byte) (*FieldMap, error) // nil if not provided
	serHolder    *serializeAuditHolder
}

type parsedType struct {
	formats map[string]parsedFormat
}

// parseTypes validates and wraps all serializer/deserializer functions to a
// standard signature.
func parseTypes(suite Suite) map[string]parsedType {
	parsed := make(map[string]parsedType, len(suite.Types))
	for typeName, t := range suite.Types {
		pt := parsedType{formats: make(map[string]parsedFormat, len(t.Formats))}
		for name, f := range t.Formats {
			pf := parsedFormat{}
			if f.Serializer != nil {
				w, holder, err := buildSerializer(t.Model, f.Serializer, suite.Converters)
				if err != nil {
					panic(fmt.Sprintf("suite type %q format %q serializer: %v", typeName, name, err))
				}
				pf.serializer = w
				pf.serHolder = holder
			}
			if f.Deserializer != nil {
				w, err := parseDeserializer(t.Model, f.Deserializer, suite.Converters)
				if err != nil {
					panic(fmt.Sprintf("suite type %q format %q deserializer: %v", typeName, name, err))
				}
				pf.deserializer = w
			}
			pt.formats[name] = pf
		}
		parsed[typeName] = pt
	}
	return parsed
}

// serializeAudit collects the audit findings for one serialize call: any struct
// mutation or output aliasing the holder recorded during the call, plus a
// stability check that serializes a second time and compares the bytes.
func serializeAudit(
	holder *serializeAuditHolder,
	serialize func(*FieldMap) ([]byte, error),
	fm *FieldMap,
	out []byte,
) map[string]any {
	audit := make(map[string]any)

	// Read the holder BEFORE the stability re-call below overwrites it.
	if holder != nil {
		if len(holder.LastMutations) > 0 {
			audit["mutations"] = holder.LastMutations
		}
		if len(holder.LastOutputZC) > 0 {
			audit["output_zero_copy_fields"] = holder.LastOutputZC
		}
	}

	if out2, err := serialize(fm); err != nil || !bytes.Equal(out, out2) {
		audit["stable"] = false
	}
	return audit
}

// deserializeAudit collects the audit findings for one deserialize call: whether
// the worker mutated the input buffer, whether deserializing the same bytes again
// yields the same FieldMap, and which fields alias the input buffer.
//
// buf is the buffer handed to the worker (which it may have mutated); snapshot is
// a pristine clone taken beforehand.
func deserializeAudit(
	deserialize func([]byte) (*FieldMap, error),
	fm *FieldMap,
	buf, snapshot []byte,
) map[string]any {
	audit := make(map[string]any)

	if DetectInputMutation(snapshot, buf) {
		audit["input_mutated"] = true
	}

	// Re-deserialize from a fresh clone of the pristine snapshot, not from buf:
	// the zero-copy probe below corrupts buf, and the worker may already have.
	fm2, err := deserialize(bytes.Clone(snapshot))
	if err != nil || len(CompareFieldMaps(fm, fm2)) > 0 {
		audit["deser_stable"] = false
	}

	// Must run LAST: it overwrites buf to see which fields change with it.
	if zcFields := DetectZeroCopy(fm, buf); len(zcFields) > 0 {
		audit["zero_copy_fields"] = zcFields
	}
	return audit
}

// Run executes the NDJSON worker protocol. Suite.In and Suite.Out default to
// os.Stdin and os.Stdout. All serializer/deserializer functions are validated
// and wrapped at startup; a signature mismatch causes an immediate panic.
//
//nolint:gocognit,gocyclo,cyclop,funlen // NDJSON protocol loop: one branch per wire op, each with its own error paths
func Run(suite Suite) {
	in := suite.In
	if in == nil {
		in = os.Stdin
	}
	out := suite.Out
	if out == nil {
		out = os.Stdout
	}

	types := parseTypes(suite)

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, maxLineBytes), maxLineBytes)
	writer := bufio.NewWriter(out)

	var schema []SchemaField
	var currentSerialize func(*FieldMap) ([]byte, error)
	var currentDeserialize func([]byte) (*FieldMap, error)
	var auditEnabled bool
	var currentSerHolder *serializeAuditHolder

	emit := func(v any) {
		b, _ := json.Marshal(v)
		_, _ = writer.Write(b)
		_ = writer.WriteByte('\n')
		_ = writer.Flush()
	}
	emitErr := func(id *string, op protocol.Op, status protocol.Status, msg string) {
		m := map[string]any{"op": op, wireStatus: status, "error": msg}
		if id != nil {
			m["id"] = *id
		}
		emit(m)
	}

	// emitSkip reports that this worker does not implement the type or format the
	// runner just asked for. The bindings are cleared first so a later serialize
	// on the same connection cannot silently run against the previous type.
	emitSkip := func() {
		currentSerialize = nil
		currentDeserialize = nil
		currentSerHolder = nil
		emit(map[string]any{
			"op":     protocol.OpBind,
			"status": protocol.StatusSkipped,
		})
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var base struct {
			Op string `json:"op"`
			ID string `json:"id"`
		}
		if err := json.Unmarshal(line, &base); err != nil {
			continue
		}

		switch base.Op {
		case string(protocol.OpPing):
			// Health check: report liveness and the protocol revision this
			// library speaks. Binds nothing.
			emit(map[string]any{
				"op":                protocol.OpPing,
				wireStatus:          protocol.StatusOK,
				wireProtocolVersion: protocol.ProtocolVersion,
			})

		case string(protocol.OpBind):
			var msg struct {
				Schema []SchemaField `json:"schema"`
				Type   string        `json:"type"`
				Format string        `json:"format"`
				Audit  bool          `json:"audit"`
			}
			if err := json.Unmarshal(line, &msg); err != nil {
				emitErr(nil, protocol.OpBind, protocol.StatusError, err.Error())
				continue
			}
			schema = msg.Schema

			// Both type and format are required; the runner always sends them.
			if msg.Type == "" {
				emitErr(nil, protocol.OpBind, protocol.StatusError, `bind requires a "type" field`)
				continue
			}
			bt, ok := types[msg.Type]
			if !ok {
				emitSkip()
				continue
			}
			if msg.Format == "" {
				emitErr(nil, protocol.OpBind, protocol.StatusError, `bind requires a "format" field`)
				continue
			}
			bf, found := bt.formats[msg.Format]
			if !found {
				emitSkip()
				continue
			}
			currentSerialize = bf.serializer
			currentDeserialize = bf.deserializer

			auditEnabled = msg.Audit
			currentSerHolder = bf.serHolder
			if currentSerHolder != nil {
				currentSerHolder.Enabled = auditEnabled
			}

			emit(map[string]any{
				"op": protocol.OpBind,
			})

		case string(protocol.OpSerialize):
			id := base.ID
			if currentSerialize == nil {
				if currentDeserialize != nil {
					emit(map[string]any{
						"id":     id,
						"op":     protocol.OpSerialize,
						"status": protocol.StatusSkipped,
						"reason": "direction not registered",
					})
				} else {
					emitErr(&id, protocol.OpSerialize, protocol.StatusError, "no serializer configured (call bind first)")
				}
				continue
			}
			var msg struct {
				Data map[string]json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(line, &msg); err != nil {
				emitErr(&id, protocol.OpSerialize, protocol.StatusError, err.Error())
				continue
			}
			fm, err := DecodeFieldMap(msg.Data, schema)
			if err != nil {
				emitErr(&id, protocol.OpSerialize, protocol.StatusError, err.Error())
				continue
			}
			b, err := currentSerialize(fm)
			if err != nil {
				emitErr(&id, protocol.OpSerialize, protocol.StatusError, err.Error())
				continue
			}

			var audit map[string]any
			if auditEnabled {
				audit = serializeAudit(currentSerHolder, currentSerialize, fm, b)
			}

			resp := map[string]any{
				"id":       id,
				"op":       protocol.OpSerialize,
				wireStatus: protocol.StatusOK,
				"hex":      hex.EncodeToString(b),
			}
			if len(audit) > 0 {
				resp["audit"] = audit
			}
			emit(resp)

		case string(protocol.OpDeserialize):
			id := base.ID
			if currentDeserialize == nil {
				if currentSerialize != nil {
					emit(map[string]any{
						"id":     id,
						"op":     protocol.OpDeserialize,
						"status": protocol.StatusSkipped,
						"reason": "direction not registered",
					})
				} else {
					emitErr(&id, protocol.OpDeserialize, protocol.StatusError, "no deserializer configured (call bind first)")
				}
				continue
			}
			var msg struct {
				Hex string `json:"hex"`
			}
			if err := json.Unmarshal(line, &msg); err != nil {
				emitErr(&id, protocol.OpDeserialize, protocol.StatusError, err.Error())
				continue
			}
			b, err := hex.DecodeString(msg.Hex)
			if err != nil {
				emitErr(&id, protocol.OpDeserialize, protocol.StatusError, err.Error())
				continue
			}

			var bufSnapshot []byte
			if auditEnabled {
				bufSnapshot = bytes.Clone(b)
			}

			fm, err := currentDeserialize(b)
			if err != nil {
				emitErr(&id, protocol.OpDeserialize, protocol.StatusError, err.Error())
				continue
			}

			var audit map[string]any
			if auditEnabled {
				audit = deserializeAudit(currentDeserialize, fm, b, bufSnapshot)
			}

			data, err := EncodeFieldMap(fm, schema)
			if err != nil {
				emitErr(&id, protocol.OpDeserialize, protocol.StatusError, err.Error())
				continue
			}

			resp := map[string]any{
				"id":       id,
				"op":       protocol.OpDeserialize,
				wireStatus: protocol.StatusOK,
				"data":     data,
			}
			if len(audit) > 0 {
				resp["audit"] = audit
			}
			emit(resp)

		case string(protocol.OpExit):
			os.Exit(0)

		default:
			// Always answer: a silently dropped request would leave the runner
			// waiting for a response until its timeout.
			id := base.ID
			emitErr(&id, protocol.Op(base.Op), protocol.StatusError,
				fmt.Sprintf("unknown op %q", base.Op))
		}
	}
}
