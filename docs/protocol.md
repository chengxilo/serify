# Serify NDJSON Protocol

This document specifies the wire protocol between the `serify` CLI runner and
language worker subprocesses. A third-party library author can use this spec to
implement a worker in any language without referencing the Go source.

## Transport

The runner starts a worker subprocess and communicates over **stdin** (requests)
and **stdout** (responses). Each message is a single **JSON object terminated by
a newline (`\n`)** — i.e. [NDJSON](https://github.com/ndjson/ndjson-spec).
Stderr is reserved for diagnostic output and is never parsed by the runner.

## Messages

### Ping (runner → worker)

The first message after startup, and the only one the runner sends before it
knows anything about the worker:

```json
{"op": "ping"}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `op` | string | yes | Must be `"ping"` |

Ping is a health check with a payload. It answers two questions at once: did the
process come up and speak the protocol, and is its worker library the same
revision as the runner?

Ping names no type and carries no schema, so it **binds nothing**. A serialize or
deserialize sent after only a ping must fail the same way it would before any
bind at all. Workers must not treat ping as a reason to change bound state.

### Ping response (worker → runner)

```json
{
  "op": "ping",
  "status": "OK",
  "protocol_version": 1
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `op` | string | yes | Must be `"ping"` |
| `status` | string | yes | Must be `"OK"` — a worker that can answer at all is healthy |
| `protocol_version` | integer | yes | The revision of this document the worker library implements |
| `error` | string | no | Error message (when status is `"ERROR"`) |

`"SKIPPED"` is not a valid ping response: no type was named, so there is nothing
to skip.

### Protocol version

`protocol_version` is a single integer, currently **1**, incremented on every
breaking change to the messages in this document. The runner requires an **exact
match** and refuses to start a worker that reports anything else:

```
Error: start rust worker: ping rust worker: protocol version mismatch:
  worker library speaks 1, this serify needs 2 — rebuild the worker
  against the current library
```

There is no version range and nothing to negotiate. The runner and every worker
library ship from the same repository, so a mismatch never means "an older peer
we should accommodate" — it means one side was built from stale sources, which is
a build problem to fix rather than a case to handle at runtime.

A worker does not report which *types* it implements. That question is answered
per (type, format) by the `"SKIPPED"` bind response below, where it is actually
needed and cannot drift out of date.

### Bind (runner → worker)

Sent before each group of cases for one (type, format). It declares the schema
and tells the worker which type and format to prepare for. Unlike ping, bind
always binds — that is the whole operation, and it happens once per group rather
than once per process, which is why it is not called init.

```json
{
  "op": "bind",
  "schema": [
    {
      "name": "field_name",
      "type": "uint64",
      "fields": [],
      "key_type": "",
      "tags": {}
    }
  ],
  "type": "user",
  "format": "binary",
  "audit": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `op` | string | yes | Must be `"bind"` |
| `schema` | array | yes | Ordered list of schema fields |
| `type` | string | yes | Type name (e.g. `"user"`) — must be non-empty |
| `format` | string | yes | Format name (e.g. `"binary"`, `"json"`) — must be non-empty |
| `audit` | boolean | no | If true, enable unsafe-behaviour detection checks |

Each schema field:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Field name |
| `type` | string | Type string (see [Type System](#type-system)) |
| `fields` | array | Nested schema for struct/list\<struct\>/map\<K,struct\> |
| `key_type` | string | Map key type (e.g. `"string"`) |
| `variants` | array | Arms of a `sum<...>`; absent for every other type |
| `tags` | object | Arbitrary metadata from case files |

Each entry in `variants`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Variant tag |
| `payload` | object | The payload's own schema field; **absent for a unit variant** |

The payload is a full schema field, so it nests: a struct payload carries its own
`fields`, and a worker decodes it by re-entering its normal field decoder.

### Bind response (worker → runner)

```json
{
  "op": "bind",
  "status": "OK"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `op` | string | yes | Must be `"bind"` |
| `status` | string | yes | `"OK"`, `"SKIPPED"`, or `"ERROR"` |
| `error` | string | no | Error message (when status is `"ERROR"`) |
| `reason` | string | no | Why the type was declined (when status is `"SKIPPED"`) |

**Strictness:** The runner requires a non-empty `type` AND `format` in the bind
request; either being empty is a fatal error. A worker never uses bind to report
anything about itself.

### Serialize (runner → worker)

```json
{
  "id": "user/binary/basic",
  "op": "serialize",
  "data": {
    "user_id": "42",
    "score": "00000000",
    "active": true,
    "metadata": "deadbeef"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Test identifier (globally unique: `type/format/case`) |
| `op` | string | Must be `"serialize"` |
| `data` | object | Encoded field values (see [Data Encoding](#data-encoding)) |

### Serialize response (worker → runner)

```json
{
  "id": "user/binary/basic",
  "op": "serialize",
  "status": "OK",
  "hex": "010000002a0000000000000000000000",
  "audit": {
    "mutations": [],
    "stable": true,
    "zero_copy_fields": [],
    "input_mutated": false
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Must match the request `id` |
| `op` | string | yes | Must be `"serialize"` |
| `status` | string | yes | `"OK"`, `"ERROR"`, or `"SKIPPED"` |
| `hex` | string | no | Serialized bytes as hex (when OK) |
| `error` | string | no | Error message (when ERROR) |
| `reason` | string | no | Skip reason (when SKIPPED) |
| `audit` | object | no | Audit findings (only when `--audit` is active) |

### Deserialize (runner → worker)

```json
{
  "id": "user/binary/basic",
  "op": "deserialize",
  "hex": "010000002a0000000000000000000000"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Test identifier |
| `op` | string | Must be `"deserialize"` |
| `hex` | string | Hex-encoded bytes to deserialize (from the reference worker's serialize) |

### Deserialize response (worker → runner)

```json
{
  "id": "user/binary/basic",
  "op": "deserialize",
  "status": "OK",
  "data": {
    "user_id": "42",
    "score": "00000000",
    "active": true,
    "metadata": "deadbeef"
  },
  "audit": {
    "mutations": [],
    "stable": true,
    "zero_copy_fields": [],
    "input_mutated": false
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Must match the request `id` |
| `op` | string | yes | Must be `"deserialize"` |
| `status` | string | yes | `"OK"`, `"ERROR"`, or `"SKIPPED"` |
| `data` | object | no | Decoded field values (when OK) |
| `audit` | object | no | Audit findings (only when `--audit` is active) |

### Exit (runner → worker)

```json
{"op": "exit"}
```

The worker should terminate cleanly. The runner closes stdin after sending this
message. No response is expected.

## Status codes

| Status | Meaning |
|--------|---------|
| `OK` | Operation completed successfully |
| `ERROR` | Operation failed (serialization/deserialization error) |
| `SKIPPED` | Operation skipped (e.g. unsupported format, or direction not registered when a format supports only serialize or only deserialize) |

## Audit report

When the `--audit` flag is set, the runner passes `"audit": true` in the bind
message. The worker should enable unsafe-behaviour detection and populate the
`audit` field in serialize/deserialize responses:

```json
{
  "mutations": ["value"],
  "stable": false,
  "zero_copy_fields": ["payload"],
  "input_mutated": true
}
```

Four detections are defined:

| Detection | When | How |
|-----------|------|-----|
| **Serialize mutation** | Serializer mutates the input | Snapshot before, compare after |
| **Serialize instability** | Serializer produces different output on repeat | Call twice, compare hex |
| **Deserialize zero-copy** | Deserialized fields alias the input buffer | XOR-flip buffer, check which fields change |
| **Input-buffer mutation** | Deserializer modifies the raw input | Snapshot buffer before, compare after |

Audit findings are **warnings, not errors** — they appear in the report as
`WARN` and do not cause a non-zero exit.

## Data encoding

Test data values travel through the wire in encoded form to avoid JSON number
precision loss. Workers must decode these representations before applying their
own byte encoding.

| Type kind | Wire encoding | Example |
|-----------|---------------|---------|
| `uint8`–`uint32`, `int8`–`int32` | JSON number (native) | `42` |
| `uint64`, `int64`, `uint128`, `int128` | Decimal string | `"18446744073709551615"` |
| `float32`, `float64` | IEEE 754 little-endian hex | `"0000803f"` (1.0f) |
| `bool` | JSON boolean | `true` |
| `string` | JSON string | `"hello"` |
| `bytes` | Hex string | `"deadbeef"` |
| `optional<T>` | Encoded `T` or `null` | `"42"` or `null` |
| `list<T>` | JSON array of encoded `T`; `T` may be any scalar or `struct` | `["42", "43"]` |
| `array<T,N>` | JSON array of encoded `T`, length N | `[1, 2, 3, 4]` |
| `map<K,V>` | JSON object with encoded values | `{"a": "42"}` |
| `struct` | JSON object with encoded fields | `{"street": "Main", "zip": 12345}` |
| `enum<a,b,c>` | Variant name as a JSON string | `"b"` |
| `sum<a, b: T>` | Single-key object `{tag: encoded payload}`; `null` payload for a unit variant | `{"b": "42"}`, `{"a": null}` |

### Key gotchas

- **64/128-bit integers**: JSON numbers lose precision beyond 2^53. Always
  encode as decimal strings on the wire, parse them into native integers in the
  worker, and use native integer types in the byte format. The runner receives
  decoded data back from workers via JSON, so values that arrive as `float64`
  after the JSON round-trip are compared with go-cmp against the original
  encoded values (strings).

- **Floats**: The wire uses IEEE 754 little-endian hex to avoid any
  decimal↔binary rounding. Workers should parse the hex into native `f32`/`f64`.

- **Bytes**: Encoded as hex. Common mistake: storing bytes in the wire format
  without hex-decoding first.

## Comparison oracles

How a worker's serialize output is judged is declared per **(type, format)**, in
the type's own `formats:` list. Declaring it is **mandatory** — there is no
default, because the choice decides whether a disagreement between two workers
is a failure or is allowed wire freedom, and that is not something a file should
pick silently.

```yaml
# customer.yaml — holds a map<K,V>, so entry order is free
formats:
  - name: binary
    oracle: semantic

# ledger.yaml — no map; byte parity across all nine languages
formats:
  - name: binary
    oracle: bytes
```

Note that both files name `binary` and resolve it differently. The oracle is a
property of the *pair*, not of the format name: one type holding a map wants
`semantic` for a format that a map-free sibling needs on `bytes`. A suite-level
`oracle:` could not express that, and declaring one in `_config.yaml` is an
error rather than a silently-ignored setting.

The suite's `<cases>/_config.yaml` declares only the format-name *universe*
every type must pick from (plus `reference_language`):

```yaml
# _config.yaml
reference_language: go
formats: [binary, json]
```

| Oracle | How the verdict is reached | Use it for |
|--------|----------------------------|------------|
| `bytes` | The worker's serialized bytes are compared **byte-for-byte** against the reference worker's. | Canonical/deterministic formats, where the exact wire layout is the contract (a wire protocol whose bytes are hashed, signed, framed, or stored). |
| `semantic` | The reference worker **deserializes the worker's bytes** and the decoded value is compared to the expected case data (order-insensitive for maps). | Non-canonical formats whose serialization is deliberately unordered (e.g. protobuf: map-entry and field order unspecified), where byte comparison would raise false positives. |

Under `semantic`, the two directions together give full conformance: the reference
decodes each worker's output (checks the worker's **serializer**), and each worker
decodes the reference's output (checks its **deserializer** — the deserialize round
below). No byte equality is required.

### Maps: the format decides, not serify

A `map<K,V>` is unordered, so entry order is a property of the **format**, and
the `oracle:` declaration is where the format says which it is. Both are
first-class:

| The format is… | Declare | The worker |
|----------------|---------|-----------|
| canonical over maps — its spec fixes entry order | `oracle: bytes` | sorts, explicitly, and every worker agrees byte-for-byte |
| not canonical — entry order is free | `oracle: semantic` | emits its own map's order and never sorts |

`test/cases/happy/all_types` carries the same data under both: `binary` and
`json` are canonical and sorted; `binary_unordered` is the identical layout with
the sort removed. Flipping the latter to `bytes` fails across the languages
immediately, which is what makes the pair a check on the oracles themselves.

**serify no longer imposes a collation.** It used to: the `bytes` oracle
demanded *ascending by the key's UTF-8 bytes* for every map, everywhere. That
made `map<K,V>` the one type whose serialization required implementing an order
the language's own map does not have. In UTF-16 languages (JavaScript, Java, C#)
the native string comparator is *wrong* for it — it orders by UTF-16 code unit,
placing astral-plane keys (≥ U+10000) before keys in U+E000–U+FFFF, the reverse
of UTF-8 order — so each of those workers grew a hand-written `byteCompare` that
existed for no other reason.

If you do declare `bytes` over a map, sort on the key's **UTF-8 bytes**, not the
language's default string comparator, for exactly that reason. And do not reach
for an ordered container to get it for free: the C++ library held map values in
a `std::map` for years purely because the old rule made that convenient, which
left it satisfying a contract it had never actually implemented. It is a
`std::unordered_map` now, and its canonical formats sort in the open.

## Type system

The canonical type names used in the `type` field of schema entries:

| Category | Names |
|----------|-------|
| Unsigned ints | `uint8`, `uint16`, `uint32`, `uint64`, `uint128` |
| Signed ints | `int8`, `int16`, `int32`, `int64`, `int128` |
| Floats | `float32` (alias: `float`), `float64` (alias: `double`) |
| Other scalars | `bool` (alias: `boolean`), `string`, `bytes` |
| Compound | `optional<T>`, `list<T>`, `array<T,N>`, `map<K,V>`, `struct` |
| Enum | `enum<a,b,c>` — self-describing variant list; the variant name travels as a string, and a worker can derive an ordinal for its byte layout from the declared order |
| Sum | `sum<a, b: T, c: U>` — a tagged union: a value is *exactly one* variant, its tag plus a typed payload |

Note that `sum<...>` is a **wire** spelling. Case files do not write it: a sum
is declared with a `variants:` section on its own type file, whose entries are
the variants. The runner renders that into the type string above, so a worker
library only ever sees this form.

### `enum` vs `sum`

`sum` is the type-theory name for a **sum type** (a.k.a. tagged union or
coproduct); each of its arms is a **variant**, and a variant's discriminant is
its **tag**. The pick between the two constructs is by whether the variants
carry data:

- **`enum<a,b,c>`** — named constants, no payload. The value is *just a name*,
  and it travels as a plain string. Model it as a string (or a native enum that
  maps to its name); it is **not** a sum.
- **`sum<a, b: T>`** — a tagged union whose variants carry typed payloads.

This mirrors the *decision* Protocol Buffers makes with `enum` vs `oneof`, but
serify's `sum` is a genuine sum type, not protobuf's `oneof` (see the
differences below).

Use `sum` whenever a variant has data. Do **not** model a sum type as an
`enum` tag plus separate flat payload fields: every inactive field then has to
be filled with a default on the deserialize round-trip, and nothing stops a case
from setting a payload that does not belong to the declared tag. A `sum`
carries exactly one variant, so both problems disappear.

`sum` is a sum-of-products — a variant's payload is any single type, so arity
needs no special syntax:

| Variant arity | Wire spelling | Case-file entry |
|---------------|---------------|-----------------|
| 0 (unit) | `balanced` | `- balanced:` |
| 1 | `partition_id: uint32` | `- partition_id: uint32` |
| N | `configured: my_struct` | `- configured: my_struct` (payload is a struct holding the N fields) |

Two deliberate differences from protobuf's `oneof`: unit variants are allowed,
where protobuf requires every arm to have a type; and a serify `sum` carries
**exactly one** variant, where a protobuf `oneof` also has a "none set" state. A
case file that names zero variants (or two) is a load error, not a default.
(protobuf's `oneof` is also a field-grouping within a message, not a reusable
named type — a serify `sum` is a first-class type you declare once and `import`.)

### `sum` and your own types

Every library maps a `sum` onto whatever sum type the language already has, so
in seven of the nine there is nothing to declare beyond the type itself:

| Language | Sum type | What you write |
|----------|----------|----------------|
| Rust | `enum` | nothing — `#[derive(SerifyModel)]` covers it |
| Java | `sealed interface` + records | nothing — `permits` names the arms |
| Python | union of dataclasses (`A \| B`) | nothing — the annotation names the arms |
| PHP | property union type (`A\|B`) | nothing — the declaration names the arms |
| C# | `abstract record` + nested sealed records | nothing — the nested types are the arms |
| Elixir | tagged tuple (`{:sms, v}`) | nothing — the tag is in the data |
| C++ | `std::variant` | `SERIFY_SUM(T, "a", "b")` — C++ has no reflection, so only the *names* are missing |
| Node/TS | (erased at runtime) | `@Serify.sum([A, B])` — the union type does not survive compilation |
| Go | interface | a `serify.Converter` — Go cannot enumerate an interface's implementations |

An arm's tag is its type name in snake_case (`Sms` → `sms`), and its payload
follows the arity rule above: no fields is a unit variant, one field is that
value, N fields is a struct. The **tag ordinal is not on the wire** — only the
name is — so each worker is free to choose its own byte encoding for the tag.

All libraries use these canonical names internally for type negotiation. Aliases
(`float`→`float32`, `double`→`float64`, `boolean`→`bool`) are normalized at
case-file parse time.

## Rebinding

The runner may re-send `bind` at any time to switch the worker to a different
type or format, without restarting the process. After rebinding, the worker's
serialize/deserialize handlers must operate on the new type/format.

A worker that does not implement the requested (type, format) answers
`"SKIPPED"` and clears its bindings, optionally with a `reason`. That is a
declaration, not a failure: the runner records the skip for that group and keeps
the worker for the rest of the run. Startup never sees this, because it uses ping
rather than a throwaway bind.

This is the only place a worker says anything about what it supports, and it says
it about exactly the (type, format) in hand. There is no standing capability list
to keep in sync.

## Response matching

Every response **MUST** echo the `op` it is answering, and — for `serialize` and
`deserialize` — the request's `id`. This holds for `ERROR` and `SKIPPED`
responses too, not just successful ones. The runner validates both and kills the
worker on either mismatch: a desynchronized stream is unrecoverable.

Both fields are load-bearing, for different reasons:

- `id` is what distinguishes one case from another.
- `op` is what distinguishes the two *directions* of the same case. A case's
  serialize and deserialize rounds share one `id`, so `id` alone cannot tell a
  serialize response from a deserialize response. Without the `op` check, a
  stale serialize response consumed as the answer to a deserialize request would
  match on `id`, arrive with `hex` where `data` was expected, and be reported as
  a data mismatch — a transport fault disguised as a conformance failure.

`ping` and `bind` carry no `id` at all, so for them `op` is the only correlation
there is.
