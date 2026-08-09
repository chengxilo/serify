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

import io.serify.WorkerLib; import io.serify.WorkerLib.*; import java.nio.charset.*; import java.util.*;
public final class ISWorker {
    static byte[] ser(FieldMap fm){return fm.getString("id").getBytes(StandardCharsets.UTF_8);}
    static FieldMap deser(byte[] d){var fm=new FieldMap();fm.setString("id",new String(d,StandardCharsets.UTF_8));return fm;}
    public static void main(String[]a){WorkerLib.runSuite(Map.of("invalid_schema",TypeEntry.formats(Map.of("byte",new FormatPair(ISWorker::ser,ISWorker::deser)))));}
}
