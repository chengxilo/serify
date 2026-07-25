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

/**
 * Worker — NDJSON protocol loop for the serify conformance harness.
 *
 * Usage:
 *   Worker::run(
 *       serialize: fn(FieldMap $fm): string => $yourSerializer($fm),
 *       deserialize: fn(string $bytes): FieldMap => $yourDeserializer($bytes),
 *   );
 */
class Worker
{
    /** @return array{0: callable, 1: callable}|null */
    private static function resolveFormat(array $suite, string $type, string $format): ?array
    {
        $formats = $suite[$type] ?? $suite['*'] ?? null;
        if (!is_array($formats)) {
            return null;
        }
        $pair = $formats[$format] ?? $formats['*'] ?? null;
        return is_array($pair) ? $pair : null;
    }

    /**
     * The protocol revision this library speaks. The runner requires an exact
     * match and refuses to start a worker reporting anything else.
     */
    private const PROTOCOL_VERSION = 2;

    /** Single-type worker: handles whatever type/format the runner asks for. */
    public static function run(callable $serialize, callable $deserialize, array $schema = []): void
    {
        self::runSuite(['*' => ['*' => [$serialize, $deserialize]]], $schema);
    }

    /**
     * Multi-type worker. $suite maps type name -> format name -> [serialize,
     * deserialize]. A (type, format) that is not registered is reported to the
     * runner as SKIPPED rather than failing the run. The '*' wildcard matches any
     * type/format and backs the single-type run() above.
     */
    public static function runSuite(array $suite, array $schema = []): void
    {
        $serialize = null;
        $deserialize = null;
        $auditEnabled = false;

        $stdin = fopen('php://stdin', 'r');
        if ($stdin === false) {
            return;
        }

        while (($line = fgets($stdin)) !== false) {
            $line = trim($line);
            if ($line === '') {
                continue;
            }

            $msg = json_decode($line, true);
            if (!is_array($msg)) {
                continue;
            }

            $op = $msg['op'] ?? '';
            $id = (string) ($msg['id'] ?? '');

            switch ($op) {
                case 'ping':
                    // Health check: report liveness and the protocol revision
                    // this library speaks. Binds nothing.
                    self::emit([
                        'op' => 'ping',
                        'status' => 'OK',
                        'protocol_version' => self::PROTOCOL_VERSION,
                    ]);
                    break;

                case 'bind':
                    $schema = self::parseSchemaFields($msg['schema'] ?? []);
                    $auditEnabled = (bool) ($msg['audit'] ?? false);
                    $pair = self::resolveFormat($suite, (string) ($msg['type'] ?? ''), (string) ($msg['format'] ?? ''));
                    if ($pair === null) {
                        $serialize = null;
                        $deserialize = null;
                        self::emit(['op' => 'bind', 'status' => 'SKIPPED']);
                        break;
                    }
                    [$serialize, $deserialize] = $pair;
                    self::emit(['op' => 'bind']);
                    break;

                case 'serialize':
                    try {
                        $fm = self::decodeFieldMap($msg['data'], $schema);

                        $before = $auditEnabled ? self::encodeFieldMap($fm, $schema) : null;

                        $bytes = $serialize($fm);
                        $hex = bin2hex($bytes);

                        $audit = [];
                        if ($auditEnabled && $before !== null) {
                            // Mutation: compare before/after
                            $after = self::encodeFieldMap($fm, $schema);
                            $diffs = self::dictDiffs($before, $after);
                            if (!empty($diffs)) {
                                $audit['mutations'] = $diffs;
                            }

                            // Output zero-copy is not detectable (or possible) in
                            // PHP: strings are copy-on-write values, so model
                            // fields can never mutably alias the output buffer.

                            // Stability: serialize again
                            try {
                                $bytes2 = $serialize($fm);
                                if ($hex !== bin2hex($bytes2)) {
                                    $audit['stable'] = false;
                                }
                            } catch (\Throwable $e) {
                                $audit['stable'] = false;
                            }
                        }

                        $resp = ['id' => $id, 'op' => 'serialize', 'status' => 'OK', 'hex' => $hex];
                        if (!empty($audit)) {
                            $resp['audit'] = $audit;
                        }
                        self::emit($resp);
                    } catch (\Throwable $e) {
                        self::emit(['id' => $id, 'op' => 'serialize', 'status' => 'ERROR', 'error' => $e->getMessage()]);
                    }
                    break;

                case 'deserialize':
                    try {
                        $bytes = hex2bin($msg['hex']);
                        $bufSnapshot = $auditEnabled ? $bytes : null;

                        $fm = $deserialize($bytes);
                        $data = self::encodeFieldMap($fm, $schema);

                        $audit = [];
                        if ($auditEnabled && $bufSnapshot !== null) {
                            // Input-buffer mutation
                            if ($bufSnapshot !== $bytes) {
                                $audit['input_mutated'] = true;
                            }

                            // Deser-stability: re-deserialize from a clone
                            try {
                                $bufClone = $bufSnapshot;
                                $fm2 = $deserialize($bufClone);
                                $data2 = self::encodeFieldMap($fm2, $schema);
                                if ($data2 !== $data) {
                                    $audit['deser_stable'] = false;
                                }
                            } catch (\Throwable $e) {
                                $audit['deser_stable'] = false;
                            }

                            // Zero-copy: XOR-flip, check, restore
                            $zc = self::detectZeroCopy($fm, $bytes);
                            if (!empty($zc)) {
                                $audit['zero_copy_fields'] = $zc;
                            }
                        }

                        $resp = ['id' => $id, 'op' => 'deserialize', 'status' => 'OK', 'data' => $data];
                        if (!empty($audit)) {
                            $resp['audit'] = $audit;
                        }
                        self::emit($resp);
                    } catch (\Throwable $e) {
                        self::emit(['id' => $id, 'op' => 'deserialize', 'status' => 'ERROR', 'error' => $e->getMessage()]);
                    }
                    break;

                case 'exit':
                    exit(0);
            }
        }
        fclose($stdin);
    }

