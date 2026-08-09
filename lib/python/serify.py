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

"""
workerlib.py - Python workerlib for the cross-language serialization test framework.

Usage in worker:
    import sys, os
    sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../lib/python'))
    from serify import FieldMap, run
"""

from __future__ import annotations

import dataclasses
import json
import struct
import sys
import time
import typing
from typing import Any, Callable, Sequence, Union, cast

SerializeFn = Callable[["FieldMap"], bytes]
DeserializeFn = Callable[[bytes], "FieldMap"]
_Schema = list[dict[str, Any]]


class FieldMap:
    def __init__(self) -> None:
        self._fields: dict[str, Any] = {}

    def get_u8(self, key: str) -> int:           return cast(int,             self._fields[key])
    def get_u16(self, key: str) -> int:          return cast(int,             self._fields[key])
    def get_u32(self, key: str) -> int:          return cast(int,             self._fields[key])
    def get_u64(self, key: str) -> int:          return cast(int,             self._fields[key])
    def get_i8(self, key: str) -> int:           return cast(int,             self._fields[key])
    def get_i16(self, key: str) -> int:          return cast(int,             self._fields[key])
    def get_i32(self, key: str) -> int:          return cast(int,             self._fields[key])
    def get_i64(self, key: str) -> int:          return cast(int,             self._fields[key])
    def get_f32(self, key: str) -> float:        return cast(float,           self._fields[key])
    def get_f64(self, key: str) -> float:        return cast(float,           self._fields[key])
    def get_bool(self, key: str) -> bool:        return cast(bool,            self._fields[key])
    def get_string(self, key: str) -> str:       return cast(str,             self._fields[key])
    def get_bytes(self, key: str) -> bytes:      return cast(bytes,           self._fields[key])
    def get_list_string(self, key: str) -> list[str]:    return cast(list[str],   self._fields[key])
    def get_list_u32(self, key: str) -> list[int]:       return cast(list[int],   self._fields.get(key, []))
    def get_list_u64(self, key: str) -> list[int]:       return cast(list[int],   self._fields.get(key, []))
    def get_list_f32(self, key: str) -> list[float]:     return cast(list[float], self._fields.get(key, []))
    def get_list_struct(self, key: str) -> list[FieldMap]:   return cast(list[FieldMap], self._fields.get(key, []))
    def get_optional_string(self, key: str) -> str | None:   return cast(str | None,     self._fields.get(key))
    def get_struct(self, key: str) -> FieldMap | None:        return cast(FieldMap | None, self._fields.get(key))
    def get_optional_struct(self, key: str) -> FieldMap | None: return cast(FieldMap | None, self._fields.get(key))
    def get_map(self, key: str) -> dict[str, Any]:              return cast(dict[str, Any], self._fields.get(key, {}))

    def set_u8(self, key: str, v: int) -> None:           self._fields[key] = v
    def set_u16(self, key: str, v: int) -> None:          self._fields[key] = v
    def set_u32(self, key: str, v: int) -> None:          self._fields[key] = v
    def set_u64(self, key: str, v: int) -> None:          self._fields[key] = v
    def set_i8(self, key: str, v: int) -> None:           self._fields[key] = v
    def set_i16(self, key: str, v: int) -> None:          self._fields[key] = v
    def set_i32(self, key: str, v: int) -> None:          self._fields[key] = v
    def set_i64(self, key: str, v: int) -> None:          self._fields[key] = v
    def set_f32(self, key: str, v: float) -> None:        self._fields[key] = v
    def set_f64(self, key: str, v: float) -> None:        self._fields[key] = v
    def set_bool(self, key: str, v: bool) -> None:        self._fields[key] = v
    def set_string(self, key: str, v: str) -> None:       self._fields[key] = v
    def set_bytes(self, key: str, v: bytes) -> None:      self._fields[key] = v
    def set_list_string(self, key: str, v: list[str]) -> None:  self._fields[key] = v
    def set_optional_string(self, key: str, v: str | None) -> None: self._fields[key] = v
    def set_struct(self, key: str, v: FieldMap) -> None:        self._fields[key] = v
    def set_list_struct(self, key: str, v: list[FieldMap]) -> None:  self._fields[key] = v
    def set_optional_struct(self, key: str, v: FieldMap | None) -> None: self._fields[key] = v
    def set_list_u32(self, key: str, v: list[int]) -> None:    self._fields[key] = v
    def set_list_u64(self, key: str, v: list[int]) -> None:    self._fields[key] = v
    def set_list_f32(self, key: str, v: list[float]) -> None:  self._fields[key] = v
    def set_map(self, key: str, v: dict[str, Any]) -> None:   self._fields[key] = v

    def set_variant(self, key: str, tag: str, value: Any = None) -> None:
        """Store a sum value: the active variant's tag and payload (None for a unit variant)."""
        self._fields[key] = Variant(tag, value)

    def get_variant(self, key: str) -> "Variant":
        """Return the sum value stored at key."""
        v = self._fields[key]
        if not isinstance(v, Variant):
            raise ValueError(f"field {key!r} is not a variant (got {type(v).__name__})")
        return v


@dataclasses.dataclass(frozen=True)
class Variant:
    """One arm of a sum: a tag and its decoded payload (None for a unit variant)."""
    tag: str
    value: Any = None


