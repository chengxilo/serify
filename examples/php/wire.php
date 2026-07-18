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
 * Byte-level primitives shared by the models in this worker.
 *
 * PHP's int is only 64-bit, so it cannot hold an int128 — and even a u64
 * overflows it — which is why FieldMap carries every 64/128-bit value as a
 * decimal string and these helpers convert through ext-gmp.
 *
 * Go is the --ref language and owns the layout these reproduce; see the comment
 * at the top of examples/go/wire.go.
 *
 * Requires ext-gmp (apt install php-gmp).
 */

declare(strict_types=1);

/**
 * Encode a decimal string as $numBytes little-endian bytes.
 *
 * Reducing mod 2^bits maps a negative onto its residue class, which is exactly
 * two's complement, so this handles signed and unsigned alike.
 */
function encodeInt(string $decimal, int $numBytes): string
{
    $u = gmp_mod(gmp_init($decimal, 10), gmp_pow(2, $numBytes * 8));
    $le = gmp_export($u, 1, GMP_LSW_FIRST | GMP_LITTLE_ENDIAN);
    return str_pad($le, $numBytes, "\0", STR_PAD_RIGHT); // gmp_export drops leading zeros
}

/** Decode little-endian bytes as an unsigned decimal string. */
function decodeUnsigned(string $bytes): string
{
    return gmp_strval(gmp_import($bytes, 1, GMP_LSW_FIRST | GMP_LITTLE_ENDIAN));
}

/** Decode little-endian two's-complement bytes as a signed decimal string. */
function decodeSigned(string $bytes): string
{
    $bits = strlen($bytes) * 8;
    $n = gmp_import($bytes, 1, GMP_LSW_FIRST | GMP_LITTLE_ENDIAN);
    if (gmp_testbit($n, $bits - 1)) { // top bit set => negative
        $n = gmp_sub($n, gmp_pow(2, $bits));
    }
    return gmp_strval($n);
}

/** A u32 byte length followed by the bytes themselves. */
function lenPrefixed(string $body): string
{
    return pack('V', strlen($body)) . $body;
}

/** Read one length-prefixed string at $off, advancing it past the whole field. */
function readLenPrefixed(string $data, int &$off): string
{
    $n = unpack('V', substr($data, $off, 4))[1];
    $off += 4 + $n;
    return substr($data, $off - $n, $n);
}
