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

// Audit helpers, kept out of workerlib.ts because that file is the package's
// `main` entry. detectZeroCopy mutates its buffer argument in place with no
// synchronization, which is safe only because the NDJSON loop that calls it is
// sequential.

import { FieldMap, Variant } from './workerlib';

type FieldSnap = { fm: FieldMap; key: string; orig: Buffer | Variant };

/** Recursively walks a FieldMap and collects a snapshot of every Buffer value. */
function collectByteSnaps(fm: FieldMap, snaps: FieldSnap[]): void {
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

  for (const { fm: targetFm, key, orig } of snaps) {
    targetFm._fields.set(key, orig);
  }

  return aliased;
}
