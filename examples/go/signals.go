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
	"math"
	"math/big"
	"slices"
)

// SignalCapture mirrors examples/cases/signals.yaml, which uses every scalar the
// schema allows as a list element. Go is the --ref language, so the bytes below
// are the layout the other eight workers have to reproduce.
//
// Binary layout follows wire.go: each list is a u32 element count followed by
// the elements back to back, each in its natural fixed-width little-endian form.
// The two variable-width element types keep their own length prefix per element
// (`list<string>` and `list<bytes>` are a count of length-prefixed values), and
// `int128`/`uint128` are 16 bytes two's complement, as in ledger.go.
type SignalCapture struct {
	CaptureID     uint64     `serify:"capture_id"`
	Flags         []bool     `serify:"flags"`
	RawFrame      []uint8    `serify:"raw_frame"`
	PortNumbers   []uint16   `serify:"port_numbers"`
	SampleCounts  []uint32   `serify:"sample_counts"`
	ByteTotals    []uint64   `serify:"byte_totals"`
	TrimOffsets   []int8     `serify:"trim_offsets"`
	DriftDeltas   []int16    `serify:"drift_deltas"`
	TemperaturesC []int32    `serify:"temperatures_c"`
	TimestampsNs  []int64    `serify:"timestamps_ns"`
	Counters      []*big.Int `serify:"counters"`
	Balances      []*big.Int `serify:"balances"`
	Gains         []float32  `serify:"gains"`
	Voltages      []float64  `serify:"voltages"`
	ChannelNames  []string   `serify:"channel_names"`
	Payloads      [][]byte   `serify:"payloads"`
	Checksum      [4]uint8   `serify:"checksum"`
	Window        [3]int16   `serify:"window"`
	DroppedFrames *uint32    `serify:"dropped_frames"`
	Mode          string     `serify:"mode"`
}

// signalModes is the declaration order of the `mode` enum in signals.yaml. An
// enum travels as its variant name; the ordinal below is this worker's own
// byte-layout choice, so the list must match the case file's order.
var signalModes = []string{"idle", "active", "fault", "calibrating"}

// appendCount writes a list's u32 element count. Every list carries one, even
// when empty — an empty list is a count of zero, not an absent field.
func appendCount[T any](buf []byte, v []T) []byte {
	return binary.LittleEndian.AppendUint32(buf, uint32(len(v)))
}

// readCount reads a list's u32 element count.
func readCount(b []byte) (int, []byte, error) {
	if len(b) < 4 {
		return 0, b, errTruncated
	}
	return int(binary.LittleEndian.Uint32(b[:4])), b[4:], nil
}

func (s *SignalCapture) MarshalBinary() ([]byte, error) {
	buf := binary.LittleEndian.AppendUint64(nil, s.CaptureID)

	buf = appendCount(buf, s.Flags)
	for _, v := range s.Flags {
		if v {
			buf = append(buf, 1)
		} else {
			buf = append(buf, 0)
		}
	}

	buf = appendCount(buf, s.RawFrame)
	buf = append(buf, s.RawFrame...)

	buf = appendCount(buf, s.PortNumbers)
	for _, v := range s.PortNumbers {
		buf = binary.LittleEndian.AppendUint16(buf, v)
	}

	buf = appendCount(buf, s.SampleCounts)
	for _, v := range s.SampleCounts {
		buf = binary.LittleEndian.AppendUint32(buf, v)
	}

	buf = appendCount(buf, s.ByteTotals)
	for _, v := range s.ByteTotals {
		buf = binary.LittleEndian.AppendUint64(buf, v)
	}

	buf = appendCount(buf, s.TrimOffsets)
	for _, v := range s.TrimOffsets {
		buf = append(buf, byte(v))
	}

	buf = appendCount(buf, s.DriftDeltas)
	for _, v := range s.DriftDeltas {
		buf = binary.LittleEndian.AppendUint16(buf, uint16(v))
	}

	buf = appendCount(buf, s.TemperaturesC)
	for _, v := range s.TemperaturesC {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(v))
	}

	buf = appendCount(buf, s.TimestampsNs)
	for _, v := range s.TimestampsNs {
		buf = binary.LittleEndian.AppendUint64(buf, uint64(v))
	}

	buf = appendCount(buf, s.Counters)
	for _, v := range s.Counters {
		buf = appendInt128(buf, v)
	}

	buf = appendCount(buf, s.Balances)
	for _, v := range s.Balances {
		buf = appendInt128(buf, v)
	}

	buf = appendCount(buf, s.Gains)
	for _, v := range s.Gains {
		buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(v))
	}

	buf = appendCount(buf, s.Voltages)
	for _, v := range s.Voltages {
		buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(v))
	}

	buf = appendCount(buf, s.ChannelNames)
	for _, v := range s.ChannelNames {
		buf = appendLenStr(buf, v)
	}

	buf = appendCount(buf, s.Payloads)
	for _, v := range s.Payloads {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(v)))
		buf = append(buf, v...)
	}

	// array<T,N> carries no count: N is fixed by the schema.
	buf = append(buf, s.Checksum[:]...)
	for _, v := range s.Window {
		buf = binary.LittleEndian.AppendUint16(buf, uint16(v))
	}

	// optional<uint32>: a presence flag, then the value if present.
	if s.DroppedFrames == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = binary.LittleEndian.AppendUint32(buf, *s.DroppedFrames)
	}

	// enum: a u8 ordinal, the variant's position in the case file.
	ord := slices.Index(signalModes, s.Mode)
	if ord < 0 {
		return nil, fmt.Errorf("unknown signal mode %q", s.Mode)
	}
	buf = append(buf, byte(ord))

	return buf, nil
}

