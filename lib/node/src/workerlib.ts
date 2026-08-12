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

﻿import * as readline from 'readline';
import * as process from 'process';

// FieldMap

export class FieldMap {
  readonly _fields: Map<string, unknown> = new Map();

  getU8(key: string): number    { return this._fields.get(key) as number; }
  getU16(key: string): number   { return this._fields.get(key) as number; }
  getU32(key: string): number   { return this._fields.get(key) as number; }
  getU64(key: string): bigint   { return this._fields.get(key) as bigint; }
  getI8(key: string): number    { return this._fields.get(key) as number; }
  getI16(key: string): number   { return this._fields.get(key) as number; }
  getI32(key: string): number   { return this._fields.get(key) as number; }
  getI64(key: string): bigint   { return this._fields.get(key) as bigint; }
  getF32(key: string): number   { return this._fields.get(key) as number; }
  getF64(key: string): number   { return this._fields.get(key) as number; }
  getBool(key: string): boolean { return this._fields.get(key) as boolean; }
  getString(key: string): string { return this._fields.get(key) as string; }
  getBytes(key: string): Buffer  { return this._fields.get(key) as Buffer; }
  getListString(key: string): string[] { return this._fields.get(key) as string[]; }
  getOptionalString(key: string): string | null { return (this._fields.get(key) ?? null) as string | null; }
  getStruct(key: string): FieldMap | null { return (this._fields.get(key) ?? null) as FieldMap | null; }
  getListStruct(key: string): FieldMap[] { return (this._fields.get(key) ?? []) as FieldMap[]; }
  getOptionalStruct(key: string): FieldMap | null { return (this._fields.get(key) ?? null) as FieldMap | null; }
  /**
   * An `optional<T>` of any element type: the value, or null when the case data
   * carried a null. getOptionalString and getOptionalStruct are just the named
   * spellings of this; every other element type — `optional<uint32>`,
   * `optional<float64>` — goes through here rather than needing an accessor of
   * its own.
   */
  getOptional<T>(key: string): T | null { return (this._fields.get(key) ?? null) as T | null; }
  getListU8(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListU16(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListU32(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListU64(key: string): bigint[] { return (this._fields.get(key) ?? []) as bigint[]; }
  getListU128(key: string): bigint[] { return (this._fields.get(key) ?? []) as bigint[]; }
  getListI8(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListI16(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListI32(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListI64(key: string): bigint[] { return (this._fields.get(key) ?? []) as bigint[]; }
  getListI128(key: string): bigint[] { return (this._fields.get(key) ?? []) as bigint[]; }
  getListF32(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListF64(key: string): number[] { return (this._fields.get(key) ?? []) as number[]; }
  getListBool(key: string): boolean[] { return (this._fields.get(key) ?? []) as boolean[]; }
  getListBytes(key: string): Buffer[] { return (this._fields.get(key) ?? []) as Buffer[]; }
  getMap(key: string): Map<string, unknown> { return (this._fields.get(key) ?? new Map()) as Map<string, unknown>; }

  setU8(key: string, v: number)    { this._fields.set(key, v); }
  setU16(key: string, v: number)   { this._fields.set(key, v); }
  setU32(key: string, v: number)   { this._fields.set(key, v); }
  setU64(key: string, v: bigint)   { this._fields.set(key, v); }
  setI8(key: string, v: number)    { this._fields.set(key, v); }
  setI16(key: string, v: number)   { this._fields.set(key, v); }
  setI32(key: string, v: number)   { this._fields.set(key, v); }
  setI64(key: string, v: bigint)   { this._fields.set(key, v); }
  setF32(key: string, v: number)   { this._fields.set(key, v); }
  setF64(key: string, v: number)   { this._fields.set(key, v); }
  setBool(key: string, v: boolean)  { this._fields.set(key, v); }
  setString(key: string, v: string) { this._fields.set(key, v); }
  setBytes(key: string, v: Buffer)   { this._fields.set(key, v); }
  setListString(key: string, v: string[]) { this._fields.set(key, v); }
  setOptionalString(key: string, v: string | null) { this._fields.set(key, v); }
  setStruct(key: string, v: FieldMap) { this._fields.set(key, v); }
  setListStruct(key: string, v: FieldMap[]) { this._fields.set(key, v); }
  setOptionalStruct(key: string, v: FieldMap | null) { this._fields.set(key, v); }
  setListU8(key: string, v: number[]) { this._fields.set(key, v); }
  setListU16(key: string, v: number[]) { this._fields.set(key, v); }
  setListU32(key: string, v: number[]) { this._fields.set(key, v); }
  setListU64(key: string, v: bigint[]) { this._fields.set(key, v); }
  setListU128(key: string, v: bigint[]) { this._fields.set(key, v); }
  setListI8(key: string, v: number[]) { this._fields.set(key, v); }
  setListI16(key: string, v: number[]) { this._fields.set(key, v); }
  setListI32(key: string, v: number[]) { this._fields.set(key, v); }
  setListI64(key: string, v: bigint[]) { this._fields.set(key, v); }
  setListI128(key: string, v: bigint[]) { this._fields.set(key, v); }
  setListF32(key: string, v: number[]) { this._fields.set(key, v); }
  setListF64(key: string, v: number[]) { this._fields.set(key, v); }
  setListBool(key: string, v: boolean[]) { this._fields.set(key, v); }
  setListBytes(key: string, v: Buffer[]) { this._fields.set(key, v); }
  setMap(key: string, v: Map<string, unknown>) { this._fields.set(key, v); }

  /** Store a sum value: the active variant's tag and payload (null for a unit variant). */
  setVariant(key: string, tag: string, value: unknown) { this._fields.set(key, new Variant(tag, value)); }

  /** Read the sum value stored at key. */
  getVariant(key: string): Variant {
    const v = this._fields.get(key);
    if (!(v instanceof Variant)) {
      throw new Error(`field "${key}" is not a variant`);
    }
    return v;
  }
}

/**
 * One arm of a sum: a tag and its decoded payload (null for a unit variant).
 * A sum field stores a Variant.
 */
export class Variant {
  constructor(readonly tag: string, readonly value: unknown = null) {}
}

// Schema

export interface SchemaField {
  name: string;
  type: string;
  fields?: SchemaField[];
  variants?: SchemaVariant[];
  tags?: Record<string, string>;
}

/** One arm of a sum: a tag and its payload schema (null for a unit variant). */
export interface SchemaVariant {
  name: string;
  payload?: SchemaField | null;
}

function parseSchemaFields(arr: unknown[]): SchemaField[] {
  return (arr || []).map((f: unknown) => {
    const o = f as Record<string, unknown>;
    return {
      name:     o['name'] as string,
      type:     o['type'] as string,
      fields:   parseSchemaFields((o['fields'] as unknown[]) || []),
      variants: parseSchemaVariants((o['variants'] as unknown[]) || []),
      tags:     (o['tags'] as Record<string, string>) || {},
    };
  });
}

function parseSchemaVariants(arr: unknown[]): SchemaVariant[] {
  return (arr || []).map((v: unknown) => {
    const o = v as Record<string, unknown>;
    const payload = o['payload'] as Record<string, unknown> | undefined | null;
    return {
      name:    o['name'] as string,
      payload: payload ? parseSchemaFields([payload])[0] : null,
    };
  });
}

function findSchemaVariant(sf: SchemaField, tag: string): SchemaVariant {
  const sv = (sf.variants || []).find(v => v.name === tag);
  if (!sv) throw new Error(`unknown variant "${tag}"`);
  return sv;
}

// JSON decode/encode

export function decodeFieldMap(data: Record<string, unknown>, schema: SchemaField[]): FieldMap {
  const fm = new FieldMap();
  for (const sf of schema) {
    const v = data[sf.name];
    if (v === undefined) continue;
    decodeField(fm, sf, v);
  }
  return fm;
}

function decodeField(fm: FieldMap, sf: SchemaField, v: unknown): void {
  const name = sf.name;
  const typ  = sf.type;
  if (typ === 'uint8')  { fm.setU8(name, Number(v)); return; }
  if (typ === 'uint16') { fm.setU16(name, Number(v)); return; }
  if (typ === 'uint32') { fm.setU32(name, Number(v)); return; }
  if (typ === 'uint64' || typ === 'uint128') { fm.setU64(name, BigInt(v as string)); return; }
  if (typ === 'int8')  { fm.setI8(name, Number(v)); return; }
  if (typ === 'int16') { fm.setI16(name, Number(v)); return; }
  if (typ === 'int32') { fm.setI32(name, Number(v)); return; }
  if (typ === 'int64' || typ === 'int128') { fm.setI64(name, BigInt(v as string)); return; }
  if (typ === 'float32') {
    fm.setF32(name, Buffer.from(v as string, 'hex').readFloatLE(0)); return;
  }
  if (typ === 'float64') {
    fm.setF64(name, Buffer.from(v as string, 'hex').readDoubleLE(0)); return;
  }
  if (typ === 'bool')   { fm.setBool(name, Boolean(v)); return; }
  if (typ === 'string') { fm.setString(name, String(v)); return; }
  if (typ === 'bytes')  { fm.setBytes(name, Buffer.from(v as string, 'hex')); return; }
  if (typ === 'struct') {
    fm.setStruct(name, decodeFieldMap(v as Record<string, unknown>, sf.fields || [])); return;
  }
  if (typ.startsWith('list<')) {
    const elem = typ.slice(5, -1);
    decodeList(fm, sf, elem, v as unknown[]);
    return;
  }
  if (typ.startsWith('optional<')) {
    const elem = typ.slice(9, -1);
    decodeOptional(fm, sf, elem, v);
    return;
  }
  if (typ.startsWith('array<')) {
    // An array<T,N> is a list whose length the schema fixes, so it shares
    // decodeList outright and adds only the length check. A separate
    // representation is what pinned array<T,N> to uint32 with N = 4.
    const [elem, n] = splitArrayType(typ);
    decodeList(fm, sf, elem, v as unknown[]);
    const got = (fm._fields.get(name) as unknown[]).length;
    if (got !== n) throw new Error(`array ${name}: expected ${n} elements, got ${got}`);
    return;
  }
  // enum<a,b,c>: the variant name travels as a string.
  if (typ.startsWith('enum<')) { fm.setString(name, String(v)); return; }
  if (typ.startsWith('sum<')) { decodeSum(fm, sf, v); return; }
  if (typ.startsWith('map<')) {
    const [, valType] = splitMapTypes(typ);
    fm.setMap(name, decodeMap(valType, sf.fields || [], v as Record<string, unknown>)); return;
  }
  // Falling off the end left the field absent from the FieldMap, which surfaces
  // far downstream as a missing value rather than as "this library does not
  // know that type".
  throw new Error(`unknown type "${typ}"`);
}

/**
 * Decode a sum wire value {tag: payload} (payload null for a unit variant)
 * into a Variant, decoding the payload per the variant's own schema.
 */
function decodeSum(fm: FieldMap, sf: SchemaField, v: unknown): void {
  const obj = v as Record<string, unknown>;
  const tags = Object.keys(obj);
  if (tags.length !== 1) {
    throw new Error(`sum must name exactly one variant, got ${tags.length}`);
  }
  const tag = tags[0];
  const sv = findSchemaVariant(sf, tag);
  if (!sv.payload) { fm.setVariant(sf.name, tag, null); return; }
  const inner = new FieldMap();
  decodeField(inner, sv.payload, obj[tag]);
  fm.setVariant(sf.name, tag, inner._fields.get(sv.payload.name));
}

function splitMapTypes(typ: string): [string, string] {
  const inner = typ.slice(4, -1);
  let depth = 0;
  for (let i = 0; i < inner.length; i++) {
    if (inner[i] === '<' || inner[i] === '[') depth++;
    else if (inner[i] === '>' || inner[i] === ']') depth--;
    else if (inner[i] === ',' && depth === 0) {
      return [inner.slice(0, i).trim(), inner.slice(i + 1).trim()];
    }
  }
  return ['', inner.trim()];
}

function decodeMap(valType: string, nestedSchema: SchemaField[], obj: Record<string, unknown>): Map<string, unknown> {
  const m = new Map<string, unknown>();
  for (const [k, item] of Object.entries(obj)) {
    if (valType === 'struct') {
      m.set(k, decodeFieldMap(item as Record<string, unknown>, nestedSchema));
    } else if (valType === 'uint64' || valType === 'uint128') {
      m.set(k, BigInt(item as string));
    } else if (valType === 'int64' || valType === 'int128') {
      m.set(k, BigInt(item as string));
    } else if (valType === 'float32') {
      m.set(k, Buffer.from(item as string, 'hex').readFloatLE(0));
    } else if (valType === 'float64') {
      m.set(k, Buffer.from(item as string, 'hex').readDoubleLE(0));
    } else if (valType === 'bytes') {
      m.set(k, Buffer.from(item as string, 'hex'));
    } else {
      m.set(k, item);
    }
  }
  return m;
}

/**
 * Element types a list may carry: every scalar, plus a nested struct.
 */
const LIST_ELEMS = new Set([
  'uint8', 'uint16', 'uint32', 'uint64', 'uint128',
  'int8', 'int16', 'int32', 'int64', 'int128',
  'float32', 'float64', 'bool', 'string', 'bytes',
  'struct',
]);

/**
 * Decodes every element through decodeField, so a list supports exactly the
 * element types a bare field does. This used to carry its own switch, which is
 * why uint16/int8/int16/float64/bytes were declarable in a case file and
 * accepted by `serify validate`, but blew up once a worker ran.
 */
/** Split "array<T,N>" into its element type and length. */
function splitArrayType(typ: string): [string, number] {
  const inner = typ.slice(6, -1);
  const comma = inner.lastIndexOf(',');
  return [inner.slice(0, comma).trim(), parseInt(inner.slice(comma + 1).trim(), 10)];
}

function decodeList(fm: FieldMap, sf: SchemaField, elem: string, arr: unknown[]): void {
  if (!LIST_ELEMS.has(elem)) throw new Error(`unsupported list element type "${elem}"`);
  if (elem === 'struct') {
    fm.setListStruct(sf.name, arr.map(item =>
      decodeFieldMap(item as Record<string, unknown>, sf.fields || [])
    ));
    return;
  }
  const elemSf: SchemaField = { name: 'e', type: elem, fields: sf.fields };
  fm._fields.set(sf.name, arr.map((item, i) => {
    const tmp = new FieldMap();
    try {
      decodeField(tmp, elemSf, item);
    } catch (e) {
      throw new Error(`[${i}]: ${(e as Error).message}`);
    }
    return tmp._fields.get('e');
  }));
}

function decodeOptional(fm: FieldMap, sf: SchemaField, elem: string, v: unknown): void {
  if (v === null || v === undefined) {
    fm._fields.set(sf.name, null); return;
  }
  switch (elem) {
    case 'string': fm.setOptionalString(sf.name, String(v)); return;
    case 'struct':
      fm.setOptionalStruct(sf.name, decodeFieldMap(v as Record<string, unknown>, sf.fields || [])); return;
    default: {
      const inner = new FieldMap();
      decodeField(inner, { name: sf.name, type: elem, fields: sf.fields }, v);
      fm._fields.set(sf.name, inner._fields.get(sf.name));
      return;
    }
  }
}

export function encodeFieldMap(fm: FieldMap, schema: SchemaField[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const sf of schema) {
    const v = fm._fields.get(sf.name);
    if (v === undefined) continue;
    out[sf.name] = encodeField(sf, v);
  }
  return out;
}

function encodeField(sf: SchemaField, v: unknown): unknown {
  const typ = sf.type;
  // A bigint would reach JSON.stringify, which refuses it, so narrow first: at
  // 32 bits or less the value always fits a JS number exactly.
  if (typ === 'uint8' || typ === 'uint16' || typ === 'uint32' || typ === 'int8' || typ === 'int16' || typ === 'int32') {
    return typeof v === 'bigint' ? Number(v) : v;
  }
  if (typ === 'uint64' || typ === 'uint128' || typ === 'int64' || typ === 'int128') {
    // A number is accepted, but only where it still holds the value exactly.
    // This is the guard that used to live in the model binding, which had to
    // guess from the value; here the schema has already said "integer", so it
    // fires on the case it was written for and not on floats.
    if (typeof v === 'number' && !Number.isSafeInteger(v)) {
      throw new Error(
        `serify: ${v} for field "${sf.name}" is ${typ} but arrived as a JS number that ` +
        `cannot hold it exactly; use a bigint to keep full precision`);
    }
    return (v as bigint | number).toString();
  }
  // Number() so a field that arrived as a bigint still encodes: the width is
  // the schema's to decide, and a caller writing 1n for a float is not wrong.
  if (typ === 'float32') { const b = Buffer.alloc(4); b.writeFloatLE(Number(v), 0); return b.toString('hex'); }
  if (typ === 'float64') { const b = Buffer.alloc(8); b.writeDoubleLE(Number(v), 0); return b.toString('hex'); }
  if (typ === 'bool')   return v;
  if (typ === 'string') return v;
  if (typ === 'bytes')  return (v as Buffer).toString('hex');
  if (typ === 'struct') return encodeFieldMap(v as FieldMap, sf.fields || []);
  if (typ.startsWith('list<')) {
    const elem = typ.slice(5, -1);
    return encodeList(sf, elem, v as unknown[]);
  }
  if (typ.startsWith('optional<')) {
    const elem = typ.slice(9, -1);
    return encodeOptional(sf, elem, v);
  }
  if (typ.startsWith('array<')) {
    const [elem] = splitArrayType(typ);
    return encodeList(sf, elem, v as unknown[]);
  }
  // enum<a,b,c>: the variant name goes back out as a plain string.
  if (typ.startsWith('enum<')) return v;
  if (typ.startsWith('sum<')) return encodeSum(sf, v);
  if (typ.startsWith('map<')) {
    const [, valType] = splitMapTypes(typ);
    return encodeMap(valType, sf.fields || [], v as Map<string, unknown>);
  }
  throw new Error(`unknown type "${typ}"`);
}

/** Inverse of decodeSum: a Variant becomes {tag: payload}. */
function encodeSum(sf: SchemaField, v: unknown): unknown {
  if (!(v instanceof Variant)) {
    throw new Error(`expected a Variant for sum field "${sf.name}"`);
  }
  const sv = findSchemaVariant(sf, v.tag);
  if (!sv.payload) return { [v.tag]: null };
  return { [v.tag]: encodeField(sv.payload, v.value) };
}

function encodeMap(valType: string, nestedSchema: SchemaField[], m: Map<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  const keys = Array.from(m.keys()).sort();
  for (const k of keys) {
    const item = m.get(k);
    if (valType === 'struct') {
      out[k] = encodeFieldMap(item as FieldMap, nestedSchema);
    } else if (valType === 'uint64' || valType === 'uint128') {
      out[k] = (item as bigint).toString();
    } else if (valType === 'int64' || valType === 'int128') {
      out[k] = (item as bigint).toString();
    } else if (valType === 'float32') {
      const b = Buffer.alloc(4); b.writeFloatLE(item as number, 0); out[k] = b.toString('hex');
    } else if (valType === 'float64') {
      const b = Buffer.alloc(8); b.writeDoubleLE(item as number, 0); out[k] = b.toString('hex');
    } else if (valType === 'bytes') {
      out[k] = (item as Buffer).toString('hex');
    } else {
      out[k] = item;
    }
  }
  return out;
}

/**
 * Inverse of decodeList: every element goes back out through encodeField, so
 * the two directions cannot cover different element types. The old version fell
 * through to returning `arr` untouched for anything it did not name.
 */
function encodeList(sf: SchemaField, elem: string, arr: unknown[]): unknown {
  if (!LIST_ELEMS.has(elem)) throw new Error(`unsupported list element type "${elem}"`);
  if (elem === 'struct') return (arr as FieldMap[]).map(item => encodeFieldMap(item, sf.fields || []));
  const elemSf: SchemaField = { name: 'e', type: elem, fields: sf.fields };
  return arr.map(x => encodeField(elemSf, x));
}

function encodeOptional(sf: SchemaField, elem: string, v: unknown): unknown {
  if (v === null || v === undefined) return null;
  switch (elem) {
    case 'string': return v;
    case 'struct': return encodeFieldMap(v as FieldMap, sf.fields || []);
    default: return encodeField({ name: sf.name, type: elem, fields: sf.fields }, v);
  }
}

// --- audit helpers ----------------------------------------------------------
//
// The functions below are low-level audit helpers. A worker driven by the serify
// runner never calls them: register serialize/deserialize in a suite, and run()
// handles audit itself when --audit is passed. They are exported for audit-style
// checks outside the runner.

type FieldSnap = { fm: FieldMap; key: string; orig: Buffer | Variant };

/** Recursively walks a FieldMap and collects a snapshot of every Buffer value. */
export function collectByteSnaps(fm: FieldMap, snaps: FieldSnap[]): void {
  const keys = Array.from(fm._fields.keys()).sort();
  for (const k of keys) {
    const v = fm._fields.get(k);
    if (Buffer.isBuffer(v)) {
      snaps.push({ fm, key: k, orig: Buffer.from(v) });
    } else if (v instanceof FieldMap) {
      collectByteSnaps(v, snaps);
    } else if (Array.isArray(v)) {
      for (const item of v) {
        if (item instanceof FieldMap) collectByteSnaps(item, snaps);
      }
    } else if (v instanceof Map) {
      for (const item of (v as Map<string, unknown>).values()) {
        if (item instanceof FieldMap) collectByteSnaps(item, snaps);
      }
    } else if (v instanceof Variant) {
      // The variant itself cannot alias, but its payload can: snapshot the whole
      // field so a zero-copy payload shows up as a change.
      if (Buffer.isBuffer(v.value)) {
        snaps.push({ fm, key: k, orig: new Variant(v.tag, Buffer.from(v.value)) });
      } else if (v.value instanceof FieldMap) {
        collectByteSnaps(v.value, snaps);
      }
    }
  }
}

/** Compares two plain objects and returns the keys whose values differ. */
export function dictDiffs(before: Record<string, unknown>, after: Record<string, unknown>): string[] {
  const diffs: string[] = [];
  const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
  for (const k of Array.from(keys).sort()) {
    if (JSON.stringify(before[k]) !== JSON.stringify(after[k])) {
      diffs.push(k);
    }
  }
  return diffs;
}

/**
 * XOR-flips the input buffer and reports which FieldMap fields changed with it,
 * i.e. which alias it. Restores the original values before returning.
 */
export function detectZeroCopy(fm: FieldMap, buf: Buffer): string[] {
  if (buf.length === 0) return [];

  const snaps: FieldSnap[] = [];
  collectByteSnaps(fm, snaps);

  // XOR-flip
  for (let i = 0; i < buf.length; i++) buf[i] ^= 0xFF;

  const aliased: string[] = [];
  for (const { fm: targetFm, key, orig } of snaps) {
    const cur = targetFm._fields.get(key);
    if (Buffer.isBuffer(orig)) {
      if (Buffer.isBuffer(cur) && !cur.equals(orig)) aliased.push(key);
    } else if (cur instanceof Variant && Buffer.isBuffer(cur.value) && Buffer.isBuffer(orig.value)) {
      if (!cur.value.equals(orig.value)) aliased.push(key);
    }
  }

  // Restore
  for (const { fm: targetFm, key, orig } of snaps) {
    targetFm._fields.set(key, orig);
  }

  return aliased;
}

// run()

// The protocol revision this library speaks. The runner requires an exact
// match and refuses to start a worker reporting anything else.
const PROTOCOL_VERSION = 2;

// --- Serify decorators (requires experimentalDecorators, no reflect-metadata) ---

const _serifyKeys  = new WeakMap<object, string[]>();
const _serifyKeyMap = new WeakMap<object, Map<string, string>>();
const _serifyModels = new WeakMap<object, Map<string, { new (): any }>>();
const _serifyClass = new WeakMap<Function, { keys: string[] }>();

export namespace Serify {
  /** Options for @Serify.field() */
  export interface FieldOptions {
    /** Override the schema key name (default: property name) */
    rename?: string;
    /**
     * The model class behind a `struct`, `list<struct>`, `optional<struct>` or
     * `map<K,struct>` field.
     *
     * Only the way *back* needs it. Going out, a nested model is recognised by
     * its `toFieldMap` method and no declaration is required; coming in, all
     * that arrives is a FieldMap, and TypeScript has erased the property's type
     * by then — the same erasure that forces @Serify.sum to list its arms.
     * Without this the property would be handed a raw FieldMap.
     */
    model?: { new (): any };
  }

  /**
   * Class decorator: registers the class as a Serify model.
   */
  export function Model(): ClassDecorator {
    return function (target: any) {
      const proto = target.prototype;
      const keys = _serifyKeys.get(proto) ?? [];
      _serifyClass.set(target, { keys });
    };
  }

  /**
   * Property decorator: marks a property as a Serify field.
   */
  export function field(opts?: FieldOptions): PropertyDecorator {
    return function (target: Object, propertyKey: string | symbol) {
      const key = typeof propertyKey === 'string' ? propertyKey : String(propertyKey);
      const serKey = opts?.rename ?? key;

      // Track field list on prototype
      let keys = _serifyKeys.get(target);
      if (!keys) { keys = []; _serifyKeys.set(target, keys); }
      if (!keys.includes(key)) keys.push(key);

      // Track rename map on prototype
      let keyMap = _serifyKeyMap.get(target);
      if (!keyMap) { keyMap = new Map(); _serifyKeyMap.set(target, keyMap); }
      if (serKey !== key) keyMap.set(key, serKey);

      // Track the nested model class, if one was named
      if (opts?.model) {
        let models = _serifyModels.get(target);
        if (!models) { models = new Map(); _serifyModels.set(target, models); }
        models.set(key, opts.model);
      }
    };
  }

  // ── sum ─────────────────────────────────────────────────────────────
  //
  // Every other serify binding reads the arms off the language's own sum type.
  // TypeScript cannot: a union type is erased before the code runs — even
  // emitDecoratorMetadata reports nothing for `Silent | Sms` — so the arms are
  // the one thing that has to be named at runtime, and @Serify.sum is the
  // whole of it.
  //
  // Each arm is a plain class; its own properties are the payload, and the arity
  // rule is the same one every serify binding uses:
  //
  //     0 properties -> a unit variant, no payload
  //     1 property   -> that property's value is the payload
  //     N properties -> the payload is a struct holding the N properties
  //
  // Constructor parameters must have defaults, because the arity is read off a
  // freshly constructed arm.

  type Ctor = { new (...args: any[]): any };

  const _serifyArms = new WeakMap<object, Map<string, Ctor[]>>();

  /** Schema tag for an arm: its class name in snake_case. */
  function armTag(arm: Ctor): string {
    return arm.name.replace(/(?!^)[A-Z]/g, (c) => '_' + c).toLowerCase();
  }

  /** An arm's payload property names, in declaration order. */
  function armProps(arm: Ctor): string[] {
    try {
      return Object.keys(new arm());
    } catch (e) {
      throw new Error(
        `serify: cannot inspect sum arm ${arm.name} — its constructor parameters ` +
        `need defaults so the arm can be constructed empty (${e})`);
    }
  }

  function toVariant(arms: Ctor[], key: string, val: any): Variant {
    const arm = arms.find((a) => val instanceof a);
    if (!arm) {
      throw new Error(
        `${key}: ${val?.constructor?.name ?? typeof val} is not one of ` +
        arms.map((a) => a.name).join(', '));
    }
    const props = armProps(arm);
    if (props.length === 0) return new Variant(armTag(arm), null);
    if (props.length === 1) {
      const payload = val[props[0]];
      // A single payload that is itself a model travels as a struct.
      return new Variant(armTag(arm),
        isModel(payload) ? toFieldMap(payload) : payload);
    }
    const payload = new FieldMap();                       // N properties -> a struct
    for (const p of props) setFieldMapValue(payload, defaultKey(p), val[p]);
    return new Variant(armTag(arm), payload);
  }

  function fromVariant(arms: Ctor[], v: Variant): any {
    const arm = arms.find((a) => armTag(a) === v.tag);
    if (!arm) {
      throw new Error(
        `unknown variant "${v.tag}" (declared: ${arms.map(armTag).join(', ')})`);
    }
    const props = armProps(arm);
    if (props.length === 0) return new arm();
    if (props.length === 1) return new arm(v.value);
    if (!(v.value instanceof FieldMap)) {
      throw new Error(`variant "${v.tag}" needs a struct payload`);
    }
    return new arm(...props.map((p) => (v.value as FieldMap)._fields.get(defaultKey(p))));
  }

  /**
   * Property decorator: marks a property as a `sum`, naming its arms.
   *
   * TypeScript erases the union type, so unlike every other serify binding the
   * arms cannot be discovered and must be listed here — in the case file's
   * declaration order.
   */
  export function sum(arms: Ctor[], opts?: FieldOptions): PropertyDecorator {
    return function (target: Object, propertyKey: string | symbol) {
      const key = typeof propertyKey === 'string' ? propertyKey : String(propertyKey);
      field(opts)(target, propertyKey);
      let map = _serifyArms.get(target);
      if (!map) { map = new Map(); _serifyArms.set(target, map); }
      map.set(key, arms);
    };
  }

  /** camelCase property name to its snake_case schema key. */
  function defaultKey(prop: string): string {
    return prop.replace(/(?!^)[A-Z]/g, (c) => '_' + c).toLowerCase();
  }

  /** Retrieve the serify key for a property. */
  function getSerKey(proto: object, prop: string): string {
    const keyMap = _serifyKeyMap.get(proto);
    return keyMap?.get(prop) ?? prop;
  }

  function getKeys(proto: object): string[] {
    return _serifyKeys.get(proto) ?? [];
  }

  /** Convert a model instance to a FieldMap. */
  export function toFieldMap(instance: any): FieldMap {
    const fm = new FieldMap();
    const proto = Object.getPrototypeOf(instance);
    for (const prop of getKeys(proto)) {
      const serKey = getSerKey(proto, prop);
      const arms = _serifyArms.get(proto)?.get(prop);
      if (arms) fm._fields.set(serKey, toVariant(arms, serKey, (instance as any)[prop]));
      else setFieldMapValue(fm, serKey, (instance as any)[prop]);
    }
    return fm;
  }

  /**
   * Is this a class the worker registered with @Serify.Model()?
   *
   * This is how a nested struct is recognised on the way out. The check used to
   * be `typeof val.toFieldMap === 'function'`, which no model has ever
   * satisfied — the binding exposes toFieldMap as a namespace function, not a
   * method — so every one of those branches was unreachable and a nested model
   * fell through to `setString(key, String(val))`, arriving as the string
   * "[object Object]". Nothing caught it because no example had a nested struct
   * until customer.
   */
  function isModel(val: any): boolean {
    return val !== null && typeof val === 'object'
      && _serifyClass.has(val.constructor as Function);
  }

  /** Populate a model instance from a FieldMap. */
  export function fromFieldMap<T extends { new(): InstanceType<T> }>(
    ctor: T, fm: FieldMap
  ): InstanceType<T> {
    const instance = new ctor();
    const proto = Object.getPrototypeOf(instance);
    for (const prop of getKeys(proto)) {
      const serKey = getSerKey(proto, prop);
      if (fm._fields.has(serKey)) {
        const val = fm._fields.get(serKey);
        const arms = _serifyArms.get(proto)?.get(prop);
        const model = _serifyModels.get(proto)?.get(prop);
        (instance as any)[prop] =
          arms && val instanceof Variant ? fromVariant(arms, val)
            : model ? reviveModel(model, val)
              : val;
      }
    }
    return instance;
  }

  /**
   * Rebuild the declared model class out of whatever shape the field arrived
   * in: a struct is one FieldMap, a list<struct> an array of them, a
   * map<K,struct> a Map of them, and an absent optional<struct> a null.
   */
  function reviveModel(model: { new (): any }, val: any): any {
    if (val === null || val === undefined) return null;
    if (val instanceof FieldMap) return fromFieldMap(model as any, val);
    if (Array.isArray(val)) return val.map((x) => reviveModel(model, x));
    if (val instanceof Map) {
      return new Map([...val].map(([k, v]) => [k, reviveModel(model, v)]));
    }
    return val;
  }

  function setFieldMapValue(fm: FieldMap, key: string, val: any): void {
    // An absent property is not a field; an explicit null is — it is how an
    // `optional<T>` says "no value", and dropping it would silently omit the key.
    if (val === undefined) { return; }
    if (val === null) { fm._fields.set(key, null); return; }
    const t = typeof val;
    if (t === 'bigint') {
      fm.setI64(key, val);
    } else if (t === 'number') {
      // Store the number as-is, for the same reason the list branch below does:
      // the schema, not the value, decides what this field is. Classifying by
      // Number.isInteger meant an integral-valued float — 0.0, -0.0, and every
      // float32 boundary, all of which satisfy it — was stored as a bigint and
      // then handed to the float encoder, which cannot take one. Large ones did
      // not even get that far: they were rejected outright as unrepresentable
      // integers while being perfectly good floats.
      fm._fields.set(key, val);
    } else if (t === 'boolean') {
      fm.setBool(key, val);
    } else if (t === 'string') {
      fm.setString(key, val);
    } else if (Buffer.isBuffer(val)) {
      fm.setBytes(key, val);
    } else if (Array.isArray(val)) {
      // Store the list as-is: the schema, not the value, decides the element
      // type on the wire, and encodeList reads it from there. Guessing from
      // val[0] meant an empty list — and any list of bigints, booleans or
      // Buffers — was stored as though its elements were strings.
      fm._fields.set(key, val.map(x => (isModel(x) ? toFieldMap(x) : x)));
    } else if (val instanceof FieldMap) {
      fm.setStruct(key, val);
    } else if (val instanceof Map) {
      // Convert model values the way the list branch above does. Storing the
      // Map as-is left a map<K,struct> holding model instances the encoder has
      // no idea what to do with — the list case was handled and this one was
      // not, purely because no example had a map of structs until customer.
      fm.setMap(key, new Map([...val].map(([k, v]) => [k, isModel(v) ? toFieldMap(v) : v])));
    } else if (val instanceof Variant) {
      fm._fields.set(key, val);
    } else if (isModel(val)) {
      fm.setStruct(key, toFieldMap(val));
    } else {
      fm.setString(key, String(val));
    }
  }
}

export type SerializeFn   = (fm: FieldMap) => Buffer;
export type DeserializeFn = (data: Buffer) => FieldMap;

export type FormatPair = { serialize: SerializeFn; deserialize?: DeserializeFn };

/** A format whose functions speak the model `M` and never see a FieldMap. */
export type ModelFormatPair<M> = {
  serialize: (model: M) => Buffer;
  deserialize?: (data: Buffer) => M;
};

/** A model class: default-constructible, which is what `fromFieldMap` needs. */
export type ModelCtor<M> = { new (): M };

/**
 * One data type: a model, and the formats whose functions speak it. serify
 * converts FieldMap <-> model on the way in and out.
 *
 *   type('ledger', LedgerEntry, { binary: {
 *     serialize: (e) => e.marshal(),
 *     deserialize: (d) => LedgerEntry.unmarshal(d) } })
 */
export type TypeEntry<M> = { model: ModelCtor<M>; formats: Record<string, ModelFormatPair<M>> };

/**
 * A registered type is either a `TypeEntry` carrying a model, or the plain
 * format -> FormatPair record that workers used before models — which is what a
 * type with no natural class needs, the audit fixtures being the case in point.
 */
export type Registered = TypeEntry<any> | Record<string, FormatPair>;
export type Suite = Record<string, Registered>;

/** Build a `TypeEntry`; the helper exists so `M` is inferred from the model. */
export function type<M>(
  model: ModelCtor<M>,
  formats: Record<string, ModelFormatPair<M>>,
): TypeEntry<M> {
  return { model, formats };
}

function isTypeEntry(entry: Registered): entry is TypeEntry<any> {
  return typeof (entry as TypeEntry<any>).model === 'function'
    && typeof (entry as TypeEntry<any>).formats === 'object';
}

/**
 * Look one (type, format) up across both registration spellings.
 *
 * Exported so it can be tested without stdin: an unresolved (type, format) is
 * reported SKIPPED, so a shape this fails to understand yields a *green*
 * conformance run made entirely of SKIPs, indistinguishable from a worker that
 * honestly does not implement the type.
 */
export function resolveRegistered(
  suite: Suite,
  typeName: string,
  formatName: string,
): FormatPair | undefined {
  const entry = suite[typeName];
  if (entry === undefined) return undefined;

  if (!isTypeEntry(entry)) return (entry as Record<string, FormatPair>)[formatName];

  const fmt = entry.formats[formatName];
  if (fmt === undefined) return undefined;

  const { model } = entry;
  const { serialize, deserialize } = fmt;
  return {
    serialize: (fm: FieldMap) => serialize(Serify.fromFieldMap(model as any, fm)),
    deserialize: deserialize
      ? (data: Buffer) => Serify.toFieldMap(deserialize(data))
      : undefined,
  };
}

/** Single-type worker: handles whatever type/format the runner asks for. */
export function run(serialize: SerializeFn, deserialize: DeserializeFn): void {
  runSuite({}, () => ({ serialize, deserialize }));
}

/**
 * Multi-type worker. A (type, format) that is not registered is reported to the
 * runner as SKIPPED rather than failing the run.
 */
export function runSuite(
  suite: Suite,
  resolve: (type: string, format: string) => FormatPair | undefined =
    (t, f) => resolveRegistered(suite, t, f),
): void {
  let schema: SchemaField[] = [];
  let auditEnabled = false;
  let serialize: SerializeFn | undefined;
  let deserialize: DeserializeFn | undefined;

  const emit = (obj: unknown) => process.stdout.write(JSON.stringify(obj) + '\n');

  const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });

  rl.on('line', (line: string) => {
    line = line.trim();
    if (!line) return;
    let msg: Record<string, unknown>;
    try { msg = JSON.parse(line); } catch { return; }

    const op = msg['op'] as string;
    const id = (msg['id'] as string) ?? '';

    switch (op) {
      case 'ping':
        // Health check: report liveness and the protocol revision this
        // library speaks. Binds nothing.
        emit({ op: 'ping', status: 'OK', protocol_version: PROTOCOL_VERSION });
        break;

      case 'bind': {
        schema = parseSchemaFields((msg['schema'] as unknown[]) ?? []);
        auditEnabled = (msg['audit'] as boolean) ?? false;
        const pair = resolve((msg['type'] as string) ?? '', (msg['format'] as string) ?? '');
        if (!pair) {
          serialize = undefined;
          deserialize = undefined;
          emit({ op: 'bind', status: 'SKIPPED' });
          break;
        }
        serialize = pair.serialize;
        deserialize = pair.deserialize;
        emit({ op: 'bind' });
        break;
      }

      case 'serialize':
        try {
          if (!serialize) {
            if (deserialize) {
              emit({ id, op: 'serialize', status: 'SKIPPED', reason: 'direction not registered' });
            } else {
              emit({ id, op: 'serialize', status: 'ERROR', error: 'no serializer configured (call bind first)' });
            }
            break;
          }
          const fm = decodeFieldMap(msg['data'] as Record<string, unknown>, schema);

          const before = auditEnabled ? encodeFieldMap(fm, schema) : null;

          const buf = serialize!(fm);
          const hex = buf.toString('hex');

          const audit: Record<string, unknown> = {};
          if (auditEnabled && before) {
            // Mutation: compare FieldMap before/after serialization.
            const after = encodeFieldMap(fm, schema);
            const diffs = dictDiffs(before, after);
            if (diffs.length > 0) audit['mutations'] = diffs;

            // Output zero-copy: does returned buffer alias model fields?
            if (buf.length > 0) {
              const beforeClone = encodeFieldMap(fm, schema);
              for (let i = 0; i < buf.length; i++) buf[i] ^= 0xFF;
              const afterFlip = encodeFieldMap(fm, schema);
              for (let i = 0; i < buf.length; i++) buf[i] ^= 0xFF; // restore
              const ozc = dictDiffs(beforeClone, afterFlip);
              if (ozc.length > 0) audit['output_zero_copy_fields'] = ozc;
            }

            // Stability: serialize again, compare output.
            try {
              const b2 = serialize!(fm);
              if (hex !== b2.toString('hex')) {
                audit['stable'] = false;
              }
            } catch {
              audit['stable'] = false;
            }
          }

          const resp: Record<string, unknown> = { id, op: 'serialize', status: 'OK', hex };
          if (Object.keys(audit).length > 0) resp['audit'] = audit;
          emit(resp);
        } catch (e) {
          emit({ id, op: 'serialize', status: 'ERROR', error: String(e) });
        }
        break;

      case 'deserialize':
        try {
          if (!deserialize) {
            if (serialize) {
              emit({ id, op: 'deserialize', status: 'SKIPPED', reason: 'direction not registered' });
            } else {
              emit({ id, op: 'deserialize', status: 'ERROR', error: 'no deserializer configured (call bind first)' });
            }
            break;
          }
          const buf = Buffer.from(msg['hex'] as string, 'hex');
          const bufSnapshot = auditEnabled ? Buffer.from(buf) : null;

          const fm = deserialize!(buf);

          const audit: Record<string, unknown> = {};
          if (auditEnabled) {
            // Input-buffer mutation.
            if (bufSnapshot && !buf.equals(bufSnapshot)) {
              audit['input_mutated'] = true;
            }

            // Deserialize stability: re-deserialize from a fresh clone.
            if (bufSnapshot) {
              try {
                const fm2 = deserialize!(Buffer.from(bufSnapshot));
                const diffs = dictDiffs(
                  encodeFieldMap(fm, schema),
                  encodeFieldMap(fm2, schema),
                );
                if (diffs.length > 0) audit['deser_stable'] = false;
              } catch {
                audit['deser_stable'] = false;
              }
            }

            // Zero-copy: XOR-flip buffer, check FieldMap, restore.
            const zc = detectZeroCopy(fm, buf);
            if (zc.length > 0) audit['zero_copy_fields'] = zc;
          }

          const data = encodeFieldMap(fm, schema);
          const resp: Record<string, unknown> = { id, op: 'deserialize', status: 'OK', data };
          if (Object.keys(audit).length > 0) resp['audit'] = audit;
          emit(resp);
        } catch (e) {
          emit({ id, op: 'deserialize', status: 'ERROR', error: String(e) });
        }
        break;

      case 'exit':
        process.exit(0);
    }
  });
}
