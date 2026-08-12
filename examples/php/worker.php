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
 * PHP example worker.
 *
 * This file is the whole worker: it loads the serify library and names the types
 * it can handle, one serializer/deserializer pair per format. Everything else
 * lives in the files beside it — those stand in for the types an application
 * already owns, each carrying its own schema binding and byte layout.
 *
 * Requires ext-gmp (apt install php-gmp).
 *
 * Types this worker does not register (order) are reported to the runner as
 * SKIPPED.
 */

declare(strict_types=1);

require_once __DIR__ . '/../../lib/php/src/FieldMap.php';
require_once __DIR__ . '/../../lib/php/src/Worker.php';
require_once __DIR__ . '/../../lib/php/src/Attributes/SerifyModel.php';
require_once __DIR__ . '/../../lib/php/src/Attributes/SerifyField.php';
require_once __DIR__ . '/../../lib/php/src/SerifyModelHelper.php';

require_once __DIR__ . '/wire.php';
require_once __DIR__ . '/customer.php';
require_once __DIR__ . '/ledger.php';
require_once __DIR__ . '/notification.php';
require_once __DIR__ . '/signals.php';
require_once __DIR__ . '/telemetry.php';

use Serify\Type;
use Serify\Worker;

/**
 * A model that carries its own byte layout, under the one format it speaks.
 * Registered as a Type, so serify converts FieldMap <-> model around these two
 * functions and neither of them ever sees a FieldMap.
 *
 * @param class-string $model
 */
function binary(string $model): Type
{
    return new Type($model, ['binary' => [
        fn(object $m): string => $m->marshal(),
        $model::unmarshal(...),
    ]]);
}

Worker::runSuite([
    'customer'     => new Type(CustomerRecord::class, [
        'binary' => [fn(CustomerRecord $c): string => $c->marshal(), CustomerRecord::unmarshal(...)],
        'json'   => [fn(CustomerRecord $c): string => $c->toJson(), CustomerRecord::fromJson(...)],
    ]),
    'ledger'       => binary(LedgerEntry::class),
    'signals'      => binary(SignalCapture::class),
    'notification' => binary(NotificationRecord::class),
    'telemetry'    => binary(TelemetryFrame::class),
]);
