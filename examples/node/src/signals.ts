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
 * `SignalCapture` mirrors examples/cases/signals.yaml, which uses every scalar
 * the schema allows as a list element.
 *
 * TypeScript element types are erased at runtime, so the binding cannot tell a
 * `number[]` of uint16 from one of int8 — the schema does, and the FieldMap just
 * holds the list. What the declared types do carry is the 64/128-bit split: JS
 * numbers are IEEE-754 doubles, so those lists are `bigint[]`.
 *
 * Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */

import { Serify } from '@chengxilo/serify';

// Declaration order of the `mode` enum; the index is the wire ordinal.
const SIGNAL_MODES = ['idle', 'active', 'fault', 'calibrating'];
const MASK_64 = (1n << 64n) - 1n;
const MASK_128 = (1n << 128n) - 1n;

/** u32 element count, then each element written by `write`. */
function packList<T>(items: T[], size: number, write: (b: Buffer, v: T, off: number) => void): Buffer {
  const b = Buffer.alloc(4 + items.length * size);
  b.writeUInt32LE(items.length, 0);
  items.forEach((v, i) => write(b, v, 4 + i * size));
  return b;
}

/** Inverse of packList for fixed-width elements. */
function unpackList<T>(data: Buffer, pos: number, size: number, read: (b: Buffer, off: number) => T): [T[], number] {
  const count = data.readUInt32LE(pos);
  pos += 4;
  const out: T[] = [];
  for (let i = 0; i < count; i++) out.push(read(data, pos + i * size));
  return [out, pos + count * size];
}

// int128/uint128: 16 bytes little-endian, two's complement, written as two
// 64-bit halves since Buffer has no 128-bit accessor.
function writeInt128LE(v: bigint): Buffer {
  const u = v & MASK_128;
  const b = Buffer.alloc(16);
  b.writeBigUInt64LE(u & MASK_64, 0);
  b.writeBigUInt64LE(u >> 64n, 8);
  return b;
}

function readInt128LE(data: Buffer, off: number, signed: boolean): bigint {
  const lo = data.readBigUInt64LE(off);
  const hi = data.readBigUInt64LE(off + 8);
  const u = (hi << 64n) | lo;
  return signed && u >= 1n << 127n ? u - (1n << 128n) : u;
}

@Serify.Model()
export class SignalCapture {
  @Serify.field({ rename: 'capture_id' }) captureId: bigint = 0n;
  @Serify.field() flags: boolean[] = [];
  @Serify.field({ rename: 'raw_frame' }) rawFrame: number[] = [];
  @Serify.field({ rename: 'port_numbers' }) portNumbers: number[] = [];
  @Serify.field({ rename: 'sample_counts' }) sampleCounts: number[] = [];
  @Serify.field({ rename: 'byte_totals' }) byteTotals: bigint[] = [];
  @Serify.field({ rename: 'trim_offsets' }) trimOffsets: number[] = [];
  @Serify.field({ rename: 'drift_deltas' }) driftDeltas: number[] = [];
  @Serify.field({ rename: 'temperatures_c' }) temperaturesC: number[] = [];
  @Serify.field({ rename: 'timestamps_ns' }) timestampsNs: bigint[] = [];
  @Serify.field() counters: bigint[] = [];
  @Serify.field() balances: bigint[] = [];
  @Serify.field() gains: number[] = [];
  @Serify.field() voltages: number[] = [];
  @Serify.field({ rename: 'channel_names' }) channelNames: string[] = [];
  @Serify.field() payloads: Buffer[] = [];
  @Serify.field() checksum: number[] = [];
  @Serify.field() window: number[] = [];
  @Serify.field({ rename: 'dropped_frames' }) droppedFrames: number | null = null;
  @Serify.field() mode = '';

