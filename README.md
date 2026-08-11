# serify

Cross-language binary serialization test framework. Define a schema once, verify that every language's serialization implementation produces identical bytes.

serify is a conformance harness, not a tool for writing a good serializer. It
assumes you already have one language whose tests you trust, and makes that the
reference every other language is held to. The two things it exists to spare you
are the alternatives: a golden file full of JSON or hex that no one can read, and
the same test suite reimplemented in nine languages that you then have to keep in
agreement by hand.


## Contents

- [How it works](#how-it-works)
- [Install](#install-serify-cli)
- [Quick start](#quick-start)
- [Supported languages](#supported-languages)
  - [Go: writing a worker](#go)
- [Case definition syntax](#case-definition-syntax)
  - [Directory layout](#directory-layout)
  - [Defining a type](#defining-a-type)
  - [Field types](#field-types)
  - [Writing test data](#writing-test-data)
  - [Testing formats](#testing-formats)
  - [Reusing types with `import`](#reusing-types-with-import)
  - [Sum types: `variants:`](#sum-types-variants)
- [Audit mode](#audit-mode)
- [Model binding](#model-binding)
- [`serify validate`](#serify-validate)
- [Protocol](#protocol)
- [Local CI](#local-ci)

## How it works

1. You write `worker` for your target languages — a small program that reads an NDJSON protocol on stdin and answers serialize/deserialize requests.
2. `serify` CLI drives all workers through the same test cases and compares their outputs byte-for-byte.

```mermaid
flowchart LR
    Y["cases/customer.yaml<br/>schema + test data"] --> R["serify runner"]
    R -->|"1. serialize"| W["workers<br/>go · rust · python · node · csharp<br/>cpp · elixir · java · php"]
    W -->|"2. bytes"| R
    R -->|"3. deserialize the reference's bytes"| W
    W -->|"4. decoded data"| R
    R --> V["report<br/>per-language verdict"]
```

[`docs/basic-example.md`](docs/basic-example.md) draws the same flow with a real
case file in it.

## Install Serify CLI

```bash
go install github.com/chengxilo/serify/cmd/serify@latest
```

## Quick start

**Work inside a clone for now.** The worker libraries are not published to any
package registry yet, so every example worker refers to its library by relative
path (`replace => ../../`, `path = "../../lib/rust/serify"`, `file:../../lib/node`,
…). A worker copied outside the repository will not build until those are
published — see [Supported languages](#supported-languages).

```bash
git clone https://github.com/chengxilo/serify && cd serify
go install ./cmd/serify

# Run the bundled suite across two workers. --cases is a directory of per-type
# case files; --ref names the worker the others are compared against, and it
# must be one of the workers you pass.
serify run --ref go --cases examples/cases examples/go examples/rust
```

To start your own worker, copy the example for your language *inside* `examples/`
so its relative path to `lib/<lang>` still resolves, then run it against a
reference worker in a **different** language:

```bash
cp -r examples/python examples/my-worker
serify run --ref go --cases examples/cases examples/go examples/my-worker
```

> Results are reported per language, so only one worker per language can run at a
> time; passing two of the same language is rejected. Compare across languages,
> which is what the harness is for.

## Supported languages

**No library is published to a package registry yet.** The manifests exist and
name the intended coordinates, but nothing is released, so a worker takes its
library by relative path from inside this repository. Go is the exception: Go
modules resolve straight from the repo, so `go get` works today.

| Language | Example worker    | Library                                     | How a worker takes it today |
|----------|-------------------|---------------------------------------------|------------------------------|
| Go       | `examples/go`     | `github.com/chengxilo/serify/lib/go/serify` | `go get` works; the example also uses a `replace` |
| Rust     | `examples/rust`   | `serify` crate (unpublished)                | `path = "../../lib/rust/serify"` |
| Python   | `examples/python` | `serify.py` (unpublished)                   | `sys.path` entry for `lib/python` |
| Node/TS  | `examples/node`   | `@chengxilo/serify` (unpublished)           | `file:../../lib/node` |
| C#       | `examples/csharp` | `Serify.cs` (unpublished)                   | compiled into the worker project |
| C++      | `examples/cpp`    | `serify.hpp` (header-only, unpublished)     | `-I lib/cpp` |
| Elixir   | `examples/elixir` | `serify.ex` (unpublished)                   | path dependency on `lib/elixir` |
| Java     | `examples/java`   | `io.serify:workerlib` (unpublished)         | local Maven module |
| PHP      | `examples/php`    | `chengxilo/serify` (unpublished)            | `require_once lib/php/src/*.php` |

### Go

Define your struct with `serify:` tags for field mapping, then provide named serializer/deserializer functions in a `Suite`. Field mapping (schema ↔ struct) is handled automatically via reflection; byte encoding is entirely under your control.

- Field names map to schema keys via snake_case conversion (`Username` → `username`)
- Use `serify:"key"` tag to override (e.g. ```UserID uint64 `serify:"user_id"` ```)
- Nested structs are mapped recursively

```go
import (
    "encoding/binary"
    "math"

    "github.com/chengxilo/serify/lib/go/serify"
)

type UserRecord struct {
    UserID   uint64 `serify:"user_id"`
    Username string
    Score    float32
}

// You own the byte layout — encode however your format requires.
func serialize(u *UserRecord) ([]byte, error) {
    var b []byte
    b = binary.LittleEndian.AppendUint64(b, u.UserID)
    b = binary.LittleEndian.AppendUint32(b, uint32(len(u.Username)))
    b = append(b, u.Username...)
    b = binary.LittleEndian.AppendUint32(b, math.Float32bits(u.Score))
    return b, nil
}

func deserialize(u *UserRecord, data []byte) error {
    u.UserID = binary.LittleEndian.Uint64(data[:8])
    n := binary.LittleEndian.Uint32(data[8:12])
    u.Username = string(data[12 : 12+n])
    u.Score = math.Float32frombits(binary.LittleEndian.Uint32(data[12+n : 16+n]))
    return nil
}

func main() {
    serify.Run(serify.Suite{
        Types: map[string]serify.Type{
            "user": {
                Model: &UserRecord{},
                Formats: map[string]serify.Format{
                    "binary": {Serializer: serialize, Deserializer: deserialize},
                },
            },
        },
    })
}
```

A serializer is any `func(*T) ([]byte, error)` (and a deserializer any
`func(*T, []byte) error` or `func([]byte) (*T, error)`), so if your struct
already implements `encoding.BinaryMarshaler` / `BinaryUnmarshaler` you can pass
the method expressions directly:

```go
Formats: map[string]serify.Format{
    "binary": {
        Serializer:   (*UserRecord).MarshalBinary,
        Deserializer: (*UserRecord).UnmarshalBinary,
    },
},
```

Multiple types and multiple formats are supported:

```go
serify.Run(serify.Suite{
    Types: map[string]serify.Type{
        "user": {Model: &User{}, Formats: map[string]serify.Format{
            "binary": {Serializer: ..., Deserializer: ...},
            "json":   {Serializer: ..., Deserializer: ...},
        }},
        "order": {Model: &Order{}, Formats: map[string]serify.Format{
            "binary": {Serializer: ..., Deserializer: ...},
        }},
    },
})
```

See [`examples/go/`](examples/go/) for more detail.

## Case definition syntax

Test cases live in a **directory** of YAML files. You define each data type once
(its schema), attach test cases to it, and reuse types across files with
`import`. The schema describes the *logical* shape of your data — the actual byte
layout is decided by each worker's serializer, which is exactly what serify tests.

### Directory layout

```text
cases/
  user.yaml         # type "user"
  order.yaml        # type "order"
  address.yaml      # type "address"
```

- Each non `_`-prefixed `*.yaml` defines **exactly one type**, named after the
  file: `user.yaml` → type `user`. (There is no `type:` field — the filename is
  the name, so they can never disagree. Files starting with `_` are ignored.)
- `cases` decides only whether a type is **tested** (run in the suite). Any type
  can be **imported** and reused regardless — `import` takes a file's schema and
  ignores its cases. A type with no `cases` is simply reusable-only.
- The reference language is declared by the suite, in `_config.yaml`
  (`reference_language: go`). `--ref` overrides it, and is required only for a
  suite that declares none.

### Defining a type

A type file has a `formats:` list, an optional `import:` list, a `fields:`
section (or `variants:` for a sum, see below), and a `cases:` list.

```yaml
# cases/user.yaml
formats:                  # serialization formats to test this type with
  - name: binary          # every format must name its comparison oracle:
    oracle: bytes         #   bytes    — compared byte-for-byte
  - name: json
    oracle: semantic      #   semantic — compared by decoded value
import:
  - address.yaml          # makes the `address` type available below
fields:                   # one `name: type` per line, order is preserved
  - user_id: uint64
  - username: string
  - score: float32
  - tags: list<string>
  - address: address      # a named type -> nested struct

cases:
  - name: basic
    data:
      user_id: 42
      username: "Alice"
      score: 3.14
      tags: ["admin", "user"]
      address: { street: "Main St", zip: 10001 }
```

### Field types

Each schema entry is `name: <type>`, where `<type>` is one of:

| Category   |                                         Types                               |
|------------|-----------------------------------------------------------------------------|
| Unsigned   | `uint8` `uint16` `uint32` `uint64` `uint128`                                |
| Signed     | `int8` `int16` `int32` `int64` `int128`                                     |
| Float      | `float32` `float64` (aliases `float` `double`)                              |
| Other      | `bool` (alias `boolean`) `string` `bytes`                                   |
| List       | `list<T>` — variable length                                                 |
| Array      | `array<T,N>` — fixed length `N`                                             |
| Optional   | `optional<T>` — a value or `null`                                           |
| Map        | `map<K,V>` — `K` is usually `string`                                        |
| Named type | the name of another type file (e.g. `address`) — inlined as a nested struct |

Containers nest freely: `map<string, list<uint32>>`, `optional<address>`,
`array<uint8,16>`, and so on.

### Writing test data

In `data:`, write each field's value in YAML according to its type:

| Type                       | How to write it           |             Example                                 |
|----------------------------|---------------------------|-----------------------------------------------------|
| integers (any width)       | a number (quoted decimal string also accepted) | `amount: 170141183460469231731687303715884105727` |
| `float32` / `float64`      | a number                  | `score: 3.14`                                       |
| `bool`                     | `true` / `false`          | `active: true`                                      |
| `string`                   | a string                  | `username: "Alice"`                                 |
| `bytes`                    | a byte array or hex string | `metadata: [0xde, 0xad]` or `metadata: "dead"` (empty = `[]` or `""`) |
| `list<T>` / `array<T,N>`   | a sequence                | `counts: [1, 2, 3, 4]`                              |
| `optional<T>`              | the value, or `null`      | `profile: null`                                     |
| named type / struct        | a mapping                 | `address: { street: "x", zip: 1 }`                  |
| `map<K,V>`                 | a mapping                 | `scores: { math: 95 }`                              |

Each case has a `name`, and its **global id is `type/format/case`** (e.g.
`user/binary/basic`) — the format is part of it because the same case runs once
per declared format. That id is what appears in the report and is sent to the
workers as the request `id`.

### Testing formats

Every tested type **must declare its `formats:`** explicitly (there is no
implicit default — a type with `cases:` but no `formats:` is rejected at load
time), and **every format must name its `oracle:`** — `bytes` to compare the
serialized bytes, `semantic` to compare the decoded value. That is mandatory too,
and for the same reason: it decides whether a disagreement between two workers is
a failure or is allowed wire freedom, which is not something to leave implicit.

Each worker is re-bound once per format and its cases run again under that
format, so a single run compares every language for each format. A worker that
doesn't implement a format is marked `SKIP` for it (and if the reference worker
lacks it, that whole format is skipped). Reusable-only types (no `cases:`,
imported by others) don't need `formats:`.

Note that byte-for-byte parity across languages only holds for formats with a
fully-specified wire layout — that is what `oracle: bytes` asserts. Text formats
like JSON often differ between implementations (float formatting, whitespace), and
any type holding a `map<K,V>` differs by construction, since a map is unordered
and workers do not sort it. Those want `oracle: semantic`, which compares the
decoded value instead. See `docs/protocol.md` § Comparison oracles.

### Reusing types with `import`

Any type file can be imported by path so your schema can reference it by name —
`import` takes the imported file's `fields:` (or `variants:`) section and ignores
any `cases:` it has. So you can import a tested type just as well; a type with no
`cases:` is just one that exists *only* to be reused:

```yaml
# cases/address.yaml — reusable-only (no cases)
fields:
  - street: string
  - zip: uint32
```

```yaml
# cases/order.yaml
import:
  - address.yaml
fields:
  - order_id: uint64
  - shipping: address     # resolves to address.yaml's schema, inlined
```

`import` is **transitive** (imports of imported files are followed) and
cycle-safe; a single file may import several others.

### Sum types: `variants:`

Under `variants:` each entry is `tag: payload`, a value is *exactly one* of them,
and an entry with no type is a unit variant. The section name is the whole
declaration — there is no separate flag.

```yaml
# cases/money.yaml — reusable-only, the struct payload below refers to it
fields:
  - currency: string
  - amount_minor: int64
```

```yaml
# cases/channel.yaml
import:
  - money.yaml
variants:
  - silent:              # no type -> a unit variant
  - sms: string          # a scalar payload
  - push: uint64
  - invoice: money       # a named type -> a struct payload
```

Referenced from another type, a sum **is** the field's type — there is no
wrapping struct:

```yaml
fields:
  - notification_id: uint32
  - channel: channel     # the variant sits directly at `channel`
  - urgent: bool
```

Tested on its own (a `variants:` file with its own `formats:` and `cases:`), the
sum is carried under the single key `value`, because the top level of a schema is
a list of named fields and a bare sum is not one:

```yaml
cases:
  - name: pushed
    data: { value: { push: 12345 } }
  - name: quiet
    data: { value: silent }   # a unit variant is written bare, as just the tag
```

The tag ordinal is **not** on the wire — only the name is — so each worker
chooses its own byte encoding for the tag, usually the declaration order.

Do **not** model a sum as a `kind` tag plus flat payload fields: every inactive
field then needs a default on the round-trip, and nothing stops a case from
setting a payload that does not belong to the declared tag. Use `enum<a,b,c>`
when the variants carry no data at all, and a `variants:` type as soon as one
does.

## Audit mode

Pass `--audit` to `serify run` to enable unsafe-behaviour detection inside every worker. The runner sends `"audit": true` in the bind message; each worker library then runs additional checks and reports findings as **warnings** (not failures — they appear in the report with status `WARN` and do not cause a non-zero exit).

Six unsafe behaviours are detected:

| Detection | How |
|-----------|-----|
| **Serialize mutation** | Snapshot FieldMap before serialization, compare after |
| **Serialize instability** | Call serializer twice, compare hex output |
| **Output zero-copy** | XOR-flip the returned buffer, re-extract the model, compare |
| **Deserialize zero-copy** | XOR-flip input buffer after deserialization, check which FieldMap entries change |
| **Input-buffer mutation** | Snapshot buffer before deserialization, compare after |
| **Deserialize instability** | Re-deserialize from a fresh clone of the input, diff FieldMaps |

All 9 worker libraries (Go, Rust, Python, Node, C#, C++, Java, Elixir, PHP) implement these. Output zero-copy is the one exception: it only applies where the language's memory model lets a model field mutably alias the output buffer, so C++, Elixir and PHP omit it (their strings/binaries cannot alias).

## Model binding

Each library provides an idiomatic way to map native structs/classes to FieldMap, avoiding manual `set_*`/`get_*` calls:

| Language | Mechanism | Key features |
|----------|-----------|-------------|
| **Go** | Struct tags | `serify:"field_name"` on struct fields; reflection-based `reflectFill`/`reflectExtract` |
| **Rust** | Derive macro | `#[derive(SerifyModel)]` + `#[serify(rename = "key")]`; compile-time codegen |
| **Python** | Decorator | `@serify_model` on `@dataclass`; field names → schema keys; `metadata={"serify": "key"}` for renames |
| **Node/TS** | Decorator | `@Serify.Model()` class + `@Serify.field({rename: "key"})` property decorators |
| **C#** | Attribute | `[SerifyModel]` class + `[SerifyField("key")]` property attributes |
| **C++** | Macro | `SERIFY_TO(Type, ...)` / `SERIFY_FROM(Type, ...)` blocks with `SERIFY_FIELD(name, kind)` |
| **Java** | Annotation | `@SerifyModel` class + `@SerifyField("key")` field annotations |
| **Elixir** | `use` macro | `use WorkerLib.Serify.Model` + `serify_field :name, :type, key: "key"` |
| **PHP** | Attributes | `#[SerifyModel]` class + `#[SerifyField('key')]` property attributes |

### Python — `@serify_model`

```python
from dataclasses import dataclass, field
from serify import serify_model

@serify_model
@dataclass
class User:
    user_id: int
    name: str
    email: str = field(default="", metadata={"serify": "email_addr"})
```

Generates `to_field_map()` and `from_field_map(fm)` class methods.

### `sum` fields

A `sum` binds onto whatever sum type the language already has, so six of the
nine need **nothing declared** — the binding reads the arms off the type itself:
a Rust `enum`, a Java `sealed interface`, a Python union of dataclasses, a PHP
property union type, a C# `abstract record` hierarchy, an Elixir tagged tuple.
The three that cannot be introspected name their arms instead: C++
`SERIFY_SUM(T, "a", "b")` (no reflection), Node `@Serify.sum([A, B])` (union
types are erased at runtime), Go a `serify.Converter` (implementations of an
interface cannot be enumerated).

All nine share one arity rule: **0 fields → unit variant, 1 field → that value is
the payload, N fields → the payload is a struct**; tags are the arm's type name
in snake_case. See `examples/*/notification.*` for a worked example per language.

### Rust — `#[derive(SerifyModel)]`

```rust
#[derive(SerifyModel)]
#[serify(rename_all = "snake_case")]
struct User {
    user_id: u64,
    name: String,
    #[serify(rename = "email_addr")]
    email: String,
}
```

Generates `to_field_map(&self)` and `from_field_map(fm: &FieldMap) -> Result<Self>`.

## `serify validate`

```bash
serify validate                        # validate ./cases
serify validate --cases examples/cases # validate a named directory
serify validate --cases examples/cases examples/go   # also detect a worker
```

Checks the case files for structural validity (schema consistency, cross-references, format declarations) without running workers. Useful as a pre-commit check.

Positional arguments are **worker directories**, exactly as in `serify run`; the case directory is named with `--cases`. Passing a case directory positionally is rejected with a message pointing at the flag.

## Protocol

The NDJSON wire protocol is documented in [`docs/protocol.md`](docs/protocol.md).

## Local CI

```bash
# Run the full CI pipeline locally (requires Docker)
act

# Run a single job
act -j test-go
act -j conformance --matrix language:rust
```

## Acknowledgments

- Got this idea from [Apache Iggy](https://github.com/apache/iggy)
- Implementation inspired by [Protobuf Conformance Tests](https://github.com/protocolbuffers/protobuf/blob/main/conformance/README.md)
- And thanks to whoever read this far!
