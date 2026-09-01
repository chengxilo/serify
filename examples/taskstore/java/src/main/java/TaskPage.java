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
import java.util.ArrayList;
import java.util.List;

/** The body of a list response: cases/task_page.yaml. */
@WorkerLib.SerifyModel
public final class TaskPage {
    @SerifyField public List<Task> items = List.of();
    @SerifyField public int total;

    public void put(ByteArrayOutputStream out) {
        Wire.putU32(out, items.size());
        for (var t : items) t.put(out);
        Wire.putU32(out, total);
    }

    public static TaskPage take(Wire.Reader r) {
        var p = new TaskPage();
        int n = r.u32();
        var items = new ArrayList<Task>(n);
        for (int i = 0; i < n; i++) items.add(Task.take(r));
        p.items = items;
        p.total = r.u32();
        return p;
    }
}
