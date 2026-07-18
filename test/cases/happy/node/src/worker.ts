/*
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
 * Happy-path Node worker: `all_types` in binary and json.
 *
 * Go is the --ref language and owns both byte layouts; see
 * test/cases/happy/go/type.go. The json format must match Go's encoding/json
 * byte-for-byte (with SetEscapeHTML(false)): schema field order, map keys in
 * UTF-8 byte order, []byte as base64, floats in shortest form without a
 * trailing .0, and U+2028/U+2029 escaped (Go escapes those unconditionally).
 *
 * JS numbers are IEEE-754 doubles, so uint64/int64 are BigInt. JSON.parse
 * would round them, so json deserialize quotes those two fields' number
 * literals before parsing and converts through BigInt.
 */

import { FieldMap, runSuite } from '@chengxilo/serify';

const STATUS_VARIANTS = ['pending', 'paid', 'shipped', 'delivered', 'cancelled'];

function statusOrdinal(s: string): number {
  const i = STATUS_VARIANTS.indexOf(s);
  if (i < 0) throw new Error(`unknown status ${JSON.stringify(s)}`);
  return i;
}

// UTF-8 byte order, which is code-point order — not UTF-16 code-unit order,
// which Array.sort() would use and which misplaces astral-plane keys.
function byteCompare(a: string, b: string): number {
  const ab = Buffer.from(a); const bb = Buffer.from(b);
  return ab.compare(bb);
}

// --- binary format -----------------------------------------------------------

class Writer {
  chunks: Buffer[] = [];
  push(b: Buffer) { this.chunks.push(b); }
  u8(v: number)  { this.push(Buffer.from([v & 0xff])); }
  u16(v: number) { const b = Buffer.alloc(2); b.writeUInt16LE(v); this.push(b); }
  u32(v: number) { const b = Buffer.alloc(4); b.writeUInt32LE(v >>> 0); this.push(b); }
  u64(v: bigint) { const b = Buffer.alloc(8); b.writeBigUInt64LE(BigInt.asUintN(64, v)); this.push(b); }
  i32(v: number) { const b = Buffer.alloc(4); b.writeInt32LE(v); this.push(b); }
  f32(v: number) { const b = Buffer.alloc(4); b.writeFloatLE(v); this.push(b); }
  f64(v: number) { const b = Buffer.alloc(8); b.writeDoubleLE(v); this.push(b); }
  lenStr(s: string) { const b = Buffer.from(s, 'utf8'); this.u32(b.length); this.push(b); }
  out(): Buffer { return Buffer.concat(this.chunks); }
}

class Cursor {
  off = 0;
  constructor(readonly data: Buffer) {}
  u8(): number  { return this.data.readUInt8(this.off++); }
  u16(): number { const v = this.data.readUInt16LE(this.off); this.off += 2; return v; }
  u32(): number { const v = this.data.readUInt32LE(this.off); this.off += 4; return v; }
  u64(): bigint { const v = this.data.readBigUInt64LE(this.off); this.off += 8; return v; }
  i8(): number  { return this.data.readInt8(this.off++); }
  i16(): number { const v = this.data.readInt16LE(this.off); this.off += 2; return v; }
  i32(): number { const v = this.data.readInt32LE(this.off); this.off += 4; return v; }
  i64(): bigint { const v = this.data.readBigInt64LE(this.off); this.off += 8; return v; }
  f32(): number { const v = this.data.readFloatLE(this.off); this.off += 4; return v; }
  f64(): number { const v = this.data.readDoubleLE(this.off); this.off += 8; return v; }
  lenStr(): string {
    const n = this.u32();
    const s = this.data.toString('utf8', this.off, this.off + n);
    this.off += n;
    return s;
  }
  lenBytes(): Buffer {
    const n = this.u32();
    const b = this.data.subarray(this.off, this.off + n);
    this.off += n;
    return Buffer.from(b);
  }
}

