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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identifier-shaped sum: a unit variant plus two single-payload variants.
func sumSchema() []SchemaField {
	return []SchemaField{{
		Name: "id",
		Type: "sum<balanced, numeric: uint32, name: string>",
		Variants: []SchemaVariant{
			{Name: "balanced"},
			{Name: "numeric", Payload: &SchemaField{Name: "numeric", Type: "uint32"}},
			{Name: "name", Payload: &SchemaField{Name: "name", Type: "string"}},
		},
	}}
}

func TestSum_DecodeEncodeRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		wire    string
		wantTag string
		wantVal any
	}{
		{"payload numeric", `{"id":{"numeric":5}}`, "numeric", uint32(5)},
		{"payload string", `{"id":{"name":"iggy"}}`, "name", "iggy"},
		{"unit variant", `{"id":{"balanced":null}}`, "balanced", nil},
	}
	schema := sumSchema()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.wire), &raw); err != nil {
				require.NoError(t, err)
			}

			fm, err := DecodeFieldMap(raw, schema)
			require.NoError(t, err, "decode: %v", err)
			v, err := fm.GetVariant("id")
			require.NoError(t, err, "GetVariant: %v", err)
			require.Equal(t, tc.wantTag, v.Tag, "got {%q, %v(%T)}, want {%q, %v}", v.Tag, v.Value, v.Value, tc.wantTag, tc.wantVal)
			assert.Equal(t, tc.wantVal, v.Value, "got {%q, %v(%T)}, want {%q, %v}", v.Tag, v.Value, v.Value, tc.wantTag, tc.wantVal)

			// Re-encode and compare to the original wire JSON.
			out, err := EncodeFieldMap(fm, schema)
			require.NoError(t, err, "encode: %v", err)
			got, _ := json.Marshal(out)
			assert.Equal(t, tc.wire, string(got), "round-trip: got %s, want %s", got, tc.wire)
		})
	}
}

// A sum payload is aliasing-capable, so the audit snapshot must deep-copy it.
// Without a *Variant case in cloneValue the snapshot shares the payload slice,
// a serializer mutating it in place shows no diff, and the audit goes green.
func TestSum_SnapshotDeepCopiesPayload(t *testing.T) {
	fm := NewFieldMap()
	fm.SetVariant("id", "key", []byte{1, 2, 3})

	before := SnapshotFieldMap(fm)
	payload, _ := fm.GetVariant("id")
	payload.Value.([]byte)[0] = 0xFF // a serializer mutating its input

	diffs := CompareFieldMaps(before, fm)
	assert.Equal(t, []string{"id"}, diffs, "CompareFieldMaps = %v, want [id]: a mutated sum payload must be detected", diffs)
}

// DetectZeroCopy must see through a sum: a bytes payload aliasing the input
// buffer is exactly the case iggy's partitioning<messages_key: bytes> hits.
func TestSum_DetectZeroCopyOnPayload(t *testing.T) {
	buf := []byte{1, 2, 3, 4}
	fm := NewFieldMap()
	fm.SetVariant("id", "key", buf) // aliased, not copied

	aliased := DetectZeroCopy(fm, buf)
	assert.Equal(t, []string{"id"}, aliased, "DetectZeroCopy = %v, want [id]", aliased)
}

// A worker builds a sum from scratch via SetVariant, and it encodes correctly.
func TestSum_SetVariantEncodes(t *testing.T) {
	fm := NewFieldMap()
	fm.SetVariant("id", "numeric", uint32(42))

	out, err := EncodeFieldMap(fm, sumSchema())
	require.NoError(t, err, "encode: %v", err)
	got, _ := json.Marshal(out)
	assert.Equal(t, `{"id":{"numeric":42}}`, string(got), "got %s", got)
}
