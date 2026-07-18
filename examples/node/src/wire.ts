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
 * Byte-level primitives shared by the models in this worker.
 *
 * Go is the --ref language and owns the layout these reproduce; see the comment
 * at the top of examples/go/wire.go.
 */

export function lenPrefixed(body: Buffer): Buffer {
  const len = Buffer.alloc(4);
  len.writeUInt32LE(body.length, 0);
  return Buffer.concat([len, body]);
}

export function lenPrefixedStr(s: string): Buffer {
  return lenPrefixed(Buffer.from(s, 'utf8'));
}