_SCALARS: tuple[str, ...] = (
    "uint8", "uint16", "uint32", "uint64", "uint128",
    "int8", "int16", "int32", "int64", "int128",
    "float32", "float64", "bool", "string", "bytes",
)

# Element types a list may carry: every scalar, plus a nested struct.
_LIST_ELEMS: frozenset[str] = frozenset((*_SCALARS, "struct"))

# The protocol revision this library speaks. The runner requires an exact
# match and refuses to start a worker reporting anything else.
PROTOCOL_VERSION = 2


def _find_schema_variant(sf: dict[str, Any], tag: str) -> dict[str, Any]:
    for sv in sf.get("variants") or []:
        if sv["name"] == tag:
            return cast(dict[str, Any], sv)
    raise ValueError(f"unknown variant {tag!r}")



def decode_field_map(data: dict[str, Any], schema: _Schema) -> FieldMap:
    """Convert a JSON data dict to a FieldMap using the schema for type guidance."""
    fm = FieldMap()
    for sf in schema:
        name: str = sf["name"]
        typ: str = sf["type"]
        if name not in data:
            continue
        _decode_field(fm, name, typ, sf, data[name])
    return fm


def _decode_field(fm: FieldMap, name: str, typ: str, sf: dict[str, Any], v: Any) -> None:
    if typ == "uint8":
        fm.set_u8(name, int(v))
    elif typ == "uint16":
        fm.set_u16(name, int(v))
    elif typ == "uint32":
        fm.set_u32(name, int(v))
    elif typ in ("uint64", "uint128"):
        fm.set_u64(name, int(v))
    elif typ == "int8":
        fm.set_i8(name, int(v))
    elif typ == "int16":
        fm.set_i16(name, int(v))
    elif typ == "int32":
        fm.set_i32(name, int(v))
    elif typ in ("int64", "int128"):
        fm.set_i64(name, int(v))
    elif typ == "float32":
        fm.set_f32(name, struct.unpack_from('<f', bytes.fromhex(v))[0])
    elif typ == "float64":
        fm.set_f64(name, struct.unpack_from('<d', bytes.fromhex(v))[0])
    elif typ == "bool":
        fm.set_bool(name, bool(v))
    elif typ == "string":
        fm.set_string(name, str(v))
    elif typ == "bytes":
        fm.set_bytes(name, bytes.fromhex(v))
    elif typ == "struct":
        fm.set_struct(name, decode_field_map(v, sf.get("fields", [])))
    elif typ.startswith("list<"):
        fm._fields[name] = _decode_list(typ[5:-1], sf.get("fields", []), v)
    elif typ.startswith("optional<"):
        elem_type = typ[9:-1]
        if v is None:
            fm._fields[name] = None
        elif elem_type == "struct":
            fm.set_optional_struct(name, decode_field_map(v, sf.get("fields", [])))
        else:
            tmp = FieldMap()
            _decode_field(tmp, name, elem_type, {"name": name, "type": elem_type}, v)
            fm._fields[name] = tmp._fields[name]
    elif typ.startswith("array<"):
        # An array<T,N> is a list whose length the schema fixes, so it shares
        # _decode_list outright and adds only the length check. A separate
        # representation is what pinned array<T,N> to uint32.
        elem_type, n = _split_array_type(typ)
        out = _decode_list(elem_type, sf.get("fields", []), v)
        if len(out) != n:
            raise ValueError(f"array {name}: expected {n} elements, got {len(out)}")
        fm._fields[name] = out
    # enum<a,b,c>: the variant name travels as a string; the variants live in the
    # type, so a worker can map the name onto an ordinal for its byte layout.
    elif typ.startswith("enum<"):
        fm.set_string(name, str(v))
    # sum<a, b: T>: {tag: payload} on the wire (payload null for a unit
    # variant); the payload is decoded per the variant's own schema.
    elif typ.startswith("sum<"):
        if len(v) != 1:
            raise ValueError(f"sum must name exactly one variant, got {len(v)}")
        tag, payload = next(iter(v.items()))
        sv = _find_schema_variant(sf, tag)
        psf = sv.get("payload")
        if psf is None:
            fm.set_variant(name, tag, None)
        else:
            tmp = FieldMap()
            _decode_field(tmp, psf["name"], psf["type"], psf, payload)
            fm.set_variant(name, tag, tmp._fields[psf["name"]])
    elif typ.startswith("map<"):
        _, val_type = _split_map_types(typ)
        fm._fields[name] = _decode_map(val_type, sf.get("fields", []), v)
    else:
        # Silently skipping an unrecognised type would leave the field absent
        # from the FieldMap and surface far downstream as a missing value.
        raise ValueError(f"unknown type {typ!r}")


def _split_array_type(typ: str) -> tuple[str, int]:
    """Return (elem_type, N) from 'array<uint8,4>'."""
    inner = typ[6:-1]
    elem, _, count = inner.rpartition(",")
    return elem.strip(), int(count.strip())


def _split_map_types(typ: str) -> tuple[str, str]:
    """Return (key_type, val_type) from 'map<string,uint32>'."""
    inner = typ[4:-1]  # strip "map<" and ">"
    depth = 0
    for i, ch in enumerate(inner):
        if ch in ("<", "["): depth += 1
        elif ch in (">", "]"): depth -= 1
        elif ch == "," and depth == 0:
            return inner[:i].strip(), inner[i+1:].strip()
    return "", inner.strip()


