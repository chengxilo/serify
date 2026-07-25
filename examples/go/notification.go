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
	"encoding/binary"
	"fmt"
)

// Channel is the `sum` from examples/cases/notification.yaml. Go has no sum
// type, so the idiomatic encoding is a sealed interface: the unexported marker
// method means only the four types below can be a Channel — with a bare `any`,
// `n.Channel = 42` would compile.
//
// Sealing is all the marker does; the schema's tag names live in exactly one
// place, the Channel converter in main.go, and nothing in this file mentions them.
//
// What sealing does *not* buy is exhaustiveness: add a fifth variant and Go will
// not point at the switches below or at the converter. Rust's `match` would.
// That gap is real, and this example does not paper over it.
type Channel interface{ isChannel() }

type (
	Silent  struct{} // arity 0 — a unit variant
	SMS     string   // arity 1 — a scalar payload
	Push    uint64   // arity 1 — a payload that exceeds 2^53
	Invoice Money    // arity N — a struct payload
)

func (Silent) isChannel()  {}
func (SMS) isChannel()     {}
func (Push) isChannel()    {}
func (Invoice) isChannel() {}

// NotificationRecord mirrors examples/cases/notification.yaml.
type NotificationRecord struct {
	NotificationID uint32  `serify:"notification_id"`
	Channel        Channel `serify:"channel"`
	Urgent         bool    `serify:"urgent"`
}

func (n *NotificationRecord) MarshalBinary() ([]byte, error) {
	if n.Channel == nil {
		return nil, fmt.Errorf("notification %d has no channel", n.NotificationID)
	}
	buf := binary.LittleEndian.AppendUint32(nil, n.NotificationID)

	// The tag ordinal is the variant's position in the case file's sum, which
	// is the declaration order of the four types above.
	switch ch := n.Channel.(type) {
	case Silent:
		buf = append(buf, 0) // a unit variant is nothing but its tag
	case SMS:
		buf = append(buf, 1)
		buf = appendLenStr(buf, string(ch))
	case Push:
		buf = append(buf, 2)
		buf = binary.LittleEndian.AppendUint64(buf, uint64(ch))
	case Invoice:
		buf = append(buf, 3)
		buf = appendLenStr(buf, ch.Currency)
		buf = binary.LittleEndian.AppendUint64(buf, uint64(ch.AmountMinor))
	default:
		return nil, fmt.Errorf("notification %d: unhandled channel %T", n.NotificationID, ch)
	}

	if n.Urgent {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	return buf, nil
}

func (n *NotificationRecord) UnmarshalBinary(b []byte) error {
	if len(b) < 5 {
		return errTruncated
	}
	n.NotificationID = binary.LittleEndian.Uint32(b)
	ord := b[4]
	b = b[5:]

	switch ord {
	case 0:
		n.Channel = Silent{}
	case 1:
		s, rest, err := readLenStr(b)
		if err != nil {
			return err
		}
		n.Channel, b = SMS(s), rest
	case 2:
		if len(b) < 8 {
			return errTruncated
		}
		n.Channel, b = Push(binary.LittleEndian.Uint64(b)), b[8:]
	case 3:
		currency, rest, err := readLenStr(b)
		if err != nil {
			return err
		}
		if len(rest) < 8 {
			return errTruncated
		}
		n.Channel = Invoice{Currency: currency, AmountMinor: int64(binary.LittleEndian.Uint64(rest))}
		b = rest[8:]
	default:
		return fmt.Errorf("unknown channel ordinal %d", ord)
	}

	if len(b) < 1 {
		return errTruncated
	}
	n.Urgent = b[0] != 0
	return nil
}