function binarySerialize(fm: FieldMap): Buffer {
  const w = new Writer();
  w.u8(fm.getU8('uint8'));
  w.u16(fm.getU16('uint16'));
  w.u32(fm.getU32('uint32'));
  w.u64(fm.getU64('uint64'));
  w.u8(fm.getI8('int8') & 0xff);
  w.u16(fm.getI16('int16') & 0xffff);
  w.u32(fm.getI32('int32') >>> 0);
  w.u64(BigInt.asUintN(64, fm.getI64('int64')));
  w.f32(fm.getF32('float32'));
  w.f64(fm.getF64('float64'));
  w.u8(fm.getBool('bool') ? 1 : 0);
  w.lenStr(fm.getString('string'));

  const raw = fm.getBytes('bytes');
  w.u32(raw.length);
  w.push(raw);

  const list = fm.getListString('list');
  w.u32(list.length);
  for (const s of list) w.lenStr(s);

  const opt = fm.getOptionalString('optional');
  if (opt === null) { w.u8(0); } else { w.u8(1); w.lenStr(opt); }

  for (const n of fm.getListU32('array')) w.u32(n);

  const p = fm.getStruct('struct')!;
  w.i32(p.getI32('x'));
  w.i32(p.getI32('y'));
  w.i32(p.getI32('z'));
  w.lenStr(p.getString('name'));

  const m = fm.getMap('map');
  const keys = Array.from(m.keys()).sort(byteCompare);
  w.u32(keys.length);
  for (const k of keys) { w.lenStr(k); w.u32(m.get(k) as number); }

  const ms = fm.getMap('map_struct');
  const mkeys = Array.from(ms.keys()).sort(byteCompare);
  w.u32(mkeys.length);
  for (const k of mkeys) {
    const t = ms.get(k) as FieldMap;
    w.lenStr(k);
    w.lenStr(t.getString('name'));
    w.u32(t.getU32('weight'));
  }

  w.u8(statusOrdinal(fm.getString('status')));
  return w.out();
}

function binaryDeserialize(data: Buffer): FieldMap {
  const c = new Cursor(data);
  const fm = new FieldMap();
  fm.setU8('uint8', c.u8());
  fm.setU16('uint16', c.u16());
  fm.setU32('uint32', c.u32());
  fm.setU64('uint64', c.u64());
  fm.setI8('int8', c.i8());
  fm.setI16('int16', c.i16());
  fm.setI32('int32', c.i32());
  fm.setI64('int64', c.i64());
  fm.setF32('float32', c.f32());
  fm.setF64('float64', c.f64());
  fm.setBool('bool', c.u8() !== 0);
  fm.setString('string', c.lenStr());
  fm.setBytes('bytes', c.lenBytes());

  const listLen = c.u32();
  const list: string[] = [];
  for (let i = 0; i < listLen; i++) list.push(c.lenStr());
  fm.setListString('list', list);

  fm.setOptionalString('optional', c.u8() !== 0 ? c.lenStr() : null);

  fm.setListU32('array', [c.u32(), c.u32(), c.u32(), c.u32()]);

  const p = new FieldMap();
  p.setI32('x', c.i32());
  p.setI32('y', c.i32());
  p.setI32('z', c.i32());
  p.setString('name', c.lenStr());
  fm.setStruct('struct', p);

  const m = new Map<string, unknown>();
  const mapLen = c.u32();
  for (let i = 0; i < mapLen; i++) { const k = c.lenStr(); m.set(k, c.u32()); }
  fm.setMap('map', m);

  const ms = new Map<string, unknown>();
  const msLen = c.u32();
  for (let i = 0; i < msLen; i++) {
    const k = c.lenStr();
    const t = new FieldMap();
    t.setString('name', c.lenStr());
    t.setU32('weight', c.u32());
    ms.set(k, t);
  }
  fm.setMap('map_struct', ms);

  const ord = c.u8();
  if (ord >= STATUS_VARIANTS.length) throw new Error(`status ordinal ${ord} out of range`);
  fm.setString('status', STATUS_VARIANTS[ord]);
  return fm;
}

// --- json format -------------------------------------------------------------

// Go's encoding/json string escaping with SetEscapeHTML(false): only \n, \r,
// \t are named (\b and \f become \u00xx), and U+2028/U+2029 are escaped
// unconditionally.
function goStr(s: string): string {
  let out = '"';
  for (const ch of s) {
    const o = ch.codePointAt(0)!;
    if (ch === '"') out += '\\"';
    else if (ch === '\\') out += '\\\\';
    else if (o < 0x20) {
      if (o === 0x0a) out += '\\n';
      else if (o === 0x0d) out += '\\r';
      else if (o === 0x09) out += '\\t';
      else out += '\\u' + o.toString(16).padStart(4, '0');
    } else if (o === 0x2028 || o === 0x2029) {
      out += '\\u' + o.toString(16);
    } else {
      out += ch;
    }
  }
  return out + '"';
}

// JS prints doubles in shortest round-trip form without a trailing .0, which
// is exactly Go's format for the value range these cases use.
function goF64(v: number): string { return String(v); }

