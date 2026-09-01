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

"""The taskstore wire format, in Python.

This is the follower's half of the contract. Go leads: the layouts below were
not designed here, they were read off go/api/wire.go and reproduced, which is
exactly the position a second language is in on a real project.

Like its Go counterpart this module imports only the standard library. The
schema binding lives in worker.py, which is the one file that knows serify
exists — see the comment there.
"""

import io
import struct
from dataclasses import dataclass, field

# The enums, in declaration order: position is the ordinal on the wire.
PRIORITIES = ("low", "normal", "high")
ERROR_CODES = ("not_found", "invalid", "conflict")


class TruncatedError(ValueError):
    """A message ended in the middle of a value."""


class _Reader:
    """A cursor over one message's bytes.

    Go threads the remaining slice through every read; Python is happier with a
    cursor. Same layout either way — the byte order and widths are the contract,
    not the shape of the code that walks them.
    """

    def __init__(self, data: bytes):
        self.data = data
        self.off = 0

    def _take(self, n: int) -> bytes:
        if self.off + n > len(self.data):
            raise TruncatedError(f"want {n} bytes at offset {self.off}, have {len(self.data) - self.off}")
        chunk = self.data[self.off:self.off + n]
        self.off += n
        return chunk

    def u8(self) -> int:
        return self._take(1)[0]

    def u32(self) -> int:
        return struct.unpack("<I", self._take(4))[0]

    def u64(self) -> int:
        return struct.unpack("<Q", self._take(8))[0]

    def i64(self) -> int:
        return struct.unpack("<q", self._take(8))[0]

    def boolean(self) -> bool:
        return self.u8() != 0

    def string(self) -> str:
        return self._take(self.u32()).decode()

    def enum(self, variants: tuple[str, ...]) -> str:
        ord_ = self.u8()
        if ord_ >= len(variants):
            raise ValueError(f"enum ordinal {ord_} out of range for {variants}")
        return variants[ord_]

    def tags(self) -> list[str]:
        return [self.string() for _ in range(self.u32())]

    def optional_i64(self) -> int | None:
        return self.i64() if self.u8() else None


def _pack_str(s: str) -> bytes:
    b = s.encode()
    return struct.pack("<I", len(b)) + b


def _pack_enum(variants: tuple[str, ...], name: str) -> bytes:
    try:
        return struct.pack("<B", variants.index(name))
    except ValueError:
        raise ValueError(f"{name!r} is not one of {variants}") from None


def _pack_tags(tags: list[str]) -> bytes:
    return struct.pack("<I", len(tags)) + b"".join(_pack_str(t) for t in tags)


def _pack_optional_i64(v: int | None) -> bytes:
    return b"\x00" if v is None else b"\x01" + struct.pack("<q", v)


# --- the records -------------------------------------------------------------


@dataclass
class Task:
    """A stored task: cases/task.yaml."""

    id: int
    title: str
    done: bool
    priority: str
    tags: list[str] = field(default_factory=list)
    due_at: int | None = None

    def pack(self) -> bytes:
        return (
            struct.pack("<Q", self.id)
            + _pack_str(self.title)
            + struct.pack("<?", self.done)
            + _pack_enum(PRIORITIES, self.priority)
            + _pack_tags(self.tags)
            + _pack_optional_i64(self.due_at)
        )

    @classmethod
    def read(cls, r: _Reader) -> "Task":
        return cls(
            id=r.u64(),
            title=r.string(),
            done=r.boolean(),
            priority=r.enum(PRIORITIES),
            tags=r.tags(),
            due_at=r.optional_i64(),
        )


@dataclass
class Draft:
    """A task the server has not assigned an id to yet: cases/draft.yaml."""

    title: str
    priority: str = "normal"
    tags: list[str] = field(default_factory=list)
    due_at: int | None = None

    def pack(self) -> bytes:
        return (
            _pack_str(self.title)
            + _pack_enum(PRIORITIES, self.priority)
            + _pack_tags(self.tags)
            + _pack_optional_i64(self.due_at)
        )

    @classmethod
    def read(cls, r: _Reader) -> "Draft":
        return cls(
            title=r.string(),
            priority=r.enum(PRIORITIES),
            tags=r.tags(),
            due_at=r.optional_i64(),
        )


@dataclass
class TaskPage:
    """The body of a list response: cases/task_page.yaml."""

    items: list[Task] = field(default_factory=list)
    total: int = 0

    def pack(self) -> bytes:
        return (
            struct.pack("<I", len(self.items))
            + b"".join(t.pack() for t in self.items)
            + struct.pack("<I", self.total)
        )

    @classmethod
    def read(cls, r: _Reader) -> "TaskPage":
        items = [Task.read(r) for _ in range(r.u32())]
        return cls(items=items, total=r.u32())


@dataclass
class ApiError:
    """A failed request: cases/api_error.yaml."""

    code: str
    message: str

    def pack(self) -> bytes:
        return _pack_enum(ERROR_CODES, self.code) + _pack_str(self.message)

    @classmethod
    def read(cls, r: _Reader) -> "ApiError":
        return cls(code=r.enum(ERROR_CODES), message=r.string())


# --- the sums ----------------------------------------------------------------
#
# A union of dataclasses is Python's sum type, and it is all the schema binding
# needs: the union names the arms, each arm's own fields give its payload, and a
# request carrying two operations at once is unwritable. Go needs a hand-written
# converter for the same five arms (see go/main.go) because its sealed interface
# cannot be enumerated at run time.
#
# The arity rule is the schema's: 0 fields is a unit variant, 1 field is that
# value as the payload, N fields make the payload a struct. Every arm here is 0
# or 1, because each payload is already a record of its own.


