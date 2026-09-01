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

/** One request frame: cases/request.yaml. */
@WorkerLib.SerifyModel
public final class Request {
    @SerifyField("request_id") public int requestId;
    @SerifyField public Op op = new Op.ListAll();

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();
        Wire.putU32(out, requestId);

        // The tag ordinal is the arm's position in the case file's sum, which is
        // the declaration order in Op.java. The schema tag *names* are the
        // binding's business and never appear here.
        //
        // An if/else chain rather than a switch: pattern matching for switch is
        // Java 21 and this project targets 17, where only `instanceof` patterns
        // are final. That also costs the exhaustiveness check a sealed switch
        // would give, hence the explicit final else.
        if (op instanceof Op.ListAll) {
            out.write(0);                        // a unit variant is nothing but its tag
        } else if (op instanceof Op.Create c) {
            out.write(1);
            c.value().put(out);
        } else if (op instanceof Op.Read r) {
            out.write(2);
            Wire.putU64(out, r.value());
        } else if (op instanceof Op.Update u) {
            out.write(3);
            u.value().put(out);
        } else if (op instanceof Op.Delete d) {
            out.write(4);
            Wire.putU64(out, d.value());
        } else {
            throw new IllegalArgumentException("unhandled op " + op);
        }
        return out.toByteArray();
    }

    public static Request unmarshal(byte[] data) {
        var r = new Wire.Reader(data);
        var req = new Request();
        req.requestId = r.u32();
        req.op = switch (r.u8()) {
            case 0 -> new Op.ListAll();
            case 1 -> new Op.Create(Draft.take(r));
            case 2 -> new Op.Read(r.u64());
            case 3 -> new Op.Update(Task.take(r));
            case 4 -> new Op.Delete(r.u64());
            default -> throw new IllegalArgumentException("unknown op tag");
        };
        return req;
    }
}
