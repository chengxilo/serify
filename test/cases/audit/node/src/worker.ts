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

/*
 * Node audit worker: 5 cross-language audit formats.
 */

import { FieldMap, runSuite } from '@chengxilo/serify';

let unstableCounter = 0;
let deserUnstableCounter = 0;

function marshalAudit(fm: FieldMap): Buffer {
  const val = fm.getU32('value');
  const tag = Buffer.from(fm.getString('tag'), 'utf8');
  const payload = fm.getBytes('payload');
  const tags = fm.getListString('tags');

  let size = 4 + 1 + tag.length + 4 + payload.length + 1;
  for (const t of tags) size += 1 + Buffer.byteLength(t);
  const buf = Buffer.alloc(size);
  let off = 0;
  buf.writeUInt32LE(val, off); off += 4;
  buf.writeUInt8(tag.length, off++); tag.copy(buf, off); off += tag.length;
  buf.writeUInt32LE(payload.length, off); off += 4;
  payload.copy(buf, off); off += payload.length;
  buf.writeUInt8(tags.length, off++);
  for (const t of tags) {
    const b = Buffer.from(t, 'utf8');
    buf.writeUInt8(b.length, off++); b.copy(buf, off); off += b.length;
  }
  return buf;
}

function unmarshalAudit(data: Buffer, copyPayload: boolean): FieldMap {
  const fm = new FieldMap();
  let off = 0;
  fm.setU32('value', data.readUInt32LE(off)); off += 4;
  const tlen = data.readUInt8(off++);
  fm.setString('tag', data.toString('utf8', off, off + tlen)); off += tlen;
  const plen = data.readUInt32LE(off); off += 4;
  fm.setBytes('payload', copyPayload ? Buffer.from(data.subarray(off, off + plen)) : data.subarray(off, off + plen));
  off += plen;
  const tcount = data.readUInt8(off++);
  const tags: string[] = [];
  for (let i = 0; i < tcount; i++) {
    const tl = data.readUInt8(off++);
    tags.push(data.toString('utf8', off, off + tl)); off += tl;
  }
  fm.setListString('tags', tags);
  return fm;
}

function cleanSer(fm: FieldMap): Buffer { return marshalAudit(fm); }
function cleanDeser(data: Buffer): FieldMap { return unmarshalAudit(data, true); }

function mutatingSer(fm: FieldMap): Buffer {
  const buf = marshalAudit(fm);
  fm.setU32('value', 0);
  return buf;
}

function unstableSer(fm: FieldMap): Buffer {
  const buf = marshalAudit(fm);
  const out = Buffer.alloc(buf.length + 1);
  buf.copy(out);
  out.writeUInt8(unstableCounter++, buf.length);
  return out;
}

function deserUnstableDeser(data: Buffer): FieldMap {
  const fm = unmarshalAudit(data, true);
  if (deserUnstableCounter++ > 0) fm.setU32('value', fm.getU32('value') + 1);
  return fm;
}

function inputMutatingDeser(data: Buffer): FieldMap {
  const fm = unmarshalAudit(data, true);
  if (data.length > 0) data[0] ^= 0xFF;
  return fm;
}

runSuite({
  audit: {
    clean: { serialize: cleanSer, deserialize: cleanDeser },
    mutating: { serialize: mutatingSer, deserialize: cleanDeser },
    unstable: { serialize: unstableSer, deserialize: cleanDeser },
    'deser-unstable': { serialize: cleanSer, deserialize: deserUnstableDeser },
    'input-mutating': { serialize: cleanSer, deserialize: inputMutatingDeser },
  },
});
