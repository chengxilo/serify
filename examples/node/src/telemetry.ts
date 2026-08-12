/**
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/**
 * `TelemetryFrame` mirrors examples/cases/telemetry.yaml — one reading from a
 * field device.
 *
 * This is the type that covers the corners the other examples do not: a
 * `uint128` address, two differently shaped fixed arrays, the suite's only
 * `optional<scalar>`, a `map<string,uint64>`, and float cases running through
 * NaN, ±Inf and negative zero. Only `binary` is declared, because NaN and Inf
 * have no JSON spelling.
 *
 * The declared types carry the one distinction TypeScript can make here: a JS
 * number is an IEEE-754 double, so anything wider than 53 bits of integer is a
 * `bigint` — the u64 device id, the u128 address, and the u64 map values. The
 * narrower widths are all `number`, and which of them a field is lives in the
 * byte layout below rather than in the type.
 *
 * Go is the --ref language and owns that layout; see examples/go/wire.go.
 */

import { Serify } from '@chengxilo/serify';

import { lenPrefixed, lenPrefixedStr } from './wire';

const MASK_64 = (1n << 64n) - 1n;
const MASK_128 = (1n << 128n) - 1n;

/** uint128: 16 bytes little-endian, written as two halves — Buffer has no 128-bit accessor. */
function writeU128LE(v: bigint): Buffer {
  const u = v & MASK_128;
  const b = Buffer.alloc(16);
  b.writeBigUInt64LE(u & MASK_64, 0);
  b.writeBigUInt64LE(u >> 64n, 8);
  return b;
}

function readU128LE(data: Buffer, off: number): bigint {
  return (data.readBigUInt64LE(off + 8) << 64n) | data.readBigUInt64LE(off);
}

@Serify.Model()
export class TelemetryFrame {
  @Serify.field({ rename: 'device_id' }) deviceId: bigint = 0n;
  @Serify.field() ipv6: bigint = 0n;
  @Serify.field({ rename: 'local_ip' }) localIp: number[] = [];
  @Serify.field() firmware: string = '';
  @Serify.field({ rename: 'boot_count' }) bootCount: number = 0;
  @Serify.field({ rename: 'rssi_dbm' }) rssiDbm: number = 0;
  @Serify.field({ rename: 'temperature_dc' }) temperatureDc: number = 0;
  @Serify.field({ rename: 'clock_drift_ms' }) clockDriftMs: number = 0;
  @Serify.field({ rename: 'battery_volts' }) batteryVolts: number = 0;
  @Serify.field() latitude: number = 0;
  @Serify.field() longitude: number = 0;
  @Serify.field({ rename: 'humidity_pct' }) humidityPct: number | null = null;
  @Serify.field({ rename: 'accel_mg' }) accelMg: number[] = [];
  @Serify.field({ rename: 'visible_cells' }) visibleCells: number[] = [];
  @Serify.field({ rename: 'packet_counts' }) packetCounts: Map<string, bigint> = new Map();
  @Serify.field({ rename: 'gps_fix' }) gpsFix: boolean = false;
  @Serify.field() signature: Buffer = Buffer.alloc(0);

