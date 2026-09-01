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
	"io"
)

// Framing. A TCP connection is a byte stream with no message boundaries in it,
// so the protocol puts them there: every message is a u32 little-endian byte
// length followed by exactly that many bytes.
//
// This sits deliberately outside the conformance suite. serify tests the
// *contents* of a message — it hands a worker one message's bytes and compares
// them — so the frame header is not part of any case, and a follower is free to
// read its socket however it likes as long as it agrees on what is inside.

// MaxFrameLen caps a single message. Without it, a bad or hostile length prefix
// is an allocation of up to 4 GiB from a header the server has not validated.
const MaxFrameLen = 1 << 20

// WriteFrame writes payload with its length prefix.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > MaxFrameLen {
		return fmt.Errorf("taskstore: message of %d bytes exceeds the %d limit", len(payload), MaxFrameLen)
	}
	buf := binary.LittleEndian.AppendUint32(make([]byte, 0, 4+len(payload)), uint32(len(payload)))
	_, err := w.Write(append(buf, payload...))
	return err
}

// ReadFrame reads one length-prefixed message. It returns io.EOF, unwrapped,
// when the stream ends cleanly between messages — that is a client hanging up,
// not a failure.
func ReadFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, ErrTruncated
		}
		return nil, err
	}
	n := binary.LittleEndian.Uint32(header[:])
	if n > MaxFrameLen {
		return nil, fmt.Errorf("taskstore: frame header claims %d bytes, over the %d limit", n, MaxFrameLen)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, ErrTruncated
	}
	return payload, nil
}
