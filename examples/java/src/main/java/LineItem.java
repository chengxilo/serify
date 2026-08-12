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

import io.serify.WorkerLib;

/**
 * Mirrors the reusable examples/cases/line_item.yaml, imported by order.
 *
 * <p>Its own {@code unit_price} is a money, which makes any type using it the
 * suite's only struct-inside-struct.
 */
@WorkerLib.SerifyModel
public final class LineItem {

    @WorkerLib.SerifyField public String sku = "";
    @WorkerLib.SerifyField("product_name") public String productName = "";
    @WorkerLib.SerifyField public short quantity;
    @WorkerLib.SerifyField("unit_price") public Money unitPrice = new Money();
    @WorkerLib.SerifyField("discount_pct") public byte discountPct;
    @WorkerLib.SerifyField("gift_wrap") public boolean giftWrap;

    public void pack(ByteArrayOutputStream out) {
        Wire.writeLenPrefixed(out, sku);
        Wire.writeLenPrefixed(out, productName);
        out.writeBytes(ByteBuffer.allocate(2).order(ByteOrder.LITTLE_ENDIAN).putShort(quantity).array());
        unitPrice.pack(out);
        out.write(discountPct);
        out.write(giftWrap ? 1 : 0);
    }

    public static LineItem unpack(ByteBuffer buf) {
        var it = new LineItem();
        it.sku = Wire.readLenString(buf);
        it.productName = Wire.readLenString(buf);
        it.quantity = buf.getShort();
        it.unitPrice = Money.unpack(buf);
        it.discountPct = buf.get();
        it.giftWrap = buf.get() != 0;
        return it;
    }
}
