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

"""`CustomerRecord` mirrors examples/cases/customer.yaml — a store account.

`Address` and `Money` mirror the reusable address.yaml and money.yaml it
imports; order.py reuses both, as the Go worker does.

This is the only type in the suite carrying two formats, and the second one is
the point: `binary` is a layout this file writes by hand, while `json` goes
through the stdlib encoder, so the two exercise completely different failure
modes. Both declare `oracle: semantic`, so what has to match the reference is
the decoded value, not the bytes — which is what makes a second format
affordable at all. Go's encoder HTML-escapes `<`, `>` and `&` and always
escapes U+2028/U+2029; no other language does, and under a byte oracle every
worker would have to reproduce that quirk.

`ensure_ascii=True` (the json default) is deliberate here: escaping every
non-ASCII character to \\uXXXX sidesteps the question of whose escaping rules
win, and Go's decoder reads the escapes back — including the surrogate pairs the
emoji in the `unicode` case turn into.

Go is the --ref language and owns the byte layout; see examples/go/wire.go.
"""

import base64
import json
import struct
from dataclasses import dataclass

from serify import serify_model


def pack_str(s: str) -> bytes:
    """u32 byte length, then the UTF-8 bytes. Public because order.py imports it
    along with Address and Money."""
    raw = s.encode('utf-8')
    return struct.pack('<I', len(raw)) + raw


def unpack_str(data: bytes, pos: int):
    n = struct.unpack_from('<I', data, pos)[0]
    pos += 4
    return data[pos:pos + n].decode('utf-8'), pos + n


@serify_model
@dataclass
class Address:
    recipient: str
    street: str
    city: str
    country: str
    postal_code: str

    def pack(self) -> bytes:
        """A struct is its fields back to back, in schema order — no count, no
        length, nothing framing it."""
        return (pack_str(self.recipient) + pack_str(self.street)
                + pack_str(self.city) + pack_str(self.country)
                + pack_str(self.postal_code))

    @classmethod
    def unpack(cls, data: bytes, pos: int):
        recipient, pos = unpack_str(data, pos)
        street, pos = unpack_str(data, pos)
        city, pos = unpack_str(data, pos)
        country, pos = unpack_str(data, pos)
        postal_code, pos = unpack_str(data, pos)
        return cls(recipient, street, city, country, postal_code), pos


@serify_model
@dataclass
class Money:
    currency: str
    amount_minor: int

    def pack(self) -> bytes:
        return pack_str(self.currency) + struct.pack('<q', self.amount_minor)

    @classmethod
    def unpack(cls, data: bytes, pos: int):
        currency, pos = unpack_str(data, pos)
        amount_minor = struct.unpack_from('<q', data, pos)[0]
        return cls(currency, amount_minor), pos + 8


