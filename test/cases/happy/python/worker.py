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

"""Happy-path Python worker: `all_types` in binary and json.

Go is the --ref language and owns both byte layouts; see
test/cases/happy/go/type.go. The json format must match Go's encoding/json
byte-for-byte (with SetEscapeHTML(false)): schema field order, map keys in
byte order, []byte as base64, floats in shortest form without a trailing .0,
and U+2028/U+2029 escaped (Go escapes those unconditionally).
"""

import base64
import json
import os
import struct
import sys

_lib_dir = os.path.join(os.path.dirname(__file__), '../../../../lib/python')
if not os.path.isdir(_lib_dir):
    raise RuntimeError(f"serify library not found at {_lib_dir}; fix the relative path")
sys.path.insert(0, _lib_dir)

from serify import FieldMap, run_suite  # noqa: E402

STATUS_VARIANTS = ["pending", "paid", "shipped", "delivered", "cancelled"]


def status_ordinal(s: str) -> int:
    return STATUS_VARIANTS.index(s)


# --- binary format -----------------------------------------------------------

def pack_len_str(s: str) -> bytes:
    b = s.encode()
    return struct.pack("<I", len(b)) + b


class _Cursor:
    def __init__(self, data: bytes):
        self.data = data
        self.off = 0

    def take(self, fmt: str):
        vals = struct.unpack_from(fmt, self.data, self.off)
        self.off += struct.calcsize(fmt)
        return vals[0] if len(vals) == 1 else vals

    def take_str(self) -> str:
        n = self.take("<I")
        s = self.data[self.off:self.off + n].decode()
        self.off += n
        return s

    def take_bytes(self) -> bytes:
        n = self.take("<I")
        b = self.data[self.off:self.off + n]
        self.off += n
        return b


def binary_serialize(fm: FieldMap) -> bytes:
    """Canonical layout: map keys sorted, so all nine workers agree byte-for-byte."""
    return _binary_serialize(fm, sort_maps=True)


def binary_unordered_serialize(fm: FieldMap) -> bytes:
    """Same layout minus the map sort — judged by the semantic oracle."""
    return _binary_serialize(fm, sort_maps=False)


def _binary_serialize(fm: FieldMap, sort_maps: bool) -> bytes:
    buf = bytearray()
    buf += struct.pack("<BHIQ", fm.get_u8("uint8"), fm.get_u16("uint16"),
                       fm.get_u32("uint32"), fm.get_u64("uint64"))
    buf += struct.pack("<bhiq", fm.get_i8("int8"), fm.get_i16("int16"),
                       fm.get_i32("int32"), fm.get_i64("int64"))
    buf += struct.pack("<fd", fm.get_f32("float32"), fm.get_f64("float64"))
    buf += struct.pack("<?", fm.get_bool("bool"))
    buf += pack_len_str(fm.get_string("string"))

    raw = fm.get_bytes("bytes")
    buf += struct.pack("<I", len(raw)) + raw

    lst = fm.get_list_string("list")
    buf += struct.pack("<I", len(lst))
    for s in lst:
        buf += pack_len_str(s)

    opt = fm.get_optional_string("optional")
    if opt is None:
        buf += b"\x00"
    else:
        buf += b"\x01" + pack_len_str(opt)

    for n in fm.get_list_u32("array"):
        buf += struct.pack("<I", n)

    p = fm.get_struct("struct")
    buf += struct.pack("<iii", p.get_i32("x"), p.get_i32("y"), p.get_i32("z"))
    buf += pack_len_str(p.get_string("name"))

    m = fm.get_map("map")
    buf += struct.pack("<I", len(m))
    for k in (sorted(m) if sort_maps else m):
        buf += pack_len_str(k) + struct.pack("<I", m[k])

    ms = fm.get_map("map_struct")
    buf += struct.pack("<I", len(ms))
    for k in (sorted(ms) if sort_maps else ms):
        buf += pack_len_str(k) + pack_len_str(ms[k].get_string("name"))
        buf += struct.pack("<I", ms[k].get_u32("weight"))

    buf += struct.pack("<B", status_ordinal(fm.get_string("status")))
    return bytes(buf)


def binary_deserialize(data: bytes) -> FieldMap:
    c = _Cursor(data)
    fm = FieldMap()
    u8, u16, u32, u64 = c.take("<BHIQ")
    fm.set_u8("uint8", u8)
    fm.set_u16("uint16", u16)
    fm.set_u32("uint32", u32)
    fm.set_u64("uint64", u64)
    i8, i16, i32, i64 = c.take("<bhiq")
    fm.set_i8("int8", i8)
    fm.set_i16("int16", i16)
    fm.set_i32("int32", i32)
    fm.set_i64("int64", i64)
    f32, f64 = c.take("<fd")
    fm.set_f32("float32", f32)
    fm.set_f64("float64", f64)
    fm.set_bool("bool", c.take("<?"))
    fm.set_string("string", c.take_str())
    fm.set_bytes("bytes", bytes(c.take_bytes()))

    fm.set_list_string("list", [c.take_str() for _ in range(c.take("<I"))])

    fm.set_optional_string("optional", c.take_str() if c.take("<B") else None)

    fm.set_list_u32("array", [c.take("<I") for _ in range(4)])

    p = FieldMap()
    x, y, z = c.take("<iii")
    p.set_i32("x", x)
    p.set_i32("y", y)
    p.set_i32("z", z)
    p.set_string("name", c.take_str())
    fm.set_struct("struct", p)

    fm.set_map("map", {c.take_str(): c.take("<I") for _ in range(c.take("<I"))})

    ms = {}
    for _ in range(c.take("<I")):
        k = c.take_str()
        t = FieldMap()
        t.set_string("name", c.take_str())
        t.set_u32("weight", c.take("<I"))
        ms[k] = t
    fm.set_map("map_struct", ms)

    fm.set_string("status", STATUS_VARIANTS[c.take("<B")])
    return fm