// Shortest decimal that round-trips through float32 (v is the f64 widening).
function goF32(v: number): string {
  for (let p = 1; p <= 9; p++) {
    const s = v.toPrecision(p);
    if (Math.fround(Number(s)) === Math.fround(v)) return String(Number(s));
  }
  return String(v);
}

function jsonSerialize(fm: FieldMap): Buffer {
  const parts: string[] = [
    `"uint8":${fm.getU8('uint8')}`,
    `"uint16":${fm.getU16('uint16')}`,
    `"uint32":${fm.getU32('uint32')}`,
    `"uint64":${fm.getU64('uint64')}`,
    `"int8":${fm.getI8('int8')}`,
    `"int16":${fm.getI16('int16')}`,
    `"int32":${fm.getI32('int32')}`,
    `"int64":${fm.getI64('int64')}`,
    `"float32":${goF32(fm.getF32('float32'))}`,
    `"float64":${goF64(fm.getF64('float64'))}`,
    `"bool":${fm.getBool('bool')}`,
    `"string":${goStr(fm.getString('string'))}`,
    `"bytes":"${fm.getBytes('bytes').toString('base64')}"`,
    `"list":[${fm.getListString('list').map(goStr).join(',')}]`,
  ];

  const opt = fm.getOptionalString('optional');
  parts.push(`"optional":${opt === null ? 'null' : goStr(opt)}`);

  parts.push(`"array":[${fm.getListU32('array').join(',')}]`);

  const p = fm.getStruct('struct')!;
  parts.push(`"struct":{"x":${p.getI32('x')},"y":${p.getI32('y')},"z":${p.getI32('z')},"name":${goStr(p.getString('name'))}}`);

  const m = fm.getMap('map');
  const keys = Array.from(m.keys()).sort(byteCompare);
  parts.push(`"map":{${keys.map(k => `${goStr(k)}:${m.get(k)}`).join(',')}}`);

  const ms = fm.getMap('map_struct');
  const mkeys = Array.from(ms.keys()).sort(byteCompare);
  parts.push(`"map_struct":{${mkeys.map(k => {
    const t = ms.get(k) as FieldMap;
    return `${goStr(k)}:{"name":${goStr(t.getString('name'))},"weight":${t.getU32('weight')}}`;
  }).join(',')}}`);

  parts.push(`"status":${goStr(fm.getString('status'))}`);
  return Buffer.from(`{${parts.join(',')}}`, 'utf8');
}

function jsonDeserialize(data: Buffer): FieldMap {
  // Quote the two 64-bit fields' number literals so JSON.parse cannot round
  // them (safe here: the happy cases never contain that text inside strings).
  const text = data.toString('utf8')
    .replace(/("uint64":)(-?\d+)/, '$1"$2"')
    .replace(/("int64":)(-?\d+)/, '$1"$2"');
  const v = JSON.parse(text);

  const fm = new FieldMap();
  fm.setU8('uint8', v.uint8);
  fm.setU16('uint16', v.uint16);
  fm.setU32('uint32', v.uint32);
  fm.setU64('uint64', BigInt(v.uint64));
  fm.setI8('int8', v.int8);
  fm.setI16('int16', v.int16);
  fm.setI32('int32', v.int32);
  fm.setI64('int64', BigInt(v.int64));
  fm.setF32('float32', Math.fround(v.float32));
  fm.setF64('float64', v.float64);
  fm.setBool('bool', v.bool);
  fm.setString('string', v.string);
  fm.setBytes('bytes', Buffer.from(v.bytes, 'base64'));
  fm.setListString('list', v.list);
  fm.setOptionalString('optional', v.optional);
  fm.setListU32('array', v.array);

  const p = new FieldMap();
  p.setI32('x', v.struct.x);
  p.setI32('y', v.struct.y);
  p.setI32('z', v.struct.z);
  p.setString('name', v.struct.name);
  fm.setStruct('struct', p);

  const m = new Map<string, unknown>();
  for (const [k, n] of Object.entries(v.map)) m.set(k, n);
  fm.setMap('map', m);

  const ms = new Map<string, unknown>();
  for (const [k, tv] of Object.entries(v.map_struct as Record<string, { name: string; weight: number }>)) {
    const t = new FieldMap();
    t.setString('name', tv.name);
    t.setU32('weight', tv.weight);
    ms.set(k, t);
  }
  fm.setMap('map_struct', ms);

  fm.setString('status', v.status);
  return fm;
}

runSuite({
  all_types: {
    binary: { serialize: binarySerialize, deserialize: binaryDeserialize },
    json: { serialize: jsonSerialize, deserialize: jsonDeserialize },
  },
});
