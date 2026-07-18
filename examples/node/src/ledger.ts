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
 * `LedgerEntry` mirrors examples/cases/ledger.yaml.
 *
 * `@Serify.Model()` plus one `@Serify.field()` per property is the entire schema
 * binding — nothing here calls a get/set accessor. Everything else is the byte
 * layout, which is the part a conformance worker exists to exercise.
 *
 * JS numbers are IEEE-754 doubles, so every 64/128-bit integer is a BigInt.
 * BigInt is unbounded, so the two int128 fields need no special type — but
 * Buffer has no 128-bit writer, so they are written as two 64-bit halves.
 *
 * Go is the --ref language and owns the layout; see examples/go/wire.go.
 */

import { Serify } from '@chengxilo/serify';

import { lenPrefixed, lenPrefixedStr } from './wire';

const MASK_64 = (1n << 64n) - 1n;
const MASK_128 = (1n << 128n) - 1n;

// int128: 16 bytes little-endian, two's complement. Masking to 128 bits maps a
// negative onto its residue class, which is exactly two's complement.
function writeInt128LE(v: bigint): Buffer {
  const u = v & MASK_128;
  const b = Buffer.alloc(16);
  b.writeBigUInt64LE(u & MASK_64, 0);
  b.writeBigUInt64LE(u >> 64n, 8);
  return b;
}

function readInt128LE(b: Buffer, off: number): bigint {
  const u = b.readBigUInt64LE(off) | (b.readBigUInt64LE(off + 8) << 64n);
  return u >= 1n << 127n ? u - (1n << 128n) : u; // re-apply the sign
}

@Serify.Model()
export class LedgerEntry {
  @Serify.field() entry_id: bigint = 0n;
  @Serify.field() block_number: bigint = 0n;
  @Serify.field() block_time: bigint = 0n;
  @Serify.field() tx_hash: Buffer = Buffer.alloc(0);
  @Serify.field() account = '';
  @Serify.field() asset = '';
  @Serify.field() amount_base_units: bigint = 0n;
  @Serify.field() balance_after: bigint = 0n;
  @Serify.field() confirmed = false;
  @Serify.field() memo: string | null = null;

  marshal(): Buffer {
    const head = Buffer.alloc(24);
    head.writeBigUInt64LE(this.entry_id, 0);
    head.writeBigUInt64LE(this.block_number, 8);
    head.writeBigInt64LE(this.block_time, 16);

    const parts: Buffer[] = [
      head,
      lenPrefixed(this.tx_hash),
      lenPrefixedStr(this.account),
      lenPrefixedStr(this.asset),
      writeInt128LE(this.amount_base_units),
      writeInt128LE(this.balance_after),
      Buffer.from([this.confirmed ? 1 : 0]),
    ];

    if (this.memo === null) {
      parts.push(Buffer.from([0]));
    } else {
      parts.push(Buffer.from([1]), lenPrefixedStr(this.memo));
    }
    return Buffer.concat(parts);
  }

  static unmarshal(data: Buffer): LedgerEntry {
    const e = new LedgerEntry();
    e.entry_id = data.readBigUInt64LE(0);
    e.block_number = data.readBigUInt64LE(8);
    e.block_time = data.readBigInt64LE(16);
    let off = 24;

    const readLenBuf = (): Buffer => {
      const n = data.readUInt32LE(off);
      off += 4;
      const b = data.subarray(off, off + n);
      off += n;
      return Buffer.from(b); // copy: a subarray would alias the input
    };

    e.tx_hash = readLenBuf();
    e.account = readLenBuf().toString('utf8');
    e.asset = readLenBuf().toString('utf8');

    e.amount_base_units = readInt128LE(data, off);
    e.balance_after = readInt128LE(data, off + 16);
    off += 32;

    e.confirmed = data[off] !== 0;
    const hasMemo = data[off + 1] !== 0;
    off += 2;
    e.memo = hasMemo ? readLenBuf().toString('utf8') : null;

    return e;
  }
}
