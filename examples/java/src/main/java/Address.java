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

import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.databind.JsonNode;

import io.serify.WorkerLib;

/**
 * Mirrors the reusable examples/cases/address.yaml, imported by customer and
 * order. A struct is its fields back to back in schema order — nothing frames
 * it, so pack/unpack take the surrounding stream rather than owning one.
 */
@WorkerLib.SerifyModel
public final class Address {

    @WorkerLib.SerifyField public String recipient = "";
    @WorkerLib.SerifyField public String street = "";
    @WorkerLib.SerifyField public String city = "";
    @WorkerLib.SerifyField public String country = "";
    @WorkerLib.SerifyField("postal_code") public String postalCode = "";

    public void pack(ByteArrayOutputStream out) {
        Wire.writeLenPrefixed(out, recipient);
        Wire.writeLenPrefixed(out, street);
        Wire.writeLenPrefixed(out, city);
        Wire.writeLenPrefixed(out, country);
        Wire.writeLenPrefixed(out, postalCode);
    }

    public static Address unpack(ByteBuffer buf) {
        var a = new Address();
        a.recipient = Wire.readLenString(buf);
        a.street = Wire.readLenString(buf);
        a.city = Wire.readLenString(buf);
        a.country = Wire.readLenString(buf);
        a.postalCode = Wire.readLenString(buf);
        return a;
    }

    public ObjectNode toJson() {
        var o = JsonNodeFactory.instance.objectNode();
        o.put("recipient", recipient);
        o.put("street", street);
        o.put("city", city);
        o.put("country", country);
        o.put("postal_code", postalCode);
        return o;
    }

    public static Address fromJson(JsonNode n) {
        var a = new Address();
        a.recipient = n.get("recipient").asText();
        a.street = n.get("street").asText();
        a.city = n.get("city").asText();
        a.country = n.get("country").asText();
        a.postalCode = n.get("postal_code").asText();
        return a;
    }
}