    // ── Schema parsing ────────────────────────────────────────────────────

    private static function parseSchemaFields(array $arr): array
    {
        $result = [];
        foreach ($arr as $f) {
            $result[] = [
                'name' => (string) ($f['name'] ?? ''),
                'type' => (string) ($f['type'] ?? ''),
                'fields' => self::parseSchemaFields($f['fields'] ?? []),
                'variants' => self::parseSchemaVariants($f['variants'] ?? []),
                'tags' => (array) ($f['tags'] ?? []),
            ];
        }
        return $result;
    }

    /** One arm of a sum: a tag and its payload schema (null for a unit variant). */
    private static function parseSchemaVariants(?array $arr): array
    {
        $result = [];
        foreach ($arr ?? [] as $v) {
            $payload = $v['payload'] ?? null;
            $result[] = [
                'name' => (string) ($v['name'] ?? ''),
                // Reuse the field parser by wrapping the payload in a one-element array.
                'payload' => $payload === null ? null : self::parseSchemaFields([$payload])[0],
            ];
        }
        return $result;
    }

    private static function findSchemaVariant(array $sf, string $tag): array
    {
        foreach ($sf['variants'] ?? [] as $sv) {
            if ($sv['name'] === $tag) {
                return $sv;
            }
        }
        throw new \RuntimeException("unknown variant \"$tag\"");
    }

    // ── Decode ────────────────────────────────────────────────────────────

    public static function decodeFieldMap(array $data, array $schema): FieldMap
    {
        $fm = new FieldMap();
        foreach ($schema as $sf) {
            $name = $sf['name'];
            if (!array_key_exists($name, $data)) {
                continue;
            }
            self::decodeField($fm, $sf, $data[$name]);
        }
        return $fm;
    }

