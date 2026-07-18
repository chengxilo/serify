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

namespace Serify;

class FieldMap
{
    private array $fields = [];

    public function raw(): array
    {
        return $this->fields;
    }

    /**
     * Store an arbitrary value under $key. raw() returns the backing array by
     * value, so writes to it are lost — use this when no typed setter fits.
     */
    public function setRaw(string $key, $v): void
    {
        $this->fields[$key] = $v;
    }

    public function has(string $key): bool
    {
        return array_key_exists($key, $this->fields);
    }

    // ── Getters ──────────────────────────────────────────────────────────

    public function getU8(string $k): int
    {
        return (int) $this->fields[$k];
    }
    public function getU16(string $k): int
    {
        return (int) $this->fields[$k];
    }
    public function getU32(string $k): int
    {
        return (int) $this->fields[$k];
    }
    /** @return string Decimal string (may exceed PHP_INT_MAX). */
    public function getU64(string $k): string
    {
        return (string) $this->fields[$k];
    }
    /** @return string Decimal string (may exceed PHP_INT_MAX). */
    public function getU128(string $k): string
    {
        return (string) $this->fields[$k];
    }
    public function getI8(string $k): int
    {
        return (int) $this->fields[$k];
    }
    public function getI16(string $k): int
    {
        return (int) $this->fields[$k];
    }
    public function getI32(string $k): int
    {
        return (int) $this->fields[$k];
    }
    /** @return string Decimal string (may be negative, exceeds PHP_INT_MIN). */
    public function getI64(string $k): string
    {
        return (string) $this->fields[$k];
    }
    /** @return string Decimal string (may be negative, exceeds PHP_INT_MIN). */
    public function getI128(string $k): string
    {
        return (string) $this->fields[$k];
    }
    public function getF32(string $k): float
    {
        return (float) $this->fields[$k];
    }
    public function getF64(string $k): float
    {
        return (float) $this->fields[$k];
    }
    public function getBool(string $k): bool
    {
        return (bool) $this->fields[$k];
    }
    public function getString(string $k): string
    {
        return (string) $this->fields[$k];
    }
    public function getBytes(string $k): string
    {
        return (string) $this->fields[$k];
    }
    public function getListString(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    public function getOptionalString(string $k): ?string
    {
        $v = $this->fields[$k] ?? null;
        return $v === null ? null : (string) $v;
    }
    public function getStruct(string $k): ?FieldMap
    {
        return ($this->fields[$k] ?? null) instanceof FieldMap ? $this->fields[$k] : null;
    }
    public function getListStruct(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    public function getOptionalStruct(string $k): ?FieldMap
    {
        $v = $this->fields[$k] ?? null;
        return $v instanceof FieldMap ? $v : null;
    }
    public function getListU8(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    public function getListU32(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    /** @return string[] Decimal strings. */
    public function getListU64(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    /** @return string[] Decimal strings. */
    public function getListU128(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    public function getListI32(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    /** @return string[] Decimal strings. */
    public function getListI64(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    /** @return string[] Decimal strings. */
    public function getListI128(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    public function getListF32(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    public function getListBool(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }
    public function getMap(string $k): array
    {
        return (array) ($this->fields[$k] ?? []);
    }

    // ── Setters ──────────────────────────────────────────────────────────

    public function setU8(string $k, int $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setU16(string $k, int $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setU32(string $k, int $v): void
    {
        $this->fields[$k] = $v;
    }
    /** @param string|int $v Decimal string (preferred) or int for small values. */
    public function setU64(string $k, $v): void
    {
        $this->fields[$k] = (string) $v;
    }
    /** @param string|int $v Decimal string (preferred) or int for small values. */
    public function setU128(string $k, $v): void
    {
        $this->fields[$k] = (string) $v;
    }
    public function setI8(string $k, int $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setI16(string $k, int $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setI32(string $k, int $v): void
    {
        $this->fields[$k] = $v;
    }
    /** @param string|int $v Decimal string (preferred) or int for small values. */
    public function setI64(string $k, $v): void
    {
        $this->fields[$k] = (string) $v;
    }
    /** @param string|int $v Decimal string (preferred) or int for small values. */
    public function setI128(string $k, $v): void
    {
        $this->fields[$k] = (string) $v;
    }
    public function setF32(string $k, float $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setF64(string $k, float $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setBool(string $k, bool $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setString(string $k, string $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setBytes(string $k, string $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setListString(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setOptionalString(string $k, ?string $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setStruct(string $k, FieldMap $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setListStruct(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setOptionalStruct(string $k, ?FieldMap $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setListU8(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setListU32(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }
    /** @param (string|int)[] $v */
    public function setListU64(string $k, array $v): void
    {
        $this->fields[$k] = array_map('strval', $v);
    }
    /** @param (string|int)[] $v */
    public function setListU128(string $k, array $v): void
    {
        $this->fields[$k] = array_map('strval', $v);
    }
    public function setListI32(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }
    /** @param (string|int)[] $v */
    public function setListI64(string $k, array $v): void
    {
        $this->fields[$k] = array_map('strval', $v);
    }
    /** @param (string|int)[] $v */
    public function setListI128(string $k, array $v): void
    {
        $this->fields[$k] = array_map('strval', $v);
    }
    public function setListF32(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setListBool(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }
    public function setMap(string $k, array $v): void
    {
        $this->fields[$k] = $v;
    }

    /** Store a oneof value: the active variant's tag and payload (null for a unit variant). */
    public function setVariant(string $k, string $tag, $v = null): void
    {
        $this->fields[$k] = new Variant($tag, $v);
    }

    /** Return the oneof value stored at $k. */
    public function getVariant(string $k): Variant
    {
        $v = $this->fields[$k] ?? null;
        if (!$v instanceof Variant) {
            throw new \RuntimeException("field \"$k\" is not a oneof variant");
        }
        return $v;
    }
}

/**
 * One arm of a oneof: a tag and its decoded payload (null for a unit variant).
 * A oneof field stores a Variant.
 */
final class Variant
{
    public function __construct(public readonly string $tag, public readonly mixed $value = null)
    {
    }
}
