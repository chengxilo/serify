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

/**
 * One response frame: cases/response.yaml. {@code requestId} echoes the
 * request's, so a client with several in flight can tell them apart.
 */
@WorkerLib.SerifyModel
public final class Response {
    @SerifyField("request_id") public int requestId;
    @SerifyField public ApiResult result = new ApiResult.Accepted();

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();
        Wire.putU32(out, requestId);

        if (result instanceof ApiResult.Accepted) {
            out.write(0);
        } else if (result instanceof ApiResult.Found f) {
            out.write(1);
            f.value().put(out);
        } else if (result instanceof ApiResult.Listing l) {
            out.write(2);
            l.value().put(out);
        } else if (result instanceof ApiResult.Failed e) {
            out.write(3);
            e.value().put(out);
        } else {
            throw new IllegalArgumentException("unhandled result " + result);
        }
        return out.toByteArray();
    }

    public static Response unmarshal(byte[] data) {
        var r = new Wire.Reader(data);
        var resp = new Response();
        resp.requestId = r.u32();
        resp.result = switch (r.u8()) {
            case 0 -> new ApiResult.Accepted();
            case 1 -> new ApiResult.Found(Task.take(r));
            case 2 -> new ApiResult.Listing(TaskPage.take(r));
            case 3 -> new ApiResult.Failed(ApiError.take(r));
            default -> throw new IllegalArgumentException("unknown result tag");
        };
        return resp;
    }
}
