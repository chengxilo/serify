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

"""`TelemetryFrame` mirrors examples/cases/telemetry.yaml — one reading from a
field device.

This is the type that covers the corners the other examples do not: a `uint128`
address, two differently shaped fixed arrays, the suite's only
`optional<scalar>`, a `map<string,uint64>`, and float cases running through NaN,
±Inf and negative zero. Only `binary` is declared, because NaN and Inf have no
JSON spelling.

Two Python-specific notes. `int` covers every integer width the schema has, so
the annotations cannot say which one a field is — the byte layout below is where
the width lives, which is the part a conformance worker exists to exercise. And
a Python float is always a double: packing `<f` narrows to float32 on the way
out and unpacking widens back, so a float32 field round-trips through the value
float32 can actually hold, which is what the reference worker also stores.

Go is the --ref language and owns the layout; see examples/go/wire.go.
"""

import struct
from dataclasses import dataclass

from serify import serify_model


def _pack_str(s: str) -> bytes:
    """u32 byte length, then the UTF-8 bytes."""
    raw = s.encode('utf-8')
    return struct.pack('<I', len(raw)) + raw


def _unpack_str(data: bytes, pos: int):
    n = struct.unpack_from('<I', data, pos)[0]
    pos += 4
    return data[pos:pos + n].decode('utf-8'), pos + n


@serify_model
@dataclass
class TelemetryFrame:
    device_id: int
    ipv6: int
    local_ip: "list[int]"
    firmware: str
    boot_count: int
    rssi_dbm: int
    temperature_dc: int
    clock_drift_ms: int
    battery_volts: float
    latitude: float
    longitude: float
    humidity_pct: "float | None"
    accel_mg: "list[int]"
    visible_cells: "list[int]"
    packet_counts: "dict[str, int]"
    gps_fix: bool
    signature: bytes

    def marshal(self) -> bytes:
        buf = bytearray()
        buf += struct.pack('<Q', self.device_id)

        # uint128: the same 16 little-endian bytes int128 uses, with no sign to
        # re-apply on the way back.
        buf += self.ipv6.to_bytes(16, 'little')

        # array<T,N> carries no count: N is fixed by the schema.
        buf += bytes(self.local_ip)

        buf += _pack_str(self.firmware)
        buf += struct.pack('<H', self.boot_count)
        buf += struct.pack('<b', self.rssi_dbm)
        buf += struct.pack('<h', self.temperature_dc)
        buf += struct.pack('<i', self.clock_drift_ms)
        buf += struct.pack('<f', self.battery_volts)
        buf += struct.pack('<d', self.latitude)
        buf += struct.pack('<d', self.longitude)

        # optional<float32>: a presence flag, then the value if present.
        if self.humidity_pct is None:
            buf += struct.pack('<B', 0)
        else:
            buf += struct.pack('<Bf', 1, self.humidity_pct)

        for v in self.accel_mg:
            buf += struct.pack('<h', v)

        buf += struct.pack('<I', len(self.visible_cells))
        for v in self.visible_cells:
            buf += struct.pack('<I', v)

        # Entry order is the dict's own — deliberately not sorted. A map is
        # unordered, so telemetry declares `oracle: semantic` and the decoded
        # value is what gets compared. See docs/protocol.md.
        buf += struct.pack('<I', len(self.packet_counts))
        for k, v in self.packet_counts.items():
            buf += _pack_str(k)
            buf += struct.pack('<Q', v)

        buf += struct.pack('<B', 1 if self.gps_fix else 0)
        buf += struct.pack('<I', len(self.signature)) + self.signature

        return bytes(buf)

    @classmethod
    def unmarshal(cls, data: bytes) -> "TelemetryFrame":
        pos = 0
        device_id = struct.unpack_from('<Q', data, pos)[0]
        pos += 8

        ipv6 = int.from_bytes(data[pos:pos + 16], 'little', signed=False)
        pos += 16

        local_ip = list(data[pos:pos + 4])
        pos += 4

        firmware, pos = _unpack_str(data, pos)

        # One unpack for the run of fixed-width scalars. The '<' prefix selects
        # standard sizes with no alignment padding, which is what the wire has.
        (boot_count, rssi_dbm, temperature_dc, clock_drift_ms,
         battery_volts, latitude, longitude) = struct.unpack_from('<Hbhifdd', data, pos)
        pos += 2 + 1 + 2 + 4 + 4 + 8 + 8

        if data[pos] == 0:
            humidity_pct = None
            pos += 1
        else:
            humidity_pct = struct.unpack_from('<f', data, pos + 1)[0]
            pos += 5

        accel_mg = [struct.unpack_from('<h', data, pos + i * 2)[0] for i in range(3)]
        pos += 6

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        visible_cells = [struct.unpack_from('<I', data, pos + i * 4)[0] for i in range(n)]
        pos += n * 4

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        packet_counts = {}
        for _ in range(n):
            k, pos = _unpack_str(data, pos)
            packet_counts[k] = struct.unpack_from('<Q', data, pos)[0]
            pos += 8

        gps_fix = data[pos] != 0
        pos += 1

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        signature = bytes(data[pos:pos + n])

        return cls(
            device_id=device_id, ipv6=ipv6, local_ip=local_ip, firmware=firmware,
            boot_count=boot_count, rssi_dbm=rssi_dbm, temperature_dc=temperature_dc,
            clock_drift_ms=clock_drift_ms, battery_volts=battery_volts,
            latitude=latitude, longitude=longitude, humidity_pct=humidity_pct,
            accel_mg=accel_mg, visible_cells=visible_cells,
            packet_counts=packet_counts, gps_fix=gps_fix, signature=signature,
        )
