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
 * Happy-path PHP worker: `all_types` in binary and json.
 *
 * Go is the --ref language and owns both byte layouts; see
 * test/cases/happy/go/type.go. The json format must match Go's encoding/json
 * byte-for-byte (with SetEscapeHTML(false)): schema field order, map keys in
 * byte order, []byte as base64, floats in shortest form without a trailing
 * .0, and U+2028/U+2029 escaped (Go escapes those unconditionally).
 *
 * PHP's int is 64-bit signed, so a u64 can overflow it: FieldMap carries
 * 64-bit values as decimal strings and this worker converts through GMP.
 * json deserialize uses JSON_BIGINT_AS_STRING for the same reason.
 *
 * Requires ext-gmp (apt install php-gmp).
 */

declare(strict_types=1);

require_once __DIR__ . '/../../../../lib/php/src/FieldMap.php';
require_once __DIR__ . '/../../../../lib/php/src/Worker.php';

use Serify\FieldMap;
use Serify\Worker;

const STATUS_VARIANTS = ['pending', 'paid', 'shipped', 'delivered', 'cancelled'];

function statusOrdinal(string $s): int
{
    $i = array_search($s, STATUS_VARIANTS, true);
    if ($i === false) {
        throw new RuntimeException("unknown status \"$s\"");
    }
    return $i;
}

/** Encode a decimal string as $numBytes little-endian two's-complement bytes. */
function encodeInt(string $decimal, int $numBytes): string
{
    $u = gmp_mod(gmp_init($decimal, 10), gmp_pow(2, $numBytes * 8));
    $le = gmp_export($u, 1, GMP_LSW_FIRST | GMP_LITTLE_ENDIAN);
    return str_pad($le, $numBytes, "\0", STR_PAD_RIGHT);
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
    if (gmp_testbit($n, $bits - 1)) {
        $n = gmp_sub($n, gmp_pow(2, $bits));
    }
    return gmp_strval($n);
}

function packLenStr(string $s): string
{
    return pack('V', strlen($s)) . $s;
}

// --- binary format -----------------------------------------------------------

function binarySerialize(FieldMap $fm): string
{
    $out = chr($fm->getU8('uint8'));
    $out .= pack('v', $fm->getU16('uint16'));
    $out .= pack('V', $fm->getU32('uint32'));
    $out .= encodeInt($fm->getU64('uint64'), 8);
    $out .= encodeInt((string)$fm->getI8('int8'), 1);
    $out .= encodeInt((string)$fm->getI16('int16'), 2);
    $out .= encodeInt((string)$fm->getI32('int32'), 4);
    $out .= encodeInt($fm->getI64('int64'), 8);
    $out .= pack('g', $fm->getF32('float32'));
    $out .= pack('e', $fm->getF64('float64'));
    $out .= chr($fm->getBool('bool') ? 1 : 0);
    $out .= packLenStr($fm->getString('string'));

    $raw = $fm->getBytes('bytes');
    $out .= pack('V', strlen($raw)) . $raw;

    $list = $fm->getListString('list');
    $out .= pack('V', count($list));
    foreach ($list as $s) {
        $out .= packLenStr($s);
    }

    $opt = $fm->getOptionalString('optional');
    $out .= $opt === null ? "\x00" : "\x01" . packLenStr($opt);

    foreach ($fm->getListU32('array') as $n) {
        $out .= pack('V', $n);
    }

    $p = $fm->getStruct('struct');
    $out .= encodeInt((string)$p->getI32('x'), 4);
    $out .= encodeInt((string)$p->getI32('y'), 4);
    $out .= encodeInt((string)$p->getI32('z'), 4);
    $out .= packLenStr($p->getString('name'));

    $m = $fm->getMap('map');
    $keys = array_keys($m);
    sort($keys, SORT_STRING);
    $out .= pack('V', count($keys));
    foreach ($keys as $k) {
        $out .= packLenStr((string)$k) . pack('V', $m[$k]);
    }

    $ms = $fm->getMap('map_struct');
    $keys = array_keys($ms);
    sort($keys, SORT_STRING);
    $out .= pack('V', count($keys));
    foreach ($keys as $k) {
        $out .= packLenStr((string)$k);
        $out .= packLenStr($ms[$k]->getString('name'));
        $out .= pack('V', $ms[$k]->getU32('weight'));
    }

    $out .= chr(statusOrdinal($fm->getString('status')));
    return $out;
}

