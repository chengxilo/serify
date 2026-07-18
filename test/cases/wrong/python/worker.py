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

"""Python half of the `wrong` meta-test."""

import json, os, struct, sys, time
_lib_dir = os.path.join(os.path.dirname(__file__), '../../../../lib/python')
if not os.path.isdir(_lib_dir):
    raise RuntimeError(f"serify library not found at {_lib_dir}; fix the relative path")
sys.path.insert(0, _lib_dir)
from serify import FieldMap, run_suite  # noqa: E402

SELF_LANG = "python"

def to_upper_self(langs):
    return [s.upper() if s == SELF_LANG else s for s in langs]

def binary_serialize(fm):
    langs = fm.get_list_string("langs")
    if not fm.get_bool("binary_serialize"):
        langs = to_upper_self(langs)
    buf = bytearray([fm.get_bool("binary_serialize"), fm.get_bool("binary_deserialize"),
                     fm.get_bool("json_serialize"), fm.get_bool("json_deserialize")])
    buf += struct.pack("<I", len(langs))
    for s in langs:
        b = s.encode()
        buf += struct.pack("<I", len(b)) + b
    return bytes(buf)

def binary_deserialize(data):
    fm = FieldMap()
    flags = struct.unpack_from("<BBBB", data, 0)
    off = 4
    n = struct.unpack_from("<I", data, off)[0]
    off += 4
    langs = []
    for _ in range(n):
        slen = struct.unpack_from("<I", data, off)[0]
        off += 4
        langs.append(data[off:off + slen].decode())
        off += slen
    bs, bd, js, jd = [bool(f) for f in flags]
    if not bd:
        langs = to_upper_self(langs)
    fm.set_bool("binary_serialize", bs)
    fm.set_bool("binary_deserialize", bd)
    fm.set_bool("json_serialize", js)
    fm.set_bool("json_deserialize", jd)
    fm.set_list_string("langs", langs)
    return fm

def json_serialize(fm):
    d = {"binary_serialize": fm.get_bool("binary_serialize"),
         "binary_deserialize": fm.get_bool("binary_deserialize"),
         "json_serialize": fm.get_bool("json_serialize"),
         "json_deserialize": fm.get_bool("json_deserialize")}
    langs = fm.get_list_string("langs")
    if not d["json_serialize"]:
        langs = to_upper_self(langs)
    d["langs"] = langs
    return json.dumps(d, separators=(',', ':')).encode()

def json_deserialize(data):
    d = json.loads(data)
    fm = FieldMap()
    fm.set_bool("binary_serialize", d["binary_serialize"])
    fm.set_bool("binary_deserialize", d["binary_deserialize"])
    fm.set_bool("json_serialize", d["json_serialize"])
    fm.set_bool("json_deserialize", d["json_deserialize"])
    langs = d["langs"]
    if not d["json_deserialize"]:
        langs = to_upper_self(langs)
    fm.set_list_string("langs", langs)
    return fm

# --- fault formats ---

def err_ser(fm):
    raise RuntimeError("injected serialize error")

def err_deser(data):
    raise RuntimeError("injected deserialize error")

def hang_ser(fm):
    time.sleep(3)
    return binary_serialize(fm)

def crash_ser(fm):
    os._exit(3)

if __name__ == '__main__':
    run_suite({"wrong": {
        "binary": (binary_serialize, binary_deserialize),
        "json": (json_serialize, json_deserialize),
        "err_ser": (err_ser, binary_deserialize),
        "err_deser": (binary_serialize, err_deser),
        "hang": (hang_ser, binary_deserialize),
        "crash": (crash_ser, binary_deserialize),
    }})
