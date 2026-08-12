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
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import com.fasterxml.jackson.databind.node.ObjectNode;

import io.serify.WorkerLib;

/**
 * Mirrors examples/cases/customer.yaml — a store account.
 *
 * <p>This is the only type in the suite carrying two formats, and the second one
 * is the point: {@code binary} is a layout written by hand below, {@code json}
 * goes through Jackson, so the two fail in completely different ways. Both
 * declare {@code oracle: semantic}, so what has to match the reference is the
 * decoded value rather than the bytes — Go's encoder HTML-escapes {@code <},
 * {@code >} and {@code &} and always escapes U+2028/U+2029, and under a byte
 * oracle every worker would have to reproduce that quirk.
 *
 * <p>It is also the first Java model with nested structs. Nothing here declares
 * that: a field typed with another {@code @SerifyModel} is a struct, a
 * {@code List} of them a list&lt;struct&gt;, a {@code Map} of them a
 * map&lt;K,struct&gt;. The binding reads the element type off the field's
 * generic signature, which survives erasure.
 *
 * <p>Java has no unsigned primitives, so {@code customerId} is a {@code long}
 * holding the unsigned value — at max uint64 it reads as -1. That is invisible
 * on the wire and unmissable in JSON, where the number has to be written out in
 * full; see toJson.
 *
 * <p>Go is the --ref language and owns the byte layout; see examples/go/wire.go.
 */
@WorkerLib.SerifyModel
public final class CustomerRecord {

    @WorkerLib.SerifyField("customer_id") public long customerId;
    @WorkerLib.SerifyField public String email = "";
    @WorkerLib.SerifyField("display_name") public String displayName = "";
    @WorkerLib.SerifyField public byte age;
    @WorkerLib.SerifyField("email_verified") public boolean emailVerified;
    @WorkerLib.SerifyField("fraud_score") public float fraudScore;
    @WorkerLib.SerifyField("loyalty_points") public int loyaltyPoints;
    @WorkerLib.SerifyField("signup_ts") public long signupTs;
    @WorkerLib.SerifyField("avatar_sha256") public byte[] avatarSha256 = new byte[0];
    @WorkerLib.SerifyField public List<Byte> pin = List.of();
    @WorkerLib.SerifyField("referral_code") public String referralCode;
    @WorkerLib.SerifyField("store_credit") public Money storeCredit = new Money();
    @WorkerLib.SerifyField("shipping_addresses") public List<Address> shippingAddresses = List.of();
    @WorkerLib.SerifyField("address_book") public Map<String, Address> addressBook = Map.of();
    @WorkerLib.SerifyField("wishlist_skus") public List<String> wishlistSkus = List.of();
    @WorkerLib.SerifyField public Map<String, String> preferences = Map.of();