def _decode_map(val_type: str, nested_schema: _Schema, obj: dict[str, Any]) -> dict[str, Any]:
    """Decode a JSON object into a dict of FieldMap-compatible values.

    Every value goes through _decode_field, so a map supports exactly the value
    types a bare field does. This used to be its own chain of type tests that
    silently fell through to the raw JSON for anything it did not name.
    """
    if val_type not in _LIST_ELEMS:
        raise ValueError(f"unsupported map value type {val_type!r}")
    if val_type == "struct":
        return {k: decode_field_map(item, nested_schema) for k, item in obj.items()}

    sf = {"name": "v", "type": val_type, "fields": nested_schema}
    result: dict[str, Any] = {}
    for k, item in obj.items():
        tmp = FieldMap()
        try:
            _decode_field(tmp, "v", val_type, sf, item)
        except Exception as exc:
            raise ValueError(f"[{k!r}]: {exc}") from exc
        result[k] = tmp._fields["v"]
    return result


def _decode_list(elem_type: str, nested_schema: _Schema, arr: list[Any]) -> Any:
    """Decode every element through _decode_field, so a list supports exactly the
    element types a bare field does.

    This used to be its own chain of type tests, which silently fell through to
    `list(arr)` for anything it did not name — so a `list<float64>` handed the
    worker the raw hex strings instead of floats, with no error anywhere.
    """
    if elem_type not in _LIST_ELEMS:
        raise ValueError(f"unsupported list element type {elem_type!r}")
    if elem_type == "struct":
        return [decode_field_map(item, nested_schema) for item in arr]

    sf = {"name": "e", "type": elem_type, "fields": nested_schema}
    out = []
    for i, item in enumerate(arr):
        tmp = FieldMap()
        try:
            _decode_field(tmp, "e", elem_type, sf, item)
        except Exception as exc:
            raise ValueError(f"[{i}]: {exc}") from exc
        out.append(tmp._fields["e"])
    return out


def encode_field_map(fm: FieldMap, schema: _Schema) -> dict[str, Any]:
    """Convert a FieldMap back to a JSON-serializable dict using the schema."""
    out: dict[str, Any] = {}
    for sf in schema:
        name: str = sf["name"]
        if name not in fm._fields:
            continue
        out[name] = _encode_field(sf["type"], sf, fm._fields[name])
    return out


def _encode_field(typ: str, sf: dict[str, Any], v: Any) -> Any:
    if typ in ("uint8", "uint16", "uint32", "int8", "int16", "int32", "bool", "string"):
        return v
    if typ in ("uint64", "uint128", "int64", "int128"):
        return str(v)
    if typ == "float32":
        return struct.pack('<f', v).hex()
    if typ == "float64":
        return struct.pack('<d', v).hex()
    if typ == "bytes":
        return cast(bytes, v).hex()
    if typ == "struct":
        return encode_field_map(v, sf.get("fields", []))
    if typ.startswith("list<"):
        return _encode_list(typ[5:-1], sf.get("fields", []), v)
    if typ.startswith("optional<"):
        elem_type = typ[9:-1]
        if v is None:
            return None
        if elem_type == "struct":
            return encode_field_map(v, sf.get("fields", []))
        return _encode_field(elem_type, {"name": "", "type": elem_type}, v)
    if typ.startswith("array<"):
        elem_type, _ = _split_array_type(typ)
        return _encode_list(elem_type, sf.get("fields", []), v)
    if typ.startswith("enum<"):
        return v
    if typ.startswith("sum<"):
        if not isinstance(v, Variant):
            raise ValueError(f"expected a Variant for sum, got {type(v).__name__}")
        sv = _find_schema_variant(sf, v.tag)
        psf = sv.get("payload")
        if psf is None:
            return {v.tag: None}
        return {v.tag: _encode_field(psf["type"], psf, v.value)}
    if typ.startswith("map<"):
        _, val_type = _split_map_types(typ)
        return _encode_map(val_type, sf.get("fields", []), v)
    # Mirror of _decode_field: passing an unrecognised type through untouched
    # puts the wrong shape on the wire with nothing reported anywhere.
    raise ValueError(f"unknown type {typ!r}")


def _encode_map(val_type: str, nested_schema: _Schema, m: dict[str, Any]) -> dict[str, Any]:
    """Inverse of _decode_map: every value goes back out through _encode_field,
    so the two directions cannot cover different value types.

    Keys stay sorted: the wire form has to be deterministic, and two workers that
    disagree on key order produce different bytes for identical data.
    """
    if val_type not in _LIST_ELEMS:
        raise ValueError(f"unsupported map value type {val_type!r}")
    if val_type == "struct":
        return {k: encode_field_map(m[k], nested_schema) for k in sorted(m.keys())}

    sf = {"name": "v", "type": val_type, "fields": nested_schema}
    return {k: _encode_field(val_type, sf, m[k]) for k in sorted(m.keys())}


def _encode_list(elem_type: str, nested_schema: _Schema, arr: Any) -> Any:
    """Inverse of _decode_list: every element goes back out through
    _encode_field, so the two directions cannot cover different element types."""
    if elem_type not in _LIST_ELEMS:
        raise ValueError(f"unsupported list element type {elem_type!r}")
    if elem_type == "struct":
        return [encode_field_map(item, nested_schema) for item in arr]

    sf = {"name": "e", "type": elem_type, "fields": nested_schema}
    return [_encode_field(elem_type, sf, x) for x in arr]


