# Copyright 2026 Chengxi Luo
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""`OrderRecord` mirrors examples/cases/order.yaml — a placed order.

`LineItem` mirrors the reusable line_item.yaml it imports, which itself nests
money — so any type using it exercises struct-inside-struct. `Address` and
`Money` come from customer.py, as they do in the Go worker.

Between them the fields cover the four composite types nothing else in the suite
exercises end to end: an `enum`, a `list<struct>`, a `map<string,struct>` and an
`optional<struct>`.

An enum needs nothing from the library: it travels as its variant *name*, so the
model holds a plain `str`. The u8 ordinal below is this worker's own byte-layout
choice, which is why _STATUSES has to match the case file's declaration order.

Go is the --ref language and owns the layout; see examples/go/wire.go.
"""

import struct
from dataclasses import dataclass

from customer import Address, Money, pack_str, unpack_str
from serify import serify_model

# Declaration order of the `status` enum in examples/cases/order.yaml.
_STATUSES = ["pending", "paid", "shipped", "delivered", "cancelled"]


@serify_model
@dataclass
class LineItem:
    sku: str
    product_name: str
    quantity: int
    unit_price: Money
    discount_pct: int
    gift_wrap: bool

    def pack(self) -> bytes:
        return (pack_str(self.sku) + pack_str(self.product_name)
                + struct.pack('<H', self.quantity)
                + self.unit_price.pack()
                + struct.pack('<BB', self.discount_pct, 1 if self.gift_wrap else 0))

    @classmethod
    def unpack(cls, data: bytes, pos: int):
        sku, pos = unpack_str(data, pos)
        product_name, pos = unpack_str(data, pos)
        quantity = struct.unpack_from('<H', data, pos)[0]
        pos += 2
        unit_price, pos = Money.unpack(data, pos)
        discount_pct, gift_wrap = struct.unpack_from('<BB', data, pos)
        return cls(sku, product_name, quantity, unit_price, discount_pct, gift_wrap != 0), pos + 2


@serify_model
@dataclass
class OrderRecord:
    order_id: int
    customer_id: int
    created_at: int
    status: str
    items: "list[LineItem]"
    subtotal: Money
    adjustments: "dict[str, Money]"
    total: Money
    shipping_address: Address
    billing_address: "Address | None"
    coupon_codes: "list[str]"
    tracking_number: "str | None"

    def marshal(self) -> bytes:
        buf = bytearray()
        buf += struct.pack('<QQq', self.order_id, self.customer_id, self.created_at)

        # enum: a u8 ordinal, the variant's position in the case file.
        buf += struct.pack('<B', _STATUSES.index(self.status))

        buf += struct.pack('<I', len(self.items))
        for it in self.items:
            buf += it.pack()

        buf += self.subtotal.pack()

        # Entry order is the dict's own — deliberately not sorted. A map is
        # unordered, so order declares `oracle: semantic` and the decoded value
        # is what gets compared. See docs/protocol.md.
        buf += struct.pack('<I', len(self.adjustments))
        for k, m in self.adjustments.items():
            buf += pack_str(k) + m.pack()

        buf += self.total.pack()
        buf += self.shipping_address.pack()

        # optional<struct>: a presence flag, then the struct's fields inline.
        if self.billing_address is None:
            buf += struct.pack('<B', 0)
        else:
            buf += struct.pack('<B', 1) + self.billing_address.pack()

        buf += struct.pack('<I', len(self.coupon_codes))
        for c in self.coupon_codes:
            buf += pack_str(c)

        if self.tracking_number is None:
            buf += struct.pack('<B', 0)
        else:
            buf += struct.pack('<B', 1) + pack_str(self.tracking_number)

        return bytes(buf)

    @classmethod
    def unmarshal(cls, data: bytes) -> "OrderRecord":
        order_id, customer_id, created_at, status_ord = struct.unpack_from('<QQqB', data, 0)
        pos = 25

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        items = []
        for _ in range(n):
            it, pos = LineItem.unpack(data, pos)
            items.append(it)

        subtotal, pos = Money.unpack(data, pos)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        adjustments = {}
        for _ in range(n):
            k, pos = unpack_str(data, pos)
            adjustments[k], pos = Money.unpack(data, pos)

        total, pos = Money.unpack(data, pos)
        shipping_address, pos = Address.unpack(data, pos)

        if data[pos] == 0:
            billing_address = None
            pos += 1
        else:
            billing_address, pos = Address.unpack(data, pos + 1)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        coupon_codes = []
        for _ in range(n):
            c, pos = unpack_str(data, pos)
            coupon_codes.append(c)

        if data[pos] == 0:
            tracking_number = None
        else:
            tracking_number, pos = unpack_str(data, pos + 1)

        return cls(
            order_id=order_id, customer_id=customer_id, created_at=created_at,
            status=_STATUSES[status_ord], items=items, subtotal=subtotal,
            adjustments=adjustments, total=total, shipping_address=shipping_address,
            billing_address=billing_address, coupon_codes=coupon_codes,
            tracking_number=tracking_number,
        )
