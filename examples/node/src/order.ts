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

/**
 * `OrderRecord` mirrors examples/cases/order.yaml — a placed order.
 *
 * `LineItem` mirrors the reusable line_item.yaml it imports, which itself nests
 * money — so any type using it exercises struct-inside-struct. `Address`,
 * `Money` and the buffer cursor come from customer.ts, as they do in the Go
 * worker.
 *
 * Between them the fields cover the four composite types nothing else in the
 * suite exercises end to end: an `enum`, a `list<struct>`, a
 * `map<string,struct>` and an `optional<struct>`. `billing_address` is the
 * suite's only optional<struct>, and it is what proves the binding's `model:`
 * handles a null as well as a value.
 *
 * An enum needs nothing from the binding: it travels as its variant *name*, so
 * the property is a plain string. The u8 ordinal in the layout is this worker's
 * own choice, which is why STATUSES has to match the case file's declaration
 * order.
 *
 * Go is the --ref language and owns the layout; see examples/go/wire.go.
 */

import { Serify } from '@chengxilo/serify';

import { Address, Money, Reader } from './customer';
import { lenPrefixedStr } from './wire';

/** Declaration order of the `status` enum in examples/cases/order.yaml. */
const STATUSES = ['pending', 'paid', 'shipped', 'delivered', 'cancelled'];

@Serify.Model()
export class LineItem {
  @Serify.field() sku = '';
  @Serify.field() product_name = '';
  @Serify.field() quantity = 0;
  @Serify.field({ model: Money }) unit_price = new Money();
  @Serify.field() discount_pct = 0;
  @Serify.field() gift_wrap = false;

  pack(): Buffer {
    const qty = Buffer.alloc(2);
    qty.writeUInt16LE(this.quantity, 0);
    return Buffer.concat([
      lenPrefixedStr(this.sku), lenPrefixedStr(this.product_name), qty,
      this.unit_price.pack(),
      Buffer.from([this.discount_pct, this.gift_wrap ? 1 : 0]),
    ]);
  }

  static unpack(r: Reader): LineItem {
    const it = new LineItem();
    it.sku = r.str();
    it.product_name = r.str();
    it.quantity = r.buf.readUInt16LE(r.off);
    r.off += 2;
    it.unit_price = Money.unpack(r);
    it.discount_pct = r.u8();
    it.gift_wrap = r.u8() !== 0;
    return it;
  }
}

@Serify.Model()
export class OrderRecord {
  @Serify.field() order_id = 0n;
  @Serify.field() customer_id = 0n;
  @Serify.field() created_at = 0n;
  @Serify.field() status = '';
  @Serify.field({ model: LineItem }) items: LineItem[] = [];
  @Serify.field({ model: Money }) subtotal = new Money();
  @Serify.field({ model: Money }) adjustments = new Map<string, Money>();
  @Serify.field({ model: Money }) total = new Money();
  @Serify.field({ model: Address }) shipping_address = new Address();
  @Serify.field({ model: Address }) billing_address: Address | null = null;
  @Serify.field() coupon_codes: string[] = [];
  @Serify.field() tracking_number: string | null = null;

  marshal(): Buffer {
    const head = Buffer.alloc(25);
    head.writeBigUInt64LE(this.order_id, 0);
    head.writeBigUInt64LE(this.customer_id, 8);
    head.writeBigInt64LE(this.created_at, 16);

    // enum: a u8 ordinal, the variant's position in the case file.
    const ord = STATUSES.indexOf(this.status);
    if (ord < 0) throw new Error(`unknown order status "${this.status}"`);
    head.writeUInt8(ord, 24);

    const parts: Buffer[] = [head, count(this.items.length)];
    for (const it of this.items) parts.push(it.pack());

    parts.push(this.subtotal.pack());

    // Entry order is the Map's own — deliberately not sorted. A map is
    // unordered, so order declares `oracle: semantic` and the decoded value is
    // what gets compared. See docs/protocol.md.
    parts.push(count(this.adjustments.size));
    for (const [k, m] of this.adjustments) parts.push(lenPrefixedStr(k), m.pack());

    parts.push(this.total.pack(), this.shipping_address.pack());

    // optional<struct>: a presence flag, then the struct's fields inline.
    if (this.billing_address === null) parts.push(Buffer.from([0]));
    else parts.push(Buffer.from([1]), this.billing_address.pack());

    parts.push(count(this.coupon_codes.length));
    for (const c of this.coupon_codes) parts.push(lenPrefixedStr(c));

    if (this.tracking_number === null) parts.push(Buffer.from([0]));
    else parts.push(Buffer.from([1]), lenPrefixedStr(this.tracking_number));

    return Buffer.concat(parts);
  }

  static unmarshal(data: Buffer): OrderRecord {
    const o = new OrderRecord();
    const r = new Reader(data);

    o.order_id = data.readBigUInt64LE(0);
    o.customer_id = data.readBigUInt64LE(8);
    o.created_at = data.readBigInt64LE(16);
    const ord = data.readUInt8(24);
    if (ord >= STATUSES.length) throw new Error(`status ordinal ${ord} is out of range`);
    o.status = STATUSES[ord];
    r.off = 25;

    o.items = [];
    for (let n = r.u32(); n > 0; n--) o.items.push(LineItem.unpack(r));

    o.subtotal = Money.unpack(r);

    o.adjustments = new Map();
    for (let n = r.u32(); n > 0; n--) o.adjustments.set(r.str(), Money.unpack(r));

    o.total = Money.unpack(r);
    o.shipping_address = Address.unpack(r);
    o.billing_address = r.u8() === 0 ? null : Address.unpack(r);

    o.coupon_codes = [];
    for (let n = r.u32(); n > 0; n--) o.coupon_codes.push(r.str());

    o.tracking_number = r.u8() === 0 ? null : r.str();

    return o;
  }
}

function count(n: number): Buffer {
  const b = Buffer.alloc(4);
  b.writeUInt32LE(n, 0);
  return b;
}