# ─── serify_model decorator ─────────────────────────────────────────────────

def serify_model(cls: type | None = None, *, rename: dict[str, str] | None = None) -> Any:
    """
    Class decorator that augments a @dataclass with to_field_map() and
    from_field_map() methods.  Field names are used as schema keys unless
    overridden with `rename` or per-field `metadata={"serify": "key"}`.

    Usage:
        from dataclasses import dataclass, field

        @serify_model
        @dataclass
        class User:
            user_id: int
            name: str
            email: str = field(default="", metadata={"serify": "email_addr"})
    """

    def _wrap(cls: type) -> type:
        return _apply_serify_model(cls, rename or {})

    # Support both @serify_model and @serify_model(rename={...})
    if cls is None:
        return _wrap
    return _wrap(cls)


def _apply_serify_model(cls: type, rename_map: dict[str, str]) -> type:
    """Augment *cls* with to_field_map / from_field_map."""
    import dataclasses as _dc

    _fields = list(_dc.fields(cls))
    if not _fields:
        raise TypeError(f"{cls.__name__}: serify_model requires a dataclass with fields")

    # Collect type annotations
    hints = _get_type_hints(cls)

    # Build (python_attr, serify_key, type_hint) triples
    triples: list[tuple[str, str, type]] = []
    for fld in _fields:
        py_name = fld.name
        # Determine schema key: explicit rename map → metadata → field name
        key = rename_map.get(py_name)
        if key is None:
            key = fld.metadata.get("serify", py_name)
        hint = hints.get(py_name, str)
        triples.append((py_name, key, hint))

    def _to_field_map(self: Any) -> FieldMap:
        fm = FieldMap()
        for py_name, key, hint in triples:
            _set_fm(fm, key, getattr(self, py_name), hint)
        return fm

    def _from_field_map(fm: FieldMap) -> Any:
        kwargs: dict[str, Any] = {}
        for py_name, key, hint in triples:
            kwargs[py_name] = _get_fm(fm, key, hint)
        return cls(**kwargs)

    # Attaching methods to an arbitrary class is dynamic by nature; the cast
    # keeps that honest rather than pretending `type` declares these.
    cls_any = cast(Any, cls)
    cls_any.to_field_map = _to_field_map
    cls_any.from_field_map = staticmethod(_from_field_map)
    return cls


# Helpers to get type hints, handling forward refs
def _get_type_hints(cls: type) -> dict[str, Any]:
    """Return resolved type hints dict for *cls*."""
    # Built-in types that may appear as bare names
    g = {
        "FieldMap": FieldMap,
    }
    try:
        # localns: keep the class's own module globals for user-defined forward
        # refs while making FieldMap resolvable without an import.
        return typing.get_type_hints(cls, localns=g)
    except Exception:
        return cast(dict[str, Any], vars(cls).get("__annotations__", {}))


# Map python type → (FieldMap setter, FieldMap getter, optional inner type)
def _type_info(hint: Any) -> tuple[str, Any]:
    """Return (kind, extra) where kind is one of:
       u8/u16/u32/u64/i8/i16/i32/i64/f32/f64/bool/string/bytes
       list_scalar/optional_string
       struct/list_struct/optional_struct/map

    `extra` is deliberately Any: it is a class for struct/list_struct/sum, an
    inner type hint for optionals, and None otherwise — a union a caller reads
    reflectively (`.from_field_map`, etc.), not something a static type helps.
    """
    origin = getattr(hint, "__origin__", None)
    args: tuple[Any, ...] = getattr(hint, "__args__", ())

    # Scalars
    if hint is int or hint == "int":
        return ("int64", None)
    if hint is float:
        return ("float64", None)
    if hint is bool:
        return ("bool", None)
    if hint is str:
        return ("string", None)
    if hint is bytes:
        return ("bytes", None)

    # Map of known types
    _kind_map = {
        "int": "int64", "float": "float64", "bool": "bool", "str": "string", "bytes": "bytes",
    }
    if isinstance(hint, str) and hint.lower() in _kind_map:
        return (_kind_map[hint.lower()], None)

    # sum: a union of dataclasses. Checked before the optional branch, which
    # claims every union mentioning None.
    arms = _sum_arms(hint)
    if arms is not None:
        return ("sum", arms)

    # Optional (Union[X, None] / X | None)
    if _is_union_with_none(hint):
        inner = _unwrap_optional(args)
        kind, _ = _type_info(inner) if inner else ("string", None)
        if kind == "struct":
            return ("optional_struct", inner)
        elif kind == "string":
            return ("optional_string", None)
        else:
            return ("optional_string", inner)

    # list[X]
    if origin is list:
        inner = args[0] if args else str
        if isinstance(inner, type) and hasattr(inner, "from_field_map"):
            return ("list_struct", inner)
        if inner in (str, int, float, bool, bytes):
            # One kind for every scalar element type. A Python hint cannot say
            # which width `list[int]` means — the schema does — and the FieldMap
            # stores the list as-is either way, so distinguishing here would only
            # reintroduce the per-element-type gap this used to have (it named
            # str/int/float and silently fell back to list_string for the rest,
            # so a list[bool] was stored as though it were strings).
            return ("list_scalar", None)
        raise ValueError(f"unsupported list element type {inner!r}")

    # dict[str, X] → map
    if origin is dict:
        val_inner = args[1] if len(args) > 1 else str
        return ("map", val_inner)

    # Struct: any class with from_field_map
    if isinstance(hint, type) and hasattr(hint, "from_field_map"):
        return ("struct", hint)

    # Fallback
    return ("string", None)


