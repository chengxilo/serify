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

import io.serify.WorkerLib;
import io.serify.WorkerLib.FormatPair;
import io.serify.WorkerLib.SerifyModelHelper;

import java.util.Map;

/**
 * Java example worker.
 *
 * <p>This file is the whole worker: it names the types it can handle, one
 * serializer/deserializer pair per format. Everything else lives in the files
 * beside it — those stand in for the types an application already owns, each
 * carrying its own schema binding and byte layout.
 *
 * <p>Types this worker does not register (customer, order, telemetry) are
 * reported to the runner as SKIPPED.
 */
public final class Worker {

    public static void main(String[] args) {
        WorkerLib.runSuite(Map.of(
                "ledger", Map.of("binary", new FormatPair(
                        fm -> SerifyModelHelper.fromFieldMap(fm, LedgerEntry.class).marshal(),
                        data -> SerifyModelHelper.toFieldMap(LedgerEntry.unmarshal(data)))),
                "signals", Map.of("binary", new FormatPair(
                        fm -> SerifyModelHelper.fromFieldMap(fm, SignalCapture.class).marshal(),
                        data -> SerifyModelHelper.toFieldMap(SignalCapture.unmarshal(data)))),
                "notification", Map.of("binary", new FormatPair(
                        fm -> SerifyModelHelper.fromFieldMap(fm, Notification.class).marshal(),
                        data -> SerifyModelHelper.toFieldMap(Notification.unmarshal(data))))));
    }

    private Worker() {}
}
