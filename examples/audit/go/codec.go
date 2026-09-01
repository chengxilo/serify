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
	"unsafe"
)

// Frame is the model: an ordinary struct with ordinary methods, which is what a
// worker author actually writes. serify converts between it and the FieldMap on
// the way in and out, so none of the codecs below ever names a field twice.
//
// Audit sees straight through it: the FieldMap serify extracts shares backing
// memory with the struct's slices and strings, so a codec that aliases the
// input buffer or scribbles on its caller is caught here exactly as it would be
// at the FieldMap boundary.
type Frame struct {
	StreamID uint32   `serify:"stream_id"`
	Title    string   `serify:"title"`
	Tags     []string `serify:"tags"`
	Payload  []byte   `serify:"payload"`
}

// The three codecs. Every one of them reads and writes the same layout, so for
// any given case all three produce byte-identical output — `serify run` alone
// cannot tell them apart. What differs is memory behaviour, and that is what
// `serify run --audit` inspects.

// --- shared encoder ---------------------------------------------------------

// Marshal writes the frame. `safe` and `fast` both register it unchanged: the
// two formats differ only in how they decode.
func (f *Frame) Marshal() ([]byte, error) {
	buf := binary.LittleEndian.AppendUint32(nil, f.StreamID)
	buf = appendStr(buf, f.Title)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(f.Tags)))
	for _, t := range f.Tags {
		buf = appendStr(buf, t)
	}
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(f.Payload)))
	return append(buf, f.Payload...), nil
}

// --- format: safe -----------------------------------------------------------

// unmarshalSafe copies every field out of the input buffer. The decoded frame
// owns its memory and outlives the buffer it came from.
func unmarshalSafe(data []byte) (*Frame, error) {
	return decode(data, false)
}

// --- format: fast -----------------------------------------------------------

// unmarshalFast points every string and the payload straight at the input
// buffer instead of copying. On a hot path that is a real optimization: this
// decoder does no allocation for the frame's contents at all.
//
// It also quietly changes the contract. The returned frame is only valid while
// `data` is unmodified and alive — so a caller that reuses a read buffer, or a
// pool that hands the buffer back, now corrupts frames that already looked
// decoded. Nothing about the bytes says so, which is why audit exists.
func unmarshalFast(data []byte) (*Frame, error) {
	return decode(data, true)
}

// decode walks the layout once. `alias` picks copying or aliasing for the three
// fields that can be either; keeping both in one function is what makes it
// clear that the formats agree about the bytes and disagree about nothing else.
func decode(data []byte, alias bool) (*Frame, error) {
	if len(data) < 4 {
		return nil, errTruncated
	}
	f := &Frame{StreamID: binary.LittleEndian.Uint32(data)}
	pos := 4

	title, pos, err := takeString(data, pos, alias)
	if err != nil {
		return nil, err
	}
	f.Title = title

	if pos+4 > len(data) {
		return nil, errTruncated
	}
	count := int(binary.LittleEndian.Uint32(data[pos:]))
	pos += 4
	f.Tags = make([]string, 0, min(count, 1024))
	for range count {
		var tag string
		if tag, pos, err = takeString(data, pos, alias); err != nil {
			return nil, err
		}
		f.Tags = append(f.Tags, tag)
	}

	n, body, err := readLen(data, pos)
	if err != nil {
		return nil, err
	}
	payload := data[body : body+n]
	if !alias {
		payload = bytes.Clone(payload)
	}
	f.Payload = payload

	return f, nil
}

// takeString reads one length-prefixed string, either copying it or aliasing
// the buffer.
//
// The zero-length guard is load-bearing in both directions: unsafe.String needs
// a valid pointer, and at the end of the buffer there is no byte to take the
// address of. An empty string also has nothing to alias, which is why the
// `empty` case reports no finding.
func takeString(data []byte, pos int, alias bool) (string, int, error) {
	n, body, err := readLen(data, pos)
	if err != nil {
		return "", 0, err
	}
	if n == 0 {
		return "", body, nil
	}
	if alias {
		return unsafe.String(&data[body], n), body + n, nil
	}
	return string(data[body : body+n]), body + n, nil
}

// --- format: handoff --------------------------------------------------------

// MarshalHandoff writes the frame and then scrubs the payload buffer, on the
// reasoning that the bytes are out and the buffer can go back to the pool
// clean.
//
// That is the bug: it empties the caller's frame as a side effect of being
// asked to read it. The output bytes are correct and identical to `safe`, so
// conformance stays green; audit catches it by comparing the model before and
// after the call.
//
// It reports instability too, and that finding explains the first: serify
// serializes twice, and the second call reads a payload the first already
// wiped. One bug seen from both ends.
//
// Scrubbing rather than reassigning `f.Payload = nil` is deliberate. Both are
// mutations, but only this one also reads as unstable: serify rebuilds the
// model from the same FieldMap for the repeat call, so a reassigned field is
// restored while scribbled-on shared bytes are not.
func (f *Frame) MarshalHandoff() ([]byte, error) {
	out, err := f.Marshal()
	if err != nil {
		return nil, err
	}
	clear(f.Payload) // "we are done with it"
	return out, nil
}
