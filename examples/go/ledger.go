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
	"math/big"
)

// LedgerEntry mirrors examples/cases/ledger.yaml. Go has no native 128-bit
// integer, so the two int128 fields are *big.Int — the only representation that
// holds the full range (the suite's boundary case carries ±2^127).
//
// Binary layout follows the conventions documented in wire.go, with one addition:
//
//	int128   16 bytes, little-endian, two's complement
type LedgerEntry struct {
	EntryID         uint64   `serify:"entry_id"`
	BlockNumber     uint64   `serify:"block_number"`
	BlockTime       int64    `serify:"block_time"`
	TxHash          []byte   `serify:"tx_hash"`
	Account         string   `serify:"account"`
	Asset           string   `serify:"asset"`
	AmountBaseUnits *big.Int `serify:"amount_base_units"`
	BalanceAfter    *big.Int `serify:"balance_after"`
	Confirmed       bool     `serify:"confirmed"`
	Memo            *string  `serify:"memo"`
}

const int128ByteLen = 16

// appendInt128 writes n as 16 little-endian bytes in two's complement.
func appendInt128(buf []byte, n *big.Int) []byte {
	if n == nil {
		n = big.NewInt(0)
	}
	// Map the value into the unsigned 2^128 residue class, which is exactly what
	// two's complement is: -1 becomes 0xffff...ff.
	mod := new(big.Int).Lsh(big.NewInt(1), 128)
	u := new(big.Int).Mod(n, mod)

	var out [int128ByteLen]byte
	be := u.Bytes() // big-endian, no leading zeros
	for i, b := range be {
		out[len(be)-1-i] = b
	}
	return append(buf, out[:]...)
}

// readInt128 reads 16 little-endian two's-complement bytes back into a *big.Int.
func readInt128(b []byte) (*big.Int, []byte, error) {
	if len(b) < int128ByteLen {
		return nil, b, errTruncated
	}
	be := make([]byte, int128ByteLen)
	for i := range int128ByteLen {
		be[int128ByteLen-1-i] = b[i]
	}
	n := new(big.Int).SetBytes(be)

	// Re-apply the sign: anything with the top bit set is negative.
	if n.Bit(127) == 1 {
		n.Sub(n, new(big.Int).Lsh(big.NewInt(1), 128))
	}
	return n, b[int128ByteLen:], nil
}

func (l *LedgerEntry) MarshalBinary() ([]byte, error) {
	var buf []byte

	buf = binary.LittleEndian.AppendUint64(buf, l.EntryID)
	buf = binary.LittleEndian.AppendUint64(buf, l.BlockNumber)
	buf = binary.LittleEndian.AppendUint64(buf, uint64(l.BlockTime))

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(l.TxHash)))
	buf = append(buf, l.TxHash...)

	buf = appendLenStr(buf, l.Account)
	buf = appendLenStr(buf, l.Asset)

	buf = appendInt128(buf, l.AmountBaseUnits)
	buf = appendInt128(buf, l.BalanceAfter)

	if l.Confirmed {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}

	if l.Memo == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = appendLenStr(buf, *l.Memo)
	}

	return buf, nil
}

func (l *LedgerEntry) UnmarshalBinary(data []byte) error {
	b := data
	var err error

	if len(b) < 24 {
		return errTruncated
	}
	l.EntryID = binary.LittleEndian.Uint64(b[:8])
	l.BlockNumber = binary.LittleEndian.Uint64(b[8:16])
	l.BlockTime = int64(binary.LittleEndian.Uint64(b[16:24]))
	b = b[24:]

	if len(b) < 4 {
		return errTruncated
	}
	hashLen := int(binary.LittleEndian.Uint32(b[:4]))
	b = b[4:]
	if len(b) < hashLen {
		return errTruncated
	}
	l.TxHash = make([]byte, hashLen)
	copy(l.TxHash, b[:hashLen])
	b = b[hashLen:]

	if l.Account, b, err = readLenStr(b); err != nil {
		return err
	}
	if l.Asset, b, err = readLenStr(b); err != nil {
		return err
	}

	if l.AmountBaseUnits, b, err = readInt128(b); err != nil {
		return err
	}
	if l.BalanceAfter, b, err = readInt128(b); err != nil {
		return err
	}

	if len(b) < 2 {
		return errTruncated
	}
	l.Confirmed = b[0] != 0
	hasMemo := b[1] != 0
	b = b[2:]

	l.Memo = nil
	if hasMemo {
		var s string
		if s, _, err = readLenStr(b); err != nil {
			return err
		}
		l.Memo = &s
	}

	return nil
}
