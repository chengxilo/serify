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

declare(strict_types=1);
require_once __DIR__ . '/../../../../lib/php/src/FieldMap.php';
require_once __DIR__ . '/../../../../lib/php/src/Worker.php';
use Serify\FieldMap; use Serify\Worker;

$unstableCtr = 0; $deserUnstableCtr = 0;

function _marshal(FieldMap $fm): string {
    $val = $fm->getU32('value'); $tag = $fm->getString('tag');
    $payload = $fm->getBytes('payload'); $tags = $fm->getListString('tags');
    $out = pack('V', $val) . chr(strlen($tag)) . $tag;
    $out .= pack('V', strlen($payload)) . $payload;
    $out .= chr(count($tags));
    foreach ($tags as $t) { $out .= chr(strlen($t)) . $t; }
    return $out;
}
function _unmarshal(string $data, bool $copy=true): FieldMap {
    $fm = new FieldMap(); $off = 0;
    $fm->setU32('value', unpack('V', substr($data, $off, 4))[1]); $off += 4;
    $tlen = ord($data[$off++]); $fm->setString('tag', substr($data, $off, $tlen)); $off += $tlen;
    $plen = unpack('V', substr($data, $off, 4))[1]; $off += 4;
    $p = substr($data, $off, $plen); $fm->setBytes('payload', $copy ? $p : $p); $off += $plen;
    $tcount = ord($data[$off++]); $tags = [];
    for ($i=0; $i<$tcount; $i++) { $tl=ord($data[$off++]); $tags[]=substr($data,$off,$tl); $off+=$tl; }
    $fm->setListString('tags', $tags);
    return $fm;
}
function cleanSer(FieldMap $fm): string { return _marshal($fm); }
function cleanDeser(string $d): FieldMap { return _unmarshal($d, true); }
function mutatingSer(FieldMap $fm): string { $b=_marshal($fm); $fm->setU32('value',0); return $b; }
function unstableSer(FieldMap $fm): string { global $unstableCtr; return _marshal($fm).chr($unstableCtr++); }
function deserUnstableDeser(string $d): FieldMap { global $deserUnstableCtr; $fm=_unmarshal($d,true); if($deserUnstableCtr++>0)$fm->setU32('value',$fm->getU32('value')+1); return $fm; }
function inputMutDeser(string $d): FieldMap { $fm=_unmarshal($d,true); if(strlen($d))$d[0]=chr(ord($d[0])^0xFF); return $fm; }
Worker::runSuite(['audit'=>['clean'=>['cleanSer','cleanDeser'],'mutating'=>['mutatingSer','cleanDeser'],'unstable'=>['unstableSer','cleanDeser'],'deser-unstable'=>['cleanSer','deserUnstableDeser'],'input-mutating'=>['cleanSer','inputMutDeser']]]);
