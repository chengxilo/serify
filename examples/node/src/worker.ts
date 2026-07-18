/**
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
 * Node example worker.
 *
 * This file is the whole worker: it names the types it can handle, one
 * serializer/deserializer pair per format. Everything else lives in the modules
 * beside it — those stand in for the types an application already owns, each
 * carrying its own schema binding and byte layout.
 *
 * Types this worker does not register (customer, order, telemetry) are reported
 * to the runner as SKIPPED.
 */

import { FieldMap, Serify, runSuite } from '@chengxilo/serify';

import { LedgerEntry } from './ledger';
import { NotificationRecord } from './notification';
import { SignalCapture } from './signals';

/**
 * One serializer/deserializer pair for a model that carries its own byte
 * layout: FieldMap in, model, bytes out — and back.
 */
function binary<T extends { marshal(): Buffer }>(
  model: { new (): T; unmarshal(data: Buffer): T },
) {
  return {
    serialize: (fm: FieldMap) => Serify.fromFieldMap(model, fm).marshal(),
    deserialize: (data: Buffer) => Serify.toFieldMap(model.unmarshal(data)),
  };
}

runSuite({
  ledger: { binary: binary(LedgerEntry) },
  signals: { binary: binary(SignalCapture) },
  notification: { binary: binary(NotificationRecord) },
});
