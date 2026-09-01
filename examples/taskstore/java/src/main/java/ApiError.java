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

/** A failed request: cases/api_error.yaml. */
@WorkerLib.SerifyModel
public final class ApiError {
    @SerifyField public String code = "not_found";
    @SerifyField public String message = "";

    public void put(ByteArrayOutputStream out) {
        Wire.putEnum(out, Wire.ERROR_CODES, code);
        Wire.putStr(out, message);
    }

    public static ApiError take(Wire.Reader r) {
        var e = new ApiError();
        e.code = r.enumOf(Wire.ERROR_CODES);
        e.message = r.str();
        return e;
    }
}