def _is_union_with_none(hint: Any) -> bool:
    """Check if hint is Union[..., None]."""
    args = getattr(hint, "__args__", ())
    return type(None) in args  # noqa: E721


def _unwrap_optional(args: tuple[Any, ...]) -> Any:
    """Return first non-None type from union args."""
    for a in args:
        if a is not type(None):  # noqa: E721
            return a
    return None


# sum: a union of dataclasses is Python's sum type
#
# `Channel = Silent | Sms | Push | Invoice` says everything the binding needs, so
# a `sum` field needs no converter and no registration — the union names the
# arms and each arm's own dataclass fields give its payload. This mirrors what
# the Rust derive does with a native `enum`; the arity rule is identical, because
# a serify `sum` is a sum-of-products:
#
#     0 fields  -> a unit variant, no payload
#     1 field   -> that field's value is the payload
#     N fields  -> the payload is a struct holding the N fields


def _sum_arms(hint: Any) -> tuple[Any, ...] | None:
    """Return the arm classes if *hint* is a union of dataclasses, else None.

    `str | None` is an `optional<string>`, not a sum type, so any union
    mentioning None is rejected here and handled by the optional branch.
    """
    args = getattr(hint, "__args__", ())
    if len(args) < 2 or type(None) in args:  # noqa: E721
        return None
    if not all(dataclasses.is_dataclass(a) for a in args):
        return None
    return args


def _arm_tag(arm: type) -> str:
    """Schema tag for an arm: its class name in snake_case, or an explicit
    `serify_tag` class attribute when the two cannot agree."""
    explicit = arm.__dict__.get("serify_tag")
    if explicit is not None:
        return cast(str, explicit)
    out = []
    for i, ch in enumerate(arm.__name__):
        if ch.isupper():
            if i:
                out.append("_")
            out.append(ch.lower())
        else:
            out.append(ch)
    return "".join(out)


def _arm_by_tag(arms: tuple[Any, ...], tag: str) -> type:
    for arm in arms:
        if _arm_tag(arm) == tag:
            return cast(type, arm)
    known = ", ".join(_arm_tag(a) for a in arms)
    raise ValueError(f"unknown variant {tag!r} (declared: {known})")


def _set_sum(fm: FieldMap, key: str, val: Any, arms: tuple[Any, ...]) -> None:
    """Write the active arm as a Variant. The wire encoding of the payload is
    driven by the schema, so this only has to hand over the right Python value."""
    arm = type(val)
    if arm not in arms:
        known = ", ".join(a.__name__ for a in arms)
        raise ValueError(f"{key}: {arm.__name__} is not one of {known}")

    flds = dataclasses.fields(arm)
    if not flds:
        fm.set_variant(key, _arm_tag(arm), None)          # unit variant
    elif len(flds) == 1:
        payload = getattr(val, flds[0].name)
        # A single payload that is itself a model travels as a struct.
        hint = _get_type_hints(arm).get(flds[0].name)
        if isinstance(hint, type) and hasattr(hint, "to_field_map"):
            payload = hint.to_field_map(payload)
        fm.set_variant(key, _arm_tag(arm), payload)
    else:
        payload = FieldMap()                              # N fields -> a struct
        hints = _get_type_hints(arm)
        for f in flds:
            _set_fm(payload, f.metadata.get("serify", f.name), getattr(val, f.name),
                    hints.get(f.name, str))
        fm.set_variant(key, _arm_tag(arm), payload)


def _get_sum(fm: FieldMap, key: str, arms: tuple[Any, ...]) -> Any:
    """Rebuild the active arm from a Variant."""
    var = fm.get_variant(key)
    arm = _arm_by_tag(arms, var.tag)

    flds = dataclasses.fields(arm)
    if not flds:
        return arm()
    if len(flds) == 1:
        hint = _get_type_hints(arm).get(flds[0].name)
        if isinstance(hint, type) and hasattr(hint, "from_field_map") and isinstance(var.value, FieldMap):
            return arm(hint.from_field_map(var.value))
        return arm(var.value)
    if not isinstance(var.value, FieldMap):
        raise ValueError(f"{key}: variant {var.tag!r} needs a struct payload")
    hints = _get_type_hints(arm)
    return arm(**{
        f.name: _get_fm(var.value, f.metadata.get("serify", f.name), hints.get(f.name, str))
        for f in flds
    })


# _type_info kind → FieldMap accessor suffix (set_u64/get_u64 etc.).
_ACCESSOR_SUFFIX = {
    "uint8": "u8", "uint16": "u16", "uint32": "u32", "uint64": "u64",
    "int8": "i8", "int16": "i16", "int32": "i32", "int64": "i64",
    "float32": "f32", "float64": "f64",
}


