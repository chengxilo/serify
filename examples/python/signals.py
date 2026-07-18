# Copyright 2026 Chengxi Luo
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""`SignalCapture` mirrors examples/cases/signals.yaml, which uses every scalar
the schema allows as a list element.

Python's annotations cannot distinguish the integer widths — `list[int]` covers
uint8 through int128 — so `@serify_model` binds them all as plain lists and the
schema decides the wire width. The byte layout below is where the widths
actually matter, and that is the part a conformance worker exists to exercise.

Go is the --ref language and owns that layout; see examples/go/wire.go.
"""

import struct
from dataclasses import dataclass

from serify import serify_model

_U128_MASK = (1 << 128) - 1
# Declaration order of the `mode` enum; the index is the wire ordinal.
_SIGNAL_MODES = ["idle", "active", "fault", "calibrating"]


def _pack_list(fmt: str, values) -> bytes:
    """u32 element count, then each value in `fmt` (little-endian)."""
    return struct.pack('<I', len(values)) + b''.join(struct.pack(fmt, v) for v in values)


def _unpack_list(fmt: str, size: int, data: bytes, pos: int):
    count = struct.unpack_from('<I', data, pos)[0]
    pos += 4
    out = [struct.unpack_from(fmt, data, pos + i * size)[0] for i in range(count)]
    return out, pos + count * size


def _pack_i128(v: int) -> bytes:
    return (v & _U128_MASK).to_bytes(16, 'little')


def _unpack_i128(data: bytes, pos: int, signed: bool) -> int:
    return int.from_bytes(data[pos:pos + 16], 'little', signed=signed)


@serify_model
@dataclass
class SignalCapture:
    capture_id: int
    flags: "list[bool]"
    raw_frame: "list[int]"
    port_numbers: "list[int]"
    sample_counts: "list[int]"
    byte_totals: "list[int]"
    trim_offsets: "list[int]"
    drift_deltas: "list[int]"
    temperatures_c: "list[int]"
    timestamps_ns: "list[int]"
    counters: "list[int]"
    balances: "list[int]"
    gains: "list[float]"
    voltages: "list[float]"
    channel_names: "list[str]"
    payloads: "list[bytes]"
    checksum: "list[int]"
    window: "list[int]"
    dropped_frames: "int | None"
    mode: str

    def marshal(self) -> bytes:
        buf = bytearray()
        buf += struct.pack('<Q', self.capture_id)

        buf += struct.pack('<I', len(self.flags))
        buf += bytes(1 if f else 0 for f in self.flags)

        buf += struct.pack('<I', len(self.raw_frame))
        buf += bytes(self.raw_frame)

        buf += _pack_list('<H', self.port_numbers)
        buf += _pack_list('<I', self.sample_counts)
        buf += _pack_list('<Q', self.byte_totals)
        buf += _pack_list('<b', self.trim_offsets)
        buf += _pack_list('<h', self.drift_deltas)
        buf += _pack_list('<i', self.temperatures_c)
        buf += _pack_list('<q', self.timestamps_ns)

        buf += struct.pack('<I', len(self.counters))
        for v in self.counters:
            buf += _pack_i128(v)
        buf += struct.pack('<I', len(self.balances))
        for v in self.balances:
            buf += _pack_i128(v)

        buf += _pack_list('<f', self.gains)
        buf += _pack_list('<d', self.voltages)

        buf += struct.pack('<I', len(self.channel_names))
        for s in self.channel_names:
            raw = s.encode('utf-8')
            buf += struct.pack('<I', len(raw)) + raw

        buf += struct.pack('<I', len(self.payloads))
        for p in self.payloads:
            buf += struct.pack('<I', len(p)) + p

        # array<T,N> carries no count: N is fixed by the schema.
        buf += bytes(self.checksum)
        for v in self.window:
            buf += struct.pack('<h', v)

        # optional<uint32>: a presence flag, then the value if present.
        if self.dropped_frames is None:
            buf += struct.pack('<B', 0)
        else:
            buf += struct.pack('<BI', 1, self.dropped_frames)

        # enum: a u8 ordinal, the variant's position in the case file.
        buf += struct.pack('<B', _SIGNAL_MODES.index(self.mode))

        return bytes(buf)

    @classmethod
    def unmarshal(cls, data: bytes) -> "SignalCapture":
        pos = 0
        capture_id = struct.unpack_from('<Q', data, pos)[0]
        pos += 8

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        flags = [data[pos + i] != 0 for i in range(n)]
        pos += n

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        raw_frame = list(data[pos:pos + n])
        pos += n

        port_numbers, pos = _unpack_list('<H', 2, data, pos)
        sample_counts, pos = _unpack_list('<I', 4, data, pos)
        byte_totals, pos = _unpack_list('<Q', 8, data, pos)
        trim_offsets, pos = _unpack_list('<b', 1, data, pos)
        drift_deltas, pos = _unpack_list('<h', 2, data, pos)
        temperatures_c, pos = _unpack_list('<i', 4, data, pos)
        timestamps_ns, pos = _unpack_list('<q', 8, data, pos)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        counters = []
        for _ in range(n):
            counters.append(_unpack_i128(data, pos, signed=False))
            pos += 16

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        balances = []
        for _ in range(n):
            balances.append(_unpack_i128(data, pos, signed=True))
            pos += 16

        gains, pos = _unpack_list('<f', 4, data, pos)
        voltages, pos = _unpack_list('<d', 8, data, pos)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        channel_names = []
        for _ in range(n):
            ln = struct.unpack_from('<I', data, pos)[0]
            pos += 4
            channel_names.append(data[pos:pos + ln].decode('utf-8'))
            pos += ln

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        payloads = []
        for _ in range(n):
            ln = struct.unpack_from('<I', data, pos)[0]
            pos += 4
            payloads.append(bytes(data[pos:pos + ln]))
            pos += ln

        checksum = list(data[pos:pos + 4])
        pos += 4
        window = [struct.unpack_from('<h', data, pos + i * 2)[0] for i in range(3)]
        pos += 6

        if data[pos] == 0:
            dropped_frames = None
            pos += 1
        else:
            dropped_frames = struct.unpack_from('<I', data, pos + 1)[0]
            pos += 5

        mode = _SIGNAL_MODES[data[pos]]
        pos += 1

        return cls(
            capture_id=capture_id, flags=flags, raw_frame=raw_frame,
            port_numbers=port_numbers, sample_counts=sample_counts,
            byte_totals=byte_totals, trim_offsets=trim_offsets,
            drift_deltas=drift_deltas, temperatures_c=temperatures_c,
            timestamps_ns=timestamps_ns, counters=counters, balances=balances,
            gains=gains, voltages=voltages, channel_names=channel_names,
            payloads=payloads, checksum=checksum, window=window,
            dropped_frames=dropped_frames, mode=mode,
        )