    private static function decodeField(FieldMap $fm, array $sf, $v): void
    {
        $name = $sf['name'];
        $type = $sf['type'];

        switch ($type) {
            case 'uint8':
            case 'uint16':
            case 'uint32':
                $fm->setU32($name, (int) $v);
                break;
            case 'uint64':
            case 'uint128':
                $fm->setU64($name, (string) $v);
                break;
            case 'int8':
            case 'int16':
            case 'int32':
                $fm->setI32($name, (int) $v);
                break;
            case 'int64':
            case 'int128':
                $fm->setI64($name, (string) $v);
                break;
            case 'float32':
                // PHP unpack: 'g' = little-endian float (the wire byte order).
                $b = hex2bin($v);
                $fm->setF32($name, unpack('g', $b)[1]);
                break;
            case 'float64':
                $b = hex2bin($v);
                $fm->setF64($name, unpack('e', $b)[1]);
                break;
            case 'bool':
                $fm->setBool($name, (bool) $v);
                break;
            case 'string':
                $fm->setString($name, (string) $v);
                break;
            case 'bytes':
                $fm->setBytes($name, hex2bin($v));
                break;
            case 'struct':
                $fm->setStruct($name, self::decodeFieldMap((array) $v, $sf['fields']));
                break;
            default:
                if (str_starts_with($type, 'list<')) {
                    $elem = substr($type, 5, -1);
                    self::decodeList($fm, $sf, $elem, (array) $v);
                } elseif (str_starts_with($type, 'optional<')) {
                    $elem = substr($type, 9, -1);
                    self::decodeOptional($fm, $sf, $elem, $v);
                } elseif (str_starts_with($type, 'array<')) {
                    // An array<T,N> is a list whose length the schema fixes, so
                    // it shares decodeList outright and adds only the length
                    // check. A separate representation is what pinned
                    // array<T,N> to integers cast through (int).
                    [$elem, $n] = self::splitArrayType($type);
                    self::decodeList($fm, $sf, $elem, (array) $v);
                    $got = count($fm->raw()[$name]);
                    if ($got !== $n) {
                        throw new \InvalidArgumentException("array $name: expected $n elements, got $got");
                    }
                } elseif (str_starts_with($type, 'enum<')) {
                    // enum<a,b,c>: the variant name travels as a string.
                    $fm->setString($name, (string) $v);
                } elseif (str_starts_with($type, 'sum<')) {
                    self::decodeSum($fm, $sf, (array) $v);
                } elseif (str_starts_with($type, 'map<')) {
                    [, $valType] = self::splitMapTypes($type);
                    $fm->setMap($name, self::decodeMap($valType, $sf['fields'], (array) $v));
                } else {
                    // Breaking out silently left the field absent from the
                    // FieldMap, which surfaces far downstream as a missing value
                    // rather than as "this library does not know that type".
                    throw new \InvalidArgumentException("unknown type \"$type\"");
                }
                break;
        }
    }

    /**
     * Decode a sum wire value {tag: payload} (payload null for a unit variant)
     * into a Variant, decoding the payload per the variant's own schema.
     */
    private static function decodeSum(FieldMap $fm, array $sf, array $v): void
    {
        if (count($v) !== 1) {
            throw new \RuntimeException('sum must name exactly one variant, got ' . count($v));
        }
        $tag = array_key_first($v);
        $sv = self::findSchemaVariant($sf, (string) $tag);
        if ($sv['payload'] === null) {
            $fm->setVariant($sf['name'], (string) $tag);
            return;
        }
        $tmp = new FieldMap();
        self::decodeField($tmp, $sv['payload'], $v[$tag]);
        $fm->setVariant($sf['name'], (string) $tag, $tmp->raw()[$sv['payload']['name']]);
    }

    /** Split "array<T,N>" into its element type and length. */
    private static function splitArrayType(string $type): array
    {
        $inner = substr($type, 6, -1);
        $comma = strrpos($inner, ',');
        return [trim(substr($inner, 0, $comma)), (int) trim(substr($inner, $comma + 1))];
    }

    /**
     * Element types a list may carry: every scalar, plus a nested struct.
     *
     * @var string[]
     */
    private const LIST_ELEMS = [
        'uint8', 'uint16', 'uint32', 'uint64', 'uint128',
        'int8', 'int16', 'int32', 'int64', 'int128',
        'float32', 'float64', 'bool', 'string', 'bytes',
        'struct',
    ];

    /**
     * Decode every element through decodeField, so a list supports exactly the
     * element types a bare field does. This used to carry its own switch, which
     * is why uint16/int8/int16/float64/bytes were declarable in a case file and
     * accepted by `serify validate`, but threw once a worker actually ran.
     */
    private static function decodeList(FieldMap $fm, array $sf, string $elem, array $arr): void
    {
        if (!in_array($elem, self::LIST_ELEMS, true)) {
            throw new \InvalidArgumentException("unsupported list element type \"$elem\"");
        }
        if ($elem === 'struct') {
            $fm->setListStruct($sf['name'], array_map(
                fn($x) => self::decodeFieldMap((array) $x, $sf['fields']),
                $arr
            ));
            return;
        }

        $elemSf = ['name' => 'e', 'type' => $elem, 'fields' => $sf['fields'] ?? []];
        $out = [];
        foreach ($arr as $i => $item) {
            $tmp = new FieldMap();
            self::decodeField($tmp, $elemSf, $item);
            $out[] = $tmp->raw()['e'];
        }
        $fm->setRaw($sf['name'], $out);
    }

