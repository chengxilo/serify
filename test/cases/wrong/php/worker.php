<?php
// Copyright 2026 Chengxi Luo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
 * PHP half of the `wrong` meta-test. Mirrors the Go worker's byte/JSON layout.
 */

declare(strict_types=1);

require_once __DIR__ . '/../../../../lib/php/src/FieldMap.php';
require_once __DIR__ . '/../../../../lib/php/src/Worker.php';

use Serify\FieldMap;
use Serify\Worker;

const SELF_LANG = 'php';

function toUpperSelf(array $langs): array
{
    return array_map(fn($s) => $s === SELF_LANG ? 'PHP' : $s, $langs);
}

// --- binary ----------------------------------------------------------------

function binarySerialize(FieldMap $fm): string
{
    $langs = $fm->getListString('langs');
    if (!$fm->getBool('binary_serialize')) {
        $langs = toUpperSelf($langs);
    }
    $out = chr($fm->getBool('binary_serialize') ? 1 : 0)
         . chr($fm->getBool('binary_deserialize') ? 1 : 0)
         . chr($fm->getBool('json_serialize') ? 1 : 0)
         . chr($fm->getBool('json_deserialize') ? 1 : 0);
    $out .= pack('V', count($langs));
    foreach ($langs as $s) {
        $out .= pack('V', strlen($s)) . $s;
    }
    return $out;
}

function binaryDeserialize(string $data): FieldMap
{
    $fm = new FieldMap();
    $bs = ord($data[0]) !== 0;
    $bd = ord($data[1]) !== 0;
    $js = ord($data[2]) !== 0;
    $jd = ord($data[3]) !== 0;
    $fm->setBool('binary_serialize', $bs);
    $fm->setBool('binary_deserialize', $bd);
    $fm->setBool('json_serialize', $js);
    $fm->setBool('json_deserialize', $jd);

    $off = 4;
    $n = unpack('V', substr($data, $off, 4))[1];
    $off += 4;
    $langs = [];
    for ($i = 0; $i < $n; $i++) {
        $slen = unpack('V', substr($data, $off, 4))[1];
        $off += 4;
        $langs[] = substr($data, $off, $slen);
        $off += $slen;
    }
    if (!$bd) {
        $langs = toUpperSelf($langs);
    }
    $fm->setListString('langs', $langs);
    return $fm;
}

// --- json ------------------------------------------------------------------

function jsonSerialize(FieldMap $fm): string
{
    $d = [
        'binary_serialize' => $fm->getBool('binary_serialize'),
        'binary_deserialize' => $fm->getBool('binary_deserialize'),
        'json_serialize' => $fm->getBool('json_serialize'),
        'json_deserialize' => $fm->getBool('json_deserialize'),
        'langs' => $fm->getListString('langs'),
    ];
    if (!$d['json_serialize']) {
        $d['langs'] = toUpperSelf($d['langs']);
    }
    return json_encode($d, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
}

function jsonDeserialize(string $data): FieldMap
{
    $d = json_decode($data, true, 512, JSON_THROW_ON_ERROR);
    $fm = new FieldMap();
    $fm->setBool('binary_serialize', $d['binary_serialize']);
    $fm->setBool('binary_deserialize', $d['binary_deserialize']);
    $fm->setBool('json_serialize', $d['json_serialize']);
    $fm->setBool('json_deserialize', $d['json_deserialize']);
    $langs = $d['langs'];
    if (!$d['json_deserialize']) {
        $langs = toUpperSelf($langs);
    }
    $fm->setListString('langs', $langs);
    return $fm;
}

// --- fault formats ---------------------------------------------------------

function errSer(FieldMap $fm): string
{
    throw new RuntimeException('injected serialize error');
}

function errDeser(string $data): FieldMap
{
    throw new RuntimeException('injected deserialize error');
}

function hangSer(FieldMap $fm): string
{
    sleep(3);
    return binarySerialize($fm);
}

function crashSer(FieldMap $fm): string
{
    exit(3);
}

Worker::runSuite([
    'wrong' => [
        'binary' => ['binarySerialize', 'binaryDeserialize'],
        'json' => ['jsonSerialize', 'jsonDeserialize'],
        'err_ser' => ['errSer', 'binaryDeserialize'],
        'err_deser' => ['binarySerialize', 'errDeser'],
        'hang' => ['hangSer', 'binaryDeserialize'],
        'crash' => ['crashSer', 'binaryDeserialize'],
    ],
]);