@serify_model
@dataclass
class CustomerRecord:
    customer_id: int
    email: str
    display_name: str
    age: int
    email_verified: bool
    fraud_score: float
    loyalty_points: int
    signup_ts: int
    avatar_sha256: bytes
    pin: "list[int]"
    referral_code: "str | None"
    store_credit: Money
    shipping_addresses: "list[Address]"
    address_book: "dict[str, Address]"
    wishlist_skus: "list[str]"
    preferences: "dict[str, str]"

    def marshal(self) -> bytes:
        buf = bytearray()
        buf += struct.pack('<Q', self.customer_id)
        buf += pack_str(self.email)
        buf += pack_str(self.display_name)
        buf += struct.pack('<BB', self.age, 1 if self.email_verified else 0)
        buf += struct.pack('<fIq', self.fraud_score, self.loyalty_points, self.signup_ts)

        buf += struct.pack('<I', len(self.avatar_sha256)) + self.avatar_sha256

        # array<T,N> carries no count: N is fixed by the schema.
        buf += bytes(self.pin)

        # optional<string>: a presence flag, then the value if present. An empty
        # string is present, which is why the flag cannot be inferred from it.
        if self.referral_code is None:
            buf += struct.pack('<B', 0)
        else:
            buf += struct.pack('<B', 1) + pack_str(self.referral_code)

        buf += self.store_credit.pack()

        buf += struct.pack('<I', len(self.shipping_addresses))
        for a in self.shipping_addresses:
            buf += a.pack()

        # Entry order is the dict's own — deliberately not sorted. A map is
        # unordered, so customer declares `oracle: semantic` and the decoded
        # value is what gets compared. See docs/protocol.md.
        buf += struct.pack('<I', len(self.address_book))
        for k, a in self.address_book.items():
            buf += pack_str(k) + a.pack()

        buf += struct.pack('<I', len(self.wishlist_skus))
        for s in self.wishlist_skus:
            buf += pack_str(s)

        buf += struct.pack('<I', len(self.preferences))
        for k, v in self.preferences.items():
            buf += pack_str(k) + pack_str(v)

        return bytes(buf)

    @classmethod
    def unmarshal(cls, data: bytes) -> "CustomerRecord":
        pos = 0
        customer_id = struct.unpack_from('<Q', data, pos)[0]
        pos += 8

        email, pos = unpack_str(data, pos)
        display_name, pos = unpack_str(data, pos)

        age, verified, fraud_score, loyalty_points, signup_ts = struct.unpack_from('<BBfIq', data, pos)
        pos += 2 + 4 + 4 + 8

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        avatar_sha256 = bytes(data[pos:pos + n])
        pos += n

        pin = list(data[pos:pos + 4])
        pos += 4

        if data[pos] == 0:
            referral_code = None
            pos += 1
        else:
            referral_code, pos = unpack_str(data, pos + 1)

        store_credit, pos = Money.unpack(data, pos)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        shipping_addresses = []
        for _ in range(n):
            a, pos = Address.unpack(data, pos)
            shipping_addresses.append(a)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        address_book = {}
        for _ in range(n):
            k, pos = unpack_str(data, pos)
            address_book[k], pos = Address.unpack(data, pos)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        wishlist_skus = []
        for _ in range(n):
            s, pos = unpack_str(data, pos)
            wishlist_skus.append(s)

        n = struct.unpack_from('<I', data, pos)[0]
        pos += 4
        preferences = {}
        for _ in range(n):
            k, pos = unpack_str(data, pos)
            preferences[k], pos = unpack_str(data, pos)

        return cls(
            customer_id=customer_id, email=email, display_name=display_name,
            age=age, email_verified=verified != 0, fraud_score=fraud_score,
            loyalty_points=loyalty_points, signup_ts=signup_ts,
            avatar_sha256=avatar_sha256, pin=pin, referral_code=referral_code,
            store_credit=store_credit, shipping_addresses=shipping_addresses,
            address_book=address_book, wishlist_skus=wishlist_skus,
            preferences=preferences,
        )

    def to_json(self) -> bytes:
        """`bytes` is base64 in JSON, because that is what the reference worker's
        `[]byte` marshals to and the semantic oracle decodes our output with it."""
        obj = {
            "customer_id": self.customer_id,
            "email": self.email,
            "display_name": self.display_name,
            "age": self.age,
            "email_verified": self.email_verified,
            "fraud_score": self.fraud_score,
            "loyalty_points": self.loyalty_points,
            "signup_ts": self.signup_ts,
            "avatar_sha256": base64.b64encode(self.avatar_sha256).decode('ascii'),
            "pin": list(self.pin),
            "referral_code": self.referral_code,
            "store_credit": {
                "currency": self.store_credit.currency,
                "amount_minor": self.store_credit.amount_minor,
            },
            "shipping_addresses": [_addr_json(a) for a in self.shipping_addresses],
            "address_book": {k: _addr_json(a) for k, a in self.address_book.items()},
            "wishlist_skus": list(self.wishlist_skus),
            "preferences": dict(self.preferences),
        }
        return json.dumps(obj, separators=(',', ':')).encode('ascii')

    @classmethod
    def from_json(cls, data: bytes) -> "CustomerRecord":
        o = json.loads(data.decode('utf-8'))
        return cls(
            customer_id=o["customer_id"],
            email=o["email"],
            display_name=o["display_name"],
            age=o["age"],
            email_verified=o["email_verified"],
            # A JSON number is a double; narrow it the way the wire does, so a
            # float32 field holds a value float32 can actually represent.
            fraud_score=struct.unpack('<f', struct.pack('<f', o["fraud_score"]))[0],
            loyalty_points=o["loyalty_points"],
            signup_ts=o["signup_ts"],
            avatar_sha256=base64.b64decode(o["avatar_sha256"] or ""),
            pin=list(o["pin"]),
            referral_code=o["referral_code"],
            store_credit=Money(o["store_credit"]["currency"], o["store_credit"]["amount_minor"]),
            shipping_addresses=[_addr_from_json(a) for a in (o["shipping_addresses"] or [])],
            address_book={k: _addr_from_json(a) for k, a in (o["address_book"] or {}).items()},
            wishlist_skus=list(o["wishlist_skus"] or []),
            preferences=dict(o["preferences"] or {}),
        )


def _addr_json(a: Address) -> dict:
    return {
        "recipient": a.recipient, "street": a.street, "city": a.city,
        "country": a.country, "postal_code": a.postal_code,
    }


def _addr_from_json(o: dict) -> Address:
    return Address(o["recipient"], o["street"], o["city"], o["country"], o["postal_code"])