    private static function decodeOptional(FieldMap $fm, array $sf, string $elem, $v): void
    {
        if ($v === null) {
            switch ($elem) {
                case 'string':
                    $fm->setOptionalString($sf['name'], null);
                    break;
                case 'struct':
                    $fm->setOptionalStruct($sf['name'], null);
                    break;
                default:
                    $fm->setRaw($sf['name'], null);
                    break;
            }
            return;
        }
        switch ($elem) {
            case 'string':
                $fm->setOptionalString($sf['name'], (string) $v);
                break;
            case 'struct':
                $fm->setOptionalStruct($sf['name'], self::decodeFieldMap((array) $v, $sf['fields']));
                break;
            default:
                $tmp = new FieldMap();
                self::decodeField($tmp, ['name' => $sf['name'], 'type' => $elem, 'fields' => $sf['fields'], 'tags' => $sf['tags']], $v);
                $raw = $tmp->raw();
                if (array_key_exists($sf['name'], $raw)) {
                    $fm->setRaw($sf['name'], $raw[$sf['name']]);
                }
                break;
        }
    }

    // ── Encode ────────────────────────────────────────────────────────────

    public static function encodeFieldMap(FieldMap $fm, array $schema): array
    {
        $out = [];
        foreach ($schema as $sf) {
            $name = $sf['name'];
            if (!$fm->has($name)) {
                continue;
            }
            $out[$name] = self::encodeField($sf, $fm->raw()[$name]);
        }
        return $out;
    }

    private static function encodeField(array $sf, $v)
    {
        $type = $sf['type'];
        switch ($type) {
            case 'uint8':
            case 'uint16':
            case 'uint32':
            case 'int8':
            case 'int16':
            case 'int32':
                // Cast: setI64 stores every integer as a decimal string, since
                // PHP's int cannot hold the 64-bit range. At 32 bits or less it
                // always can, and the wire wants a JSON number here, not "1".
                return (int) $v;
            case 'bool':
            case 'string':
                return $v;
            case 'uint64':
            case 'uint128':
            case 'int64':
            case 'int128':
                return (string) $v;
            case 'float32':
                return bin2hex(pack('g', (float) $v));
            case 'float64':
                return bin2hex(pack('e', (float) $v));
            case 'bytes':
                return bin2hex((string) $v);
            case 'struct':
                return self::encodeFieldMap($v, $sf['fields']);
            default:
                if (str_starts_with($type, 'list<')) {
                    $elem = substr($type, 5, -1);
                    return self::encodeList($sf, $elem, $v);
                } elseif (str_starts_with($type, 'optional<')) {
                    $elem = substr($type, 9, -1);
                    return self::encodeOptional($sf, $elem, $v);
                } elseif (str_starts_with($type, 'array<')) {
                    [$elem, ] = self::splitArrayType($type);
                    return self::encodeList($sf, $elem, $v);
                } elseif (str_starts_with($type, 'enum<')) {
                    // enum<a,b,c>: the variant name travels as a string.
                    return (string) $v;
                } elseif (str_starts_with($type, 'sum<')) {
                    return self::encodeSum($sf, $v);
                } elseif (str_starts_with($type, 'map<')) {
                    [, $valType] = self::splitMapTypes($type);
                    return self::encodeMap($valType, $sf['fields'], (array) $v);
                } else {
                    throw new \InvalidArgumentException("unknown type \"$type\"");
                }
                return $v;
        }
    }

    /** Inverse of decodeSum: a Variant becomes {tag: payload}. */
    private static function encodeSum(array $sf, $v): array
    {
        if (!$v instanceof Variant) {
            throw new \RuntimeException("expected a Variant for sum field \"{$sf['name']}\"");
        }
        $sv = self::findSchemaVariant($sf, $v->tag);
        return [$v->tag => $sv['payload'] === null ? null : self::encodeField($sv['payload'], $v->value)];
    }

    /**
     * Inverse of decodeList: every element goes back out through encodeField, so
     * the two directions cannot cover different element types. The old version
     * fell through to returning the array untouched for anything it did not name.
     */
    private static function encodeList(array $sf, string $elem, $v): array
    {
        if (!in_array($elem, self::LIST_ELEMS, true)) {
            throw new \InvalidArgumentException("unsupported list element type \"$elem\"");
        }
        if ($elem === 'struct') {
            return array_map(fn($x) => self::encodeFieldMap($x, $sf['fields']), (array) $v);
        }

        $elemSf = ['name' => 'e', 'type' => $elem, 'fields' => $sf['fields'] ?? []];
        return array_map(fn($x) => self::encodeField($elemSf, $x), (array) $v);
    }

    private static function encodeOptional(array $sf, string $elem, $v)
    {
        if ($v === null) {
            return null;
        }
        switch ($elem) {
            case 'string':
                return $v;
            case 'struct':
                return self::encodeFieldMap($v, $sf['fields']);
            default:
                return self::encodeField(['name' => $sf['name'], 'type' => $elem, 'fields' => $sf['fields'], 'tags' => $sf['tags']], $v);
        }
    }