@dataclass
class ListAll:
    """arity 0 — a unit variant, no payload"""


@dataclass
class Create:
    value: Draft


@dataclass
class Read:
    value: int


@dataclass
class Update:
    value: Task


@dataclass
class Delete:
    value: int


Op = ListAll | Create | Read | Update | Delete

# Tag ordinals, in the declaration order of cases/op.yaml.
_OP_ARMS = (ListAll, Create, Read, Update, Delete)


@dataclass
class Accepted:
    """arity 0 — a delete that worked, nothing to send back"""


@dataclass
class Found:
    value: Task


@dataclass
class Listing:
    value: TaskPage


@dataclass
class Failed:
    value: ApiError


Result = Accepted | Found | Listing | Failed

# Tag ordinals, in the declaration order of cases/result.yaml.
_RESULT_ARMS = (Accepted, Found, Listing, Failed)


def _pack_op(op: Op) -> bytes:
    tag = struct.pack("<B", _OP_ARMS.index(type(op)))
    match op:
        case ListAll():
            return tag  # a unit variant is nothing but its tag
        case Create(value=draft):
            return tag + draft.pack()
        case Read(value=task_id) | Delete(value=task_id):
            return tag + struct.pack("<Q", task_id)
        case Update(value=task):
            return tag + task.pack()
    raise ValueError(f"unhandled op {type(op).__name__}")


def _read_op(r: _Reader) -> Op:
    """Rebuild the arm the tag names.

    Neither direction spells an ordinal out: writing takes it from the arm's
    position in _OP_ARMS, reading indexes back into the same tuple. Reordering
    that tuple breaks the wire, which is true and is meant to be — it is the
    declaration order of cases/op.yaml.
    """
    tag = r.u8()
    if tag >= len(_OP_ARMS):
        raise ValueError(f"unknown op tag {tag}")
    arm = _OP_ARMS[tag]
    if arm is ListAll:
        return ListAll()
    if arm is Create:
        return Create(Draft.read(r))
    if arm is Update:
        return Update(Task.read(r))
    if arm is Read:
        return Read(r.u64())
    return Delete(r.u64())  # read and delete alike carry nothing but an id


def _read_result(r: _Reader) -> Result:
    tag = r.u8()
    if tag >= len(_RESULT_ARMS):
        raise ValueError(f"unknown result tag {tag}")
    arm = _RESULT_ARMS[tag]
    if arm is Accepted:
        return Accepted()
    if arm is Found:
        return Found(Task.read(r))
    if arm is Listing:
        return Listing(TaskPage.read(r))
    return Failed(ApiError.read(r))


def _pack_result(res: Result) -> bytes:
    tag = struct.pack("<B", _RESULT_ARMS.index(type(res)))
    match res:
        case Accepted():
            return tag
        case Found(value=task):
            return tag + task.pack()
        case Listing(value=page):
            return tag + page.pack()
        case Failed(value=err):
            return tag + err.pack()
    raise ValueError(f"unhandled result {type(res).__name__}")


# --- the two messages that cross the socket ----------------------------------


@dataclass
class Request:
    """One request frame: cases/request.yaml."""

    request_id: int
    op: Op

    def marshal(self) -> bytes:
        return struct.pack("<I", self.request_id) + _pack_op(self.op)

    @classmethod
    def unmarshal(cls, data: bytes) -> "Request":
        r = _Reader(data)
        return cls(request_id=r.u32(), op=_read_op(r))


@dataclass
class Response:
    """One response frame: cases/response.yaml."""

    request_id: int
    result: Result

    def marshal(self) -> bytes:
        return struct.pack("<I", self.request_id) + _pack_result(self.result)

    @classmethod
    def unmarshal(cls, data: bytes) -> "Response":
        r = _Reader(data)
        return cls(request_id=r.u32(), result=_read_result(r))


# --- framing -----------------------------------------------------------------
#
# A TCP connection is a byte stream with no message boundaries in it, so the
# protocol puts them there: a u32 little-endian length, then that many bytes.
#
# This sits outside the conformance suite on purpose. serify tests the contents
# of a message — it hands a worker one message's bytes — so the frame header is
# in none of the cases, and a follower could read its socket some other way and
# still pass. What it may not do is disagree about what is inside.

MAX_FRAME_LEN = 1 << 20


def write_frame(w: io.BufferedIOBase, payload: bytes) -> None:
    if len(payload) > MAX_FRAME_LEN:
        raise ValueError(f"message of {len(payload)} bytes exceeds the {MAX_FRAME_LEN} limit")
    w.write(struct.pack("<I", len(payload)) + payload)
    w.flush()


def read_frame(r: io.BufferedIOBase) -> bytes:
    """Read one length-prefixed message.

    Raises EOFError when the stream ends cleanly between messages — that is the
    other side hanging up, not a failure.
    """
    header = r.read(4)
    if not header:
        raise EOFError("connection closed")
    if len(header) < 4:
        raise TruncatedError("truncated frame header")
    n = struct.unpack("<I", header)[0]
    if n > MAX_FRAME_LEN:
        raise ValueError(f"frame header claims {n} bytes, over the {MAX_FRAME_LEN} limit")

    buf = bytearray()
    while len(buf) < n:
        chunk = r.read(n - len(buf))
        if not chunk:
            raise TruncatedError(f"frame ended after {len(buf)} of {n} bytes")
        buf += chunk
    return bytes(buf)
