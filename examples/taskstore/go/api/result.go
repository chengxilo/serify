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

import "fmt"

// Result is what the server answers with — the `sum` in cases/result.yaml.
//
// The arms are named for the outcome rather than the operation, because several
// operations share one: create, read and update all answer Found.
type Result interface{ isResult() }

type (
	Accepted struct{} // arity 0 — a delete that worked, nothing to send back
	Found    Task     // arity N — the task as it now stands
	Listing  TaskPage // arity N — a page of tasks plus the total
	Failed   APIError // arity N — the request did not work
)

func (Accepted) isResult() {}
func (Found) isResult()    {}
func (Listing) isResult()  {}
func (Failed) isResult()   {}

// Tag ordinals for Result, in the declaration order of cases/result.yaml.
const (
	resultAccepted uint8 = iota
	resultFound
	resultListing
	resultFailed
)

func appendResult(buf []byte, r Result) ([]byte, error) {
	switch v := r.(type) {
	case Accepted:
		return append(buf, resultAccepted), nil
	case Found:
		return appendTask(append(buf, resultFound), Task(v))
	case Listing:
		return appendPage(append(buf, resultListing), TaskPage(v))
	case Failed:
		return appendAPIError(append(buf, resultFailed), APIError(v))
	default:
		return nil, fmt.Errorf("taskstore: unhandled result %T", r)
	}
}

func readResult(b []byte) (Result, []byte, error) {
	tag, b, err := readU8(b)
	if err != nil {
		return nil, b, err
	}
	switch tag {
	case resultAccepted:
		return Accepted{}, b, nil
	case resultFound:
		t, b, err := readTask(b)
		return Found(t), b, err
	case resultListing:
		p, b, err := readPage(b)
		return Listing(p), b, err
	case resultFailed:
		e, b, err := readAPIError(b)
		return Failed(e), b, err
	default:
		return nil, b, fmt.Errorf("taskstore: unknown result tag %d", tag)
	}
}
