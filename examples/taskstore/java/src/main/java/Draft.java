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
import io.serify.WorkerLib.SerifyField;

import java.io.ByteArrayOutputStream;
import java.util.List;

/** A task the server has not assigned an id to yet: cases/draft.yaml. */
@WorkerLib.SerifyModel
public final class Draft {
    @SerifyField public String title = "";
    @SerifyField public String priority = "normal";
    @SerifyField public List<String> tags = List.of();
    @SerifyField("due_at") public Long dueAt;

    public void put(ByteArrayOutputStream out) {
        Wire.putStr(out, title);
        Wire.putEnum(out, Wire.PRIORITIES, priority);
        Wire.putTags(out, tags);
        Wire.putOptionalI64(out, dueAt);
    }

    public static Draft take(Wire.Reader r) {
        var d = new Draft();
        d.title = r.str();
        d.priority = r.enumOf(Wire.PRIORITIES);
        d.tags = r.tags();
        d.dueAt = r.optionalI64();
        return d;
    }
}
