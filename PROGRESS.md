# Progress Summary — July 2026

## What was done

### 0. Repo-wide improvement pass (2026-07-14)

**Correctness fixes:**
- **PHP** — `decodeOptional` wrote into `FieldMap::raw()` (returned *by value*), silently dropping every `optional<scalar>` value; added `FieldMap::setRaw()`. Also removed a dead big-endian `float32` decode, a copy-paste `$out[$name]` in the enum encode branch, and extended `decodeList`/`encodeList` to the full canonical element set (uint8/int32/int64/int128/bool added).
- **Python** — `@serify_model` was broken end-to-end: `typing` was never imported (forward refs always fell back to raw annotations), `_set_fm`/`_get_fm` called nonexistent accessors (`set_int64` vs `set_i64`), and `_type_info` crashed on `type(None).__class_getitem__`. All fixed; covered by new `lib/python/test_serify.py`.
- **Go runner** — `Reinit`/`Send` now take a context (Ctrl-C interrupts in-flight waits instead of burning the full `--timeout`); a timed-out/cancelled `Reinit` now kills the worker like `Send` does (previously it leaked a reader goroutine that could race the next request on the shared scanner); `runMatrix` uses its errgroup context; the Go worker library answers unknown ops with an ERROR response instead of silence.
- **Config: schema-directed case decoding (2026-07-15, supersedes the earlier float-guard)** — case `data` is no longer unmarshalled into `map[string]any` (which let yaml.v3 guess types and degrade big integers to float64). `internal/config/casedata.go` keeps the raw `yaml.Node`s and decodes each field per its schema type: 64/128-bit integers parse exactly from their literal text via `big.Int` (bare or quoted, any size) with explicit range checks; float forms (`1e3`) and out-of-range values are load-time errors; all other kinds decode as before. The interim `validateBigInt` guard and the "128-bit must be quoted" rule are gone; JSON Schema defs now accept integer *or* quoted-string forms for all 64/128-bit fields. Verified: bare max-int128 literal in `ledger.yaml` round-trips byte-identically across all workers. Rationale: yaml.v3 silently wraps 2^64 → 9223372036854775808 when decoding into uint64 via its float path, so integer fields must never rely on the library's type guessing.
- **Node** — `decodeList` extended to the canonical element set (throws on unknown instead of passing through); model helper accepts `bigint`, converts safe-integer numbers via `BigInt`, and **throws** on unsafe integers instead of truncating.
- **C#** — `DecodeList` gained uint8/int32/int64/bool + error on unknown; missing `break` after the `array<` branch; `"enum"` added to `SUPPORTED_TYPES` (also Java) so enum cases aren't skipped.
- **Elixir** — deleted dead `list<u32>`/`list<u64>`/`list<f32>` clauses (wire types are the long forms).

**Packaging:** Python `build-backend` fixed to `setuptools.build_meta`; PHP composer requires `>=8.1` (`array_is_list`); Java poms unified on `<release>17</release>` with `--enable-preview` dropped (the one preview-feature use was rewritten to plain `instanceof` chains).

**Tests/CI:** new unit tests for `internal/orchestrate` `RunSuite` (stub shell workers: happy path, hung worker, cancelled ctx), `Reinit`-timeout + ctx-cancel worker tests, config big-int guard tests, `lib/python/test_serify.py` (pytest), `lib/node/test/workerlib.test.js` (`npm test`); CI now runs the Python and Node unit tests. `ledger` gained `unicode_strings` and `escape_strings` cases (values only — all 9 workers still pass).

**Docs:** AGENTS.md purged of stale CUE/`serify init`/`templates/` content (cases are YAML; scaffolding was removed); README quick start now says to copy an example worker; `enum` documented in `docs/protocol.md`; hex-string form of `bytes` documented in README.

**Deferred:** promoting `enum` into the shipped suite needs a schema change to a type all 9 example workers implement (byte-layout updates in every worker) — tracked as follow-up.

### 1. Core features (previous sessions)
- **Rust library audit** — all 4 detections in `lib/rust/serify/src/lib.rs`
- **Rust audit test worker** — `test/cases/audit/rust/` with 4 formats
- **Audit in all libraries** — all 9 languages
- **`map<K,V>` FieldMap** — all 9 languages

### 2. Model-binding mechanisms

| Language | Mechanism | File |
|----------|-----------|------|
| **Go** | `serify:"key"` struct tags | `reflect.go` |
| **Rust** | `#[derive(SerifyModel)]` proc macro | `lib/rust/derive/src/lib.rs` |
| **Python** | `@serify_model` decorator on `@dataclass` | `lib/python/serify.py` |
| **Node/TS** | `@Serify.Model()` + `@Serify.field()` decorators | `lib/node/src/workerlib.ts` |
| **C#** | `[SerifyModel]` + `[SerifyField]` attributes | `lib/csharp/Serify.cs` |
| **Java** | `@SerifyModel` + `@SerifyField` annotations | `lib/java/.../WorkerLib.java` |
| **C++** | `SERIFY_TO` / `SERIFY_FROM` macros | `lib/cpp/serify.hpp` |
| **Elixir** | `use WorkerLib.Serify.Model` macro | `lib/elixir/lib/serify.ex` |
| **PHP** | `#[SerifyModel]` + `#[SerifyField]` attributes (PHP 8.0+) | `lib/php/src/` |

### 3. PHP library (new — this session)
Brand new library in `lib/php/`:
- **`FieldMap.php`** — typed getters/setters for all serify types
- **`Worker.php`** — NDJSON protocol loop with all 4 audit detections (mutation, stability, zero-copy, input-mutation)
- **`Attributes/SerifyModel.php`** — `#[SerifyModel]` class attribute
- **`Attributes/SerifyField.php`** — `#[SerifyField('key')]` property attribute
- **`SerifyModelHelper.php`** — reflection-based `toFieldMap()` / `fromFieldMap()`
- **`composer.json`** — PSR-4 autoloading
- Builder detection via `composer.json` + `*.php` markers
- Default build: `composer install --no-dev`, run: `php worker.php`

### 4. CLAUDE.md
- Updated all language tables to include PHP
- Added PHP examples to model-binding section

## Current feature matrix

