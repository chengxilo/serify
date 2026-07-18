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

// Command worker is the Go half of the `audit` meta-test. It provides nine
// formats for the audit type:
//
//	clean            – correct round-trip (control group, no warnings)
//	mutating         – serializer zeroes the Value field after marshaling
//	value-mutating   – value receiver, mutates payload[0] after marshal
//	zero-copy        – deserializer returns Payload as a sub-slice of the input buffer
//	list-zero-copy   – deserializer aliases Tags via unsafe.String
//	unstable         – serializer appends a counter byte
//	deser-unstable   – deserializer adds 1 to Value on second call
//	input-mutating   – deserializer modifies input buffer after parsing
//	output-zero-copy – serializer returns a sub-slice of Payload
//
// When run with --audit, serify should flag warnings for the eight broken
// formats and report nothing for clean.
package main

import (
	"encoding/binary"
	"unsafe"

	"github.com/chengxilo/serify/lib/go/serify"
)

type Audit struct {
	Payload []byte
	Tag     string
	Value   uint32
	Tags    []string
}

// --- common binary helpers -----------------------------------------------

func marshalAudit(a *Audit) []byte {
	buf := binary.LittleEndian.AppendUint32(nil, a.Value)
	buf = append(buf, byte(len(a.Tag)))
	buf = append(buf, a.Tag...)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(a.Payload)))
	buf = append(buf, a.Payload...)
	buf = append(buf, byte(len(a.Tags)))
	for _, t := range a.Tags {
		buf = append(buf, byte(len(t)))
		buf = append(buf, t...)
	}
	return buf
}

func unmarshalAudit(a *Audit, data []byte, copyPayload bool) error {
	if len(data) < 5 {
		return errTruncated
	}
	pos := 0
	a.Value = binary.LittleEndian.Uint32(data[pos:])
	pos += 4

	tagLen := int(data[pos])
	pos++
	if pos+tagLen > len(data) {
		return errTruncated
	}
	a.Tag = string(data[pos : pos+tagLen])
	pos += tagLen

	if pos+4 > len(data) {
		return errTruncated
	}
	payloadLen := int(binary.LittleEndian.Uint32(data[pos:]))
	pos += 4
	if pos+payloadLen > len(data) {
		return errTruncated
	}
	if copyPayload {
		a.Payload = make([]byte, payloadLen)
		copy(a.Payload, data[pos:pos+payloadLen])
	} else {
		a.Payload = data[pos : pos+payloadLen]
	}
	pos += payloadLen

	if pos >= len(data) {
		return errTruncated
	}
	tagsCount := int(data[pos])
	pos++
	a.Tags = make([]string, tagsCount)
	for i := range tagsCount {
		if pos >= len(data) {
			return errTruncated
		}
		tl := int(data[pos])
		pos++
		if pos+tl > len(data) {
			return errTruncated
		}
		a.Tags[i] = string(data[pos : pos+tl])
		pos += tl
	}
	return nil
}

var errTruncated = &decodeError{"truncated"}

type decodeError struct{ msg string }

func (e *decodeError) Error() string { return e.msg }

// --- format: clean -------------------------------------------------------

func (a *Audit) MarshalClean() ([]byte, error) {
	return marshalAudit(a), nil
}

func (a *Audit) UnmarshalClean(data []byte) error {
	return unmarshalAudit(a, data, true)
}

// --- format: mutating ----------------------------------------------------

func (a *Audit) MarshalMutating() ([]byte, error) {
	data := marshalAudit(a)
	a.Value = 0 // MUTATE the struct after marshaling
	return data, nil
}

// --- format: value-mutating (Go only) ------------------------------------

// Value receiver: the serializer gets a copy, but slice backing (Payload)
// is shared. Mutation to the backing array is visible through the model.
func (a Audit) MarshalValueMutating() ([]byte, error) {
	data := marshalAudit(&a)
	a.Payload[0] ^= 0xFF // MUTATE shared backing
	return data, nil
}

// --- format: zero-copy ---------------------------------------------------

func (a *Audit) UnmarshalZeroCopy(data []byte) error {
	return unmarshalAudit(a, data, false) // aliases input buffer
}

// --- format: list-zero-copy ----------------------------------------------

