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

"""`NotificationRecord` mirrors examples/cases/notification.yaml, whose
`channel` field is a `sum`.

Python has no `enum` with payloads, but a union of dataclasses is its sum type,
and that is all `@serify_model` needs: the union names the arms, and each arm's
own fields give its payload. No converter, no registration — and no way to build
a notification carrying two targets at once.

Go is the --ref language and owns the byte layout; see the comment at the top of
examples/go/wire.go.
"""

import struct
from dataclasses import dataclass

from serify import serify_model


@dataclass
class Silent:
    """arity 0 — a unit variant"""


@dataclass
class Sms:
    """arity 1 — a scalar payload"""

    value: str


@dataclass
class Push:
    """arity 1 — a payload that exceeds 2^53"""

    value: int


@dataclass
class Invoice:
    """arity N — a struct payload"""

    currency: str
    amount_minor: int


@serify_model
@dataclass
class NotificationRecord:
    notification_id: int
    channel: Silent | Sms | Push | Invoice
    urgent: bool

    def marshal(self) -> bytes:
        buf = bytearray(struct.pack("<I", self.notification_id))

        # The tag ordinal is the variant's position in the case file's sum,
        # which is the declaration order of the four arms above. The schema tag
        # *names* are the binding's business, and never appear here.
        match self.channel:
            case Silent():
                buf += b"\x00"  # a unit variant is nothing but its tag
            case Sms(value=s):
                buf += b"\x01" + _len_str(s)
            case Push(value=n):
                buf += b"\x02" + struct.pack("<Q", n)
            case Invoice(currency=c, amount_minor=a):
                buf += b"\x03" + _len_str(c) + struct.pack("<q", a)

        buf += struct.pack("<?", self.urgent)
        return bytes(buf)

    @classmethod
    def unmarshal(cls, data: bytes) -> "NotificationRecord":
        (notification_id,) = struct.unpack_from("<I", data, 0)
        off = 5

        def read_len_str(off: int):
            (n,) = struct.unpack_from("<I", data, off)
            off += 4
            return data[off:off + n].decode(), off + n

        match data[4]:
            case 0:
                channel = Silent()
            case 1:
                s, off = read_len_str(off)
                channel = Sms(s)
            case 2:
                (n,) = struct.unpack_from("<Q", data, off)
                off += 8
                channel = Push(n)
            case 3:
                currency, off = read_len_str(off)
                (amount_minor,) = struct.unpack_from("<q", data, off)
                off += 8
                channel = Invoice(currency, amount_minor)
            case ord_:
                raise ValueError(f"unknown channel ordinal {ord_}")

        (urgent,) = struct.unpack_from("<?", data, off)
        return cls(notification_id=notification_id, channel=channel, urgent=urgent)


def _len_str(s: str) -> bytes:
    b = s.encode()
    return struct.pack("<I", len(b)) + b
