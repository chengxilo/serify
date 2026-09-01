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
 * The conformance worker: serify's entire footprint in the PHP follower.
 *
 * It registers the two types that cross the socket and hands each the very
 * functions client.php calls — Request::marshal is not a test double.
 *
 * Requires ext-gmp (apt install php-gmp).
 */

declare(strict_types=1);

require_once __DIR__ . '/../../../lib/php/src/FieldMap.php';
require_once __DIR__ . '/../../../lib/php/src/Worker.php';
require_once __DIR__ . '/../../../lib/php/src/Attributes/SerifyModel.php';
require_once __DIR__ . '/../../../lib/php/src/Attributes/SerifyField.php';
require_once __DIR__ . '/../../../lib/php/src/SerifyModelHelper.php';

require_once __DIR__ . '/wire.php';
require_once __DIR__ . '/model.php';
require_once __DIR__ . '/message.php';

use Serify\Type;
use Serify\Worker;

Worker::runSuite([
    'request' => new Type(Request::class, ['binary' => [
        fn(Request $r): string => $r->marshal(),
        fn(string $d): Request => Request::unmarshal($d),
    ]]),
    'response' => new Type(Response::class, ['binary' => [
        fn(Response $r): string => $r->marshal(),
        fn(string $d): Response => Response::unmarshal($d),
    ]]),
]);
