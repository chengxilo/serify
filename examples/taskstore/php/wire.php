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
 * Byte-level primitives. Go owns the layout these reproduce; the conventions
 * are documented at the top of go/api/wire.go.
 *
 * PHP's int is signed 64-bit, so it cannot hold a full uint64 — the task ids in
 * this schema go to 2^64-1. Every 64-bit value therefore travels as a decimal
 * string here and through the FieldMap, and converts through ext-gmp.
 *
 * Requires ext-gmp (apt install php-gmp).
 */

declare(strict_types=1);

/** Declaration order of the `enum<low, normal, high>` in cases/task.yaml. */
const PRIORITIES = ['low', 'normal', 'high'];

/** Declaration order of the enum in cases/api_error.yaml. */
const ERROR_CODES = ['not_found', 'invalid', 'conflict'];

/** A cursor over one message's bytes. */
final class Reader
{
    private int $off = 0;

    public function __construct(private readonly string $data) {}

    private function take(int $n): string
    {
        if ($this->off + $n > strlen($this->data)) {
            throw new RuntimeException("truncated: want $n bytes at offset {$this->off}");
        }
        $chunk = substr($this->data, $this->off, $n);
        $this->off += $n;
        return $chunk;
    }

    public function u8(): int
    {
        return unpack('C', $this->take(1))[1];
    }

    public function u32(): int
    {
        return unpack('V', $this->take(4))[1];
    }

    /** A decimal string, because a uint64 does not fit PHP's signed int. */
    public function u64(): string
    {
        return decodeUnsigned($this->take(8));
    }

    public function i64(): string
    {
        return decodeSigned($this->take(8));
    }

    public function bool(): bool
    {
        return $this->u8() !== 0;
    }

    public function str(): string
    {
        return $this->take($this->u32());
    }

    /**
     * An enum arrives as an ordinal into a fixed list, so a value outside the
     * declared variants has no encoding and cannot be read back.
     *
     * @param list<string> $variants
     */
    public function enumOf(array $variants): string
    {
        $ord = $this->u8();
        if ($ord >= count($variants)) {
            throw new RuntimeException("enum ordinal $ord out of range");
        }
        return $variants[$ord];
    }

    /** @return list<string> */
    public function tags(): array
    {
        $n = $this->u32();
        $out = [];
        for ($i = 0; $i < $n; $i++) {
            $out[] = $this->str();
        }
        return $out;
    }

    public function optionalI64(): ?string
    {
        return $this->u8() === 0 ? null : $this->i64();
    }
}

/**
 * Encode a decimal string as $numBytes little-endian bytes.
 *
 * Reducing mod 2^bits maps a negative onto its residue class, which is exactly
 * two's complement, so this handles signed and unsigned alike.
 */
function encodeInt(string $decimal, int $numBytes): string
{
    $u  = gmp_mod(gmp_init($decimal, 10), gmp_pow(2, $numBytes * 8));
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
    $n    = gmp_import($bytes, 1, GMP_LSW_FIRST | GMP_LITTLE_ENDIAN);
    if (gmp_testbit($n, $bits - 1)) { // top bit set => negative
        $n = gmp_sub($n, gmp_pow(2, $bits));
    }
    return gmp_strval($n);
}

function putStr(string $s): string
{
    return pack('V', strlen($s)) . $s;
}

/** @param list<string> $variants */
function putEnum(array $variants, string $name): string
{
    $ord = array_search($name, $variants, true);
    if ($ord === false) {
        throw new RuntimeException("\"$name\" is not one of " . implode(', ', $variants));
    }
    return pack('C', $ord);
}

/** @param list<string> $tags */
function putTags(array $tags): string
{
    $out = pack('V', count($tags));
    foreach ($tags as $t) {
        $out .= putStr($t);
    }
    return $out;
}

function putOptionalI64(?string $v): string
{
    return $v === null ? "\x00" : "\x01" . encodeInt($v, 8);
}
