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

import io.serify.WorkerLib;
import io.serify.WorkerLib.SerifyField;

import java.io.ByteArrayOutputStream;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;

/**
 * Mirrors examples/cases/notification.yaml, whose {@code channel} field is a
 * {@code sum}.
 *
 * <p>Java's sum type is a sealed interface, and that is all the binding needs:
 * {@code permits} names the arms and each arm's record components give its
 * payload. No converter, no registration — and the compiler will not let a
 * notification carry two targets at once.
 *
 * <p>Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */
@WorkerLib.SerifyModel
public final class Notification {

    /** The {@code sum} from the case file. */
    public sealed interface Channel permits Silent, Sms, Push, Invoice {}

    public record Silent() implements Channel {}                      // arity 0 — a unit variant
    public record Sms(String value) implements Channel {}             // arity 1 — a scalar payload
    public record Push(Long value) implements Channel {}              // arity 1 — exceeds 2^53
    public record Invoice(String currency, Long amountMinor)          // arity N — a struct payload
            implements Channel {}

    @SerifyField public Integer notificationId = 0;
    @SerifyField public Channel channel = new Silent();
    @SerifyField public Boolean urgent = false;

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN)
                .putInt(notificationId).array());

        // The tag ordinal is the variant's position in the case file's sum,
        // which is the declaration order of the four arms above. The schema tag
        // *names* are the binding's business, and never appear here.
        //
        // An if/else chain rather than a switch: pattern matching for switch is
        // Java 21, and this project targets 17, where only `instanceof` patterns
        // are final. That also costs the exhaustiveness check a sealed switch
        // would give, hence the explicit final else.
        if (channel instanceof Silent) {
            out.write(0);                                     // a unit variant is nothing but its tag
        } else if (channel instanceof Sms s) {
            out.write(1);
            Wire.writeLenPrefixed(out, s.value());
        } else if (channel instanceof Push p) {
            out.write(2);
            out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(p.value()).array());
        } else if (channel instanceof Invoice i) {
            out.write(3);
            Wire.writeLenPrefixed(out, i.currency());
            out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN)
                    .putLong(i.amountMinor()).array());
        } else {
            throw new IllegalArgumentException("unhandled channel " + channel);
        }

        out.write(urgent ? 1 : 0);
        return out.toByteArray();
    }

    public static Notification unmarshal(byte[] data) {
        var buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        var n = new Notification();

        n.notificationId = buf.getInt();
        n.channel = switch (buf.get() & 0xFF) {
            case 0 -> new Silent();
            case 1 -> new Sms(Wire.readLenString(buf));
            case 2 -> new Push(buf.getLong());
            case 3 -> new Invoice(Wire.readLenString(buf), buf.getLong());
            default -> throw new IllegalArgumentException("unknown channel ordinal");
        };
        n.urgent = buf.get() != 0;
        return n;
    }
}
