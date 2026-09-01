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


import io.serify.WorkerLib;
import io.serify.WorkerLib.ModelFormatPair;
import io.serify.WorkerLib.TypeEntry;

import java.util.Map;

/**
 * The conformance worker: serify's entire footprint in the Java follower.
 *
 * <p>It registers the two types that cross the socket and hands each the very
 * functions Client.java calls — {@code Request::marshal} is not a test double.
 */
public final class Worker {
    public static void main(String[] args) {
        WorkerLib.runSuite(Map.of(
                "request", TypeEntry.model(Request.class, Map.of(
                        "binary", new ModelFormatPair<>(Request::marshal, Request::unmarshal))),
                "response", TypeEntry.model(Response.class, Map.of(
                        "binary", new ModelFormatPair<>(Response::marshal, Response::unmarshal)))));
    }

    private Worker() {}
}
