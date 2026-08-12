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
 * Types this worker does not register (customer, order) are reported
 * to the runner as SKIPPED.
 */

import { runSuite, type } from '@chengxilo/serify';

import { LedgerEntry } from './ledger';
import { NotificationRecord } from './notification';
import { SignalCapture } from './signals';
import { TelemetryFrame } from './telemetry';

/**
 * The one format each of these models carries: its own byte layout. serify
 * converts FieldMap <-> model, so marshal/unmarshal speak the model alone.
 */
function binary<T extends { marshal(): Buffer }>(
  model: { new (): T; unmarshal(data: Buffer): T },
) {
  return type(model, {
    binary: {
      serialize: (m: T) => m.marshal(),
      deserialize: (data: Buffer) => model.unmarshal(data),
    },
  });
}

runSuite({
  ledger: binary(LedgerEntry),
  signals: binary(SignalCapture),
  notification: binary(NotificationRecord),
  telemetry: binary(TelemetryFrame),
});
