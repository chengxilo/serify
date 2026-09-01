<?php
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
 * Framing: a u32 little-endian byte length, then that many bytes.
 *
 * Outside the conformance suite on purpose. serify tests the contents of a
 * message; the frame header is in none of the cases, so a follower may read its
 * socket however it likes as long as it agrees on what is inside.
 */

declare(strict_types=1);

/** Caps a single message, so a bad length prefix is not a 1 MiB-plus read. */
const MAX_FRAME_LEN = 1 << 20;

/** @param resource $stream */
function writeFrame($stream, string $payload): void
{
    if (strlen($payload) > MAX_FRAME_LEN) {
        throw new RuntimeException('message of ' . strlen($payload) . ' bytes exceeds the limit');
    }
    fwrite($stream, pack('V', strlen($payload)) . $payload);
    fflush($stream);
}

/** @param resource $stream */
function readFrame($stream): string
{
    $header = readExact($stream, 4);
    $n      = unpack('V', $header)[1];
    if ($n > MAX_FRAME_LEN) {
        throw new RuntimeException("frame header claims $n bytes, over the limit");
    }
    return $n === 0 ? '' : readExact($stream, $n);
}

/**
 * fread stops at whatever the socket has ready, which for a frame split across
 * packets is less than was asked for — hence the loop.
 *
 * @param resource $stream
 */
function readExact($stream, int $n): string
{
    $buf = '';
    while (strlen($buf) < $n) {
        $chunk = fread($stream, $n - strlen($buf));
        if ($chunk === false || $chunk === '') {
            throw new RuntimeException('connection closed mid-frame');
        }
        $buf .= $chunk;
    }
    return $buf;
}
