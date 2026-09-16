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
	"math"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectZeroCopy_BytesAliasing(t *testing.T) {
	buf := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	aliased := buf[1:4] // sub-slice aliasing buf
	fm := NewFieldMap()
	fm.SetBytes("payload", aliased)

	got := detectZeroCopy(fm, buf)
	assert.Equal(t, []string{"payload"}, got, "expected [payload], got %v", got)
}

func TestDetectZeroCopy_BytesIndependent(t *testing.T) {
	buf := []byte{0x01, 0x02, 0x03}
	independent := make([]byte, len(buf))
	copy(independent, buf)
	fm := NewFieldMap()
	fm.SetBytes("payload", independent)

	got := detectZeroCopy(fm, buf)
	assert.Empty(t, got, "expected no aliasing, got %v", got)
}

func TestDetectZeroCopy_StringAliasing(t *testing.T) {
	buf := []byte{'h', 'e', 'l', 'l', 'o'}
	s := unsafe.String(&buf[0], len(buf))
	fm := NewFieldMap()
	fm.SetString("tag", s)

	got := detectZeroCopy(fm, buf)
	assert.Equal(t, []string{"tag"}, got, "expected [tag], got %v", got)
}

func TestDetectZeroCopy_StringCopied(t *testing.T) {
	buf := []byte{'h', 'e', 'l', 'l', 'o'}
	s := string(buf) // always copies in Go
	fm := NewFieldMap()
	fm.SetString("tag", s)

	got := detectZeroCopy(fm, buf)
	assert.Empty(t, got, "expected no aliasing, got %v", got)
}

func TestDetectZeroCopy_Nested(t *testing.T) {
	buf := []byte{0x01, 0x02, 0x03, 0x04}
	aliased := buf[1:3]
	nested := NewFieldMap()
	nested.SetBytes("street", aliased)
	fm := NewFieldMap()
	fm.SetStruct("address", nested)

	got := detectZeroCopy(fm, buf)
	assert.Equal(t, []string{"street"}, got, "expected [street], got %v", got)
}

func TestDetectZeroCopy_EmptyBuffer(t *testing.T) {
	fm := NewFieldMap()
	fm.SetBytes("payload", []byte{0x01})

	got := detectZeroCopy(fm, nil)
	assert.Nil(t, got, "expected nil for nil buffer, got %v", got)
	got = detectZeroCopy(fm, []byte{})
	assert.Nil(t, got, "expected nil for empty buffer, got %v", got)
}

func TestDetectZeroCopy_Restores(t *testing.T) {
	buf := []byte{0x01, 0x02, 0x03, 0x04}
	aliased := buf[1:3] // [0x02, 0x03]
	fm := NewFieldMap()
	fm.SetBytes("payload", aliased)

	detectZeroCopy(fm, buf)

	got, err := fm.GetBytes("payload")
	require.NoError(t, err)
	assert.Equal(t, []byte{0x02, 0x03}, got, "expected [0x02, 0x03] restored, got %v", got)
}

func TestDetectInputMutation(t *testing.T) {
	before := []byte{0x01, 0x02, 0x03}

	// Identical → no mutation
	assert.False(t, detectInputMutation(before, []byte{0x01, 0x02, 0x03}), "expected no mutation for identical buffers")

	// Different → mutation
	assert.True(t, detectInputMutation(before, []byte{0x01, 0xFF, 0x03}), "expected mutation detected for different buffers")
}

func TestCompareFieldMaps_Equal(t *testing.T) {
	fm1 := NewFieldMap()
	fm1.SetU32("value", 42)
	fm1.SetBytes("payload", []byte{0x01, 0x02})

	fm2 := NewFieldMap()
	fm2.SetU32("value", 42)
	fm2.SetBytes("payload", []byte{0x01, 0x02})

	diffs := compareFieldMaps(fm1, fm2)
	assert.Empty(t, diffs, "expected no diffs, got %v", diffs)
}

