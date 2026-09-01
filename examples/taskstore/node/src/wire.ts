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
 * Byte-level primitives. Go owns the layout these reproduce; the conventions
 * are documented at the top of go/api/wire.go.
 */

/** A cursor over one message's bytes. */
export class Reader {
  off = 0;

  constructor(public buf: Buffer) {}

  private take(n: number): Buffer {
    if (this.off + n > this.buf.length) {
      throw new Error(`truncated: want ${n} bytes at offset ${this.off}, have ${this.buf.length - this.off}`);
    }
    const out = this.buf.subarray(this.off, this.off + n);
    this.off += n;
    return out;
  }

  u8(): number {
    return this.take(1)[0];
  }

  u32(): number {
    return this.take(4).readUInt32LE(0);
  }

  /** uint64 exceeds 2^53, so it is a bigint here and everywhere it travels. */
  u64(): bigint {
    return this.take(8).readBigUInt64LE(0);
  }

  i64(): bigint {
    return this.take(8).readBigInt64LE(0);
  }

  bool(): boolean {
    return this.u8() !== 0;
  }

  str(): string {
    const n = this.u32();
    return this.take(n).toString('utf8');
  }

  /**
   * An enum arrives as an ordinal into a fixed list, so a value outside the
   * declared variants has no encoding and cannot be read back.
   */
  enumOf(variants: readonly string[]): string {
    const ord = this.u8();
    if (ord >= variants.length) {
      throw new Error(`enum ordinal ${ord} out of range for ${variants.join(', ')}`);
    }
    return variants[ord];
  }

  tags(): string[] {
    const n = this.u32();
    const out: string[] = [];
    for (let i = 0; i < n; i++) out.push(this.str());
    return out;
  }

  optionalI64(): bigint | null {
    return this.u8() === 0 ? null : this.i64();
  }
}

export function putStr(s: string): Buffer {
  const body = Buffer.from(s, 'utf8');
  const len = Buffer.alloc(4);
  len.writeUInt32LE(body.length, 0);
  return Buffer.concat([len, body]);
}

export function putU32(n: number): Buffer {
  const b = Buffer.alloc(4);
  b.writeUInt32LE(n, 0);
  return b;
}

export function putU64(n: bigint): Buffer {
  const b = Buffer.alloc(8);
  b.writeBigUInt64LE(n, 0);
  return b;
}

export function putEnum(variants: readonly string[], name: string): Buffer {
  const ord = variants.indexOf(name);
  if (ord < 0) throw new Error(`"${name}" is not one of ${variants.join(', ')}`);
  return Buffer.from([ord]);
}

export function putTags(tags: string[]): Buffer {
  return Buffer.concat([putU32(tags.length), ...tags.map(putStr)]);
}

export function putOptionalI64(v: bigint | null): Buffer {
  if (v === null) return Buffer.from([0]);
  const b = Buffer.alloc(8);
  b.writeBigInt64LE(v, 0);
  return Buffer.concat([Buffer.from([1]), b]);
}
