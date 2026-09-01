# taskstore — a small CRUD server, checked across languages

A task list with five operations, served over TCP. Go writes the server; every
other language writes a client. The two sides never share a line of code, so the
only thing holding them together is agreement about bytes — and that agreement
is what serify checks.

This is deliberately not [`examples/appdata`](../appdata). That suite is a type zoo
built to exercise every corner of the schema language. This one is shaped like a
project: one plausible feature, the smallest schema that expresses it, and a
program you can actually run.

## The problem

The server has an in-memory list of tasks and answers five requests: create,
read, update, delete, and list. It speaks a compact binary framing rather than
JSON, because the format is fixed and both ends already know the shape.

That decision is what creates the problem serify exists for. With JSON a client
in another language mostly works by accident. With a hand-packed layout, every
field width, every length prefix and every enum ordinal is a chance for the two
sides to disagree — and the failure mode is not an exception, it is a client
that reads a plausible wrong number and carries on.

## Layout

```text
taskstore/
  cases/            the schema and the test cases: the contract, written once
  go/               the leader: codec, server, client, and the serify worker
    api/            the wire format — imports the standard library and nothing else
    store/          the in-memory task store
    cmd/server/     the TCP server
    cmd/client/     the Go client
    main.go         the serify worker
  python/ rust/ node/ cpp/ csharp/ java/ php/ elixir/
                    the followers: the same codec, a client, and a worker each
  test/             conformance + an end-to-end run against the real server
```

Every follower is the same three pieces, and none of them is large:

| Piece | What it is |
|-------|------------|
| the codec | the records and their byte layout, read off [`go/api/wire.go`](go/api/wire.go), which documents the conventions the leader established |
| the worker | twenty lines naming `request` and `response` and handing serify the codec's own functions |
| the client | sockets and argument parsing, and no knowledge of the format beyond calling the codec |

Each language directory is a worker directory in serify's sense — it detects the
language from a marker file (`go.mod`, `Cargo.toml`, `*.py`, `composer.json`, …)
and builds each one before running it.

## The contract

Two types cross the socket, and both are records wrapping a sum:

```yaml
# cases/request.yaml
fields:
  - request_id: uint32
  - op: op            # cases/op.yaml
```

```yaml
# cases/op.yaml
variants:
  - list_all:         # a unit variant, no payload
  - create: draft
  - read: uint64
  - update: task
  - delete: uint64
```

A sum is the honest shape here. A delete carries an id and nothing else; a
create carries a draft and has no id at all. Written flat, with a `kind` field
beside every payload field any operation might need, each request would have to
supply defaults for the fields it does not use, and nothing would stop it
naming a `kind` whose payload was not filled in. As a sum, a request that
means two things at once cannot be constructed.

The response is the same shape over `cases/result.yaml` — `accepted`, `found`,
`listing`, or `failed`. Note that a failure is one of the four: an error is a
value the client decodes, not an exception beside the protocol.

`cases/_config.yaml` names Go the leader:

```yaml
reference_language: go
```

That is where a reference belongs, because it is a property of the cases rather
than of whoever types the command. No `--ref` flag appears anywhere below.

## Run the server

```console
$ cd examples/taskstore/go
$ go run ./cmd/server &
2026/08/22 20:41:14 taskstore listening on 127.0.0.1:9977

$ go run ./cmd/client create "Buy milk" normal errand,home 1755820800
[ ] 1001  Buy milk                        normal  #errand #home  due=1755820800
```

Now answer with another language. Same server, same store, a different process
in a different runtime:

```console
$ cd ../python
$ python3 client.py list
[ ] 1001  Buy milk                        normal  #errand #home  due=1755820800
(1 shown, 1 total)

$ python3 client.py create "牛乳を買う ☕" high errand,home 1755820800
[ ] 1002  牛乳を買う ☕                         high  #errand #home  due=1755820800

$ python3 client.py read 4242
error: not_found: no task with id 4242
```

Nothing in the Python client knows the server is written in Go, and nothing in
the server knows its client is not. Nor does any of the other seven:

```console
$ cd ..
$ (cd rust   && target/release/client list)
$ (cd node   && node dist/client.js list)
$ (cd cpp    && ./client list)
$ (cd csharp && dotnet run -c Release --project Client/Client.csproj -- list)
$ (cd java   && java -cp target/taskstore-0.1.0.jar Client list)
$ (cd php    && php client.php list)
$ (cd elixir && mix run -e 'Client.main(System.argv())' -- list)
```

Every one takes the same arguments and prints the same lines. Two of them need a
build first, because their worker build only compiles its own entry point —
`cpp` wants `g++ -O2 -std=c++17 -I../../../lib/cpp -o client client.cpp` and
`csharp` wants `dotnet build -c Release Client/Client.csproj`. The rest come out
of the worker build already.

