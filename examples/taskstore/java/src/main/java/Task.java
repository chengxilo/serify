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

/**
 * A stored task: cases/task.yaml.
 *
 * <p>{@code @SerifyModel} plus one {@code @SerifyField} per field is the entire
 * schema binding. An enum needs nothing from it — it travels as its variant
 * <em>name</em>, so {@code priority} is a plain String and Wire.PRIORITIES fixes
 * the ordinal this codec writes.
 */
@WorkerLib.SerifyModel
public final class Task {
    @SerifyField public long id;
    @SerifyField public String title = "";
    @SerifyField public boolean done;
    @SerifyField public String priority = "normal";
    @SerifyField public List<String> tags = List.of();
    @SerifyField("due_at") public Long dueAt;

    public void put(ByteArrayOutputStream out) {
        Wire.putU64(out, id);
        Wire.putStr(out, title);
        out.write(done ? 1 : 0);
        Wire.putEnum(out, Wire.PRIORITIES, priority);
        Wire.putTags(out, tags);
        Wire.putOptionalI64(out, dueAt);
    }

    public static Task take(Wire.Reader r) {
        var t = new Task();
        t.id = r.u64();
        t.title = r.str();
        t.done = r.bool();
        t.priority = r.enumOf(Wire.PRIORITIES);
        t.tags = r.tags();
        t.dueAt = r.optionalI64();
        return t;
    }
}