function binaryDeserialize(string $data): FieldMap
{
    $fm = new FieldMap();
    $off = 0;
    $take = function (int $n) use ($data, &$off): string {
        $b = substr($data, $off, $n);
        $off += $n;
        return $b;
    };
    $takeStr = function () use ($take): string {
        $n = unpack('V', $take(4))[1];
        return $take($n);
    };

    $fm->setU8('uint8', ord($take(1)));
    $fm->setU16('uint16', unpack('v', $take(2))[1]);
    $fm->setU32('uint32', unpack('V', $take(4))[1]);
    $fm->setU64('uint64', decodeUnsigned($take(8)));
    $fm->setI8('int8', (int)decodeSigned($take(1)));
    $fm->setI16('int16', (int)decodeSigned($take(2)));
    $fm->setI32('int32', (int)decodeSigned($take(4)));
    $fm->setI64('int64', decodeSigned($take(8)));
    $fm->setF32('float32', unpack('g', $take(4))[1]);
    $fm->setF64('float64', unpack('e', $take(8))[1]);
    $fm->setBool('bool', ord($take(1)) !== 0);
    $fm->setString('string', $takeStr());

    $n = unpack('V', $take(4))[1];
    $fm->setBytes('bytes', $take($n));

    $n = unpack('V', $take(4))[1];
    $list = [];
    for ($i = 0; $i < $n; $i++) {
        $list[] = $takeStr();
    }
    $fm->setListString('list', $list);

    $fm->setOptionalString('optional', ord($take(1)) !== 0 ? $takeStr() : null);

    $arr = [];
    for ($i = 0; $i < 4; $i++) {
        $arr[] = unpack('V', $take(4))[1];
    }
    $fm->setListU32('array', $arr);

    $p = new FieldMap();
    $p->setI32('x', (int)decodeSigned($take(4)));
    $p->setI32('y', (int)decodeSigned($take(4)));
    $p->setI32('z', (int)decodeSigned($take(4)));
    $p->setString('name', $takeStr());
    $fm->setStruct('struct', $p);

    $n = unpack('V', $take(4))[1];
    $m = [];
    for ($i = 0; $i < $n; $i++) {
        $k = $takeStr();
        $m[$k] = unpack('V', $take(4))[1];
    }
    $fm->setMap('map', $m);

    $n = unpack('V', $take(4))[1];
    $ms = [];
    for ($i = 0; $i < $n; $i++) {
        $k = $takeStr();
        $t = new FieldMap();
        $t->setString('name', $takeStr());
        $t->setU32('weight', unpack('V', $take(4))[1]);
        $ms[$k] = $t;
    }
    $fm->setMap('map_struct', $ms);

    $ord = ord($take(1));
    if ($ord >= count(STATUS_VARIANTS)) {
        throw new RuntimeException("status ordinal $ord out of range");
    }
    $fm->setString('status', STATUS_VARIANTS[$ord]);
    return $fm;
}

// --- json format -------------------------------------------------------------

/**
 * Go's encoding/json string escaping with SetEscapeHTML(false): only \n, \r,
 * \t are named (\b and \f become \u00xx), and U+2028/U+2029 are escaped
 * unconditionally. Operates on UTF-8 bytes; U+2028/29 are E2 80 A8/A9.
 */
function goStr(string $s): string
{
    $out = '"';
    $len = strlen($s);
    for ($i = 0; $i < $len; $i++) {
        $c = $s[$i];
        $o = ord($c);
        if ($c === '"') {
            $out .= '\\"';
        } elseif ($c === '\\') {
            $out .= '\\\\';
        } elseif ($o < 0x20) {
            $out .= match ($o) {
                0x0A => '\\n',
                0x0D => '\\r',
                0x09 => '\\t',
                default => sprintf('\\u%04x', $o),
            };
        } elseif ($o === 0xE2 && $i + 2 < $len && $s[$i + 1] === "\x80"
            && ($s[$i + 2] === "\xA8" || $s[$i + 2] === "\xA9")) {
            $out .= $s[$i + 2] === "\xA8" ? '\\u2028' : '\\u2029';
            $i += 2;
        } else {
            $out .= $c;
        }
    }
    return $out . '"';
}

/** Go prints floats in shortest round-trip form without a trailing .0. */
function goF64(float $v): string
{
    $s = json_encode($v); // serialize_precision=-1: shortest round-trip
    return str_ends_with($s, '.0') ? substr($s, 0, -2) : $s;
}

/** Shortest decimal that round-trips through float32 ($v is the f64 widening). */
function goF32(float $v): string
{
    $bits = pack('g', $v);
    for ($p = 1; $p <= 9; $p++) {
        $s = sprintf('%.' . $p . 'G', $v);
        if (pack('g', (float)$s) === $bits) {
            return str_ends_with($s, '.0') ? substr($s, 0, -2) : $s;
        }
    }
    return goF64($v);
}

