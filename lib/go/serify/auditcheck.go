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
	"bytes"
	"math"
	"reflect"
	"strings"
)

// serializeAuditHolder is captured by buildSerializer's closure and read by the
// NDJSON serialize handler to detect struct mutation during serialization.
type serializeAuditHolder struct {
	Enabled       bool
	LastMutations []string // populated after each serialize call
	LastOutputZC  []string // populated after each serialize call
}

// fieldSnap holds a snapshot of a field value before buffer corruption,
// used by DetectZeroCopy. orig is a deep clone of the whole field value.
type fieldSnap struct {
	fm   *FieldMap
	key  string
	orig any
}

// SnapshotFieldMap deep-copies a FieldMap, cloning []byte values so in-place
// mutations to the original are visible as diffs. Recurses into nested
// *FieldMap, []*FieldMap, and map[string]any containing *FieldMap.
//
// This is a low-level audit helper. If you use the serify conformance runner,
// you do not need to call this directly — register your serialize/deserialize
// functions in a Suite and Run() handles audit automatically via --audit.
// Call this directly only if you want audit-style checks outside the runner.
func SnapshotFieldMap(src *FieldMap) *FieldMap {
	if src == nil {
		return nil
	}
	dst := NewFieldMap()
	for k, v := range src.fields {
		dst.fields[k] = cloneValue(v)
	}
	return dst
}

// cloneValue deep-copies a single field value, cloning []byte slices, strings,
// []string slices, []any slices, and recursing into *FieldMap, []*FieldMap,
// and map[string]any.
//
//nolint:gocognit // deep clone must enumerate every FieldMap value kind
func cloneValue(v any) any {
	switch x := v.(type) {
	case []byte:
		if x == nil {
			return nil
		}
		return bytes.Clone(x)
	case string:
		return strings.Clone(x)
	case []string:
		if x == nil {
			return nil
		}
		out := make([]string, len(x))
		for i, s := range x {
			out[i] = strings.Clone(s)
		}
		return out
	case []any:
		if x == nil {
			return nil
		}
		out := make([]any, len(x))
		for i, elem := range x {
			out[i] = cloneValue(elem)
		}
		return out
	case *FieldMap:
		return SnapshotFieldMap(x)
	case []*FieldMap:
		if x == nil {
			return nil
		}
		out := make([]*FieldMap, len(x))
		for i, fm := range x {
			out[i] = SnapshotFieldMap(fm)
		}
		return out
	case map[string]any:
		if x == nil {
			return nil
		}
		out := make(map[string]any, len(x))
		for mk, mv := range x {
			out[mk] = cloneValue(mv)
		}
		return out
	case *Variant:
		// A oneof payload is aliasing-capable (bytes, string, nested struct), so
		// the snapshot needs its own copy or a mutation would show up in both.
		if x == nil {
			return nil
		}
		return &Variant{Tag: strings.Clone(x.Tag), Value: cloneValue(x.Value)}
	default:
		return v
	}
}

// CompareFieldMaps returns field names that differ between before and after.
// Uses bytes.Equal for []byte, recursive comparison for nested *FieldMap,
// and reflect.DeepEqual for everything else.
//
// This is a low-level audit helper. If you use the serify conformance runner,
// you do not need to call this directly — register your serialize/deserialize
// functions in a Suite and Run() handles audit automatically via --audit.
// Call this directly only if you want audit-style checks outside the runner.
func CompareFieldMaps(before, after *FieldMap) []string {
	if before == nil && after == nil {
		return nil
	}
	if before == nil || after == nil {
		return []string{"<nil>"}
	}
	var diffs []string
	for k, bv := range before.fields {
		av, ok := after.fields[k]
		if !ok {
			diffs = append(diffs, k)
			continue
		}
		if !valuesEqual(bv, av) {
			diffs = append(diffs, k)
		}
	}
	for k := range after.fields {
		if _, ok := before.fields[k]; !ok {
			diffs = append(diffs, k)
		}
	}
	return diffs
}