//nolint:gocognit,funlen // one read block per element type
func (s *SignalCapture) UnmarshalBinary(b []byte) error {
	if len(b) < 8 {
		return errTruncated
	}
	s.CaptureID = binary.LittleEndian.Uint64(b)
	b = b[8:]

	n, b, err := readCount(b)
	if err != nil {
		return err
	}
	if len(b) < n {
		return errTruncated
	}
	s.Flags = make([]bool, n)
	for i := range n {
		s.Flags[i] = b[i] != 0
	}
	b = b[n:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n {
		return errTruncated
	}
	s.RawFrame = append([]uint8(nil), b[:n]...)
	b = b[n:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*2 {
		return errTruncated
	}
	s.PortNumbers = make([]uint16, n)
	for i := range n {
		s.PortNumbers[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	b = b[n*2:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*4 {
		return errTruncated
	}
	s.SampleCounts = make([]uint32, n)
	for i := range n {
		s.SampleCounts[i] = binary.LittleEndian.Uint32(b[i*4:])
	}
	b = b[n*4:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*8 {
		return errTruncated
	}
	s.ByteTotals = make([]uint64, n)
	for i := range n {
		s.ByteTotals[i] = binary.LittleEndian.Uint64(b[i*8:])
	}
	b = b[n*8:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n {
		return errTruncated
	}
	s.TrimOffsets = make([]int8, n)
	for i := range n {
		s.TrimOffsets[i] = int8(b[i])
	}
	b = b[n:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*2 {
		return errTruncated
	}
	s.DriftDeltas = make([]int16, n)
	for i := range n {
		s.DriftDeltas[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	b = b[n*2:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*4 {
		return errTruncated
	}
	s.TemperaturesC = make([]int32, n)
	for i := range n {
		s.TemperaturesC[i] = int32(binary.LittleEndian.Uint32(b[i*4:]))
	}
	b = b[n*4:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*8 {
		return errTruncated
	}
	s.TimestampsNs = make([]int64, n)
	for i := range n {
		s.TimestampsNs[i] = int64(binary.LittleEndian.Uint64(b[i*8:]))
	}
	b = b[n*8:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	s.Counters = make([]*big.Int, n)
	for i := range n {
		if s.Counters[i], b, err = readInt128(b); err != nil {
			return err
		}
	}
	s.normalizeCounters()

	if n, b, err = readCount(b); err != nil {
		return err
	}
	s.Balances = make([]*big.Int, n)
	for i := range n {
		if s.Balances[i], b, err = readInt128(b); err != nil {
			return err
		}
	}

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*4 {
		return errTruncated
	}
	s.Gains = make([]float32, n)
	for i := range n {
		s.Gains[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	b = b[n*4:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n*8 {
		return errTruncated
	}
	s.Voltages = make([]float64, n)
	for i := range n {
		s.Voltages[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	b = b[n*8:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	s.ChannelNames = make([]string, n)
	for i := range n {
		if s.ChannelNames[i], b, err = readLenStr(b); err != nil {
			return err
		}
	}

	if n, b, err = readCount(b); err != nil {
		return err
	}
	s.Payloads = make([][]byte, n)
	for i := range n {
		var ln int
		if ln, b, err = readCount(b); err != nil {
			return err
		}
		if len(b) < ln {
			return errTruncated
		}
		s.Payloads[i] = append([]byte(nil), b[:ln]...)
		b = b[ln:]
	}

	if len(b) < len(s.Checksum)+len(s.Window)*2 {
		return errTruncated
	}
	copy(s.Checksum[:], b[:len(s.Checksum)])
	b = b[len(s.Checksum):]
	for i := range s.Window {
		s.Window[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	b = b[len(s.Window)*2:]

	if len(b) < 1 {
		return errTruncated
	}
	s.DroppedFrames = nil
	if b[0] != 0 {
		if len(b) < 5 {
			return errTruncated
		}
		v := binary.LittleEndian.Uint32(b[1:])
		s.DroppedFrames = &v
		b = b[5:]
	} else {
		b = b[1:]
	}

	if len(b) < 1 {
		return errTruncated
	}
	if int(b[0]) >= len(signalModes) {
		return fmt.Errorf("unknown signal mode ordinal %d", b[0])
	}
	s.Mode = signalModes[b[0]]

	return nil
}

// The counters list is uint128 while balances is int128; both use the same 16
// two's-complement bytes, so a uint128 above 2^127 reads back as a negative
// *big.Int unless it is re-mapped. serify hands uint128 values in as unsigned,
// and expects them back the same way.
func (s *SignalCapture) normalizeCounters() {
	mod := new(big.Int).Lsh(big.NewInt(1), 128)
	for i, v := range s.Counters {
		if v != nil && v.Sign() < 0 {
			s.Counters[i] = new(big.Int).Add(v, mod)
		}
	}
}