    // ── Map helpers ───────────────────────────────────────────────────────

    private static function splitMapTypes(string $type): array
    {
        $inner = substr($type, 4, -1); // strip "map<" and ">"
        $depth = 0;
        for ($i = 0; $i < strlen($inner); $i++) {
            $ch = $inner[$i];
            if ($ch === '<' || $ch === '[') {
                $depth++;
            } elseif ($ch === '>' || $ch === ']') {
                $depth--;
            } elseif ($ch === ',' && $depth === 0) {
                return [trim(substr($inner, 0, $i)), trim(substr($inner, $i + 1))];
            }
        }
        return ['', trim($inner)];
    }

    private static function decodeMap(string $valType, array $nestedSchema, array $obj): array
    {
        $m = [];
        foreach ($obj as $k => $item) {
            $m[$k] = match ($valType) {
                'struct' => self::decodeFieldMap((array) $item, $nestedSchema),
                'uint64', 'uint128' => (string) $item,
                'int64', 'int128' => (string) $item,
                'float32' => unpack('g', hex2bin($item))[1],
                'float64' => unpack('e', hex2bin($item))[1],
                'bytes' => hex2bin($item),
                default => $item,
            };
        }
        return $m;
    }

    private static function encodeMap(string $valType, array $nestedSchema, array $m): object
    {
        ksort($m);
        $out = [];
        foreach ($m as $k => $item) {
            $out[$k] = match ($valType) {
                'struct' => self::encodeFieldMap($item, $nestedSchema),
                'uint64', 'uint128', 'int64', 'int128' => (string) $item,
                'float32' => bin2hex(pack('g', (float) $item)),
                'float64' => bin2hex(pack('e', (float) $item)),
                'bytes' => bin2hex((string) $item),
                default => $item,
            };
        }
        // Cast so an empty map JSON-encodes as {}, not [] (a PHP array cannot
        // tell the two apart).
        return (object) $out;
    }

    // ── Audit helpers ─────────────────────────────────────────────────────

    /**
     * This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions and the run loop handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
     */
    public static function collectByteSnaps(FieldMap $fm, array &$snaps): void
    {
        $keys = array_keys($fm->raw());
        sort($keys);
        foreach ($keys as $k) {
            $v = $fm->raw()[$k];
            if (is_string($v) && !self::isPrintableUtf8($v)) {
                $snaps[] = ['fm' => $fm, 'key' => $k, 'orig' => $v];
            } elseif ($v instanceof FieldMap) {
                self::collectByteSnaps($v, $snaps);
            } elseif (is_array($v)) {
                foreach ($v as $item) {
                    if ($item instanceof FieldMap) {
                        self::collectByteSnaps($item, $snaps);
                    }
                }
            }
        }
    }

    private static function isPrintableUtf8(string $s): bool
    {
        // Heuristic: if the string is valid UTF-8 and mostly printable, it's not binary bytes.
        return mb_check_encoding($s, 'UTF-8') && !preg_match('/[^\x20-\x7E\r\n\t]/u', $s);
    }

    /**
     * This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions and the run loop handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
     */
    public static function dictDiffs(array $before, array $after): array
    {
        $diffs = [];
        $keys = array_unique(array_merge(array_keys($before), array_keys($after)));
        sort($keys);
        foreach ($keys as $k) {
            $bv = $before[$k] ?? null;
            $av = $after[$k] ?? null;
            if (json_encode($bv) !== json_encode($av)) {
                $diffs[] = $k;
            }
        }
        return $diffs;
    }

    /**
     * This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions and the run loop handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
     */
    public static function detectZeroCopy(FieldMap $fm, string &$buf): array
    {
        if (strlen($buf) === 0) {
            return [];
        }

        $snaps = [];
        self::collectByteSnaps($fm, $snaps);

        // XOR-flip
        for ($i = 0; $i < strlen($buf); $i++) {
            $buf[$i] = chr(ord($buf[$i]) ^ 0xFF);
        }

        $aliased = [];
        foreach ($snaps as $snap) {
            $cur = $snap['fm']->raw()[$snap['key']] ?? null;
            if (is_string($cur) && $cur !== $snap['orig']) {
                $aliased[] = $snap['key'];
            }
        }

        // Restore
        foreach ($snaps as $snap) {
            $snap['fm']->setBytes($snap['key'], $snap['orig']);
        }

        return array_values(array_unique($aliased));
    }

    // ── Emit ──────────────────────────────────────────────────────────────

    private static function emit(array $obj): void
    {
        fwrite(STDOUT, json_encode($obj) . "\n");
    }
}