function jsonSerialize_(FieldMap $fm): string
{
    $parts = [
        '"uint8":' . $fm->getU8('uint8'),
        '"uint16":' . $fm->getU16('uint16'),
        '"uint32":' . $fm->getU32('uint32'),
        '"uint64":' . $fm->getU64('uint64'),
        '"int8":' . $fm->getI8('int8'),
        '"int16":' . $fm->getI16('int16'),
        '"int32":' . $fm->getI32('int32'),
        '"int64":' . $fm->getI64('int64'),
        '"float32":' . goF32($fm->getF32('float32')),
        '"float64":' . goF64($fm->getF64('float64')),
        '"bool":' . ($fm->getBool('bool') ? 'true' : 'false'),
        '"string":' . goStr($fm->getString('string')),
        '"bytes":"' . base64_encode($fm->getBytes('bytes')) . '"',
        '"list":[' . implode(',', array_map('goStr', $fm->getListString('list'))) . ']',
    ];

    $opt = $fm->getOptionalString('optional');
    $parts[] = '"optional":' . ($opt === null ? 'null' : goStr($opt));

    $parts[] = '"array":[' . implode(',', $fm->getListU32('array')) . ']';

    $p = $fm->getStruct('struct');
    $parts[] = '"struct":{"x":' . $p->getI32('x') . ',"y":' . $p->getI32('y')
        . ',"z":' . $p->getI32('z') . ',"name":' . goStr($p->getString('name')) . '}';

    $m = $fm->getMap('map');
    $keys = array_keys($m);
    sort($keys, SORT_STRING);
    $entries = [];
    foreach ($keys as $k) {
        $entries[] = goStr((string)$k) . ':' . $m[$k];
    }
    $parts[] = '"map":{' . implode(',', $entries) . '}';

    $ms = $fm->getMap('map_struct');
    $keys = array_keys($ms);
    sort($keys, SORT_STRING);
    $entries = [];
    foreach ($keys as $k) {
        $t = $ms[$k];
        $entries[] = goStr((string)$k) . ':{"name":' . goStr($t->getString('name'))
            . ',"weight":' . $t->getU32('weight') . '}';
    }
    $parts[] = '"map_struct":{' . implode(',', $entries) . '}';

    $parts[] = '"status":' . goStr($fm->getString('status'));
    return '{' . implode(',', $parts) . '}';
}

function jsonDeserialize(string $data): FieldMap
{
    $v = json_decode($data, true, 512, JSON_BIGINT_AS_STRING | JSON_THROW_ON_ERROR);

    $fm = new FieldMap();
    $fm->setU8('uint8', $v['uint8']);
    $fm->setU16('uint16', $v['uint16']);
    $fm->setU32('uint32', $v['uint32']);
    $fm->setU64('uint64', (string)$v['uint64']);
    $fm->setI8('int8', $v['int8']);
    $fm->setI16('int16', $v['int16']);
    $fm->setI32('int32', $v['int32']);
    $fm->setI64('int64', (string)$v['int64']);
    $fm->setF32('float32', (float)$v['float32']);
    $fm->setF64('float64', (float)$v['float64']);
    $fm->setBool('bool', $v['bool']);
    $fm->setString('string', $v['string']);
    $fm->setBytes('bytes', base64_decode($v['bytes'], true));
    $fm->setListString('list', $v['list']);
    $fm->setOptionalString('optional', $v['optional']);
    $fm->setListU32('array', array_map('intval', $v['array']));

    $p = new FieldMap();
    $p->setI32('x', $v['struct']['x']);
    $p->setI32('y', $v['struct']['y']);
    $p->setI32('z', $v['struct']['z']);
    $p->setString('name', $v['struct']['name']);
    $fm->setStruct('struct', $p);

    $m = [];
    foreach ($v['map'] as $k => $n) {
        $m[(string)$k] = (int)$n;
    }
    $fm->setMap('map', $m);

    $ms = [];
    foreach ($v['map_struct'] as $k => $tv) {
        $t = new FieldMap();
        $t->setString('name', $tv['name']);
        $t->setU32('weight', $tv['weight']);
        $ms[(string)$k] = $t;
    }
    $fm->setMap('map_struct', $ms);

    $fm->setString('status', $v['status']);
    return $fm;
}

Worker::runSuite([
    'all_types' => [
        'binary' => ['binarySerialize', 'binaryDeserialize'],
        'json' => ['jsonSerialize_', 'jsonDeserialize'],
    ],
]);