## Check the followers

The clients agreeing on the cases you happened to try is not the same as the
clients agreeing. That is what the suite is for:

```console
$ cd ../../..
$ serify run --cases examples/taskstore/cases examples/taskstore/*/
```

```text
╭────────────────────────┬────────┬─────────────┬──────┬──────┬────────┬────────┬──────┬──────┬──────┬────────┬──────╮
│ case id                │ format │ operation   │  go  │ cpp  │ csharp │ elixir │ java │ node │ php  │ python │ rust │
├────────────────────────┼────────┼─────────────┼──────┼──────┼────────┼────────┼──────┼──────┼──────┼────────┼──────┤
│ request/list_all       │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ request/create         │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ request/read           │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ request/update         │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ request/delete_max_id  │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ response/accepted      │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ response/found         │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ response/listing       │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ response/empty_listing │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ response/failed        │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│ response/boundary      │ binary │ serialize   │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
│                        │        │ deserialize │ PASS │ PASS │  PASS  │  PASS  │ PASS │ PASS │ PASS │  PASS  │ PASS │
╰────────────────────────┴────────┴─────────────┴──────┴──────┴────────┴────────┴──────┴──────┴──────┴────────┴──────╯
FAIL: 0  XFAIL: 0  XPASS: 0  SKIPPED: 0  WARN: 0  PASSED: 198  (exit code 0)
```

Every type here declares `oracle: bytes`, so those PASSes mean the nine workers
produced *identical bytes*, not merely equivalent values. Nothing in this schema
is a map, so there is no wire freedom to grant — and for a format nine programs
have to hold a conversation over, byte-for-byte is the right bar.

You can run a subset just as well; two workers is the minimum serify can compare,
and the leader has to be one of them:

```bash
serify run --cases examples/taskstore/cases examples/taskstore/go examples/taskstore/rust
```

## What a disagreement looks like

A green table proves very little to someone who has never seen a red one. So
here is a real bug: the Python side writes its string length prefix
big-endian — one character different, and the single most common mistake when
porting a binary format.

```python
-    return struct.pack("<I", len(b)) + b
+    return struct.pack(">I", len(b)) + b
```

Run against the leader alone, to keep the table narrow:

```text
│ request/create         │ binary │ serialize   │ PASS │  FAIL  │
│                        │        │ deserialize │ PASS │  PASS  │
```

```text
[request/binary/create / python / serialize]
  length: expected 49, got 49
first divergence at offset 5 (0x5)

--- expected
00000000  02 00 00 00 01 [08] 00 00 00 42 75 79 20 6d 69 6c  |.........Buy mil|
+++ got
00000000  02 00 00 00 01 [00] 00 00 08 42 75 79 20 6d 69 6c  |.........Buy mil|
```

Three things in that output are worth noticing.

**The lengths match.** Both sides wrote 49 bytes. Every size check, every
"did it round-trip" test and every schema validator passes this bug. Only
comparing the bytes themselves finds it.

**Deserialize still passes.** The bug is one-directional: Python writes the
prefix wrongly but still *reads* it correctly, so it decodes the Go server's
bytes perfectly. A client tested only against its own output — which is what a
round-trip test is — is green. It would fail the moment it sent anything.

**Six cases fail and five pass.** The five survivors carry no strings at all:
`list_all`, `read`, `delete_max_id` and `accepted` have nothing but integers and
tags, and `empty_listing` is an empty page. Had the cases been only the obvious
ones — create a task, read it back — this bug would have shipped.

## Where serify appears, and where it does not

The server binary links no part of serify, and neither does the Go client:

```console
$ go list -deps ./cmd/server | grep lib/go/serify
$ go list -deps . | grep lib/go/serify
github.com/chengxilo/serify/lib/go/serify
```

The first comes back empty and the second does not, and that is the whole
arrangement: `api/` is standard library only, and the `serify:"…"` struct tags
that bind it to the schema are inert strings that cost nothing at run time.

Go is one of exactly two languages here that can manage that. Every binding has
to say *somewhere* how a type maps to the schema, and only two of the nine can
say it without the codec taking a dependency:

| Language | How the codec is bound | Does the codec depend on serify? |
|----------|------------------------|----------------------------------|
| Go | `serify:"…"` struct tags | **No** — a struct tag is an inert string |
| Python | `@serify_model` | **No** — `worker.py` applies it late, so `api.py` imports only `struct` and `dataclasses` |
| Rust | `#[derive(SerifyModel)]` | Yes, but `default-features = false` keeps the derive and drops the runtime |
| Node | `@Serify.Model()` / `@Serify.field()` | Yes — decorators are values, evaluated at class definition |
| C++ | `SERIFY_TO` / `SERIFY_FROM` macros | Yes — macros have to be expanded, so `serify.hpp` is on the include path |
| C# | `[SerifyModel]` / `[SerifyField]` | Yes — an attribute is a compile-time type reference |
| Java | `@SerifyModel` / `@SerifyField` | Yes, same reason |
| PHP | `#[SerifyModel]` / `#[SerifyField]` | Yes, same reason |
| Elixir | `use WorkerLib.Serify.Model` | Yes — the macro generates the conversion functions at compile time |

