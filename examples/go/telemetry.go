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
	"math"
	"math/big"
)

// TelemetryFrame mirrors examples/cases/telemetry.yaml — one reading from a
// field device.
//
// This type is what exercises the corners the other examples do not: a
// `uint128` address, two differently shaped fixed arrays, an `optional<float32>`
// (the only `optional<scalar>` in the suite), a `map<string,uint64>`, and float
// cases covering NaN, ±Inf and negative zero. Its `binary` format is the only
// one declared, because NaN and Inf have no JSON representation.
//
// Binary layout follows the conventions in wire.go, with two additions:
//
//	uint128      16 bytes, little-endian
//	array<T,N>   exactly N elements, no count prefix (N is fixed by the schema)
type TelemetryFrame struct {
	DeviceID      uint64            `serify:"device_id"`
	IPv6          *big.Int          `serify:"ipv6"`
	LocalIP       [4]uint8          `serify:"local_ip"`
	Firmware      string            `serify:"firmware"`
	BootCount     uint16            `serify:"boot_count"`
	RSSIdBm       int8              `serify:"rssi_dbm"`
	TemperatureDC int16             `serify:"temperature_dc"`
	ClockDriftMS  int32             `serify:"clock_drift_ms"`
	BatteryVolts  float32           `serify:"battery_volts"`
	Latitude      float64           `serify:"latitude"`
	Longitude     float64           `serify:"longitude"`
	HumidityPct   *float32          `serify:"humidity_pct"`
	AccelMg       [3]int16          `serify:"accel_mg"`
	VisibleCells  []uint32          `serify:"visible_cells"`
	PacketCounts  map[string]uint64 `serify:"packet_counts"`
	GPSFix        bool              `serify:"gps_fix"`
	Signature     []byte            `serify:"signature"`
}

func (t *TelemetryFrame) MarshalBinary() ([]byte, error) {
	buf := binary.LittleEndian.AppendUint64(nil, t.DeviceID)

	// uint128 is unsigned, so the same 16 two's-complement bytes ledger.go uses
	// for int128 serve here without a sign to re-apply on the way back.
	buf = appendInt128(buf, t.IPv6)

	buf = append(buf, t.LocalIP[:]...)
	buf = appendLenStr(buf, t.Firmware)
	buf = binary.LittleEndian.AppendUint16(buf, t.BootCount)
	buf = append(buf, byte(t.RSSIdBm))
	buf = binary.LittleEndian.AppendUint16(buf, uint16(t.TemperatureDC))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(t.ClockDriftMS))
	buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(t.BatteryVolts))
	buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(t.Latitude))
	buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(t.Longitude))

	// optional<float32>: a presence flag, then the value if present.
	if t.HumidityPct == nil {
		buf = append(buf, 0)
	} else {
		buf = append(buf, 1)
		buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(*t.HumidityPct))
	}

	for _, v := range t.AccelMg {
		buf = binary.LittleEndian.AppendUint16(buf, uint16(v))
	}

	buf = appendCount(buf, t.VisibleCells)
	for _, v := range t.VisibleCells {
		buf = binary.LittleEndian.AppendUint32(buf, v)
	}

	// Map keys are sorted so the bytes are deterministic; see wire.go.
	keys := sortedKeys(t.PacketCounts)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(keys)))
	for _, k := range keys {
		buf = appendLenStr(buf, k)
		buf = binary.LittleEndian.AppendUint64(buf, t.PacketCounts[k])
	}

	if t.GPSFix {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(t.Signature)))
	buf = append(buf, t.Signature...)

	return buf, nil
}

//nolint:gocognit,funlen // one read block per field
func (t *TelemetryFrame) UnmarshalBinary(b []byte) error {
	if len(b) < 8 {
		return errTruncated
	}
	t.DeviceID = binary.LittleEndian.Uint64(b)
	b = b[8:]

	var err error
	if t.IPv6, b, err = readInt128(b); err != nil {
		return err
	}
	// readInt128 re-applies a sign; uint128 has none, so map a negative back
	// into the unsigned residue class it came from.
	if t.IPv6.Sign() < 0 {
		t.IPv6 = new(big.Int).Add(t.IPv6, new(big.Int).Lsh(big.NewInt(1), 128))
	}

	if len(b) < len(t.LocalIP) {
		return errTruncated
	}
	copy(t.LocalIP[:], b[:len(t.LocalIP)])
	b = b[len(t.LocalIP):]

	if t.Firmware, b, err = readLenStr(b); err != nil {
		return err
	}

	if len(b) < 2+1+2+4+4+8+8+1 {
		return errTruncated
	}
	t.BootCount = binary.LittleEndian.Uint16(b)
	t.RSSIdBm = int8(b[2])
	t.TemperatureDC = int16(binary.LittleEndian.Uint16(b[3:]))
	t.ClockDriftMS = int32(binary.LittleEndian.Uint32(b[5:]))
	t.BatteryVolts = math.Float32frombits(binary.LittleEndian.Uint32(b[9:]))
	t.Latitude = math.Float64frombits(binary.LittleEndian.Uint64(b[13:]))
	t.Longitude = math.Float64frombits(binary.LittleEndian.Uint64(b[21:]))
	present := b[29]
	b = b[30:]

	t.HumidityPct = nil
	if present != 0 {
		if len(b) < 4 {
			return errTruncated
		}
		v := math.Float32frombits(binary.LittleEndian.Uint32(b))
		t.HumidityPct = &v
		b = b[4:]
	}

	if len(b) < len(t.AccelMg)*2 {
		return errTruncated
	}
	for i := range t.AccelMg {
		t.AccelMg[i] = int16(binary.LittleEndian.Uint16(b[i*2:]))
	}
	b = b[len(t.AccelMg)*2:]

	n, b, err := readCount(b)
	if err != nil {
		return err
	}
	if len(b) < n*4 {
		return errTruncated
	}
	t.VisibleCells = make([]uint32, n)
	for i := range n {
		t.VisibleCells[i] = binary.LittleEndian.Uint32(b[i*4:])
	}
	b = b[n*4:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	t.PacketCounts = make(map[string]uint64, n)
	for range n {
		var k string
		if k, b, err = readLenStr(b); err != nil {
			return err
		}
		if len(b) < 8 {
			return errTruncated
		}
		t.PacketCounts[k] = binary.LittleEndian.Uint64(b)
		b = b[8:]
	}

	if len(b) < 1 {
		return errTruncated
	}
	t.GPSFix = b[0] != 0
	b = b[1:]

	if n, b, err = readCount(b); err != nil {
		return err
	}
	if len(b) < n {
		return errTruncated
	}
	t.Signature = append([]byte(nil), b[:n]...)

	return nil
}
