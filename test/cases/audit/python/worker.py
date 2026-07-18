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

"""Python audit worker: 5 cross-language audit formats."""
import os, struct, sys
_lib_dir = os.path.join(os.path.dirname(__file__), '../../../../lib/python')
if not os.path.isdir(_lib_dir):
    raise RuntimeError(f"serify library not found at {_lib_dir}; fix the relative path")
sys.path.insert(0, _lib_dir)
from serify import FieldMap, run_suite  # noqa: E402

_unstable_ctr = 0
_deser_unstable_ctr = 0


def _marshal(fm):
    val = fm.get_u32("value")
    tag = fm.get_string("tag").encode()
    payload = fm.get_bytes("payload")
    tags = fm.get_list_string("tags")
    buf = struct.pack("<I", val)
    buf += struct.pack("<B", len(tag)) + tag
    buf += struct.pack("<I", len(payload)) + payload
    buf += struct.pack("<B", len(tags))
    for t in tags:
        b = t.encode()
        buf += struct.pack("<B", len(b)) + b
    return buf


def _unmarshal(data, copy_payload=True):
    fm = FieldMap()
    off = 0
    fm.set_u32("value", struct.unpack_from("<I", data, off)[0]); off += 4
    tlen = data[off]; off += 1
    fm.set_string("tag", data[off:off + tlen].decode()); off += tlen
    plen = struct.unpack_from("<I", data, off)[0]; off += 4
    payload = data[off:off + plen]; off += plen
    fm.set_bytes("payload", bytes(payload) if copy_payload else payload)
    tcount = data[off]; off += 1
    tags = []
    for _ in range(tcount):
        tl = data[off]; off += 1
        tags.append(data[off:off + tl].decode()); off += tl
    fm.set_list_string("tags", tags)
    return fm


def clean_ser(fm):
    return _marshal(fm)


def clean_deser(data):
    return _unmarshal(data, True)


def mutating_ser(fm):
    buf = _marshal(fm)
    fm.set_u32("value", 0)
    return buf


def unstable_ser(fm):
    global _unstable_ctr
    buf = _marshal(fm) + struct.pack("<B", _unstable_ctr)
    _unstable_ctr += 1
    return buf


def deser_unstable_deser(data):
    global _deser_unstable_ctr
    fm = _unmarshal(data, True)
    if _deser_unstable_ctr > 0:
        fm.set_u32("value", fm.get_u32("value") + 1)
    _deser_unstable_ctr += 1
    return fm


def input_mutating_deser(data):
    fm = _unmarshal(data, True)
    ba = bytearray(data)
    if ba:
        ba[0] ^= 0xFF
    return fm


if __name__ == '__main__':
    run_suite({"audit": {
        "clean": (clean_ser, clean_deser),
        "mutating": (mutating_ser, clean_deser),
        "unstable": (unstable_ser, clean_deser),
        "deser-unstable": (clean_ser, deser_unstable_deser),
        "input-mutating": (clean_ser, input_mutating_deser),
    }})
