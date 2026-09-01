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


import java.io.DataInputStream;
import java.io.EOFException;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;

/**
 * Framing: a u32 little-endian byte length, then that many bytes.
 *
 * <p>Outside the conformance suite on purpose. serify tests the contents of a
 * message; the frame header is in none of the cases, so a follower may read its
 * socket however it likes as long as it agrees on what is inside.
 */
public final class Frame {

    /** Caps a single message, so a bad length prefix is not a 4 GiB allocation. */
    public static final int MAX_FRAME_LEN = 1 << 20;

    public static void write(OutputStream out, byte[] payload) throws IOException {
        if (payload.length > MAX_FRAME_LEN) {
            throw new IOException("message of " + payload.length + " bytes exceeds the limit");
        }
        out.write(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(payload.length).array());
        out.write(payload);
        out.flush();
    }

    public static byte[] read(InputStream in) throws IOException {
        var data = new DataInputStream(in);
        var header = new byte[4];
        data.readFully(header);

        // Read as an int and widen: a frame is capped well under 2^31, so a
        // negative here is a length prefix that has no business being one.
        int n = ByteBuffer.wrap(header).order(ByteOrder.LITTLE_ENDIAN).getInt();
        if (n < 0 || n > MAX_FRAME_LEN) {
            throw new IOException("frame header claims " + Integer.toUnsignedString(n)
                    + " bytes, over the " + MAX_FRAME_LEN + " limit");
        }

        var payload = new byte[n];
        try {
            data.readFully(payload);
        } catch (EOFException e) {
            throw new IOException("connection closed mid-frame", e);
        }
        return payload;
    }

    private Frame() {}
}