# --- json format -------------------------------------------------------------

def go_str(s: str) -> str:
    """Go's encoding/json string escaping with SetEscapeHTML(false)."""
    out = ['"']
    for ch in s:
        o = ord(ch)
        if ch == '"':
            out.append('\\"')
        elif ch == '\\':
            out.append('\\\\')
        elif o < 0x20:
            # Go names only \n, \r, \t; \b and \f become \u00xx.
            out.append({0x0A: '\\n', 0x0D: '\\r', 0x09: '\\t'}.get(o, f'\\u{o:04x}'))
        elif o in (0x2028, 0x2029):
            out.append(f'\\u{o:04x}')
        else:
            out.append(ch)
    out.append('"')
    return ''.join(out)


def go_f64(v: float) -> str:
    s = repr(v)  # shortest round-trip decimal
    return s[:-2] if s.endswith('.0') else s


def go_f32(v: float) -> str:
    """Shortest decimal that round-trips through float32 (v is the f64 widening)."""
    bits = struct.pack('<f', v)
    for p in range(1, 10):
        s = f'%.{p}g' % v
        if struct.pack('<f', float(s)) == bits:
            return s[:-2] if s.endswith('.0') else s
    return repr(v)


def json_serialize(fm: FieldMap) -> bytes:
    parts = [
        f'"uint8":{fm.get_u8("uint8")}',
        f'"uint16":{fm.get_u16("uint16")}',
        f'"uint32":{fm.get_u32("uint32")}',
        f'"uint64":{fm.get_u64("uint64")}',
        f'"int8":{fm.get_i8("int8")}',
        f'"int16":{fm.get_i16("int16")}',
        f'"int32":{fm.get_i32("int32")}',
        f'"int64":{fm.get_i64("int64")}',
        f'"float32":{go_f32(fm.get_f32("float32"))}',
        f'"float64":{go_f64(fm.get_f64("float64"))}',
        '"bool":' + ('true' if fm.get_bool("bool") else 'false'),
        '"string":' + go_str(fm.get_string("string")),
        '"bytes":"' + base64.b64encode(fm.get_bytes("bytes")).decode() + '"',
        '"list":[' + ','.join(go_str(s) for s in fm.get_list_string("list")) + ']',
    ]

    opt = fm.get_optional_string("optional")
    parts.append('"optional":' + ('null' if opt is None else go_str(opt)))

    parts.append('"array":[' + ','.join(str(n) for n in fm.get_list_u32("array")) + ']')

    p = fm.get_struct("struct")
    parts.append('"struct":{'
                 f'"x":{p.get_i32("x")},"y":{p.get_i32("y")},"z":{p.get_i32("z")},'
                 '"name":' + go_str(p.get_string("name")) + '}')

    m = fm.get_map("map")
    parts.append('"map":{' + ','.join(f'{go_str(k)}:{m[k]}' for k in sorted(m)) + '}')

    ms = fm.get_map("map_struct")
    parts.append('"map_struct":{' + ','.join(
        f'{go_str(k)}:{{"name":{go_str(ms[k].get_string("name"))},"weight":{ms[k].get_u32("weight")}}}'
        for k in sorted(ms)) + '}')

    parts.append('"status":' + go_str(fm.get_string("status")))
    return ('{' + ','.join(parts) + '}').encode()


def json_deserialize(data: bytes) -> FieldMap:
    v = json.loads(data.decode())
    fm = FieldMap()
    fm.set_u8("uint8", v["uint8"])
    fm.set_u16("uint16", v["uint16"])
    fm.set_u32("uint32", v["uint32"])
    fm.set_u64("uint64", v["uint64"])
    fm.set_i8("int8", v["int8"])
    fm.set_i16("int16", v["int16"])
    fm.set_i32("int32", v["int32"])
    fm.set_i64("int64", v["int64"])
    fm.set_f32("float32", struct.unpack('<f', struct.pack('<f', v["float32"]))[0])
    fm.set_f64("float64", float(v["float64"]))
    fm.set_bool("bool", v["bool"])
    fm.set_string("string", v["string"])
    fm.set_bytes("bytes", base64.b64decode(v["bytes"]))
    fm.set_list_string("list", list(v["list"]))
    fm.set_optional_string("optional", v["optional"])
    fm.set_list_u32("array", [int(n) for n in v["array"]])

    p = FieldMap()
    p.set_i32("x", v["struct"]["x"])
    p.set_i32("y", v["struct"]["y"])
    p.set_i32("z", v["struct"]["z"])
    p.set_string("name", v["struct"]["name"])
    fm.set_struct("struct", p)

    fm.set_map("map", {k: int(n) for k, n in v["map"].items()})

    ms = {}
    for k, t in v["map_struct"].items():
        tag = FieldMap()
        tag.set_string("name", t["name"])
        tag.set_u32("weight", t["weight"])
        ms[k] = tag
    fm.set_map("map_struct", ms)

    fm.set_string("status", v["status"])
    return fm


if __name__ == '__main__':
    run_suite({
        "all_types": {
            "binary": (binary_serialize, binary_deserialize),
            "json": (json_serialize, json_deserialize),
            # Same bytes minus the map sort; the deserializer is shared because
            # reading never cared about entry order.
            "binary_unordered": (binary_unordered_serialize, binary_deserialize),
        },
    })