def _set_fm(fm: FieldMap, key: str, val: Any, hint: Any) -> None:
    """Set *val* into *fm* at *key* based on *hint*."""
    kind, extra = _type_info(hint)
    if kind in ("uint8", "uint16", "uint32", "uint64", "int8", "int16", "int32", "int64"):
        getattr(fm, f"set_{_ACCESSOR_SUFFIX[kind]}")(key, int(val) if val is not None else 0)
    elif kind in ("float32", "float64"):
        getattr(fm, f"set_{_ACCESSOR_SUFFIX[kind]}")(key, val)
    elif kind in ("bool", "string"):
        getattr(fm, f"set_{kind}")(key, val)
    elif kind == "bytes":
        fm.set_bytes(key, bytes(val) if isinstance(val, (bytes, bytearray)) else bytes(val or b""))
    elif kind == "list_scalar":
        fm._fields[key] = list(val or [])
    elif kind == "optional_string":
        fm.set_optional_string(key, val)
    elif kind == "struct" and extra:
        fm.set_struct(key, extra.to_field_map(val) if val else FieldMap())
    elif kind == "list_struct" and extra:
        fm.set_list_struct(key, [extra.to_field_map(x) for x in (val or [])])
    elif kind == "optional_struct" and extra:
        fm.set_optional_struct(key, extra.to_field_map(val) if val else None)
    elif kind == "map":
        _set_map_fm(fm, key, val, extra)
    elif kind == "sum" and extra:
        _set_sum(fm, key, val, extra)
    else:
        fm.set_string(key, str(val) if val is not None else "")


def _set_map_fm(fm: FieldMap, key: str, val: Any, val_hint: Any) -> None:
    """Set a map<K,V> field, handling str→FieldValue conversion."""
    out: dict[str, Any] = {}
    if isinstance(val, dict):
        for k, v in val.items():
            out[str(k)] = _map_val(v, val_hint)
    fm.set_map(key, out)


def _map_val(v: Any, val_hint: Any) -> Any:
    """Convert a Python value to a FieldMap-compatible value for map storage."""
    if isinstance(val_hint, type) and hasattr(val_hint, "to_field_map"):
        return val_hint.to_field_map(v) if v else FieldMap()
    if val_hint is int:
        return int(v)
    if val_hint is float:
        return float(v)
    if val_hint is bool:
        return bool(v)
    if val_hint is bytes:
        return v if isinstance(v, (bytes, bytearray)) else bytes(v or b"")
    return v


def _get_fm(fm: FieldMap, key: str, hint: Any) -> Any:
    """Get value from *fm* at *key* based on *hint*."""
    kind, extra = _type_info(hint)
    if kind in ("uint8", "uint16", "uint32", "uint64", "int8", "int16", "int32", "int64"):
        return getattr(fm, f"get_{_ACCESSOR_SUFFIX[kind]}")(key) if key in fm._fields else 0
    if kind in ("float32", "float64"):
        return getattr(fm, f"get_{_ACCESSOR_SUFFIX[kind]}")(key) if key in fm._fields else 0.0
    if kind == "bool":
        return fm.get_bool(key) if key in fm._fields else False
    if kind == "string":
        return fm.get_string(key) if key in fm._fields else ""
    if kind == "bytes":
        return fm.get_bytes(key) if key in fm._fields else b""
    if kind == "list_scalar":
        return list(fm._fields.get(key) or [])
    if kind == "optional_string":
        return fm.get_optional_string(key)
    if kind == "struct" and extra:
        inner = fm.get_struct(key)
        return extra.from_field_map(inner) if inner else extra()
    if kind == "list_struct" and extra:
        items = fm.get_list_struct(key)
        return [extra.from_field_map(x) for x in items]
    if kind == "optional_struct" and extra:
        inner = fm.get_optional_struct(key)
        return extra.from_field_map(inner) if inner else None
    if kind == "map":
        return _get_map_fm(fm, key, extra)
    if kind == "sum" and extra:
        return _get_sum(fm, key, extra)
    return fm.get_string(key) if key in fm._fields else ""


def _get_map_fm(fm: FieldMap, key: str, val_hint: Any) -> dict[str, Any]:
    """Read map<K,V> from FieldMap and convert back to Python dict."""
    raw = fm.get_map(key)
    out: dict[str, Any] = {}
    for k, v in raw.items():
        out[k] = _unmap_val(v, val_hint)
    return out


def _unmap_val(v: Any, val_hint: Any) -> Any:
    """Convert a FieldMap-compatible value back to Python."""
    if isinstance(val_hint, type) and hasattr(val_hint, "from_field_map"):
        if v is None:
            return val_hint()
        return val_hint.from_field_map(v) if isinstance(v, FieldMap) else v
    return v


