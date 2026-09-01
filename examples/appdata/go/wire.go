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

// Go is the --ref language for this suite, so the bytes these types produce
// define the layout every other worker has to reproduce.
//
// Binary layout — little-endian throughout:
//
//	string       u32 byte length, then the UTF-8 bytes
//	bytes        u32 byte length, then the raw bytes
//	optional<T>  u8 presence flag (0 = absent, 1 = present), then T if present
//	list<T>      u32 element count, then each element
//	map<K,V>     u32 entry count, then each key/value pair, in the map's own order
//	array<T,N>   exactly N elements, no length prefix (N is fixed by the schema)
//	struct       its fields back to back, in schema order
//	sum          u8 tag ordinal (declaration order in the case file), then the
//	             active variant's payload — nothing at all for a unit variant
//	int128       16 bytes, two's complement
//
// Map entry order is unspecified, and this worker does not sort. A map<K,V> is
// unordered, so requiring one key-derived order was requiring every worker in
// every language to implement a collation its own map type does not have — and
// to get UTF-8 vs UTF-16 collation right while doing it. Types holding a map
// declare `oracle: semantic` instead, which compares the decoded value rather
// than the bytes. See docs/protocol.md § Comparison oracles.

var errTruncated = errors.New("truncated")

func appendLenStr(buf []byte, s string) []byte {
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
	return append(buf, s...)
}

func readLenStr(b []byte) (string, []byte, error) {
	if len(b) < 4 {
		return "", b, errTruncated
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < n {
		return "", b, errTruncated
	}
	return string(b[:n]), b[n:], nil
}

// mapKeys returns the map's keys in whatever order Go hands them over —
// deliberately not sorted. A map<K,V> is unordered, so the worker emits its
// natural order and the semantic oracle judges the decoded value instead of the
// bytes. Go randomises map iteration per run, which makes this worker a live
// check that the oracle really is order-insensitive.
func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