    private static void writeCount(ByteArrayOutputStream out, int n) {
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(n).array());
    }

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();

        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(customerId).array());
        Wire.writeLenPrefixed(out, email);
        Wire.writeLenPrefixed(out, displayName);

        out.write(age);
        out.write(emailVerified ? 1 : 0);
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putFloat(fraudScore).array());
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(loyaltyPoints).array());
        out.writeBytes(ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN).putLong(signupTs).array());

        Wire.writeLenPrefixed(out, avatarSha256);

        // array<T,N> carries no count: N is fixed by the schema.
        for (Byte b : pin) out.write(b);

        // optional<string>: a presence flag, then the value if present. An empty
        // string is present, which is why the flag cannot be inferred from it.
        if (referralCode == null) {
            out.write(0);
        } else {
            out.write(1);
            Wire.writeLenPrefixed(out, referralCode);
        }

        storeCredit.pack(out);

        writeCount(out, shippingAddresses.size());
        for (Address a : shippingAddresses) a.pack(out);

        // Entry order is the map's own — deliberately not sorted. A map is
        // unordered, so customer declares `oracle: semantic` and the decoded
        // value is what gets compared. See docs/protocol.md.
        writeCount(out, addressBook.size());
        for (Map.Entry<String, Address> e : addressBook.entrySet()) {
            Wire.writeLenPrefixed(out, e.getKey());
            e.getValue().pack(out);
        }

        writeCount(out, wishlistSkus.size());
        for (String s : wishlistSkus) Wire.writeLenPrefixed(out, s);

        writeCount(out, preferences.size());
        for (Map.Entry<String, String> e : preferences.entrySet()) {
            Wire.writeLenPrefixed(out, e.getKey());
            Wire.writeLenPrefixed(out, e.getValue());
        }

        return out.toByteArray();
    }

    public static CustomerRecord unmarshal(byte[] data) {
        var buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        var c = new CustomerRecord();

        c.customerId = buf.getLong();
        c.email = Wire.readLenString(buf);
        c.displayName = Wire.readLenString(buf);

        c.age = buf.get();
        c.emailVerified = buf.get() != 0;
        c.fraudScore = buf.getFloat();
        c.loyaltyPoints = buf.getInt();
        c.signupTs = buf.getLong();

        c.avatarSha256 = Wire.readLenPrefixed(buf);

        c.pin = new ArrayList<>();
        for (int i = 0; i < 4; i++) c.pin.add(buf.get());

        c.referralCode = buf.get() == 0 ? null : Wire.readLenString(buf);

        c.storeCredit = Money.unpack(buf);

        c.shippingAddresses = new ArrayList<>();
        for (int n = buf.getInt(); n > 0; n--) c.shippingAddresses.add(Address.unpack(buf));

        c.addressBook = new LinkedHashMap<>();
        for (int n = buf.getInt(); n > 0; n--) {
            String k = Wire.readLenString(buf);
            c.addressBook.put(k, Address.unpack(buf));
        }

        c.wishlistSkus = new ArrayList<>();
        for (int n = buf.getInt(); n > 0; n--) c.wishlistSkus.add(Wire.readLenString(buf));

        c.preferences = new LinkedHashMap<>();
        for (int n = buf.getInt(); n > 0; n--) {
            String k = Wire.readLenString(buf);
            c.preferences.put(k, Wire.readLenString(buf));
        }

        return c;
    }

    private static final ObjectMapper JSON = new ObjectMapper();

    /**
     * {@code bytes} is base64 and the 64-bit integers are JSON numbers: that is
     * what the reference worker's {@code []byte} and {@code uint64} marshal to,
     * and the semantic oracle decodes our output with it.
     *
     * <p>{@code customer_id} goes out through a BigInteger built from the
     * unsigned decimal, because Java's long would otherwise write max uint64 as
     * -1. Same for the {@code pin} bytes, which are 0..255 on the wire and
     * -128..127 in a Java byte.
     */
    public byte[] toJson() {
        var o = JsonNodeFactory.instance.objectNode();
        o.put("customer_id", new BigInteger(Long.toUnsignedString(customerId)));
        o.put("email", email);
        o.put("display_name", displayName);
        o.put("age", age & 0xFF);
        o.put("email_verified", emailVerified);
        o.put("fraud_score", fraudScore);
        o.put("loyalty_points", loyaltyPoints & 0xFFFFFFFFL);
        o.put("signup_ts", signupTs);
        o.put("avatar_sha256", avatarSha256);

        ArrayNode pinNode = o.putArray("pin");
        for (Byte b : pin) pinNode.add(b & 0xFF);

        if (referralCode == null) o.putNull("referral_code");
        else o.put("referral_code", referralCode);

        o.set("store_credit", storeCredit.toJson());

        ArrayNode addrs = o.putArray("shipping_addresses");
        for (Address a : shippingAddresses) addrs.add(a.toJson());

        ObjectNode book = o.putObject("address_book");
        for (Map.Entry<String, Address> e : addressBook.entrySet()) book.set(e.getKey(), e.getValue().toJson());

        ArrayNode skus = o.putArray("wishlist_skus");
        for (String s : wishlistSkus) skus.add(s);

        ObjectNode prefs = o.putObject("preferences");
        for (Map.Entry<String, String> e : preferences.entrySet()) prefs.put(e.getKey(), e.getValue());

        try {
            return JSON.writeValueAsBytes(o);
        } catch (Exception e) {
            throw new RuntimeException("customer json encode", e);
        }
    }

    public static CustomerRecord fromJson(byte[] data) {
        try {
            return parseJson(data);
        } catch (Exception e) {
            throw new RuntimeException("customer json decode", e);
        }
    }

    private static CustomerRecord parseJson(byte[] data) throws Exception {
        JsonNode o = JSON.readTree(data);
        var c = new CustomerRecord();

        // bigIntegerValue(), not asLong(): Jackson clamps an out-of-range long,
        // and max uint64 is out of range for every one of them.
        c.customerId = o.get("customer_id").bigIntegerValue().longValue();
        c.email = o.get("email").asText();
        c.displayName = o.get("display_name").asText();
        c.age = (byte) o.get("age").asInt();
        c.emailVerified = o.get("email_verified").asBoolean();
        c.fraudScore = o.get("fraud_score").floatValue();
        c.loyaltyPoints = (int) o.get("loyalty_points").asLong();
        c.signupTs = o.get("signup_ts").asLong();
        c.avatarSha256 = o.get("avatar_sha256").binaryValue();

        c.pin = new ArrayList<>();
        for (JsonNode n : o.get("pin")) c.pin.add((byte) n.asInt());

        JsonNode ref = o.get("referral_code");
        c.referralCode = ref == null || ref.isNull() ? null : ref.asText();

        c.storeCredit = Money.fromJson(o.get("store_credit"));

        c.shippingAddresses = new ArrayList<>();
        for (JsonNode n : o.get("shipping_addresses")) c.shippingAddresses.add(Address.fromJson(n));

        c.addressBook = new LinkedHashMap<>();
        o.get("address_book").fields()
                .forEachRemaining(e -> c.addressBook.put(e.getKey(), Address.fromJson(e.getValue())));

        c.wishlistSkus = new ArrayList<>();
        for (JsonNode n : o.get("wishlist_skus")) c.wishlistSkus.add(n.asText());

        c.preferences = new LinkedHashMap<>();
        o.get("preferences").fields()
                .forEachRemaining(e -> c.preferences.put(e.getKey(), e.getValue().asText()));

        return c;
    }
}
