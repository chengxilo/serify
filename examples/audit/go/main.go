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
	"github.com/chengxilo/serify/lib/go/serify"
)

// The worker: one type, three formats, one byte layout.
//
// Run the suite normally and all three formats pass in both languages, because
// they all produce the same bytes. Run it with --audit and two of them report a
// finding. See ../README.md.
//
// The codecs speak the Frame model and never see a FieldMap, which is what a
// worker author actually writes — serify converts on the way in and out. Audit
// is not weakened by that: it snapshots the struct around the serializer and
// extracts a FieldMap that shares the struct's backing memory, so aliasing and
// mutation are caught through the model exactly as they are at the raw
// boundary.
func main() {
	serify.Run(serify.Suite{
		Types: map[string]serify.Type{
			"frame": {
				Model: &Frame{},
				Formats: map[string]serify.Format{
					// Copies out of the input buffer. The control group.
					"safe": {
						Serializer:   (*Frame).Marshal,
						Deserializer: unmarshalSafe,
					},
					// Aliases the input buffer: faster, and a lifetime
					// constraint the bytes cannot express.
					"fast": {
						Serializer:   (*Frame).Marshal,
						Deserializer: unmarshalFast,
					},
					// Scrubs the caller's payload on the way out.
					"handoff": {
						Serializer:   (*Frame).MarshalHandoff,
						Deserializer: unmarshalSafe,
					},
				},
			},
		},
	})
}
