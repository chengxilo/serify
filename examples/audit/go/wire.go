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
	"encoding/binary"
	"errors"
)

// Go is this suite's leader, so the bytes these helpers produce define the
// layout the Rust worker has to reproduce.
//
// Little-endian throughout:
//
//	string   u32 byte length, then the UTF-8 bytes
//	bytes    u32 byte length, then the raw bytes
//	list<T>  u32 element count, then each element
//
// All three formats share this layout exactly. They differ only in what their
// codecs do with memory, which is the whole subject of this example.

var errTruncated = errors.New("audit example: truncated frame")

func appendStr(buf []byte, s string) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
	return append(buf, s...)
}

// readLen reads a u32 length at pos and returns it with the position of the
// body that follows, checking that the body is actually there.
func readLen(data []byte, pos int) (n, body int, err error) {
	if pos+4 > len(data) {
		return 0, 0, errTruncated
	}
	n = int(binary.LittleEndian.Uint32(data[pos:]))
	body = pos + 4
	if body+n > len(data) {
		return 0, 0, errTruncated
	}
	return n, body, nil
}