func (a *Audit) UnmarshalListZeroCopy(data []byte) error {
	if err := unmarshalAudit(a, data, true); err != nil {
		return err
	}
	// Replace each tag string with an unsafe alias of the buffer.
	// Re-parse to find tag positions.
	pos := 0
	if len(data) < 5 {
		return errTruncated
	}
	pos += 4 // value
	tagLen := int(data[pos])
	pos++         // tag len
	pos += tagLen // tag
	if pos+4 > len(data) {
		return errTruncated
	}
	payloadLen := int(binary.LittleEndian.Uint32(data[pos:]))
	pos += 4
	pos += payloadLen // payload

	if pos >= len(data) {
		return errTruncated
	}
	tagsCount := int(data[pos])
	pos++
	for i := 0; i < tagsCount && i < len(a.Tags); i++ {
		if pos >= len(data) {
			return errTruncated
		}
		tl := int(data[pos])
		pos++
		if pos+tl > len(data) {
			return errTruncated
		}
		a.Tags[i] = unsafe.String(&data[pos], tl)
		pos += tl
	}
	return nil
}

// --- format: unstable ----------------------------------------------------

var unstableCounter int

func (a *Audit) MarshalUnstable() ([]byte, error) {
	data := marshalAudit(a)
	data = append(data, byte(unstableCounter))
	unstableCounter++
	return data, nil
}

// --- format: deser-unstable ----------------------------------------------

var deserUnstableCounter int

func (a *Audit) UnmarshalDeserUnstable(data []byte) error {
	if err := unmarshalAudit(a, data, true); err != nil {
		return err
	}
	if deserUnstableCounter > 0 {
		a.Value++ // different result on second call
	}
	deserUnstableCounter++
	return nil
}

// --- format: input-mutating ----------------------------------------------

func (a *Audit) UnmarshalInputMutating(data []byte) error {
	if err := unmarshalAudit(a, data, true); err != nil {
		return err
	}
	if len(data) > 0 {
		data[0] ^= 0xFF // MUTATE input buffer after parsing
	}
	return nil
}

// --- format: output-zero-copy --------------------------------------------

func (a *Audit) MarshalOutputZeroCopy() ([]byte, error) {
	buf := marshalAudit(a)
	// Make a.Payload alias a sub-slice of the returned buffer.
	// When audit XOR-flips the buffer, a.Payload changes and the
	// re-extraction detects output-zero-copy.
	off := 4 + 1 + len(a.Tag) + 4 // value + tagLen + tag + payloadLen
	a.Payload = buf[off : off+len(a.Payload)]
	return buf, nil // return FULL buffer for cross-language comparison
}

func main() {
	serify.Run(serify.Suite{
		Types: map[string]serify.Type{
			"audit": {
				Model: &Audit{},
				Formats: map[string]serify.Format{
					"clean": {
						Serializer:   (*Audit).MarshalClean,
						Deserializer: (*Audit).UnmarshalClean,
					},
					"mutating": {
						Serializer:   (*Audit).MarshalMutating,
						Deserializer: (*Audit).UnmarshalClean,
					},
					"value-mutating": {
						Serializer:   Audit.MarshalValueMutating,
						Deserializer: (*Audit).UnmarshalClean,
					},
					"zero-copy": {
						Serializer:   (*Audit).MarshalClean,
						Deserializer: (*Audit).UnmarshalZeroCopy,
					},
					"list-zero-copy": {
						Serializer:   (*Audit).MarshalClean,
						Deserializer: (*Audit).UnmarshalListZeroCopy,
					},
					"unstable": {
						Serializer:   (*Audit).MarshalUnstable,
						Deserializer: (*Audit).UnmarshalClean,
					},
					"deser-unstable": {
						Serializer:   (*Audit).MarshalClean,
						Deserializer: (*Audit).UnmarshalDeserUnstable,
					},
					"input-mutating": {
						Serializer:   (*Audit).MarshalClean,
						Deserializer: (*Audit).UnmarshalInputMutating,
					},
					"output-zero-copy": {
						Serializer:   (*Audit).MarshalOutputZeroCopy,
						Deserializer: (*Audit).UnmarshalClean,
					},
				},
			},
		},
	})
}
