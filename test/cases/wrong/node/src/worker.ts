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
 * Node half of the `wrong` meta-test. Mirrors the Go worker's byte/JSON layout.
 */

import { FieldMap, runSuite } from '@chengxilo/serify';

const SELF_LANG = 'node';

function toUpperSelf(langs: string[]): string[] {
  return langs.map(s => s === SELF_LANG ? 'NODE' : s);
}

// --- binary ----------------------------------------------------------------

function binarySerialize(fm: FieldMap): Buffer {
  let langs = fm.getListString('langs');
  if (!fm.getBool('binary_serialize')) langs = toUpperSelf(langs);

  const buf = Buffer.alloc(4 + langs.reduce((n, s) => n + 4 + Buffer.byteLength(s), 4));
  let off = 0;
  buf.writeUInt8(fm.getBool('binary_serialize') ? 1 : 0, off++);
  buf.writeUInt8(fm.getBool('binary_deserialize') ? 1 : 0, off++);
  buf.writeUInt8(fm.getBool('json_serialize') ? 1 : 0, off++);
  buf.writeUInt8(fm.getBool('json_deserialize') ? 1 : 0, off++);
  buf.writeUInt32LE(langs.length, off); off += 4;
  for (const s of langs) {
    const b = Buffer.from(s, 'utf8');
    buf.writeUInt32LE(b.length, off); off += 4;
    b.copy(buf, off); off += b.length;
  }
  return buf.subarray(0, off);
}

function binaryDeserialize(data: Buffer): FieldMap {
  const fm = new FieldMap();
  const bs = data[0] !== 0, bd = data[1] !== 0, js = data[2] !== 0, jd = data[3] !== 0;
  let off = 4;
  const n = data.readUInt32LE(off); off += 4;
  const langs: string[] = [];
  for (let i = 0; i < n; i++) {
    const slen = data.readUInt32LE(off); off += 4;
    langs.push(data.toString('utf8', off, off + slen)); off += slen;
  }
  fm.setBool('binary_serialize', bs);
  fm.setBool('binary_deserialize', bd);
  fm.setBool('json_serialize', js);
  fm.setBool('json_deserialize', jd);
  fm.setListString('langs', bd ? langs : toUpperSelf(langs));
  return fm;
}

// --- json ------------------------------------------------------------------

function jsonSerialize(fm: FieldMap): Buffer {
  const d: Record<string, unknown> = {
    binary_serialize: fm.getBool('binary_serialize'),
    binary_deserialize: fm.getBool('binary_deserialize'),
    json_serialize: fm.getBool('json_serialize'),
    json_deserialize: fm.getBool('json_deserialize'),
    langs: fm.getListString('langs'),
  };
  if (!d.json_serialize) d.langs = toUpperSelf(d.langs as string[]);
  return Buffer.from(JSON.stringify(d), 'utf8');
}

function jsonDeserialize(data: Buffer): FieldMap {
  const d = JSON.parse(data.toString('utf8'));
  const fm = new FieldMap();
  fm.setBool('binary_serialize', d.binary_serialize);
  fm.setBool('binary_deserialize', d.binary_deserialize);
  fm.setBool('json_serialize', d.json_serialize);
  fm.setBool('json_deserialize', d.json_deserialize);
  fm.setListString('langs', d.json_deserialize ? d.langs : toUpperSelf(d.langs));
  return fm;
}

// --- fault formats ---------------------------------------------------------

function errSer(_fm: FieldMap): Buffer { throw new Error('injected serialize error'); }
function errDeser(_data: Buffer): FieldMap { throw new Error('injected deserialize error'); }
function hangSer(fm: FieldMap): Buffer {
  const start = Date.now();
  while (Date.now() - start < 3000) { /* spin */ }
  return binarySerialize(fm);
}
function crashSer(_fm: FieldMap): Buffer { process.exit(3); }

runSuite({
  wrong: {
    binary: { serialize: binarySerialize, deserialize: binaryDeserialize },
    json: { serialize: jsonSerialize, deserialize: jsonDeserialize },
    err_ser: { serialize: errSer, deserialize: binaryDeserialize },
    err_deser: { serialize: binarySerialize, deserialize: errDeser },
    hang: { serialize: hangSer, deserialize: binaryDeserialize },
    crash: { serialize: crashSer, deserialize: binaryDeserialize },
  },
});
