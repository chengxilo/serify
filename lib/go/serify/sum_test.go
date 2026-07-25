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
				t.Fatal(err)
			}

			fm, err := DecodeFieldMap(raw, schema)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			v, err := fm.GetVariant("id")
			if err != nil {
				t.Fatalf("GetVariant: %v", err)
			}
			if v.Tag != tc.wantTag || v.Value != tc.wantVal {
				t.Fatalf("got {%q, %v(%T)}, want {%q, %v}", v.Tag, v.Value, v.Value, tc.wantTag, tc.wantVal)
			}

			// Re-encode and compare to the original wire JSON.
			out, err := EncodeFieldMap(fm, schema)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			got, _ := json.Marshal(out)
			if string(got) != tc.wire {
				t.Errorf("round-trip: got %s, want %s", got, tc.wire)
			}
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

	if diffs := CompareFieldMaps(before, fm); len(diffs) != 1 || diffs[0] != "id" {
		t.Errorf("CompareFieldMaps = %v, want [id]: a mutated sum payload must be detected", diffs)
	}
}

// DetectZeroCopy must see through a sum: a bytes payload aliasing the input
// buffer is exactly the case iggy's partitioning<messages_key: bytes> hits.
func TestSum_DetectZeroCopyOnPayload(t *testing.T) {
	buf := []byte{1, 2, 3, 4}
	fm := NewFieldMap()
	fm.SetVariant("id", "key", buf) // aliased, not copied

	if aliased := DetectZeroCopy(fm, buf); len(aliased) != 1 || aliased[0] != "id" {
		t.Errorf("DetectZeroCopy = %v, want [id]", aliased)
	}
}

// A worker builds a sum from scratch via SetVariant, and it encodes correctly.
func TestSum_SetVariantEncodes(t *testing.T) {
	fm := NewFieldMap()
	fm.SetVariant("id", "numeric", uint32(42))

	out, err := EncodeFieldMap(fm, sumSchema())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, _ := json.Marshal(out)
	if string(got) != `{"id":{"numeric":42}}` {
		t.Errorf("got %s", got)
	}
}
