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

// Package typekind defines the canonical wire type-kind names — the strings that
// appear in a schema field's "type". It is the single source of truth shared by
// the runner (internal/*) and the worker library (lib/go/serify), so the spelling never
// drifts. Names are the long form (uint8, int64, float32, …); there are no short
// aliases (u8, i64, f32).
package typekind

import "slices"

// Scalar (non-parametric) type kinds.
const (
	Uint8   = "uint8"
	Uint16  = "uint16"
	Uint32  = "uint32"
	Uint64  = "uint64"
	Uint128 = "uint128"
	Int8    = "int8"
	Int16   = "int16"
	Int32   = "int32"
	Int64   = "int64"
	Int128  = "int128"
	Float32 = "float32"
	Float64 = "float64"
	Bool    = "bool"
	String  = "string"
	Bytes   = "bytes"
)

// Composite type kinds. Struct is non-parametric; the others appear as
// parameterized type strings (list<T>, optional<T>, array<T,N>, map<K,V>).
const (
	Struct   = "struct"
	List     = "list"
	Optional = "optional"
	Array    = "array"
	Map      = "map"
	Enum     = "enum"
	// Sum is a native sum type (a.k.a. tagged union / coproduct): sum<name: T, …>.
	// A value is exactly one variant (its tag + typed payload), so there are no
	// inactive fields to default — unlike an enum tag plus separate flat fields.
	// "sum" is the whole type; each of its arms is a variant (see config.Variant).
	Sum = "sum"
)

// Scalars lists every scalar kind, in declaration order.
var Scalars = []string{
	Uint8, Uint16, Uint32, Uint64, Uint128,
	Int8, Int16, Int32, Int64, Int128,
	Float32, Float64,
	Bool, String, Bytes,
}

// AllBases lists every base kind a schema field can have (scalars + struct +
// the composite bases).
var AllBases = slices.Concat(Scalars, []string{Struct, Optional, List, Array, Map, Enum, Sum})

// SplitTopLevel splits s on every comma that is not nested inside <> or [],
// leaving each part untrimmed. A parameterized type's arguments may themselves
// be parameterized — map<string, map<uint8,uint8>>, sum<a: map<K,V>, b> — so
// the split has to track depth rather than scan for a plain comma.
//
// The runner (which parses type strings out of case files) and the worker
// library (which parses them out of the schema the runner sends) both need this,
// which is why it lives beside the names themselves.
func SplitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '<', '[':
			depth++
		case '>', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}
