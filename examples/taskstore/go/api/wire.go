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

package api

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// The byte layout every taskstore client has to reproduce.
//
// Go is this suite's leader, and these primitives are the whole reason it can
// be: the server writes its responses through them, so "the reference layout"
// and "what the running server puts on the socket" are the same code. There is
// no second, test-only encoder that could drift from the real one.
//
// Little-endian throughout:
//
//	u8/u32/u64   fixed width, little-endian
//	string       u32 byte length, then the UTF-8 bytes
//	enum         u8 ordinal — the variant's position in the case file's enum
//	optional<T>  u8 presence flag (0 = absent, 1 = present), then T if present
//	list<T>      u32 element count, then each element
//	struct       its fields back to back, in schema order
//	sum          u8 tag ordinal (declaration order in the case file), then the
//	             active variant's payload — nothing at all for a unit variant
//
// Nothing here holds a map, so every type in this suite declares `oracle: bytes`
// and the comparison is byte-for-byte. That is the strictest thing serify can
// assert, and it is the right setting for a format two programs have to agree on
// well enough to hold a conversation over a socket.

// ErrTruncated is returned when a frame ends in the middle of a value. A client
// reading from a socket sees this constantly and it is not an error worth
// distinguishing: the frame was bad, drop it.
var ErrTruncated = errors.New("taskstore: truncated message")

func appendString(buf []byte, s string) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
	return append(buf, s...)
}

func readString(b []byte) (string, []byte, error) {
	if len(b) < 4 {
		return "", b, ErrTruncated
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < n {
		return "", b, ErrTruncated
	}
	return string(b[:n]), b[n:], nil
}

func appendBool(buf []byte, v bool) []byte {
	if v {
		return append(buf, 1)
	}
	return append(buf, 0)
}

func readU8(b []byte) (uint8, []byte, error) {
	if len(b) < 1 {
		return 0, b, ErrTruncated
	}
	return b[0], b[1:], nil
}

func readU32(b []byte) (uint32, []byte, error) {
	if len(b) < 4 {
		return 0, b, ErrTruncated
	}
	return binary.LittleEndian.Uint32(b), b[4:], nil
}

func readU64(b []byte) (uint64, []byte, error) {
	if len(b) < 8 {
		return 0, b, ErrTruncated
	}
	return binary.LittleEndian.Uint64(b), b[8:], nil
}

// appendEnum writes an enum as the ordinal of its position in variants.
//
// An enum travels through serify as its *name*; the ordinal is this codec's own
// byte-layout choice, so the variants slice has to list them in the same order
// as the case file. That coupling is real, and it is exactly the kind of thing a
// conformance run catches when a follower gets it wrong.
func appendEnum(buf []byte, variants []string, name string) ([]byte, error) {
	for i, v := range variants {
		if v == name {
			return append(buf, uint8(i)), nil
		}
	}
	return nil, fmt.Errorf("taskstore: %q is not one of %v", name, variants)
}

func readEnum(b []byte, variants []string) (string, []byte, error) {
	ord, b, err := readU8(b)
	if err != nil {
		return "", b, err
	}
	if int(ord) >= len(variants) {
		return "", b, fmt.Errorf("taskstore: enum ordinal %d out of range for %v", ord, variants)
	}
	return variants[ord], b, nil
}