  marshal(): Buffer {
    const head = Buffer.alloc(8);
    head.writeBigUInt64LE(this.captureId, 0);

    const parts: Buffer[] = [head];

    parts.push(packList(this.flags, 1, (b, v, o) => b.writeUInt8(v ? 1 : 0, o)));
    parts.push(packList(this.rawFrame, 1, (b, v, o) => b.writeUInt8(v, o)));
    parts.push(packList(this.portNumbers, 2, (b, v, o) => b.writeUInt16LE(v, o)));
    parts.push(packList(this.sampleCounts, 4, (b, v, o) => b.writeUInt32LE(v, o)));
    parts.push(packList(this.byteTotals, 8, (b, v, o) => b.writeBigUInt64LE(v, o)));
    parts.push(packList(this.trimOffsets, 1, (b, v, o) => b.writeInt8(v, o)));
    parts.push(packList(this.driftDeltas, 2, (b, v, o) => b.writeInt16LE(v, o)));
    parts.push(packList(this.temperaturesC, 4, (b, v, o) => b.writeInt32LE(v, o)));
    parts.push(packList(this.timestampsNs, 8, (b, v, o) => b.writeBigInt64LE(v, o)));

    for (const list of [this.counters, this.balances]) {
      const c = Buffer.alloc(4);
      c.writeUInt32LE(list.length, 0);
      parts.push(c, ...list.map(writeInt128LE));
    }

    parts.push(packList(this.gains, 4, (b, v, o) => b.writeFloatLE(v, o)));
    parts.push(packList(this.voltages, 8, (b, v, o) => b.writeDoubleLE(v, o)));

    const nameCount = Buffer.alloc(4);
    nameCount.writeUInt32LE(this.channelNames.length, 0);
    parts.push(nameCount);
    for (const s of this.channelNames) {
      const raw = Buffer.from(s, 'utf8');
      const len = Buffer.alloc(4);
      len.writeUInt32LE(raw.length, 0);
      parts.push(len, raw);
    }

    const payloadCount = Buffer.alloc(4);
    payloadCount.writeUInt32LE(this.payloads.length, 0);
    parts.push(payloadCount);
    for (const p of this.payloads) {
      const len = Buffer.alloc(4);
      len.writeUInt32LE(p.length, 0);
      parts.push(len, p);
    }

    // array<T,N> carries no count: N is fixed by the schema.
    const chk = Buffer.alloc(4);
    this.checksum.forEach((v, i) => chk.writeUInt8(v, i));
    const win = Buffer.alloc(this.window.length * 2);
    this.window.forEach((v, i) => win.writeInt16LE(v, i * 2));
    parts.push(chk, win);

    // optional<uint32>: a presence flag, then the value if present.
    if (this.droppedFrames === null) {
      parts.push(Buffer.from([0]));
    } else {
      const b = Buffer.alloc(5);
      b.writeUInt8(1, 0);
      b.writeUInt32LE(this.droppedFrames, 1);
      parts.push(b);
    }

    // enum: a u8 ordinal, the variant's position in the case file.
    parts.push(Buffer.from([SIGNAL_MODES.indexOf(this.mode)]));

    return Buffer.concat(parts);
  }

  static unmarshal(data: Buffer): SignalCapture {
    const s = new SignalCapture();
    let pos = 0;
    s.captureId = data.readBigUInt64LE(pos); pos += 8;

    let flagCount = data.readUInt32LE(pos); pos += 4;
    s.flags = [];
    for (let i = 0; i < flagCount; i++) s.flags.push(data.readUInt8(pos + i) !== 0);
    pos += flagCount;

    [s.rawFrame, pos] = unpackList(data, pos, 1, (b, o) => b.readUInt8(o));
    [s.portNumbers, pos] = unpackList(data, pos, 2, (b, o) => b.readUInt16LE(o));
    [s.sampleCounts, pos] = unpackList(data, pos, 4, (b, o) => b.readUInt32LE(o));
    [s.byteTotals, pos] = unpackList(data, pos, 8, (b, o) => b.readBigUInt64LE(o));
    [s.trimOffsets, pos] = unpackList(data, pos, 1, (b, o) => b.readInt8(o));
    [s.driftDeltas, pos] = unpackList(data, pos, 2, (b, o) => b.readInt16LE(o));
    [s.temperaturesC, pos] = unpackList(data, pos, 4, (b, o) => b.readInt32LE(o));
    [s.timestampsNs, pos] = unpackList(data, pos, 8, (b, o) => b.readBigInt64LE(o));
    [s.counters, pos] = unpackList(data, pos, 16, (b, o) => readInt128LE(b, o, false));
    [s.balances, pos] = unpackList(data, pos, 16, (b, o) => readInt128LE(b, o, true));
    [s.gains, pos] = unpackList(data, pos, 4, (b, o) => b.readFloatLE(o));
    [s.voltages, pos] = unpackList(data, pos, 8, (b, o) => b.readDoubleLE(o));

    const nameCount = data.readUInt32LE(pos); pos += 4;
    s.channelNames = [];
    for (let i = 0; i < nameCount; i++) {
      const n = data.readUInt32LE(pos); pos += 4;
      s.channelNames.push(data.subarray(pos, pos + n).toString('utf8'));
      pos += n;
    }

    const payloadCount = data.readUInt32LE(pos); pos += 4;
    s.payloads = [];
    for (let i = 0; i < payloadCount; i++) {
      const n = data.readUInt32LE(pos); pos += 4;
      s.payloads.push(Buffer.from(data.subarray(pos, pos + n)));
      pos += n;
    }

    s.checksum = [];
    for (let i = 0; i < 4; i++) s.checksum.push(data.readUInt8(pos + i));
    pos += 4;
    s.window = [];
    for (let i = 0; i < 3; i++) s.window.push(data.readInt16LE(pos + i * 2));
    pos += 6;

    if (data.readUInt8(pos) === 0) {
      s.droppedFrames = null;
      pos += 1;
    } else {
      s.droppedFrames = data.readUInt32LE(pos + 1);
      pos += 5;
    }

    s.mode = SIGNAL_MODES[data.readUInt8(pos)];
    pos += 1;

    return s;
  }
}