| Feature | Go | Rust | Python | Node | C# | Java | C++ | Elixir | PHP |
|---------|:--:|:----:|:------:|:----:|:--:|:----:|:---:|:------:|:---:|
| Audit (4 detections) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `map<K,V>` FieldMap | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Model-binding | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Model-binding *exercised by a worker* | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ledger` byte-parity vs Go, incl. ±2^127 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `sum<...>` sum types | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `sum` bound onto a *model* (no hand FieldMap access) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `notification` byte-parity vs Go (sum) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Encoder-free (user owns serialization) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Audit test worker (`test/cases/audit/`) | ✅ | ✅ | — | — | — | — | — | — | — |

Coverage per type, as of the container/binding work below:

| Type | Implemented by | Why the others skip it |
|------|----------------|------------------------|
| `ledger`, `notification`, `signals` | all nine | — |
| `customer` | Go, Rust | JSON byte-parity needs a hand-written encoder per language |
| `telemetry` | Go, Rust | needs `optional<scalar>`, which only these two bindings can express |
| `order` | Go | needs `enum` + `list<struct>` + `map<string,struct>` + `optional<struct>` together |

A type implemented by *no* worker is now reported under the results grid — every
result for it is SKIP, which otherwise reads exactly like a healthy run.

`notification` is the `sum` type. Its four cases cover the three payload
arities the type system allows — a unit variant, a scalar payload, a u64 payload
that must survive as a decimal string, and a struct payload — and each language
reaches them through its own sum type: an `enum` (Rust), a sealed interface
(Java), a union of dataclasses (Python), a property union (PHP), an `abstract
record` hierarchy (C#), a tagged tuple (Elixir), a `std::variant` (C++), a
declared arm list (Node), or a `Converter` (Go). See "`sum` model binding in
every language" below. `TestExamples_Notification` asserts all nine agree
byte-for-byte.

## `--audit` and `sum` payloads (fixed July 2026)

Every library's audit walkers enumerate FieldMap value kinds explicitly, and
none of them had a case for that language's Variant type when `sum` landed. A
sum payload is aliasing-capable (bytes, string, a nested FieldMap), so the
audit silently skipped it. Surveyed per language, the picture was not uniform:

| Language | Zero-copy collector | Mutation snapshot |
|----------|--------------------|-------------------|
| Go | **was broken** — fixed | **was broken** (shallow `cloneValue`) — fixed |
| Node, Python, C#, Java, C++ | **was broken** — fixed | fine (diffs the encoded JSON, not a FieldMap clone) |
| Rust | fine (`FieldMap` derives `Clone` + `PartialEq`, so a boxed payload is deep-copied and deep-compared) | fine |
| PHP, Elixir | not applicable — strings/binaries are immutable, so a decoded value can never alias the input buffer; both detectors are already documented as vestigial | fine |

Go was the only language where the *mutation* path was also affected, because it
is the only one that snapshots a FieldMap rather than the encoded JSON.

Regression tests, each confirmed to fail before its fix:

- Go — `TestSum_SnapshotDeepCopiesPayload`, `TestSum_DetectZeroCopyOnPayload`
  (`lib/go/serify/sum_test.go`)
- Rust — `sum_detect_zero_copy_on_payload` (pins behaviour that was already
  correct, so a future refactor cannot regress it silently)
- Python — `test_sum_detect_zero_copy_on_payload` plus an
  `..._ignores_owned_payload` counterpart
- Node — `detectZeroCopy sees an aliased sum payload` plus an owned-payload
  counterpart

C#, Java and C++ have no unit-test harness in this repo (they never have), so
their fixes were verified with standalone drivers — aliased payload reported,
owned payload silent, plain-bytes path unchanged, variant intact after restore —
and each was canaried by deleting the fix and re-running. Giving those three a
committed test would mean either standing up three new test projects or
extending `test/cases/audit/` (whose meta-test drives only Go and Rust today, and
whose C#/Java/C++ workers implement 5 of the 9 formats). Neither is done.

## FieldMap surface: the `Must*` family removed (July 2026)

Go's `FieldMap` exported 84 methods. Measuring callers across serify and the
iggy worker showed only **7** were reachable from outside the library
(`GetI64`, `GetString`, `GetVariant`, `SetBytes`, `SetI64`, `SetString`,
`SetVariant`) — all of them from converters, which is the one place a worker
still legitimately touches a FieldMap.

All 18 `Must*` methods were called **only by their own test**. Each was a
one-line wrapper that discarded the error:

```go
func (f *FieldMap) MustU8(k string) uint8 { v, _ := f.GetU8(k); return v }
```

Despite the name none of them panics — `MustU8` on a missing field silently
returns 0. That is precisely the false-green this framework exists to catch, in
the library's own API. Deleted, with their test. Surface: 84 -> 66.

The typed `Get*`/`Set*` accessors stay. They are what a converter needs, and
deleting one half of a pair would leave a hole a converter could fall into.

### Known gap found while measuring — fixed, see below

`list<float64>`, `list<uint16>`, `list<int8>` and `list<int16>` were *declarable*
in a case file — `serify validate` accepted them — but Go's `decodeList` had no
case for them and failed with `unsupported list element type`. `FieldMap` even
had `GetListF64`/`SetListF64` and `GetListU16`/`SetListU16` accessors for values
the protocol could never carry. The accessors were deliberately **not** deleted:
that would have hidden the inconsistency rather than fixed it. Resolved by
finishing the decoder in every language — see "list elements: every scalar, in
all nine libraries" below.

## libraries no longer ship an Encoder/Decoder (July 2026)

Every library except Go and Rust used to export a fluent little-endian binary
writer/reader (`Encoder` / `Decoder`, plus PHP's `IntStr`). They are gone.

**Why.** Serialization is the thing serify *tests*, so it must be the user's code.
A library-supplied encoder means the bytes under test were written by serify itself —
testing our encoder against itself — and it quietly imposes one byte layout on
every worker. Nothing in any protocol layer used them (PHP's `Worker.php` and
`FieldMap.php` never referenced `Encoder` once); they were pure user-facing
convenience. Go and Rust never had one and were always the healthiest workers.

**What the examples use now** — each language's own standard facility, which is
what a real user would reach for:

| Language | Encoding | int128 reached via |
|----------|----------|--------------------|
| Go | `encoding/binary` (unchanged) | `math/big` |
| Rust | `to_le_bytes` (unchanged) | native `i128` |
| Python | `struct` + `int.to_bytes` | arbitrary-precision `int` |
| Node | `Buffer` (two 64-bit halves) | `BigInt` |
| C# | `System.Buffers.Binary.BinaryPrimitives` | native `Int128` |
| Java | `java.nio.ByteBuffer` | `BigInteger` |
| C++ | explicit shift-and-mask (host-endianness independent) | `__int128` |
| Elixir | bitstring syntax — the whole layout reads as one literal | `<<v::signed-little-128>>` |
| PHP | `pack`/`unpack` + **ext-gmp** | decimal strings (int is 64-bit) |

**PHP now requires ext-gmp.** PHP's int is 64-bit, so it cannot hold an int128 —
or even a u64 — which is why `FieldMap` carries those as decimal strings. Without
a library encoder the worker must do bignum conversion itself, and GMP is the standard
answer. CI installs it via `setup-php`'s `extensions: gmp`.

All nine languages agree byte-for-byte with the Go reference, including
`ledger/binary/i128_boundaries` (±2^127), asserted by `TestExamples_Ledger`.

## Deferred: `serify init` and `templates/` (removed July 2026)

The `init` command and the whole `templates/` tree were removed. They will come back
once the libraries and the protocol are stable. Notes for whoever rebuilds them, so the
same ground isn't re-covered:

**Why it was deferred.** Every template except `go/` vendored a *copy* of the library
source into the scaffolded project (`node/src/workerlib.ts`, `python/serify.py`,
`php/src/*.php`, `csharp/WorkerLib.cs`, `elixir/lib/worker_lib.ex`,
`rust/workerlib` + `derive`, `cpp/workerlib.hpp`) instead of declaring a
package-manager dependency the way `go/go.mod.tmpl` did. Go worked only because Go
modules resolve straight from the GitHub repo with no registry involved.

**What blocks the package-dependency design.** No library is published anywhere and CI
has no publish step. Registry check (July 2026): `serify` on npm is **taken** by an
unrelated package (`nparsons08`, a Twilio SMS library), so `lib/node` needs a
scoped rename (e.g. `@<owner>/serify`) before `templates/node` can depend on it.
PyPI, Packagist and NuGet all had `serify` free. C++ has no comparable registry, so
`cpp/` would most likely keep vendoring `workerlib.hpp` — and that file should keep
its Apache header, since it *is* serify's code entering someone else's project.

**Bugs that were live at removal time.** Rebuilding from the old templates verbatim
would reintroduce them:

- The scaffolder stripped the template's `//go:build ignore` tag with
  `strings.TrimPrefix`, which only fires when the tag is the *first* thing in the
  file. It never was — the Apache header came first — so the tag survived into every
  scaffolded worker, leaving it with **no buildable package** (`go build ./...`
  exited 0 with "matched no packages").
- The Go template still used `Type{Serializer, Deserializer}`; the library had long since
  moved to `Type{Model, Formats map[string]Format}`. It drifted precisely because the
  build tag meant nothing ever compiled it.
- `templates/java` could not compile at all: `Worker.java` imported
  `io.serify.WorkerLib`, but no such file shipped in the template and `pom.xml`
  declared only jackson.
- Build artifacts were checked in and embedded into the binary:
  `java/target/classes/*.class`, `python/__pycache__/*.pyc`, `rust/Cargo.lock`,
  `rust/workerlib/Cargo.lock`. Add a `.gitignore` before re-adding templates.

**Facts worth keeping (verified, not remembered).**

- A template directory named `go` embeds fine. The old `templateDir`/`reverseTemplateDir`
  maps existed to rename it to `golang`, on the stated grounds that "a directory named
  `go` containing go.mod cannot be embedded" — that reason was false. The real rule is
  that `embed` skips any subdirectory *containing a `go.mod`*, whatever its name, which
  is why `go.mod.tmpl` exists and must keep its suffix.
- `go:embed` **cannot cross a module boundary** (`cannot embed directory go: in
  different module`), so a template directory can never carry a real `go.mod`. This is
  why `examples/go/` can be its own module but `templates/go/` could not.
- Consequence: a Go template inside the module is either compiled (letting CI's
  `go build ./...` guarantee it never rots — this is what catches API drift) or excluded
  (and then it rots silently). If compiled, note that `templates/go` as a `package main`
  makes `go install ./...` produce a binary literally named `go`, which shadows the
  toolchain on most `$PATH` setups. Nothing in CI runs `go install ./...`.
- Template files must not carry serify's Apache header or any TODO comments: both get
  copied verbatim into the user's project. Vendored library files are the exception — those
  keep their header.

## Transparent newtypes: `#[serify(transparent)]` (July 2026)

A newtype that exists to *validate* — iggy's `WireName(String)`, 1–255 bytes —
used to need a hand-written `#[serify(with = "...")]` module on every field that
held one, plus a second module for the sum-payload case. Two modules, ~30
lines, repeated per newtype.

The derive now handles a single-field tuple struct directly:

```rust
#[derive(Clone, PartialEq, Eq, SerifyModel)]
#[serify(transparent, try_from = "WireName::new")]
pub struct WireName(String);
```

- `transparent` means the schema sees straight through the wrapper to the type
  it wraps — a `WireName` field is a `string` field.
- `try_from` names a **fallible** constructor. The derive never writes the
  private field directly, so validation is preserved on the way in; without it
  a newtype could be constructed from wire bytes in a state its own constructor
  would have rejected. Omit it and the derive uses `Self(raw)`.
- Three impls are generated — `SerifyField` (as a field of another model),
  `SerifyPayload` (as a `sum` payload), and `SerifyModel` (standalone, as the
  one-field record its schema declares, carrying the value under `"value"`).

A newtype without `#[serify(transparent)]` is a compile error naming the
attribute, rather than being silently treated as a struct with one unnamed field.

The generated impls delegate to the *inner type's* `SerifyField`/`SerifyPayload`
impl rather than re-recognising the inner type syntactically, so a newtype works
over anything the schema supports, including another model.

**Effect on the iggy integration:** `serify_support.rs` is down to a single hook
(`wire_identifier`), which exists only because the case files import `identifier`
as a one-field *record*, so as a request field it sits one level deeper than a
bare variant. Every other wire type maps itself. Verified at 72 PASS / 32 SKIP /
exit 0, unchanged; canaried by renaming the standalone key `"value"` → `"walue"`
in the derive, which turns that into 15 FAIL.

## `sum` model binding in every language (July 2026)

Until now `sum` was the one type serify's schema supported but its model
binding did not: only Rust (a native `enum`, handled by the derive) and Go (a
hand-written `serify.Converter`) could bind it. The other seven workers read the
variant off the FieldMap by hand.

The plan was "add a converter mechanism to the seven". **Probing each language
first showed that premise was wrong** — five of them can do what Rust does, with
nothing declared at all:

| Language | Sum type | Runtime introspection | Declared by the user |
|----------|----------|-----------------------|----------------------|
| Rust | `enum` | proc macro | nothing |
| Java | `sealed interface` + records | `isSealed()`, `getPermittedSubclasses()`, `getRecordComponents()` | nothing |
| Python | union of dataclasses | `types.UnionType`, `get_args`, `fields()` | nothing |
| PHP | property union type | `ReflectionUnionType::getTypes()` | nothing |
| C# | `abstract record` + nested sealed records | `GetNestedTypes(Public\|NonPublic)` | nothing |
| Elixir | tagged tuple | the tag *is* the data | nothing |
| C++ | `std::variant` | alternatives yes, **names no** | `SERIFY_SUM(T, "a", "b")` |
| Node/TS | — | union types are erased; `emitDecoratorMetadata` reports nothing | `@Serify.sum([A, B])` |
| Go | interface | cannot enumerate implementations | a `serify.Converter` |

Each probe was a real compiled/run program, not a recollection — the C# one is
why the implementation passes `BindingFlags.NonPublic`, since `GetNestedTypes()`
alone returns only *public* nested types and an `internal` hierarchy is ordinary.

All nine share one arity rule, the same one the Rust derive already used, since a
`sum` is a sum-of-products: **0 fields → unit variant, 1 field → that value is
the payload, N fields → the payload is a struct.** Tags are the arm's type name
in snake_case. C++ needs no wrapper structs at all because each `std::variant`
alternative *is* its payload (`std::monostate`, `std::string`, `uint64_t`, a
model).

Go is now the most verbose binding in the suite — ~50 lines of hand-written type
assertions for what eight other languages do in zero or one. Accepting an arm
list the way Node does would shrink it; not attempted here.

**Two latent library bugs surfaced, both of the same shape.** PHP's and Node's
model helpers are value-driven, and both widened *any* integer to the 64-bit
carrier (a decimal string / a `BigInt`) because the helper cannot see the
schema's width. Encoding for `uint32` passes the stored value straight through,
so the wire got `"1"` instead of `1` (PHP) and JSON.stringify refused the BigInt
outright (Node). Neither was reachable before: no PHP or Node model had ever
carried a plain small integer — `LedgerEntry` declares every integer as a string
/ bigint, and `notification` had no model. Fixed in `encodeField` on both sides
(narrow to a number at ≤32 bits) rather than in the helpers, which keeps
`getI64()` honest about its return type. The first attempt fixed it in the
helper instead and broke `workerlib.test.js`, which was right to complain.

## Rust crate: `worker` feature, so model binding costs no dependencies (July 2026)

Found while running iggy's own test suite after adding the serify dep: 461 tests
would not **compile**, on a line serify never touches —

```
error[E0283]: type annotations needed
   --> core/binary_protocol/src/primitives/user_headers.rs:499
    |    assert_eq!(wire.as_bytes(), &[]);
    = note: multiple `impl`s satisfying `u8: PartialEq<_>` found in the
            following crates: `core`, `serde_json`
```

serify pulled `serde_json` into the dependency graph, `serde_json` ships
`impl PartialEq<Value> for u8`, and an empty-slice literal in an unrelated
assertion stopped being inferrable. **Taking a conformance library as a
dependency must not change how the consumer's own code type-checks.**

The Rust crate is now split by feature:

```toml
default = ["worker"]
worker  = ["dep:serde_json", "dep:hex"]
```

`worker` covers the NDJSON protocol loop (`run_suite`) and its JSON/hex codec —
what a conformance worker binary wants, so it stays on by default. A crate that
only wants `#[derive(SerifyModel)]` on its own types takes serify with
`default-features = false` and gets a graph of exactly one entry, the derive
macro: no serde_json, no hex, no inference changes. `FieldMap`, `FieldValue`,
`SerifyModel`, `SerifyField` and `SerifyPayload` are all outside the feature.

iggy's `Cargo.toml` now takes it that way; its conformance worker keeps the
default. `cargo test -p iggy_binary_protocol` is back to 461 passing,
`-p iggy_common` to 265, and the conformance run is unchanged at 72 PASS /
32 SKIP.

## Example workers: worker code split from model code (July 2026)

Each of the nine example workers used to be one file mixing three unrelated
things: the serify registration, the stand-in "application" types, and the byte
layout. They are now split the same way in every language, so a reader can tell
at a glance which part they would actually write for their own project:

| File | What it is |
|------|------------|
| `worker.*` (`main.go`, `main.rs`) | The worker: names the types it handles, one serializer/deserializer pair per format. Nothing else. |
| `customer.*`, `ledger.*`, `notification.*` | Stand-ins for types an application already owns: the struct, its schema binding, and its byte layout. |
| `wire.*` | Byte-level primitives shared by those models. |

Go's `type.go` became `wire.go` + `customer.go`; the layout-conventions comment
every other worker cites now lives in `examples/go/wire.go` and all
cross-references were updated. Rust gained `common.rs` for the two reusable
models (`address`, `money`) that more than one type nests.

Where a model exists, the byte layout is now a method on it (`marshal` /
`unmarshal`) and the worker file only bridges FieldMap ↔ model. At the time of
the split `notification` was the exception in seven of the nine languages —
only Go (a `Converter`) and Rust (a native enum) could bind a `sum` onto a
model, so the other seven read and write the active variant against the FieldMap
directly. That gap is now closed in every language; see "`sum` model binding in
every language" above, which is what the seven were waiting on.

No behaviour changed: all nine workers still agree byte-for-byte.

**Follow-up in the same pass — schema tag names deleted from user code.** The
split exposed that Go and Rust, the two languages with a real `sum` model, were
each re-deriving what the binding already owned. Rust had `CHANNEL_TAGS`, a
`tag()` and an `ordinal()`; Go had a `channelTag()` marker method, a
`channelTags` slice and a `channelOrdinal()` lookup — so `"silent"`, `"sms"`,
`"push"`, `"invoice"` appeared in *three* places in `notification.go` alone, all
of which had to stay in sync with the case file and with each other.

A tag ordinal is genuinely the worker's business (it is a byte-layout fact), but
it is just the variant's declaration order, so it belongs in the `match`/`switch`
that already discriminates the variant — not in a parallel string table:

```rust
match &self.channel {
    Channel::Silent  => buf.push(0),  // a unit variant is nothing but its tag
    Channel::Sms(s)  => { buf.push(1); append_len_str(&mut buf, s); }
    ...
```

Both files now contain **zero** schema tag strings; in Go they survive only
inside the `Channel` converter in `main.go`, which is the one place that is
supposed to name them.
Rust's `notification.rs` lost 20 lines, Go's `notification.go` 10. Go's marker
method was retrained on its remaining job — sealing the interface, so
`n.Channel = 42` will not compile — and its doc now states plainly what sealing
does *not* buy: Go gives no exhaustiveness check when a fifth variant is added,
where Rust's `match` does.

## Test results

All 11 Go test packages pass (`go test ./...`, plus `go vet ./...` and
golangci-lint clean):

```
ok  github.com/chengxilo/serify                    0.157s
ok  github.com/chengxilo/serify/cmd/serify         0.052s
ok  github.com/chengxilo/serify/examples/test    235.560s
ok  github.com/chengxilo/serify/internal/builder    0.082s
ok  github.com/chengxilo/serify/internal/compare    0.013s
ok  github.com/chengxilo/serify/internal/config     0.115s
ok  github.com/chengxilo/serify/internal/orchestrate 0.016s
ok  github.com/chengxilo/serify/internal/protocol   0.012s
ok  github.com/chengxilo/serify/internal/report     0.021s
ok  github.com/chengxilo/serify/internal/worker     1.406s
ok  github.com/chengxilo/serify/test               31.803s
```

`examples/test` dominates the wall clock because it builds nine worker toolchains
(cargo, npm, dotnet, maven, mix, g++, …) before running a case. `serifyTimeout` in
`internal/testutil/cli.go` was raised from 2m to 10m for exactly this reason — a cold
build alone could exceed the old limit, which showed up as an intermittent
`serify timed out after 2m` in `TestExamples_Ledger` / `TestExamples_Audit`. The
timeout is only there to catch a genuinely wedged worker.

## list elements: every scalar, in all nine libraries (July 2026)

A `list<T>` accepted only a subset of `T`, and the subset differed per language.
`serify validate` accepted all fifteen scalars as element types — the runner's
`encodeValue` has always been fully generic, recursing into `encodeValue` per
element — so the gap was entirely in the worker libraries, and only surfaced
once a worker actually ran.

Surveying first showed the earlier note (`list<float64>`/`uint16`/`int8`/`int16`,
Go-only) understated it on both axes. It missed `list<bytes>`, and six other
languages were worse than Go — **and three of the nine failed silently**:

| Language | Element types before | On an unsupported element type |
|----------|---------------------|-------------------------------|
| Elixir | all — already recursed into its scalar decoder | n/a |
| Go, Node, PHP, C# | 11 of 16 | raised |
| Java | 6 of 16 | **silent** — no default arm; field left absent |
| Rust | 6 of 16 | raised |
| Python | ~10 of 16 | **silent** — fell through to `list(arr)`, so a `list<float64>` handed the worker raw hex strings |
| C++ | 7 of 16 | **silent** — no final `else`; field left absent |

The silent three are the reason this outranked the missing types: a worker got
wrong data, or no data, with nothing raised anywhere.

**The fix is one idea, applied nine times: a list decodes its elements through
the same per-field codec a bare field uses.** Go's `decodeScalar` already carried
the doc comment *"the field, list, optional and map paths all route element
decoding through here"* — `decodeList` simply never did, and had drifted into a
parallel switch. Elixir was the only library that already worked this way, and
was the only one with no gap; that is not a coincidence, and is why it needed no
change. Every other language grew the same shape, using whatever local idiom the
`optional`/`map` paths were already using to decode a value of a dynamically
known type (Rust, Java, C#, PHP, Node and Python all decode into a scratch
FieldMap under a fixed key and unwrap it; C++ does the same through
`decode_field_map`).

What survives per language is a table mapping element type to the *container* it
collects into (`[]uint16`, `Vec<i8>`, `ListF64`, …) — no decoding logic, so a
scalar cannot be reachable as a field but not as a list element. The encode side
was rebuilt to mirror it exactly, since three languages had an encoder that
silently passed the value through unconverted.

**Surface added:** the missing typed list accessors, so a worker can read what it
can now receive — Go gained `GetListI8`/`I16`/`Bytes` (+`Set*`), Rust seven
`FieldValue` variants and a full `get_list_*`/`set_list_*` set, Node/C#/C++ the
`u16`/`i8`/`i16`/`f64`/`bytes` accessors. C++ needed one wrinkle: `Bytes` and
`ListU8` are both `std::vector<uint8_t>`, and `std::variant` cannot hold the same
alternative twice, so `ListU8` is an alias rather than a new alternative.

**Verification.** Go, Rust, Python and Node have unit-test harnesses and each got
a table test walking all fifteen scalars through decode *and* re-encode,
asserting the wire form comes back byte-identical (plus, for Python and Node, an
explicit check that `list<float64>` yields numbers rather than the hex it
travelled as, and that an unknown element type raises). The Go test was canaried
by deleting the `uint16`/`float64`/`bytes` arms — it fails with exactly the
pre-fix error. C#, Java, C++ and PHP still have no harness in this repo, so each
was driven by a standalone program asserting the same fifteen round-trips; all
four print `ALL 15 ELEMENT TYPES ROUND-TRIP`. Giving those four a committed test
remains the same open problem recorded under the `--audit`/`sum` work.

### Promoted into the shipped suite: `signals`

The element types are now in the conformance suite, so cross-language *byte*
parity is asserted rather than just per-library round-tripping.
`examples/cases/signals.yaml` is a capture from a multi-channel instrument whose
sixteen fields between them use every scalar the schema allows as a list element,
implemented by **all nine** example workers and guarded by `TestExamples_Signals`.
Four cases: `typical` (ragged lengths, so a worker that carries one list's count
into the next fails), `empty_lists` (the u32 count prefix must still be written),
`width_boundaries` (min/max of every width) and `float_extremes`.

Doing this surfaced **a third layer with the same defect**, which the library-level
fix had not touched: every language's *model binding* had its own list-type table,
independent of the codec's. The bindings are value-driven, so most of them guessed
the element type from the first element and fell through for anything unrecognised:

| Language | Model-binding gap before |
|----------|--------------------------|
| Go | `reflectFill`/`reflectExtract` named `[]byte`, `[]string`, `[]*big.Int`, `[]struct` and errored on every other slice — `[]bool` was an "unsupported field type" |
| Rust | the derive classified `Vec<u8>`, `Vec<String>`, `Vec<Model>`; any other `Vec<T>` was treated as a **nested model** and failed to compile |
| Python | `list[str]/[int]/[float]/[Model]`, silently falling back to `list_string` |
| Node | guessed from `val[0]`: strings → `setListString`, numbers → `setListU32`, **everything else → `setListString`** |
| C# | `case string[] / uint[] / ulong[] / float[]`, everything else hitting the `ToString()` default |
| Java | guessed from `v.get(0)` — and an **empty list stored nothing at all**, so the field vanished from the FieldMap |
| PHP | guessed from `$val[0]`, else `setListString` |
| Elixir | `{:list, :string\|:u32\|:u64\|:f32\|:struct}` and no other element type would bind |
| C++ | none — `SERIFY_FIELD(name, kind)` expands to `set_##kind`, so it worked as soon as the accessors existed |

Each was fixed the same way as the codec: the binding stores the list as-is and
lets the schema decide the element's wire form, because the *value* cannot say
which width `list[int]` means and guessing from element zero says nothing at all
about an empty list. C++ needed no change, having never had a table to be
incomplete. Rust is the exception to "store as-is": it is statically typed, so the
derive gained `ListScalar`/`ListBytes` classifications mapping to the typed
`get_list_*`/`set_list_*` pairs.

**Two representation decisions fell out of it**, both forced rather than chosen:

- `list<uint8>` and `bytes` share one FieldMap value in Rust, C++ and C#, because
  in those languages a model spells both `Vec<u8>` / `std::vector<uint8_t>` /
  `byte[]`. Keeping them distinct would have made a field's type ambiguous to the
  binding. (C++ had no choice at all: `std::variant` cannot hold the same
  alternative twice.)
- `signals` carries **no float infinities**. The BEAM has no infinite float — the
  bitstring syntax refuses to write one — so `.inf` would be unrepresentable in
  the Elixir worker rather than merely awkward. Infinity and NaN stay covered for
  *scalar* floats by `telemetry.yaml`, which only Go and Rust implement. This is a
  real limit of the Elixir library, not of the case file.

## Containers: one element-type enumeration, not five (July 2026)

The `list<T>` work above fixed one container path. Auditing the other four found
the same defect in three of them, so they were collapsed the same way. The rule
the codebase now follows: **an element type is enumerated in exactly one place —
the scalar codec — and every container delegates to it.**

### `array<T,N>` — the one that silently corrupted data

`array<T,N>` was the worst of the remaining three, because it did not error: it
truncated. Every library carried an array as `[uint32; 4]` *regardless of what
the schema said* — Rust's decoder literally read `let mut out = [0u32; 4]` with
`.take(4)`, Node's was `[number,number,number,number]`, C++'s `ArrayU32` capped
at 4, Elixir matched the literal type string `"array<uint32,4>"`. So
`array<uint8,16>` passed `serify validate` and then lost twelve elements, and
`array<int16,3>` refused its negative values. Only Go was generic.

The fix removes the container rather than repairing it: **`array<T,N>` is now
`list<T>` plus a length assertion**, sharing its storage and element decoding.
Five container paths per library became four. That meant *deleting* the
array-specific surface, not extending it — Go's `GetArray`/`SetArray`/
`GetArrayU32`/`SetArrayU32`, Rust's `FieldValue::ArrayU32` and its accessors,
C++'s `ArrayU32` alternative — and replacing the Rust derive's two fixed shapes
(`ArrayU32x4`, `ArrayU8x4`, both length 4) with one `ArrayN(elem, len)`.

`signals` gained `checksum: array<uint8,4>` (the shape that happened to work) and
`window: array<int16,3>` (a non-4 length with a signed element type — the shape
that could not exist), implemented by all nine workers.

**Two things surfaced while doing this, both worth recording:**

- `telemetry.yaml` declares `array<int16,3>` with negative values, and **no
  example worker implements `telemetry` at all** — not even Go. It has been
  declared-but-unexercised the whole time, which is why nobody noticed that
  Rust's array decoder rejects negative elements. `order` is in the same state.
- Go's `MapOf` **silently dropped** any value whose type it did not name. It now
  stores unrecognised values as-is; a Go array is converted to the `[]T` an
  `array<T,N>` field expects.

### `optional<T>` — C++ only

Eight libraries already routed an `optional<T>` through their scalar path. C++
named only `string` and `struct` and **silently dropped the field** for anything
else, in both directions. Fixed by delegating like the other eight; the reason it
had not been done before is that C++'s `FieldValue` had no way to represent a
null scalar, so a `std::monostate` alternative was added for exactly that.

Verified with a standalone driver covering twelve element types × {value, null}.
Cross-language byte parity for `optional<scalar>` is still **not** asserted —
`customer` only exercises `optional<string>`, and `telemetry`'s
`optional<float32>` is in the unimplemented type above. Adding an
`optional<scalar>` field to `signals` would close that; it was not done here
because the defect was in one language and nine worker edits is disproportionate
to it.

### `map<K,V>` — Python only

Python's `_decode_map` was its own chain of type tests ending in
`result[k] = item`, so a `map<string,float64>` handed the worker the raw hex
strings — the same silent failure `_decode_list` had. Both directions now
delegate to `_decode_field`/`_encode_field`, and unknown value types raise.
Covered by `test_map_supports_every_scalar_value` and a rejection test.

## CLI and docs: the user-facing pass (July 2026)

### `serify validate` validated the wrong directory, silently

Positional arguments to `validate` are **worker directories**, matching
`serify run`; the case directory comes from `--cases` (default `cases`). The
README documented `serify validate <cases-dir>` instead, so following it made the
command validate whatever `--cases` defaulted to, print a full successful suite
report for it, and only then fail with a `cannot detect language` error about the
directory the user *did* name. A user with a `./cases` directory saw a green
"Total: 5 types, 33 test ids" for a directory they never mentioned.

Fixed on both sides. `runValidate` now checks the worker directories **before**
loading the cases, so nothing is reported until everything named is known good,
and a positional argument holding `*.yaml` files is rejected with a message
naming the flag to use instead. The suite header now prints the directory it
validated. `TestCLI_Validate_CasesDirPassedPositionally` pins it, canaried
against the old ordering.

### Two workers of the same language silently collapse

Found while rewriting the README quick start. `serify run --ref go examples/go
examples/my-go-worker` detects and builds both, but the report is keyed by
**language**, so the second worker's results replace the first's — the run
reports one `go` column and the pass count of a single worker, with no warning.
You would believe you had compared two implementations when you had compared one.

**Rejected rather than re-architected.** Keying results by worker directory
instead of language would touch the report model, the CSV columns and the table
renderer, and it argues against the premise: serify exists to compare *languages*,
so "which worker is `go`?" has no answer. `detectAndBuild` now fails when two
workers resolve to the same language, naming both directories.

`TestCLI_Run_DuplicateLanguage` pins it. The canary is worth recording because it
shows what the silence looked like: with the check removed, passing the same
worker directory twice reports `PASSED: 40 (exit code 0)` — a completely healthy
run that compared one worker against itself.

### README corrections

- Quick start was unrunnable end to end: it said to `cp -r examples/go ./my-worker`,
  which cannot build outside the repository (every example takes its library by
  relative path), and then gave `--ref rust` for a lone Go worker, which is
  rejected because the reference must be one of the workers passed. It now says
  to work inside a clone, gives a command that runs as written, and shows the
  copy-a-worker flow inside `examples/` where the relative paths still resolve.
- Audit section said "four unsafe behaviours"; there are six.
- Model binding section had no `sum` entry at all, despite that being the
  feature all nine libraries had most recently gained.
- The language table implied nine published libraries. Nothing is published; the
  table now states how a worker actually takes each library today.

## Unknown types now fail loudly in every library (July 2026)

Auditing the container work found the same defect one level up, in the top-level
field dispatch. Six of the nine libraries did *something* silent when handed a
type they did not recognise:

| Language | decode | encode |
|----------|--------|--------|
| Go, Rust | error | error |
| Elixir | `FunctionClauseError` — loud | loud |
| Python | raised (added with the list work) | **`return v`** — passed the value through untouched |
| Node | **dropped the field** | **`return v`** |
| C# | **`break`** — dropped the field | **`_ => v`** |
| Java | **fell off the chain** — dropped the field | `mapper.valueToTree(v)` |
| PHP | **`break`** — dropped the field | fell through |
| C++ | **fell off the chain** — dropped the field | dropped the field |

This is not hypothetical: it is exactly what happened when `sum` landed, when
every library's audit walker silently skipped the new Variant kind (see
"`--audit` and `sum` payloads" above). Adding a type to the schema and
forgetting one library should be a hard error in that library, not a field that
quietly goes missing.

All six now raise `unknown type "<t>"` in both directions. The full suite passes
unchanged, which says the fallback was dead for every type currently exercised —
the value of the change is entirely in what it does to the *next* type added.
Pinned by `test_unknown_field_type_raises_both_directions` (Python) and
`an unknown field type fails loudly in both directions` (Node); Go and Rust
already had equivalent tests.

## `telemetry` and `order` implemented; a type nobody implements is now visible (July 2026)

Both types had `cases:` in `examples/cases` and were implemented by **no worker
at all** — not even Go. They had been dead declarations the whole time, which is
why nobody noticed that Rust's old array decoder rejected negative elements:
`telemetry`'s `array<int16,3>` had never run anywhere.

**Both are now implemented in the Go reference worker**, taking the suite to
`SKIPPED: 0` for a Go-only run. They were worth implementing rather than
deleting, because between them they are the only coverage of several schema
features:

| Type | What only it exercises |
|------|------------------------|
| `telemetry` | `optional<float32>` — the suite's only `optional<scalar>`; `uint128`; two differently shaped fixed arrays; `map<string,uint64>`; NaN, ±Inf and negative zero |
| `order` | `enum<…>`; `list<struct>`; `map<string,struct>`; `optional<struct>` |

**Implementing them exposed the same defect once more, in Go's reflection shim.**
It handled `*string` and `*big.Int` and nothing else, so a Go model could express
neither `optional<float32>` nor `optional<struct>` — which is the real reason
`telemetry` had never been written. Both pointer arms are now generic: `fill`
takes nil, a scalar, or a `*FieldMap` for a struct; `extract` mirrors it. That
also closes the `optional<scalar>` coverage gap recorded above, at least for Go.

### A type no worker implements now says so

Its results are a column of SKIP, which in the grid reads exactly like a healthy
run — the suite reported `exit 0` having compared nothing about it. `Report`
now detects types where every result is a SKIP and prints them under the grid:

```
No worker implements: order, telemetry
  Every result for these is SKIP, which in the grid above reads the same as a
  healthy run — nothing about them was actually compared.
```

It fires only when *no* worker implements the type: a type implemented by one
worker and skipped by others is an ordinary partial suite and stays quiet.
Deliberately a warning and not a failure — running a subset of workers during
development is legitimate, and making it fatal would break that. Covered by
`TestUnimplementedTypes` and `TestUnimplementedTypes_FailureCounts` (a type that
FAILs is implemented — it ran and disagreed, the opposite of untested).

## `optional<scalar>` in the model bindings, and a test-suite timeout (July 2026)

### The timeout, caused by implementing `telemetry` and `order`

`examples/test` ran **five** complete 9-worker suites. Three of them —
`_Ledger`, `_Notification`, `_Signals` — issued the *identical* command and
differed only in which cells they asserted, so they each paid for a full run of
everything to check one subset. At 461s that was merely wasteful; adding
`telemetry`'s 8 cases and `order`'s 5 pushed the package past go test's
10-minute limit and the whole thing died with `panic: test timed out after
10m0s`.

Fixed by sharing one `sync.Once` run across the three, which is what they always
meant. **284s**, below where the package sat before the two new types were added.
Raising the timeout would have preserved the waste instead.

### `optional<T>` was string-and-struct-only in two more bindings

The gap found in Go's reflection shim while implementing `telemetry` turned out
to be the same in the other two bindings that carry an explicit type table:

| Language | Before | Symptom |
|----------|--------|---------|
| Go | `*string`, `*big.Int` | `unsupported field type *float32` |
| Rust | `Option<String>` only | `Option<f32>` classified as a nested model: **"the trait bound `Option<f32>: SerifyField` is not satisfied"** |
| Elixir | `{:optional, :string}`, `{:optional, :struct}` | no clause — a model naming any other optional would not compile |

All three are now generic over the element type, and Rust and Go additionally
handle `optional<struct>` (`Option<Model>` / `*Struct`), which neither could
express before. Rust gained `FieldMap::set_null` for the write side: `get_<scalar>`
already returns `None` for both an absent field and a stored `Null`, which is
exactly `Option<T>`'s semantics, but there was no way to *write* that state.

This is why `telemetry` had never been implemented in any language: its
`humidity_pct` is an `optional<float32>`, and no binding could carry one. With the
derive fixed, **Rust now implements `telemetry` too**, so the suite finally
asserts cross-language byte parity for an `optional<scalar>` — present in
`nominal`, null in `zero`, both compared. `TestExamples_Telemetry` guards it.
That closes the coverage gap recorded twice above.

The value-driven bindings (Python, Node, C#, Java, PHP) were unaffected — they
store nil or the value directly and let the schema decide the wire form. Python
does misclassify `int | None` as kind `optional_string`, which is only harmless
because its FieldMap storage is untyped; the name is wrong but the behaviour is
not, so it was left alone.

Removing the generic `{:array, _elem}` clause's now-dead predecessors surfaced
two `{:array, :u32}` clauses in Elixir that the compiler flagged as unreachable;
both deleted.

## CI was not testing the thing this project exists to test (July 2026)

Checking whether CI would pass the container work turned up two dead mechanisms
in the **conformance** job — the 9-language matrix that verifies cross-language
byte parity, which is serify's entire purpose.

**1. The job ran zero tests.** It invoked
`go test -run TestExamples_AllLanguages ./examples/test/`, and no test by that
name exists. `go test -run` matching nothing prints a warning and **exits 0**:

```
testing: warning: no tests to run
ok  github.com/chengxilo/serify/examples/test  0.305s [no tests to run]
EXIT=0
```

So every matrix leg installed a full language toolchain, ran nothing, and
reported green.

**2. `SERIFY_REQUIRE` was set but read by nothing.** The intent is clear from the
name: assert that the leg's languages are actually present. Without it a failed
toolchain install merely drops the language from `availableLangs`, and the tests
pass having compared whichever languages happened to exist.

Both fixed. `SERIFY_REQUIRE` is now honoured in `TestMain` — a required language
that is absent exits 1 naming it and why. The `-run` filter is **gone** rather
than corrected: a filter is a name that can silently match nothing, and no filter
cannot. `TestExamples_GoRust` now skips (rather than fails) when rust is absent,
so the whole package runs in every leg, with `SERIFY_REQUIRE` supplying the
must-be-present guarantee. That also moves "which languages are mandatory" to the
environment that installs them instead of hardcoding it per test.

A simulated leg now runs six real tests where it ran none:

```
=== RUN TestExamples_GoRust      --- PASS (1.56s)
=== RUN TestExamples_Ledger      --- PASS (167.58s)
=== RUN TestExamples_Notification --- PASS (0.00s)   ← shared grid
=== RUN TestExamples_Signals     --- PASS (0.00s)
=== RUN TestExamples_Telemetry   --- PASS (0.00s)
=== RUN TestExamples_Audit       --- PASS (120.20s)
```

**Also found, and not fixed:**

- The unit-test job ran `go test ./lib/... ./internal/...`, missing `./cmd/...`
  entirely — `cmd/serify/schema_test.go` was never run in CI. Added.
- `mypy --strict lib/python/serify.py` **fails at HEAD** with 46 errors, so the
  `test-python` job is red before any recent work; the uncommitted `sum` work
  adds ~16 more (untyped `_get_type_hints`, bare `tuple` annotations in the
  sum helpers). None come from the container/dispatch work. Left alone: it is
  a pre-existing failure and a separate, mechanical annotation pass.
- `test-rust` uses `working-directory: lib/rust`, which has no `Cargo.toml`;
  cargo walks up to the root workspace, so it works but tests the whole
  workspace rather than the library the step names.

## `mypy --strict` was red at HEAD; and a NaN false-positive in --audit (July 2026)

### The Python type-check baseline

CI's `test-python` job runs `mypy --strict lib/python/serify.py`. That has been
**failing at HEAD** — 46 errors in the committed file, ~16 more from the
uncommitted `sum` work — so the job was red before any of this session's work,
which erodes the signal from every other check. Brought to **zero**.

Most were annotation gaps in the `@serify_model` machinery, fixed with `Any`/
`type`/`tuple[Any, ...]`; the one design choice was making `_type_info` return
`tuple[str, Any]` rather than `tuple[str, Callable | None]`, since `extra` is a
class / inner-hint / None union read reflectively, not something a static type
helps. **Two were real:** `_run_loop` called `serialize_fn`/`deserialize_fn`
without checking they were set, so a serialize message before a successful init
would crash on `None(...)`; both branches now raise a clear error instead. None
of the 62 came from the container/dispatch/optional work — every one was
pre-existing or from the sum binding.

Verified behaviour-neutral: unit tests still 14/14, and the Python worker still
agrees with Go across a full conformance run including `--audit` (the guards and
a variable rename live in the run loop the unit tests do not exercise).

### NaN made --audit cry wolf three times

Running the Python worker under `--audit` after implementing `telemetry` surfaced
three warnings on `telemetry/binary/float_nan`, all attributed to the **Go**
reference worker: mutation, output-zero-copy, and deser-instability, all on the
float fields. One cause: `valuesEqual` falls back to `reflect.DeepEqual`, and
`DeepEqual(NaN, NaN)` is **false** because floats compare with `==` and NaN is
never equal to itself. So a perfectly deterministic worker whose data contains a
NaN looked like it mutated the input, aliased the output, and deserialized
unstably — three false warnings from one corner.

`valuesEqual` now compares float32/float64 by bit pattern, so two bit-identical
NaNs are equal while a genuinely different float still diffs.
`TestCompareFieldMaps_NaN` pins both halves, canaried against the pre-fix code
(which fails with `two NaN values must compare equal, got diffs [v]`). This was
latent until `telemetry` — the only type with a NaN case — became implementable;
the same virtuous exposure that has run through this whole session.

### CI honesty, continued

`SERIFY_REQUIRE` and the missing `-run` filter (above) plus `mypy` red and
`./cmd/...` unrun mean CI was, in aggregate, not verifying: the parity matrix ran
nothing, a red type-check job trained everyone to ignore red, and cmd tests never
ran. With these fixed the CI signal is worth trusting again.
