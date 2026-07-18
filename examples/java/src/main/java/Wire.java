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

import java.io.ByteArrayOutputStream;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;

/**
 * Byte-level primitives shared by the models in this worker.
 *
 * <p>Go is the --ref language and owns the layout these reproduce; see the
 * comment at the top of examples/go/wire.go.
 */
final class Wire {

    static void writeLenPrefixed(ByteArrayOutputStream out, byte[] body) {
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(body.length).array());
        out.writeBytes(body);
    }

    static void writeLenPrefixed(ByteArrayOutputStream out, String s) {
        writeLenPrefixed(out, s.getBytes(StandardCharsets.UTF_8));
    }

    static byte[] readLenPrefixed(ByteBuffer buf) {
        byte[] body = new byte[buf.getInt()];
        buf.get(body); // copies out of the input buffer
        return body;
    }

    static String readLenString(ByteBuffer buf) {
        return new String(readLenPrefixed(buf), StandardCharsets.UTF_8);
    }

    private Wire() {}
}
