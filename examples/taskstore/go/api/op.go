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
	"fmt"
)

// Op is the operation a request asks for — the `sum` in cases/op.yaml.
//
// Go has no sum type, so the idiomatic encoding is a sealed interface: the
// unexported marker method means only the five types below can be an Op. With a
// bare `any` you could put a `time.Time` in a request and find out at run time.
//
// What sealing does not buy is exhaustiveness — add a sixth operation and Go
// will not point at the switches below. Rust's `match` would. The conformance
// run is the backstop: a new arm the followers do not implement fails loudly
// rather than shipping.
type Op interface{ isOp() }

type (
	ListAll struct{} // arity 0 — a unit variant, no payload
	Create  Draft    // arity N — a struct payload
	Read    uint64   // arity 1 — a scalar payload: the id
	Update  Task     // arity N — the whole task, id included
	Delete  uint64   // arity 1 — the id
)

func (ListAll) isOp() {}
func (Create) isOp()  {}
func (Read) isOp()    {}
func (Update) isOp()  {}
func (Delete) isOp()  {}

// Tag ordinals for Op, in the declaration order of cases/op.yaml.
const (
	opListAll uint8 = iota
	opCreate
	opRead
	opUpdate
	opDelete
)

func appendOp(buf []byte, op Op) ([]byte, error) {
	switch v := op.(type) {
	case ListAll:
		return append(buf, opListAll), nil // a unit variant is nothing but its tag
	case Create:
		return appendDraft(append(buf, opCreate), Draft(v))
	case Read:
		return binary.LittleEndian.AppendUint64(append(buf, opRead), uint64(v)), nil
	case Update:
		return appendTask(append(buf, opUpdate), Task(v))
	case Delete:
		return binary.LittleEndian.AppendUint64(append(buf, opDelete), uint64(v)), nil
	default:
		return nil, fmt.Errorf("taskstore: unhandled op %T", op)
	}
}

func readOp(b []byte) (Op, []byte, error) {
	tag, b, err := readU8(b)
	if err != nil {
		return nil, b, err
	}
	switch tag {
	case opListAll:
		return ListAll{}, b, nil
	case opCreate:
		d, b, err := readDraft(b)
		return Create(d), b, err
	case opRead:
		id, b, err := readU64(b)
		return Read(id), b, err
	case opUpdate:
		t, b, err := readTask(b)
		return Update(t), b, err
	case opDelete:
		id, b, err := readU64(b)
		return Delete(id), b, err
	default:
		return nil, b, fmt.Errorf("taskstore: unknown op tag %d", tag)
	}
}
