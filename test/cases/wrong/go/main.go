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

// Command worker is the Go half of the `wrong` meta-test. It is a deliberately
// faulty worker: when a fault directive for the active format is disabled it
// corrupts the data by dropping its OWN language ("go") from the langs list, so
// its output disagrees with the other workers. The worker test then verifies serify
// detects and reports that disagreement.
//
// Every language's worker drops only itself, so a future multi-language run will
// surface exactly which languages diverged and how serify renders the mismatch.
// When no fault is injected the worker round-trips all five fields faithfully and
// byte-for-byte identically to the other reference workers.
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"time"

	"github.com/chengxilo/serify/lib/go/serify"
)

type Wrong struct {
	BinarySerialize   bool     `serify:"binary_serialize"   json:"binary_serialize"`
	BinaryDeserialize bool     `serify:"binary_deserialize" json:"binary_deserialize"`
	JSONSerialize     bool     `serify:"json_serialize"     json:"json_serialize"`
	JSONDeserialize   bool     `serify:"json_deserialize"   json:"json_deserialize"`
	Langs             []string `serify:"langs"              json:"langs"`
}

// toUpperSelf turn this worker's own language to upper case.
func toUpperSelf(langs []string) []string {
	if idx := slices.Index(langs, "go"); idx != -1 {
		langs[idx] = "GO"
	}
	return langs
}

func b2i(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// MarshalBinary lays out the four flags as bytes, then a u32-length-prefixed list
// of u32-length-prefixed strings. When binary serialization is disabled it drops
// "go" from the list first, so the bytes diverge from the other workers.
func (w *Wrong) MarshalBinary() ([]byte, error) {
	langs := w.Langs
	if !w.BinarySerialize {
		langs = toUpperSelf(langs)
	}
	buf := []byte{b2i(w.BinarySerialize), b2i(w.BinaryDeserialize), b2i(w.JSONSerialize), b2i(w.JSONDeserialize)}
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(langs)))
	for _, s := range langs {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
		buf = append(buf, s...)
	}
	return buf, nil
}

// UnmarshalBinary reverses MarshalBinary. When binary deserialization is disabled
// it drops "go" from the decoded list, so the round-tripped data diverges from
// the original.
func (w *Wrong) UnmarshalBinary(data []byte) error {
	r := reader{data: data}
	flags, err := r.take(4)
	if err != nil {
		return err
	}
	w.BinarySerialize = flags[0] != 0
	w.BinaryDeserialize = flags[1] != 0
	w.JSONSerialize = flags[2] != 0
	w.JSONDeserialize = flags[3] != 0
	n, err := r.u32()
	if err != nil {
		return err
	}
	langs := make([]string, n)
	for i := range langs {
		s, err := r.str()
		if err != nil {
			return err
		}
		langs[i] = s
	}
	if !w.BinaryDeserialize {
		langs = toUpperSelf(langs)
	}
	w.Langs = langs
	return nil
}

// toJSON emits standard encoding/json (driven by the json tags). It is NOT named
// MarshalJSON on purpose, so json.Marshal below does not recurse. When JSON
// serialization is disabled it drops "go" first.
func (w *Wrong) toJSON() ([]byte, error) {
	out := *w
	if !w.JSONSerialize {
		out.Langs = toUpperSelf(w.Langs)
	}
	return json.Marshal(out)
}

// fromJSON parses standard JSON, dropping "go" when JSON deserialization is
// disabled.
func (w *Wrong) fromJSON(data []byte) error {
	if err := json.Unmarshal(data, w); err != nil {
		return err
	}
	if !w.JSONDeserialize {
		w.Langs = toUpperSelf(w.Langs)
	}
	return nil
}

// Fault-format methods: these return errors, hang, or crash to exercise the
// CLI's error/timeout/crash handling. All four ignore the fault directives
// (the case yaml only carries the schema to satisfy the shared model).

func (w *Wrong) errSerialize() ([]byte, error) {
	return nil, errors.New("injected serialize error")
}

func (w *Wrong) errDeserialize(_ []byte) error {
	return errors.New("injected deserialize error")
}

func (w *Wrong) hangSerialize() ([]byte, error) {
	time.Sleep(3 * time.Second)
	return w.MarshalBinary()
}

func (w *Wrong) crashSerialize() ([]byte, error) {
	os.Exit(3)
	return nil, nil // unreachable
}

// reader is a tiny little-endian byte cursor.
type reader struct {
	data []byte
	pos  int
}

func (r *reader) take(n int) ([]byte, error) {
	if n < 0 || r.pos+n > len(r.data) {
		return nil, &cursorError{}
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *reader) u32() (uint32, error) {
	b, err := r.take(4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (r *reader) str() (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	b, err := r.take(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type cursorError struct{}

func (*cursorError) Error() string { return "unexpected end of input" }

func main() {
	serify.Run(serify.Suite{
		Types: map[string]serify.Type{
			"wrong": {
				Model: &Wrong{},
				Formats: map[string]serify.Format{
					"binary": {
						Serializer:   (*Wrong).MarshalBinary,
						Deserializer: (*Wrong).UnmarshalBinary,
					},
					"json": {
						Serializer:   (*Wrong).toJSON,
						Deserializer: (*Wrong).fromJSON,
					},
					"err_ser": {
						Serializer:   (*Wrong).errSerialize,
						Deserializer: (*Wrong).UnmarshalBinary,
					},
					"err_deser": {
						Serializer:   (*Wrong).MarshalBinary,
						Deserializer: (*Wrong).errDeserialize,
					},
					"hang": {
						Serializer:   (*Wrong).hangSerialize,
						Deserializer: (*Wrong).UnmarshalBinary,
					},
					"crash": {
						Serializer:   (*Wrong).crashSerialize,
						Deserializer: (*Wrong).UnmarshalBinary,
					},
				},
			},
		},
	})
}
