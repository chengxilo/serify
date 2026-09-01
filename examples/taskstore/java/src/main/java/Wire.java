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


import java.io.ByteArrayOutputStream;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

/**
 * Byte-level primitives. Go owns the layout these reproduce; the conventions
 * are documented at the top of go/api/wire.go.
 */
public final class Wire {

    /** Declaration order of the {@code enum<low, normal, high>} in cases/task.yaml. */
    public static final List<String> PRIORITIES = List.of("low", "normal", "high");

    /** Declaration order of the enum in cases/api_error.yaml. */
    public static final List<String> ERROR_CODES = List.of("not_found", "invalid", "conflict");

    /** A cursor over one message's bytes. */
    public static final class Reader {
        private final ByteBuffer buf;

        public Reader(byte[] data) {
            this.buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        }

        public int u8() {
            return buf.get() & 0xFF;
        }

        public int u32() {
            return buf.getInt();
        }

        /** uint64 rides in a long as its bit pattern; see Long.toUnsignedString. */
        public long u64() {
            return buf.getLong();
        }

        public long i64() {
            return buf.getLong();
        }

        public boolean bool() {
            return buf.get() != 0;
        }

        public String str() {
            var body = new byte[buf.getInt()];
            buf.get(body);
            return new String(body, StandardCharsets.UTF_8);
        }

        /**
         * An enum arrives as an ordinal into a fixed list, so a value outside
         * the declared variants has no encoding and cannot be read back.
         */
        public String enumOf(List<String> variants) {
            int ord = u8();
            if (ord >= variants.size()) {
                throw new IllegalArgumentException("enum ordinal " + ord + " out of range");
            }
            return variants.get(ord);
        }

        public List<String> tags() {
            int n = buf.getInt();
            var out = new ArrayList<String>(n);
            for (int i = 0; i < n; i++) out.add(str());
            return out;
        }

        /** The boxed Long is how {@code optional<int64>} says "absent". */
        public Long optionalI64() {
            return u8() == 0 ? null : buf.getLong();
        }
    }

    public static void putU32(ByteArrayOutputStream out, int v) {
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(v).array());
    }

    public static void putU64(ByteArrayOutputStream out, long v) {
        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(v).array());
    }

    public static void putStr(ByteArrayOutputStream out, String s) {
        var body = s.getBytes(StandardCharsets.UTF_8);
        putU32(out, body.length);
        out.writeBytes(body);
    }

    public static void putEnum(ByteArrayOutputStream out, List<String> variants, String name) {
        int ord = variants.indexOf(name);
        if (ord < 0) throw new IllegalArgumentException("\"" + name + "\" is not one of " + variants);
        out.write(ord);
    }

    public static void putTags(ByteArrayOutputStream out, List<String> tags) {
        putU32(out, tags.size());
        for (var t : tags) putStr(out, t);
    }

    public static void putOptionalI64(ByteArrayOutputStream out, Long v) {
        if (v == null) {
            out.write(0);
            return;
        }
        out.write(1);
        putU64(out, v);
    }

    private Wire() {}
}