  marshal(): Buffer {
    const parts: Buffer[] = [];

    const id = Buffer.alloc(8);
    id.writeBigUInt64LE(this.deviceId, 0);
    parts.push(id, writeU128LE(this.ipv6));

    // array<T,N> carries no count: N is fixed by the schema.
    parts.push(Buffer.from(this.localIp));

    parts.push(lenPrefixedStr(this.firmware));

    const scalars = Buffer.alloc(2 + 1 + 2 + 4 + 4 + 8 + 8);
    scalars.writeUInt16LE(this.bootCount, 0);
    scalars.writeInt8(this.rssiDbm, 2);
    scalars.writeInt16LE(this.temperatureDc, 3);
    scalars.writeInt32LE(this.clockDriftMs, 5);
    scalars.writeFloatLE(this.batteryVolts, 9);
    scalars.writeDoubleLE(this.latitude, 13);
    scalars.writeDoubleLE(this.longitude, 21);
    parts.push(scalars);

    // optional<float32>: a presence flag, then the value if present.
    if (this.humidityPct === null) {
      parts.push(Buffer.from([0]));
    } else {
      const h = Buffer.alloc(5);
      h.writeUInt8(1, 0);
      h.writeFloatLE(this.humidityPct, 1);
      parts.push(h);
    }

    const accel = Buffer.alloc(this.accelMg.length * 2);
    this.accelMg.forEach((v, i) => accel.writeInt16LE(v, i * 2));
    parts.push(accel);

    const cells = Buffer.alloc(4 + this.visibleCells.length * 4);
    cells.writeUInt32LE(this.visibleCells.length, 0);
    this.visibleCells.forEach((v, i) => cells.writeUInt32LE(v, 4 + i * 4));
    parts.push(cells);

    // Entry order is the Map's own — deliberately not sorted. A map is
    // unordered, so telemetry declares `oracle: semantic` and the decoded value
    // is what gets compared. See docs/protocol.md.
    const count = Buffer.alloc(4);
    count.writeUInt32LE(this.packetCounts.size, 0);
    parts.push(count);
    for (const [k, v] of this.packetCounts) {
      const val = Buffer.alloc(8);
      val.writeBigUInt64LE(v, 0);
      parts.push(lenPrefixedStr(k), val);
    }

    parts.push(Buffer.from([this.gpsFix ? 1 : 0]), lenPrefixed(this.signature));

    return Buffer.concat(parts);
  }

  static unmarshal(data: Buffer): TelemetryFrame {
    const t = new TelemetryFrame();
    let pos = 0;

    t.deviceId = data.readBigUInt64LE(pos);
    pos += 8;
    t.ipv6 = readU128LE(data, pos);
    pos += 16;

    t.localIp = Array.from(data.subarray(pos, pos + 4));
    pos += 4;

    const nameLen = data.readUInt32LE(pos);
    pos += 4;
    t.firmware = data.subarray(pos, pos + nameLen).toString('utf8');
    pos += nameLen;

    t.bootCount = data.readUInt16LE(pos);
    t.rssiDbm = data.readInt8(pos + 2);
    t.temperatureDc = data.readInt16LE(pos + 3);
    t.clockDriftMs = data.readInt32LE(pos + 5);
    t.batteryVolts = data.readFloatLE(pos + 9);
    t.latitude = data.readDoubleLE(pos + 13);
    t.longitude = data.readDoubleLE(pos + 21);
    pos += 29;

    if (data.readUInt8(pos) === 0) {
      t.humidityPct = null;
      pos += 1;
    } else {
      t.humidityPct = data.readFloatLE(pos + 1);
      pos += 5;
    }

    t.accelMg = [0, 1, 2].map((i) => data.readInt16LE(pos + i * 2));
    pos += 6;

    const cellCount = data.readUInt32LE(pos);
    pos += 4;
    t.visibleCells = [];
    for (let i = 0; i < cellCount; i++) t.visibleCells.push(data.readUInt32LE(pos + i * 4));
    pos += cellCount * 4;

    const entries = data.readUInt32LE(pos);
    pos += 4;
    t.packetCounts = new Map();
    for (let i = 0; i < entries; i++) {
      const klen = data.readUInt32LE(pos);
      pos += 4;
      const k = data.subarray(pos, pos + klen).toString('utf8');
      pos += klen;
      t.packetCounts.set(k, data.readBigUInt64LE(pos));
      pos += 8;
    }

    t.gpsFix = data.readUInt8(pos) !== 0;
    pos += 1;

    const sigLen = data.readUInt32LE(pos);
    pos += 4;
    t.signature = Buffer.from(data.subarray(pos, pos + sigLen));

    return t;
  }
}
