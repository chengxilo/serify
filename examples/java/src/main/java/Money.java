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

import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.databind.JsonNode;

import io.serify.WorkerLib;

/** Mirrors the reusable examples/cases/money.yaml, imported by customer and order. */
@WorkerLib.SerifyModel
public final class Money {

    @WorkerLib.SerifyField public String currency = "";
    @WorkerLib.SerifyField("amount_minor") public long amountMinor;

    public void pack(ByteArrayOutputStream out) {
        Wire.writeLenPrefixed(out, currency);
        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(amountMinor).array());
    }

    public static Money unpack(ByteBuffer buf) {
        var m = new Money();
        m.currency = Wire.readLenString(buf);
        m.amountMinor = buf.getLong();
        return m;
    }

    public ObjectNode toJson() {
        var o = JsonNodeFactory.instance.objectNode();
        o.put("currency", currency);
        o.put("amount_minor", amountMinor);
        return o;
    }

    public static Money fromJson(JsonNode n) {
        var m = new Money();
        m.currency = n.get("currency").asText();
        m.amountMinor = n.get("amount_minor").asLong();
        return m;
    }
}
