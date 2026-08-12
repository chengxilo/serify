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
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import io.serify.WorkerLib;

/**
 * Mirrors examples/cases/order.yaml — a placed order.
 *
 * <p>Between them the fields cover the four composite types nothing else in the
 * suite exercises end to end: an {@code enum}, a {@code list<struct>}, a
 * {@code map<string,struct>} and an {@code optional<struct>}.
 * {@code billingAddress} is the suite's only optional&lt;struct&gt;, so it is
 * what proves the binding's nested-model path hands back a null as well as a
 * value.
 *
 * <p>An enum needs nothing from the binding: it travels as its variant *name*,
 * so the field is a plain {@code String}. The u8 ordinal in the layout is this
 * worker's own choice, which is why STATUSES has to match the case file's
 * declaration order.
 *
 * <p>Go is the --ref language and owns the layout; see examples/go/wire.go.
 */
@WorkerLib.SerifyModel
public final class OrderRecord {

    /** Declaration order of the `status` enum in examples/cases/order.yaml. */
    private static final List<String> STATUSES =
            List.of("pending", "paid", "shipped", "delivered", "cancelled");

    @WorkerLib.SerifyField("order_id") public long orderId;
    @WorkerLib.SerifyField("customer_id") public long customerId;
    @WorkerLib.SerifyField("created_at") public long createdAt;
    @WorkerLib.SerifyField public String status = "";
    @WorkerLib.SerifyField public List<LineItem> items = List.of();
    @WorkerLib.SerifyField public Money subtotal = new Money();
    @WorkerLib.SerifyField public Map<String, Money> adjustments = Map.of();
    @WorkerLib.SerifyField public Money total = new Money();
    @WorkerLib.SerifyField("shipping_address") public Address shippingAddress = new Address();
    @WorkerLib.SerifyField("billing_address") public Address billingAddress;
    @WorkerLib.SerifyField("coupon_codes") public List<String> couponCodes = List.of();
    @WorkerLib.SerifyField("tracking_number") public String trackingNumber;

    private static void writeCount(ByteArrayOutputStream out, int n) {
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(n).array());
    }

    public byte[] marshal() {
        var out = new ByteArrayOutputStream();
        var head = ByteBuffer.allocate(24).order(ByteOrder.LITTLE_ENDIAN);
        head.putLong(orderId).putLong(customerId).putLong(createdAt);
        out.writeBytes(head.array());

        // enum: a u8 ordinal, the variant's position in the case file.
        int ord = STATUSES.indexOf(status);
        if (ord < 0) throw new IllegalStateException("unknown order status \"" + status + "\"");
        out.write(ord);

        writeCount(out, items.size());
        for (LineItem it : items) it.pack(out);

        subtotal.pack(out);

        // Entry order is the map's own — deliberately not sorted. A map is
        // unordered, so order declares `oracle: semantic` and the decoded value
        // is what gets compared. See docs/protocol.md.
        writeCount(out, adjustments.size());
        for (Map.Entry<String, Money> e : adjustments.entrySet()) {
            Wire.writeLenPrefixed(out, e.getKey());
            e.getValue().pack(out);
        }

        total.pack(out);
        shippingAddress.pack(out);

        // optional<struct>: a presence flag, then the struct's fields inline.
        if (billingAddress == null) {
            out.write(0);
        } else {
            out.write(1);
            billingAddress.pack(out);
        }

        writeCount(out, couponCodes.size());
        for (String c : couponCodes) Wire.writeLenPrefixed(out, c);

        if (trackingNumber == null) {
            out.write(0);
        } else {
            out.write(1);
            Wire.writeLenPrefixed(out, trackingNumber);
        }

        return out.toByteArray();
    }

    public static OrderRecord unmarshal(byte[] data) {
        var buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        var o = new OrderRecord();

        o.orderId = buf.getLong();
        o.customerId = buf.getLong();
        o.createdAt = buf.getLong();

        int ord = buf.get() & 0xFF;
        if (ord >= STATUSES.size()) {
            throw new IllegalStateException("status ordinal " + ord + " is out of range");
        }
        o.status = STATUSES.get(ord);

        o.items = new ArrayList<>();
        for (int n = buf.getInt(); n > 0; n--) o.items.add(LineItem.unpack(buf));

        o.subtotal = Money.unpack(buf);

        o.adjustments = new LinkedHashMap<>();
        for (int n = buf.getInt(); n > 0; n--) {
            String k = Wire.readLenString(buf);
            o.adjustments.put(k, Money.unpack(buf));
        }

        o.total = Money.unpack(buf);
        o.shippingAddress = Address.unpack(buf);
        o.billingAddress = buf.get() == 0 ? null : Address.unpack(buf);

        o.couponCodes = new ArrayList<>();
        for (int n = buf.getInt(); n > 0; n--) o.couponCodes.add(Wire.readLenString(buf));

        o.trackingNumber = buf.get() == 0 ? null : Wire.readLenString(buf);

        return o;
    }
}
