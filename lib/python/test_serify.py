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

"""Unit tests for the @serify_model decorator and FieldMap.

This module deliberately uses ``from __future__ import annotations`` so that
every type hint is a string (forward reference). Resolving them requires a
working typing.get_type_hints path in serify._get_type_hints — a regression
test for the bug where the ``typing`` module was never imported and resolution
always silently fell back to raw ``__annotations__``.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from serify import FieldMap, Variant, _detect_zero_copy, decode_field_map, encode_field_map, serify_model


@serify_model
@dataclass
class Money:
    amount: int
    currency: str


@serify_model
@dataclass
class User:
    user_id: int
    name: str
    active: bool
    score: float
    email: str = field(default="", metadata={"serify": "email_addr"})
    balance: Money = None  # type: ignore[assignment]
    tags: list[str] = field(default_factory=list)
    limits: dict[str, int] = field(default_factory=dict)


def make_user() -> User:
    return User(
        user_id=42,
        name="Dana",
        active=True,
        score=1.5,
        email="dana@example.com",
        balance=Money(amount=1999, currency="USD"),
        tags=["a", "b"],
        limits={"daily": 10, "monthly": 300},
    )


def test_forward_refs_resolve():
    # With future annotations, hints are strings; the nested model and builtins
    # must still resolve to real types instead of falling back to raw strings.
    from serify import _get_type_hints

    hints = _get_type_hints(User)
    assert hints["user_id"] is int
    assert hints["balance"] is Money
    assert hints["tags"] == list[str]


def test_round_trip_scalars_and_rename():
    u = make_user()
    fm = u.to_field_map()

    assert fm.get_i64("user_id") == 42
    assert fm.get_string("name") == "Dana"
    assert fm.get_bool("active") is True
    assert fm.get_f64("score") == 1.5
    # Renamed via field metadata.
    assert fm.get_string("email_addr") == "dana@example.com"

    back = User.from_field_map(fm)
    assert back == u


def test_nested_struct_maps_to_field_map():
    fm = make_user().to_field_map()
    nested = fm.get_struct("balance")
    assert isinstance(nested, FieldMap)
    assert nested.get_i64("amount") == 1999
    assert nested.get_string("currency") == "USD"


def test_dict_maps_to_map():
    fm = make_user().to_field_map()
    assert fm.get_map("limits") == {"daily": 10, "monthly": 300}


def test_list_string_round_trip():
    fm = make_user().to_field_map()
    assert fm.get_list_string("tags") == ["a", "b"]
    assert User.from_field_map(fm).tags == ["a", "b"]


def test_large_int_precision():
    # Values beyond 2^53 must survive exactly (Python ints are arbitrary
    # precision; nothing in the model path may route them through float).
    u = make_user()
    u.user_id = 18446744073709551615
    fm = u.to_field_map()
    assert fm.get_u64("user_id") == 18446744073709551615
    assert User.from_field_map(fm).user_id == 18446744073709551615


# --- oneof ------------------------------------------------------------------


def oneof_schema():
    """A oneof carrying a bytes payload — the shape zero-copy detection must see."""
    return [{
        "name": "channel",
        "type": "oneof<silent, receipt: bytes>",
        "fields": [],
        "variants": [
            {"name": "silent"},
            {"name": "receipt", "payload": {"name": "receipt", "type": "bytes", "fields": []}},
        ],
    }]


def test_oneof_round_trip():
    schema = oneof_schema()
    for data in ({"channel": {"silent": None}}, {"channel": {"receipt": "deadbeef"}}):
        fm = decode_field_map(data, schema)
        assert encode_field_map(fm, schema) == data


def test_oneof_detect_zero_copy_on_payload():
    # A deserializer that aliases the input buffer into a oneof bytes payload
    # must be reported. Before _collect_bytes_snaps grew a Variant branch the
    # walker never descended into the variant and this returned [].
    buf = bytearray(b"\x01\x02\x03\x04")
    fm = FieldMap()
    fm.set_variant("channel", "receipt", memoryview(buf))  # aliased, not copied

    assert _detect_zero_copy(fm, buf) == ["channel"]


def test_oneof_detect_zero_copy_ignores_owned_payload():
    # The mirror image: a payload that copied out of the buffer is not aliasing
    # and must stay silent, or every correct worker would be warned at.
    buf = bytearray(b"\x01\x02\x03\x04")
    fm = FieldMap()
    fm.set_variant("channel", "receipt", bytes(buf))  # copied

    assert _detect_zero_copy(fm, buf) == []


# ─── list element types ─────────────────────────────────────────────────────

def test_list_supports_every_scalar_elem():
    """A list must support every element type a bare field does.

    _decode_list used to be its own chain of type tests that silently fell
    through to ``list(arr)``: list<float64> handed the worker raw hex strings
    and list<int8>/list<int16>/list<bytes> were never converted either, with no
    error raised anywhere.
    """
    import struct as _struct

    cases = [
        ("uint8", [0, 255]),
        ("uint16", [0, 65535]),
        ("uint32", [0, 4294967295]),
        ("uint64", ["0", "18446744073709551615"]),
        ("int8", [-128, 127]),
        ("int16", [-32768, 32767]),
        ("int32", [-2147483648, 2147483647]),
        ("int64", ["-9223372036854775808", "0"]),
        ("uint128", ["340282366920938463463374607431768211455", "0"]),
        ("int128", ["-170141183460469231731687303715884105728", "0"]),
        ("float32", [_struct.pack("<f", 1.5).hex()]),
        ("float64", [_struct.pack("<d", -2.0).hex()]),
        ("bool", [True, False]),
        ("string", ["a", ""]),
        ("bytes", ["dead", ""]),
    ]

    for elem, sent in cases:
        schema = [{"name": "v", "type": f"list<{elem}>"}]
        fm = decode_field_map({"v": sent}, schema)

        # Re-encoding must reproduce the wire form the runner sent, so the two
        # directions cannot drift apart for any element type.
        assert encode_field_map(fm, schema) == {"v": sent}, f"list<{elem}> did not round-trip"

    # float64 in particular must arrive as floats, not the hex it travelled as.
    fm = decode_field_map(
        {"v": [_struct.pack("<d", 1.5).hex(), _struct.pack("<d", -2.0).hex()]},
        [{"name": "v", "type": "list<float64>"}],
    )
    assert fm._fields["v"] == [1.5, -2.0]


def test_list_rejects_unknown_elem_type():
    """An unsupported element type must raise, not pass the raw JSON through."""
    try:
        decode_field_map({"v": [1, 2]}, [{"name": "v", "type": "list<nope>"}])
    except ValueError as exc:
        assert "nope" in str(exc)
    else:
        raise AssertionError("expected ValueError for list<nope>")


def test_map_supports_every_scalar_value():
    """A map must support every value type a bare field does.

    _decode_map used to be its own chain of type tests that silently fell
    through to the raw JSON, so a map<string,float64> handed the worker hex
    strings — the same defect list<float64> had.
    """
    import struct as _struct

    cases = [
        ("uint8", {"a": 0, "b": 255}),
        ("uint16", {"a": 65535}),
        ("uint32", {"a": 4294967295}),
        ("uint64", {"a": "18446744073709551615"}),
        ("int8", {"a": -128}),
        ("int16", {"a": -32768}),
        ("int32", {"a": -2147483648}),
        ("int64", {"a": "-9223372036854775808"}),
        ("float32", {"a": _struct.pack("<f", 1.5).hex()}),
        ("float64", {"a": _struct.pack("<d", -2.0).hex()}),
        ("bool", {"a": True, "b": False}),
        ("string", {"a": "x"}),
        ("bytes", {"a": "dead"}),
    ]

    for val_type, sent in cases:
        schema = [{"name": "m", "type": f"map<string,{val_type}>"}]
        fm = decode_field_map({"m": sent}, schema)
        assert encode_field_map(fm, schema) == {"m": sent}, f"map<string,{val_type}> did not round-trip"

    # float64 in particular must arrive as floats, not the hex it travelled as.
    fm = decode_field_map(
        {"m": {"a": _struct.pack("<d", 1.5).hex()}},
        [{"name": "m", "type": "map<string,float64>"}],
    )
    assert fm._fields["m"] == {"a": 1.5}


def test_map_rejects_unknown_value_type():
    try:
        decode_field_map({"m": {"a": 1}}, [{"name": "m", "type": "map<string,nope>"}])
    except ValueError as exc:
        assert "nope" in str(exc)
    else:
        raise AssertionError("expected ValueError for map<string,nope>")


def test_unknown_field_type_raises_both_directions():
    """A type the library does not know must fail loudly, not silently.

    Decoding used to fall off the chain (leaving the field absent from the
    FieldMap) and encoding used to `return v` untouched. Both surface far
    downstream as a missing or wrong value rather than as "this library does not
    know that type" — which is exactly what happened to every library's audit
    walker when `oneof` was added.
    """
    schema = [{"name": "v", "type": "nope"}]
    try:
        decode_field_map({"v": 1}, schema)
    except ValueError as exc:
        assert "nope" in str(exc)
    else:
        raise AssertionError("expected ValueError decoding an unknown type")

    fm = FieldMap()
    fm.set_u32("v", 1)
    try:
        encode_field_map(fm, schema)
    except ValueError as exc:
        assert "nope" in str(exc)
    else:
        raise AssertionError("expected ValueError encoding an unknown type")