func TestCompareFieldMaps_ByteDiff(t *testing.T) {
	fm1 := NewFieldMap()
	fm1.SetBytes("payload", []byte{0x01, 0x02, 0x03})

	fm2 := NewFieldMap()
	fm2.SetBytes("payload", []byte{0xFF, 0x02, 0x03})

	diffs := compareFieldMaps(fm1, fm2)
	assert.Equal(t, []string{"payload"}, diffs, "expected [payload], got %v", diffs)
}

func TestCompareFieldMaps_ScalarDiff(t *testing.T) {
	fm1 := NewFieldMap()
	fm1.SetU64("value", 42)

	fm2 := NewFieldMap()
	fm2.SetU64("value", 99)

	diffs := compareFieldMaps(fm1, fm2)
	assert.Equal(t, []string{"value"}, diffs, "expected [value], got %v", diffs)
}

// TestCompareFieldMaps_NaN pins that two bit-identical NaN payloads compare
// equal. reflect.DeepEqual(NaN, NaN) is false — NaN is never == itself — so
// without a bit-pattern compare a field holding NaN made a deterministic worker
// look like it mutated, aliased and deserialized unstably: three false audit
// warnings from one corner. A genuinely different float still diffs.
func TestCompareFieldMaps_NaN(t *testing.T) {
	nan := math.NaN()

	same1 := NewFieldMap()
	same1.SetF64("v", nan)
	same2 := NewFieldMap()
	same2.SetF64("v", nan)
	assert.Empty(t, compareFieldMaps(same1, same2), "two NaN values must compare equal")

	// float32 NaN too.
	f1 := NewFieldMap()
	f1.SetF32("v", float32(math.NaN()))
	f2 := NewFieldMap()
	f2.SetF32("v", float32(math.NaN()))
	assert.Empty(t, compareFieldMaps(f1, f2), "two float32 NaN values must compare equal")

	// A real change is still a diff — the fix must not blind the comparison.
	changed := NewFieldMap()
	changed.SetF64("v", 1.5)
	diffs := compareFieldMaps(same1, changed)
	assert.Equal(t, []string{"v"}, diffs, "NaN vs 1.5 must diff, got %v", diffs)
}

func TestSnapshotFieldMap_DeepCopy(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03}
	fm := NewFieldMap()
	fm.SetBytes("payload", payload)
	fm.SetString("tag", "hello")

	snap := snapshotFieldMap(fm)

	// Mutate original
	payload[0] = 0xFF
	fm.SetString("tag", "world")

	// Snapshot should be unchanged
	got, _ := snap.GetBytes("payload")
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, got, "expected [0x01, 0x02, 0x03], got %v", got)
	gotStr, _ := snap.GetString("tag")
	assert.Equal(t, "hello", gotStr, "expected hello, got %v", gotStr)
}

// TestCompareFieldMaps_NilValuesAreNotMutations guards cloneValue's typed nils.
// A bare `nil` there loses the type, valuesEqual's typed branches stop matching,
// and any worker holding a nil slice is told it mutated its input, aliased its
// output and deserialized unstably.
func TestCompareFieldMaps_NilValuesAreNotMutations(t *testing.T) {
	fm := NewFieldMap()
	fm.SetBytes("payload", nil)
	fm.SetListString("tags", nil)
	fm.SetListStruct("items", nil)
	fm.SetMap("labels", nil)
	fm.SetVariant("choice", "silent", nil)
	fm.fields["raw"] = []any(nil)

	// A snapshot of an untouched FieldMap must compare equal to it.
	assert.Empty(t, compareFieldMaps(snapshotFieldMap(fm), fm),
		"a FieldMap full of nil values reported mutations against its own snapshot")

	// A real change to one of those fields must still be caught.
	before := snapshotFieldMap(fm)
	fm.SetBytes("payload", []byte{1})
	assert.Equal(t, []string{"payload"}, compareFieldMaps(before, fm))
}