def _collect_bytes_snaps(fm: FieldMap, snaps: list[tuple[FieldMap, str, Any]]) -> None:
    """Walk fm recursively; append (fm, key, clone) for every bytes/bytearray/memoryview
    field and for whole list_string / map-with-bytes fields."""
    for key in sorted(fm._fields.keys()):
        val = fm._fields[key]
        if isinstance(val, (bytes, bytearray, memoryview)):
            snaps.append((fm, key, bytes(val)))
        elif isinstance(val, FieldMap):
            _collect_bytes_snaps(val, snaps)
        elif isinstance(val, list):
            # list<string> — snapshot whole list if strings present
            if val and isinstance(val[0], str):
                snaps.append((fm, key, list(val)))
            else:
                for item in val:
                    if isinstance(item, FieldMap):
                        _collect_bytes_snaps(item, snaps)
        elif isinstance(val, dict):
            # map<K,V> — snapshot whole map if values are bytes/bytearray
            if any(isinstance(v, (bytes, bytearray, memoryview)) for v in val.values()):
                snaps.append((fm, key, {k: bytes(v) if isinstance(v, (bytes, bytearray, memoryview)) else v for k, v in val.items()}))
            else:
                for item in val.values():
                    if isinstance(item, FieldMap):
                        _collect_bytes_snaps(item, snaps)
        elif isinstance(val, Variant):
            # The variant itself cannot alias, but its payload can: snapshot the
            # whole field so a zero-copy payload shows up as a change.
            if isinstance(val.value, (bytes, bytearray, memoryview)):
                snaps.append((fm, key, Variant(val.tag, bytes(val.value))))
            elif isinstance(val.value, FieldMap):
                _collect_bytes_snaps(val.value, snaps)


def _dict_diffs(before: dict[str, Any] | None, after: dict[str, Any]) -> list[str]:
    """Return sorted list of top-level keys that differ between two dicts."""
    if before is None:
        return []
    diffs: list[str] = []
    for k in sorted(set(before.keys()) | set(after.keys())):
        bv = before.get(k)
        av = after.get(k)
        if bv != av:
            diffs.append(k)
    return diffs


def _detect_zero_copy(fm: FieldMap, buf: bytearray) -> list[str]:
    """XOR-flip buf; report any FieldMap fields that change (aliasing)."""
    if not buf:
        return []

    snaps: list[tuple[FieldMap, str, Any]] = []
    _collect_bytes_snaps(fm, snaps)

    # XOR-flip
    for i in range(len(buf)):
        buf[i] ^= 0xFF

    aliased: list[str] = []
    for target_fm, key, orig in snaps:
        cur = target_fm._fields.get(key)
        if isinstance(orig, bytes):
            if isinstance(cur, (bytes, bytearray, memoryview)) and bytes(cur) != orig:
                aliased.append(key)
        elif isinstance(orig, Variant):
            if isinstance(cur, Variant) and bytes(cur.value) != orig.value:
                aliased.append(key)
        elif cur != orig:
            aliased.append(key)

    # Restore
    for target_fm, key, orig in snaps:
        target_fm._fields[key] = orig

    return aliased


def run(serialize_fn: SerializeFn, deserialize_fn: DeserializeFn) -> None:
    """Single-type worker: handles whatever type/format the runner asks for."""
    _run_loop(lambda _type, _format: (serialize_fn, deserialize_fn))


class Format:
    """One named format: a serializer, and optionally a deserializer.

    Under a `Type` that carries a model, the functions speak that model and
    never see a FieldMap — serify converts on the way in and out:

        Format(LedgerEntry.marshal, LedgerEntry.unmarshal)

    Under a `Type` with no model, they take and return the FieldMap itself,
    which is what a type with no natural class needs (the audit fixtures
    mutate a FieldMap on purpose).
    """

    def __init__(self, serializer: Any, deserializer: Any = None) -> None:
        self.serializer = serializer
        self.deserializer = deserializer

    def _bind(self, model: type | None) -> tuple[SerializeFn | None, DeserializeFn | None]:
        """Wrap the worker's functions into the FieldMap-typed pair the run loop
        calls. With no model the functions already are that pair."""
        ser, deser = self.serializer, self.deserializer

        if model is None:
            return cast("SerializeFn | None", ser), cast("DeserializeFn | None", deser)

        from_fm = model.from_field_map  # type: ignore[attr-defined]

        def _serialize(fm: FieldMap) -> bytes:
            return cast(bytes, ser(from_fm(fm)))

        def _deserialize(data: bytes) -> FieldMap:
            return cast(FieldMap, deser(data).to_field_map())

        return (
            _serialize if ser is not None else None,
            _deserialize if deser is not None else None,
        )


class Type:
    """One data type: a model and its named formats.

    `model` is the class the format functions speak — anything with
    `from_field_map`/`to_field_map`, which `@serify_model` generates. Pass
    None to register FieldMap-typed functions instead.
    """

    def __init__(self, model: type | None, formats: dict[str, Format]) -> None:
        self.model = model
        self.formats = formats

    def _resolve(self, format_name: str) -> tuple[SerializeFn | None, DeserializeFn | None] | None:
        fmt = self.formats.get(format_name)
        if fmt is None:
            return None
        return fmt._bind(self.model)


# A registered type is either a Type, or the plain nested dict of
# (serialize, deserialize) tuples that workers used before Type existed.
_Registered = Union["Type", dict[str, tuple[SerializeFn, DeserializeFn]]]


def _resolve_registered(
    types: dict[str, _Registered], type_name: str, format_name: str
) -> tuple[SerializeFn | None, DeserializeFn | None] | None:
    """Look one (type, format) up across both registration spellings.

    Separate from run_suite so it can be tested without stdin: an unresolved
    (type, format) is reported SKIPPED, so a registration shape this function
    silently fails to understand produces a *green* conformance run made
    entirely of SKIPs. Nothing downstream can tell that apart from a worker
    that legitimately does not implement the type.
    """
    entry = types.get(type_name)
    if entry is None:
        return None
    if isinstance(entry, Type):
        return entry._resolve(format_name)
    return entry.get(format_name)


