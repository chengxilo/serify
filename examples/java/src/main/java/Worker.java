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
import io.serify.WorkerLib.ModelFormatPair;
import io.serify.WorkerLib.TypeEntry;

import java.util.Map;

/**
 * Java example worker.
 *
 * <p>This file is the whole worker: it names the types it can handle, one
 * serializer/deserializer pair per format. Everything else lives in the files
 * beside it — those stand in for the types an application already owns, each
 * carrying its own schema binding and byte layout.
 *
 * <p>Types this worker does not register (customer, order) are
 * reported to the runner as SKIPPED.
 */
public final class Worker {

    public static void main(String[] args) {
        WorkerLib.runSuite(Map.of(
                "ledger", TypeEntry.model(LedgerEntry.class, Map.of(
                        "binary", new ModelFormatPair<>(LedgerEntry::marshal, LedgerEntry::unmarshal))),
                "signals", TypeEntry.model(SignalCapture.class, Map.of(
                        "binary", new ModelFormatPair<>(SignalCapture::marshal, SignalCapture::unmarshal))),
                "notification", TypeEntry.model(Notification.class, Map.of(
                        "binary", new ModelFormatPair<>(Notification::marshal, Notification::unmarshal))),
                "telemetry", TypeEntry.model(TelemetryFrame.class, Map.of(
                        "binary", new ModelFormatPair<>(TelemetryFrame::marshal, TelemetryFrame::unmarshal)))));
    }

    private Worker() {}
}
