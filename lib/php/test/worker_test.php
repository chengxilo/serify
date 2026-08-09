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
 * Unit tests for the PHP worker library. Run with `php lib/php/test/worker_test.php`;
 * it exits non-zero on the first failure.
 *
 * No PHPUnit and no composer: the library itself has no dependencies, and a
 * worker loads it with plain require_once, so the tests load it the same way.
 * What is under test here is the part a conformance run cannot reach — see
 * the registration test below.
 */

declare(strict_types=1);

require_once __DIR__ . '/../src/FieldMap.php';
require_once __DIR__ . '/../src/Attributes/SerifyModel.php';
require_once __DIR__ . '/../src/Attributes/SerifyField.php';
require_once __DIR__ . '/../src/SerifyModelHelper.php';
require_once __DIR__ . '/../src/Worker.php';

use Serify\Attributes\SerifyField;
use Serify\Attributes\SerifyModel;
use Serify\FieldMap;
use Serify\Type;
use Serify\Worker;

$failures = 0;

function check(bool $ok, string $what): void
{
    global $failures;
    if ($ok) {
        echo "ok   $what\n";
        return;
    }
    echo "FAIL $what\n";
    $failures++;
}

/** A model with a byte layout of its own: one u32, little-endian. */
#[SerifyModel]
class Rec
{
    #[SerifyField('n')]
    public int $n = 0;

    public function marshal(): string
    {
        return pack('V', $this->n);
    }

    public static function unmarshal(string $data): self
    {
        $r = new self();
        $r->n = unpack('V', $data)[1];
        return $r;
    }
}

// ── Worker::resolveRegistered ────────────────────────────────────────────
//
// The test a conformance run cannot replace. An unresolved (type, format) is
// reported SKIPPED, so a spelling resolveRegistered fails to understand yields
// a *green* run made entirely of SKIPs — indistinguishable from a worker that
// honestly does not implement the type. The `instanceof Type` that separates
// the two spellings is a run-time check, and PHP's instanceof against a class
// that was never loaded is silently false, which is the same failure wearing a
// different hat.

$asType = ['rec' => new Type(Rec::class, [
    'binary' => [fn(Rec $r): string => $r->marshal(), Rec::unmarshal(...)],
])];
$asArray = ['rec' => [
    'binary' => [fn(FieldMap $fm): string => '', fn(string $d): FieldMap => new FieldMap()],
]];

foreach (['Type' => $asType, 'array' => $asArray] as $name => $suite) {
    $pair = Worker::resolveRegistered($suite, 'rec', 'binary');
    check($pair !== null, "$name registration resolves");
    check(is_callable($pair[0] ?? null), "$name registration keeps its serializer");
    check(is_callable($pair[1] ?? null), "$name registration keeps its deserializer");
}

check(Worker::resolveRegistered($asType, 'rec', 'json') === null, 'unknown format resolves to null');
check(Worker::resolveRegistered($asType, 'nope', 'binary') === null, 'unknown type resolves to null (Type)');
check(Worker::resolveRegistered($asArray, 'nope', 'binary') === null, 'unknown type resolves to null (array)');

// ── A model-backed format converts FieldMap <-> model on both sides ───────

$pair = Worker::resolveRegistered($asType, 'rec', 'binary');
if ($pair === null) {
    // Already reported above; going on would only turn those failures into a
    // fatal that hides the tests below.
    check(false, 'model conversion (skipped: the Type registration resolved to nothing)');
} else {
    $fm = new FieldMap();
    $fm->setU32('n', 7);
    check(bin2hex($pair[0]($fm)) === '07000000', 'model serializer receives the model, not the FieldMap');
    check($pair[1](pack('V', 9))->getU32('n') === 9, 'model deserializer returns the model, and serify maps it back');
}

// ── A Type with no model passes the FieldMap straight through ─────────────
//
// The path a type with no natural class needs; the audit fixtures mutate a
// FieldMap on purpose and would break if serify tried to bind one to a model.

$modelLess = ['raw' => new Type(null, [
    'binary' => [fn(FieldMap $fm): string => $fm->getString('s'), fn(string $d): FieldMap => new FieldMap()],
])];
$pair = Worker::resolveRegistered($modelLess, 'raw', 'binary');
if ($pair === null) {
    check(false, 'a Type with no model resolves at all');
} else {
    $fm = new FieldMap();
    $fm->setString('s', 'through');
    check($pair[0]($fm) === 'through', 'a Type with no model hands the serializer the FieldMap itself');
}

// ── The '*' wildcard, which backs the single-type Worker::run ─────────────

$wild = ['*' => ['*' => [fn(FieldMap $fm): string => 'w', fn(string $d): FieldMap => new FieldMap()]]];
check(Worker::resolveRegistered($wild, 'anything', 'anyhow') !== null, 'the * wildcard still matches any (type, format)');

echo $failures === 0 ? "\nall tests passed\n" : "\n$failures test(s) failed\n";
exit($failures === 0 ? 0 : 1);