def run_suite(types: dict[str, _Registered]) -> None:
    """Multi-type worker. `types` maps a type name to a `Type`, or to the older
    format-name -> (serialize, deserialize) dict, which still works. A (type,
    format) that is not registered is reported to the runner as SKIPPED rather
    than failing the run."""
    _run_loop(lambda t, f: _resolve_registered(types, t, f))


def _run_loop(
    resolve: Callable[[str, str], tuple[SerializeFn | None, DeserializeFn | None] | None],
) -> None:
    serialize_fn: SerializeFn | None = None
    deserialize_fn: DeserializeFn | None = None

    if hasattr(sys.stdin, 'reconfigure'):
        sys.stdin.reconfigure(encoding='utf-8')
    if hasattr(sys.stdout, 'reconfigure'):
        sys.stdout.reconfigure(encoding='utf-8')

    schema: _Schema = []
    audit_enabled: bool = False
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg: dict[str, Any] = json.loads(line)
        except json.JSONDecodeError:
            continue

        op: str = msg.get("op", "")
        msg_id: str = msg.get("id", "")

        if op == "ping":
            # Health check: report liveness and the protocol revision this
            # library speaks. Binds nothing.
            _emit({"op": "ping", "status": "OK", "protocol_version": PROTOCOL_VERSION})

        elif op == "bind":
            schema = msg.get("schema", [])
            audit_enabled = msg.get("audit", False)
            pair = resolve(msg.get("type", ""), msg.get("format", ""))
            if pair is None:
                serialize_fn = deserialize_fn = None
                _emit({"op": "bind", "status": "SKIPPED"})
                continue
            serialize_fn, deserialize_fn = pair
            _emit({"op": "bind"})

        elif op == "serialize":
            try:
                if serialize_fn is None:
                    raise RuntimeError("serialize before a successful bind")
                fm = decode_field_map(msg["data"], schema)

                before = encode_field_map(fm, schema) if audit_enabled else None

                b = serialize_fn(fm)
                hex_str = b.hex()

                audit: dict[str, Any] = {}
                if audit_enabled:
                    # Mutation: compare FieldMap before/after serialization.
                    after = encode_field_map(fm, schema)
                    diffs = _dict_diffs(before, after)
                    if diffs:
                        audit["mutations"] = diffs

                    # Output zero-copy: does returned buffer alias model fields?
                    # Only a mutable return type (bytearray/memoryview) can be
                    # flipped in place; immutable bytes cannot alias mutably, so
                    # there is nothing to detect for them.
                    if isinstance(b, (bytearray, memoryview)) and len(b) > 0:
                        before_clone = encode_field_map(fm, schema)
                        for i in range(len(b)):
                            b[i] ^= 0xFF
                        after_flip = encode_field_map(fm, schema)
                        for i in range(len(b)):
                            b[i] ^= 0xFF  # restore
                        ozc = _dict_diffs(before_clone, after_flip)
                        if ozc:
                            audit["output_zero_copy_fields"] = ozc

                    # Stability: serialize again, compare output.
                    try:
                        b2 = serialize_fn(fm)
                        if hex_str != b2.hex():
                            audit["stable"] = False
                    except Exception:
                        audit["stable"] = False

                resp: dict[str, Any] = {"id": msg_id, "op": "serialize", "status": "OK", "hex": hex_str}
                if audit:
                    resp["audit"] = audit
                _emit(resp)
            except Exception as e:
                _emit({"id": msg_id, "op": "serialize", "status": "ERROR", "error": str(e)})

        elif op == "deserialize":
            try:
                if deserialize_fn is None:
                    raise RuntimeError("deserialize before a successful bind")
                raw = bytes.fromhex(msg["hex"])
                buf = bytearray(raw)
                buf_snapshot = bytes(buf) if audit_enabled else None

                fm = deserialize_fn(bytes(buf))

                daudit: dict[str, Any] = {}
                if audit_enabled:
                    # Input-buffer mutation.
                    if buf_snapshot != bytes(buf):
                        daudit["input_mutated"] = True

                    # Deserialize stability: re-deserialize from a fresh clone.
                    if buf_snapshot is not None:
                        try:
                            fm2 = deserialize_fn(buf_snapshot)
                            diffs = _dict_diffs(
                                encode_field_map(fm, schema),
                                encode_field_map(fm2, schema),
                            )
                            if diffs:
                                daudit["deser_stable"] = False
                        except Exception:
                            daudit["deser_stable"] = False

                    # Zero-copy: XOR-flip buffer, check FieldMap, restore.
                    zc = _detect_zero_copy(fm, buf)
                    if zc:
                        daudit["zero_copy_fields"] = zc

                data = encode_field_map(fm, schema)
                resp = {"id": msg_id, "op": "deserialize", "status": "OK", "data": data}
                if daudit:
                    resp["audit"] = daudit
                _emit(resp)
            except Exception as e:
                _emit({"id": msg_id, "op": "deserialize", "status": "ERROR", "error": str(e)})

        elif op == "exit":
            sys.exit(0)


def _emit(obj: dict[str, Any]) -> None:
    print(json.dumps(obj), flush=True)
