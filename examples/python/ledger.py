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

"""`LedgerEntry` mirrors examples/cases/ledger.yaml.

`@serify_model` is the entire schema binding — it reads the dataclass annotations
and generates `from_field_map` / `to_field_map`, so nothing here calls a
`get_*`/`set_*` accessor. Everything else is the byte layout, which is the part a
conformance worker exists to exercise.

Go is the --ref language and owns that layout; see the comment at the top of
examples/go/wire.go.
"""

import struct
from dataclasses import dataclass

from serify import serify_model

INT128_MASK = (1 << 128) - 1


@serify_model
@dataclass
class LedgerEntry:
    entry_id: int
    block_number: int
    block_time: int
    tx_hash: bytes
    account: str
    asset: str
    amount_base_units: int
    balance_after: int
    confirmed: bool
    memo: "str | None"

    def marshal(self) -> bytes:
        buf = bytearray()

        buf += struct.pack("<QQq", self.entry_id, self.block_number, self.block_time)
        buf += struct.pack("<I", len(self.tx_hash)) + self.tx_hash

        for s in (self.account, self.asset):
            b = s.encode()
            buf += struct.pack("<I", len(b)) + b

        # int128: 16 bytes little-endian, two's complement. Masking to 128 bits
        # maps a negative onto its residue class, which is exactly two's complement.
        for v in (self.amount_base_units, self.balance_after):
            buf += (v & INT128_MASK).to_bytes(16, "little")

        buf += struct.pack("<?", self.confirmed)

        if self.memo is None:
            buf += b"\x00"
        else:
            m = self.memo.encode()
            buf += b"\x01" + struct.pack("<I", len(m)) + m

        return bytes(buf)

    @classmethod
    def unmarshal(cls, data: bytes) -> "LedgerEntry":
        off = 0
        entry_id, block_number, block_time = struct.unpack_from("<QQq", data, off)
        off += 24

        (n,) = struct.unpack_from("<I", data, off)
        off += 4
        tx_hash = data[off:off + n]
        off += n

        strs = []
        for _ in range(2):
            (n,) = struct.unpack_from("<I", data, off)
            off += 4
            strs.append(data[off:off + n].decode())
            off += n

        ints = []
        for _ in range(2):
            v = int.from_bytes(data[off:off + 16], "little")
            off += 16
            if v >= 1 << 127:  # re-apply the sign
                v -= 1 << 128
            ints.append(v)

        (confirmed,) = struct.unpack_from("<?", data, off)
        off += 1

        memo = None
        if data[off]:
            off += 1
            (n,) = struct.unpack_from("<I", data, off)
            off += 4
            memo = data[off:off + n].decode()

        return cls(
            entry_id=entry_id, block_number=block_number, block_time=block_time,
            tx_hash=tx_hash, account=strs[0], asset=strs[1],
            amount_base_units=ints[0], balance_after=ints[1],
            confirmed=confirmed, memo=memo,
        )