// valuesEqual compares two field values for equality.
//
//nolint:gocognit // deep compare must enumerate every FieldMap value kind
func valuesEqual(a, b any) bool {
	ab, aIsBytes := a.([]byte)
	bb, bIsBytes := b.([]byte)
	if aIsBytes && bIsBytes {
		return bytes.Equal(ab, bb)
	}
	// Compare floats by bit pattern, not value: a NaN is never == itself, so
	// reflect.DeepEqual(NaN, NaN) is false. Without this a field holding NaN
	// makes a perfectly deterministic worker look like it mutated, aliased and
	// deserialized unstably — three false audit warnings from one corner.
	if af, ok := a.(float64); ok {
		if bf, ok := b.(float64); ok {
			return math.Float64bits(af) == math.Float64bits(bf)
		}
	}
	if af, ok := a.(float32); ok {
		if bf, ok := b.(float32); ok {
			return math.Float32bits(af) == math.Float32bits(bf)
		}
	}
	afm, aIsFM := a.(*FieldMap)
	bfm, bIsFM := b.(*FieldMap)
	if aIsFM && bIsFM {
		return len(CompareFieldMaps(afm, bfm)) == 0
	}
	afms, aIsFMS := a.([]*FieldMap)
	bfms, bIsFMS := b.([]*FieldMap)
	if aIsFMS && bIsFMS {
		if len(afms) != len(bfms) {
			return false
		}
		for i := range afms {
			if len(CompareFieldMaps(afms[i], bfms[i])) > 0 {
				return false
			}
		}
		return true
	}
	am, aIsMap := a.(map[string]any)
	bm, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		if len(am) != len(bm) {
			return false
		}
		for k, av := range am {
			bv, ok := bm[k]
			if !ok || !valuesEqual(av, bv) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(a, b)
}

// xorFlip XORs every byte in buf with 0xFF. Calling it twice restores the
// original content (involution).
func xorFlip(buf []byte) {
	for i := range buf {
		buf[i] ^= 0xFF
	}
}

// DetectZeroCopy performs an active overwrite test: snapshots all aliasing-
// capable fields ([]byte, string, and container fields containing them),
// XOR-flips the input buffer, and checks which fields changed. Fields that
// changed were aliasing the buffer. Restores original values after.
// Returns the list of field names that exhibited zero-copy aliasing.
//
// This is a low-level audit helper. If you use the serify conformance runner,
// you do not need to call this directly — register your serialize/deserialize
// functions in a Suite and Run() handles audit automatically via --audit.
// Call this directly only if you want audit-style checks outside the runner.
func DetectZeroCopy(fm *FieldMap, buf []byte) []string {
	if len(buf) == 0 {
		return nil
	}

	// 1. Walk recursively and snapshot all aliasing-capable fields
	var snaps []fieldSnap
	collectFieldSnaps(fm, &snaps)

	// 2. XOR-flip every byte in buf
	xorFlip(buf)

	// 3. Check which fields changed
	var aliased []string
	for _, s := range snaps {
		current := s.fm.fields[s.key]
		if !valuesEqual(current, s.orig) {
			aliased = append(aliased, s.key)
		}
	}

	// 4. Restore original values
	for _, s := range snaps {
		s.fm.fields[s.key] = s.orig
	}

	return aliased
}

// collectFieldSnaps recursively walks a FieldMap and appends snapshots of all
// aliasing-capable fields. For scalar bytes/string fields, snapshots the
// individual value. For containers with aliasing-capable leaves ([]string,
// []any with strings, map[string]any with bytes/string values), snapshots
// the whole field value. Recurse into struct-shaped values. Scalar-only
// containers are skipped.
//
//nolint:gocognit // snapshots every FieldMap value kind for the audit diff
func collectFieldSnaps(fm *FieldMap, snaps *[]fieldSnap) {
	if fm == nil {
		return
	}
	for k, v := range fm.fields {
		switch x := v.(type) {
		case []byte:
			*snaps = append(*snaps, fieldSnap{fm: fm, key: k, orig: bytes.Clone(x)})
		case string:
			*snaps = append(*snaps, fieldSnap{fm: fm, key: k, orig: strings.Clone(x)})
		case []string:
			*snaps = append(*snaps, fieldSnap{fm: fm, key: k, orig: cloneValue(x)})
		case []any:
			if hasStringOrBytesInSlice(x) {
				*snaps = append(*snaps, fieldSnap{fm: fm, key: k, orig: cloneValue(x)})
			} else {
				for _, elem := range x {
					if nested, ok := elem.(*FieldMap); ok {
						collectFieldSnaps(nested, snaps)
					}
				}
			}
		case *FieldMap:
			collectFieldSnaps(x, snaps)
		case []*FieldMap:
			for _, nested := range x {
				collectFieldSnaps(nested, snaps)
			}
		case map[string]any:
			if hasBytesOrStringValue(x) {
				*snaps = append(*snaps, fieldSnap{fm: fm, key: k, orig: cloneValue(x)})
			} else {
				for _, mv := range x {
					if nested, ok := mv.(*FieldMap); ok {
						collectFieldSnaps(nested, snaps)
					}
				}
			}
		case *Variant:
			// The variant itself is not aliasing-capable, but its payload is:
			// snapshot the whole field so a zero-copy payload shows as a change.
			if x == nil {
				continue
			}
			switch payload := x.Value.(type) {
			case []byte, string:
				*snaps = append(*snaps, fieldSnap{fm: fm, key: k, orig: cloneValue(x)})
			case *FieldMap:
				collectFieldSnaps(payload, snaps)
			}
		}
	}
}

// hasStringOrBytesInSlice returns true if the slice contains any string or
// []byte elements (both aliasing-capable).
func hasStringOrBytesInSlice(x []any) bool {
	for _, elem := range x {
		switch elem.(type) {
		case string, []byte:
			return true
		}
	}
	return false
}

// hasBytesOrStringValue returns true if the map has any []byte or string values.
func hasBytesOrStringValue(x map[string]any) bool {
	for _, v := range x {
		switch v.(type) {
		case []byte, string:
			return true
		}
	}
	return false
}

// DetectInputMutation checks if the input buffer was modified during deserialization.
//
// This is a low-level audit helper. If you use the serify conformance runner,
// you do not need to call this directly — register your serialize/deserialize
// functions in a Suite and Run() handles audit automatically via --audit.
// Call this directly only if you want audit-style checks outside the runner.
func DetectInputMutation(before, current []byte) bool {
	return !bytes.Equal(before, current)
}
