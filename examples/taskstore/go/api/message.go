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

import "encoding/binary"

// Request and Response are the two types that actually cross the socket, and
// the only two the conformance suite tests. Everything else in this package is
// reachable from one of them.

// Request is one request frame: cases/request.yaml.
type Request struct {
	RequestID uint32 `serify:"request_id"`
	Op        Op     `serify:"op"`
}

// Response is one response frame: cases/response.yaml. RequestID echoes the
// request's, so a client that has several in flight can tell them apart.
type Response struct {
	RequestID uint32 `serify:"request_id"`
	Result    Result `serify:"result"`
}

func (r *Request) MarshalBinary() ([]byte, error) {
	buf := binary.LittleEndian.AppendUint32(nil, r.RequestID)
	return appendOp(buf, r.Op)
}

func (r *Request) UnmarshalBinary(data []byte) error {
	id, b, err := readU32(data)
	if err != nil {
		return err
	}
	op, _, err := readOp(b)
	if err != nil {
		return err
	}
	r.RequestID, r.Op = id, op
	return nil
}

func (r *Response) MarshalBinary() ([]byte, error) {
	buf := binary.LittleEndian.AppendUint32(nil, r.RequestID)
	return appendResult(buf, r.Result)
}

func (r *Response) UnmarshalBinary(data []byte) error {
	id, b, err := readU32(data)
	if err != nil {
		return err
	}
	res, _, err := readResult(b)
	if err != nil {
		return err
	}
	r.RequestID, r.Result = id, res
	return nil
}
