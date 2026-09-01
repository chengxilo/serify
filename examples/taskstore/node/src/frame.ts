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
 * Framing: a u32 little-endian byte length, then that many bytes.
 *
 * Outside the conformance suite on purpose. serify tests the contents of a
 * message; the frame header is in none of the cases, so a follower may read its
 * socket however it likes as long as it agrees on what is inside.
 */

import { Socket } from 'net';

/** Caps a single message, so a bad length prefix is not a 4 GiB allocation. */
export const MAX_FRAME_LEN = 1 << 20;

export function writeFrame(sock: Socket, payload: Buffer): void {
  if (payload.length > MAX_FRAME_LEN) {
    throw new Error(`message of ${payload.length} bytes exceeds the ${MAX_FRAME_LEN} limit`);
  }
  const header = Buffer.alloc(4);
  header.writeUInt32LE(payload.length, 0);
  sock.write(Buffer.concat([header, payload]));
}

/**
 * Resolves with the first complete frame on the socket.
 *
 * Node's socket is a stream of arbitrary chunks, so this buffers until the
 * length prefix says it has enough — the one place where the asynchronous
 * runtime shows through. Everything below the frame is the same bytes in every
 * language.
 */
export function readFrame(sock: Socket): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    let buf = Buffer.alloc(0);

    const onData = (chunk: Buffer): void => {
      buf = Buffer.concat([buf, chunk]);
      if (buf.length < 4) return;
      const n = buf.readUInt32LE(0);
      if (n > MAX_FRAME_LEN) {
        cleanup();
        reject(new Error(`frame header claims ${n} bytes, over the ${MAX_FRAME_LEN} limit`));
        return;
      }
      if (buf.length < 4 + n) return;
      cleanup();
      resolve(buf.subarray(4, 4 + n));
    };
    const onEnd = (): void => {
      cleanup();
      reject(new Error('connection closed before a complete frame arrived'));
    };
    const onError = (e: Error): void => {
      cleanup();
      reject(e);
    };
    const cleanup = (): void => {
      sock.off('data', onData);
      sock.off('end', onEnd);
      sock.off('error', onError);
    };

    sock.on('data', onData);
    sock.on('end', onEnd);
    sock.on('error', onError);
  });
}