That is a real difference and the example does not smooth it over. It is also
not a ranking: a compile-time binding is checked by the compiler, and Go's inert
tags are checked by nothing until the run.

Whatever the binding costs, serify's *runtime* footprint is one file per
language — `go/main.go`, `python/worker.py`, `rust/src/bin/worker.rs` and their
six siblings — and each is short because it hands over the **same** functions
the server and clients call. `api.Request.MarshalBinary` is not a test double; it
is the function `cmd/server` runs on every reply. That is what makes the
conformance run mean something: the bytes the followers are checked against are
the bytes the running server produces, not a reference encoder written beside it
that could quietly drift.

### The sum, which is where the languages really differ

Both messages here are a record wrapping a sum, and a sum is the one construct
whose binding cost varies wildly:

| Language | What it maps onto | What has to be declared |
|----------|-------------------|-------------------------|
| Elixir | a tagged tuple | **nothing at all** — `{:create, payload}` already is a tag and a payload |
| Rust | `enum` | nothing — the derive reads the variants |
| Python | a union of dataclasses | nothing — the union names the arms |
| Java | a sealed interface | nothing — `permits` names them |
| C# | an abstract record with nested sealed records | nothing — the nested types are the arms |
| PHP | a property union type | nothing — the union names the arms |
| C++ | `std::variant` | the tag *names*, via `SERIFY_SUM` — C++ has no reflection |
| Node | plain classes | the arm *list*, via `@Serify.sum([...])` — TypeScript erases unions |
| Go | a sealed interface | **the whole conversion**, by hand — an interface's implementations cannot be enumerated at run time |

Go pays the most and Elixir the least, and both are visible in the files:
[`go/main.go`](go/main.go) spells out all nine arms twice, while
[`elixir/lib/message.ex`](elixir/lib/message.ex) declares `serify_field(:op, :sum)`
and stops. Elixir's bill arrives elsewhere, though — declaring nothing means the
binding does not know which module a struct payload belongs to, so it arrives as
a plain field map and the codec converts at the boundary.

Two other things worth knowing before you write the tenth follower, both of
which cost an hour here:

- **`Task` is a taken name in two languages.** Elixir has a built-in `Task`
  module and shadowing it breaks `mix` itself; C# has `System.Threading.Tasks.Task`
  under `ImplicitUsings`. Both are `TaskItem` here. The class name never reaches
  the wire, so renaming costs nothing.
- **A uint64 does not fit every language's integer.** PHP's int is signed
  64-bit, so every 64-bit value there is a decimal string converted through
  ext-gmp; Java holds the bit pattern in a `long` and prints it with
  `Long.toUnsignedString`. `response/boundary` carries `id: 18446744073709551615`
  precisely so that a language that quietly truncates it fails.

## What the tests cover

```bash
go test ./examples/taskstore/test/
```

Two tests, and the second is the one that is easy to forget.
`TestTaskstore_Conformance` runs the suite and requires every cell to pass.
`TestTaskstore_Clients` starts the real server on an ephemeral port and drives it
with **every** client — each one creates a task with a non-ASCII title, lists the
tasks the other eight created, and asks for an id that is not there; then the Go
client lists once more and has to see all nine.

That second test exists because a conformance run compares codecs and never
opens a socket. If the server were rewired to some second, private encoder,
every cell in the table above would still be green.

Both skip the languages whose toolchain is absent, unless `SERIFY_REQUIRE` names
them — CI sets it to all nine, so a toolchain that failed to install fails the
job instead of quietly shrinking it.

## Adding a follower

All nine of serify's languages are here, so there is nothing left to add — but
the shape is the same if you are copying this into a project of your own. A
follower needs three files:

1. **the codec** — the records and their byte layout, read off
   [`go/api/wire.go`](go/api/wire.go), which documents the conventions the
   leader established.
2. **the worker** — twenty lines naming `request` and `response` and handing
   serify the codec's own functions.
3. **the client** — sockets and argument parsing, and no knowledge of the format
   beyond calling the codec.

Then add it to the run:

```bash
serify run --cases examples/taskstore/cases \
  examples/taskstore/go examples/taskstore/<lang>
```

Start with the worker and get the suite green before writing a line of the
client. Every follower here was built that way, and every mistake made along the
way was a conformance failure with a byte offset on it rather than a socket that
hung. A worker that registers neither type reports SKIP rather than passing
quietly, so a partial follower is always honest about how much of the contract
it actually keeps.
