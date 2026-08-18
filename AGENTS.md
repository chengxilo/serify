# AGENTS.md

This file provides guidance to coding agents working in this repository.
`CLAUDE.md` is a one-line pointer here; keep the content in this file.

## What this is

serify is a cross-language serialization conformance harness. You define a schema and test cases once; each target language ships a small **worker** program that serializes/deserializes the data, and the `serify` CLI drives every worker through the same cases and compares their output **byte-for-byte**. The schema/field-mapping is shared; each worker owns its byte layout.

## Two halves of the codebase

Understanding this split is essential — they are different audiences with their own languages.

1. **The runner** — the `serify` CLI (Go). `cmd/serify/` (cobra `main` package: `run`, `validate`, `schema`, `table`) → `internal/`:
   - `internal/config` — parses per-type case YAML files and `worker.yaml` manifests.
   - `internal/builder` — auto-detects worker language from marker files, resolves build/run commands (with defaults), and builds workers. The build command always runs (unless `--no-build`): each language's build tool is already incremental and knows its own dependency graph, so serify does not second-guess it with a cache of its own.
   - `internal/worker` — manages one worker subprocess and the NDJSON handshake.
   - `internal/protocol` — the NDJSON wire format and `EncodeData` (test values → wire-safe forms).
   - `internal/orchestrate` — runs the test rounds across all workers concurrently and compares results.
   - `internal/report` — the canonical result model + renderers (see below).
   - `internal/compare` — hex and decoded-data diffs.

2. **The worker libraries** — what a worker author imports, one implementation per language:
   - `lib/go/serify` (package `serify`, imported as `github.com/chengxilo/serify/lib/go/serify`) — the **Go** worker lib: `serify.Run`, `Suite`/`Type`/`Format`, `FieldMap`, reflection-based field mapping, and the NDJSON loop. Same module as the CLI (`cmd/serify/`); the Thrift-style `lib/go/serify` path keeps the import's last element equal to the package name.
   - `lib/<lang>/` — equivalents for rust/python/node/csharp/cpp/elixir/java/php. `lib/rust/serify` is the Rust crate (`serify`); `lib/rust/derive` is the proc-macro for `#[derive(SerifyModel)]`.
   - `examples/<lang>/` — reference worker implementations, exercised by `examples/test/` (not by `test/`, which drives its own fixtures under `test/cases/`). `examples/cases/` holds the shared conformance suite (one `.yaml` file per type; tested: `customer`, `order`, `telemetry`, `ledger`, `notification`, `signals`, plus reusable `address`/`money`/`line_item`). `notification` is the `sum` type and is implemented by **all nine** example workers, so `TestExamples_Notification` is what actually guards sum-type parity across the libraries. `signals` plays the same role for `list<T>` and `array<T,N>`: its fields use every scalar the schema allows as a list element, so `TestExamples_Signals` is what keeps a library from quietly dropping one — three libraries once failed *silently* on the element types they did not handle. `telemetry` (**all but elixir** — the BEAM has no NaN and no infinity, so its float cases are unrepresentable there and the gap is declared in `examples/cases/expected_skips/elixir.yaml`) carries the only `optional<scalar>`, the only `uint128`, and the float NaN/Inf cases; `customer` (**all nine**) is the only type with a `json` format beside `binary`, so it is what keeps the JSON encoders honest — and, being the only type with nested structs, it is what found the same dead nested-struct branch in the node, csharp, java and php bindings; `order` (**all nine**) is the only type combining `enum`, `list<struct>`, `map<string,struct>` and `optional<struct>`, and through line_item the only struct nested inside a struct. Every worker now implements every type except elixir/telemetry, so that one entry is the entire content of `expected_skips/`. `--expect-skips` (wired into `examples/test/`) turns any *undeclared* skip into a failure — a column of SKIP otherwise reads exactly like a passing run. Each file carries a `fields:` (or `variants:`) section plus `formats:`/`cases:`; generated JSON Schemas (`.schemas/*.schema.json`, from `scripts/gen-schemas.sh`) give editors save-time validation of case data.
   - There is **no `serify init` / `templates/` scaffolding** — it was removed and will be rebuilt once libraries/protocol stabilize; new workers start by copying an `examples/<lang>` worker.

**Go and Rust are the reference implementations.** When changing protocol or library behavior, update Go (`lib/go/serify`) and Rust (`lib/rust/`) first, then propagate to the other languages. All 9 libraries are currently at parity for core features (audit, `map<K,V>`, `sum<...>`, FieldMap types).

## Model-binding mechanisms per language

Each library provides an idiomatic way to map native structs/classes to FieldMap, avoiding manual `set_*`/`get_*` calls.

| Language | Mechanism | Key features |
|----------|-----------|-------------|
| **Go** | Struct tags | `serify:"field_name"` on struct fields; reflection-based `reflectFill`/`reflectExtract` |
| **Rust** | Derive macro | `#[derive(SerifyModel)]` + `#[serify(rename = "key")]`; compile-time codegen in `lib/rust/derive` |
| **Python** | Decorator | `@serify_model` on `@dataclass`; field names → schema keys; `metadata={"serify": "key"}` for renames |
| **Node/TS** | Decorator | `@Serify.Model()` class + `@Serify.field({rename: "key"})` property decorators; `WeakMap`-based metadata, no external deps |
| **C#** | Attribute | `[SerifyModel]` class + `[SerifyField("key")]` property attributes; `SerifyModel.ToFieldMap`/`FromFieldMap<T>` reflection helpers |
| **Java** | Annotation | `@SerifyModel` class + `@SerifyField("key")` field annotations; `SerifyModelHelper.toFieldMap`/`fromFieldMap` reflection helpers |
| **C++** | Macro | `SERIFY_TO(Type, ...)` / `SERIFY_FROM(Type, ...)` blocks with `SERIFY_FIELD(name, kind)`; generates free functions `to_field_map`/`from_field_map` |
| **Elixir** | `use` macro | `use WorkerLib.Serify.Model` + `serify_field :name, :type, key: "key"`; compile-time `@serify_fields` → `to_field_map/1`/`from_field_map/1` |
| **PHP** | Attributes | `#[SerifyModel]` class + `#[SerifyField('key')]` property attributes; `SerifyModelHelper::toFieldMap`/`fromFieldMap` reflection helpers |

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

Generates `to_field_map()` and `from_field_map(fm)` class methods. Nested structs work via `@serify_model` on the nested type. `dict[str, X]` maps to `map<string,X>`.

### Node — `@Serify.Model` / `@Serify.field`

```typescript
@Serify.Model()
class User {
  @Serify.field() userId: number = 0;
  @Serify.field({ rename: 'email_addr' }) email: string = '';
}
// Serify.toFieldMap(user) / Serify.fromFieldMap(User, fm)
```

Requires `experimentalDecorators: true` in tsconfig. No `reflect-metadata` needed.

### C# — `[SerifyModel]` / `[SerifyField]`

```csharp
[SerifyModel]
public class User {
    [SerifyField("user_id")] public ulong UserId { get; set; }
    [SerifyField] public string Name { get; set; }
}
// SerifyModel.ToFieldMap(user) / SerifyModel.FromFieldMap<User>(fm)
```

### Java — `@SerifyModel` / `@SerifyField`

```java
@SerifyModel
public class User {
    @SerifyField("user_id") public long userId;
    @SerifyField public String name;
}
// SerifyModelHelper.toFieldMap(user) / SerifyModelHelper.fromFieldMap(fm, User.class)
```

### C++ — `SERIFY_TO` / `SERIFY_FROM`

```cpp
struct User { uint64_t user_id; std::string name; };

SERIFY_TO(User,
    SERIFY_FIELD(user_id, u64)
    SERIFY_FIELD(name, string)
)
SERIFY_FROM(User,
    SERIFY_FROM_FIELD(user_id, u64)
    SERIFY_FROM_FIELD(name, string)
)
// auto fm = to_field_map(user); auto u = from_field_map(fm);
```

For renamed fields: `SERIFY_FIELD_RENAMED(email, string, "email_addr")` / `SERIFY_FROM_FIELD_RENAMED(email, string, "email_addr")`.

### Elixir — `use WorkerLib.Serify.Model`

```elixir
defmodule MyApp.User do
  use WorkerLib.Serify.Model

  serify_field :user_id, :u64
  serify_field :name,    :string
  serify_field :email,   :string, key: "email_addr"
  serify_field :address, :struct, module: Address
  serify_field :scores,  :map, of: :u32
  serify_field :labels,  :map, of: {:struct, Label}
end
# User.to_field_map(user) / User.from_field_map(fm)
```

### PHP — `#[SerifyModel]` / `#[SerifyField]`

```php
use Serify\Attributes\SerifyModel;
use Serify\Attributes\SerifyField;

#[SerifyModel]
class User
{
    #[SerifyField('user_id')]
    public int $userId;

    #[SerifyField]
    public string $name;

    #[SerifyField('email_addr')]
    public string $email;
}
// SerifyModelHelper::toFieldMap($user) / SerifyModelHelper::fromFieldMap($fm, User::class)
```

### `sum` fields

A `sum` binds onto whatever sum type the language already has — a Rust `enum`,
a Java `sealed interface`, a Python union of dataclasses, a PHP property union
type, a C# `abstract record` hierarchy, an Elixir tagged tuple. Those six need
**nothing declared**: the binding reads the arms off the type itself. The three
that cannot be introspected declare the arms instead — C++ `SERIFY_SUM(T,
"a", "b")` (no reflection), Node `@Serify.sum([A, B])` (union types are erased
at runtime), Go a `serify.Converter` (implementations of an interface cannot be
enumerated).

All nine share one arity rule: **0 fields → unit variant, 1 field → that value is
the payload, N fields → the payload is a struct**; tags are the arm's type name
in snake_case. See "`sum` and your own types" in `docs/protocol.md` for the
full table, and `examples/*/notification.*` for a worked example per language.

### Registering a model with the suite

The binding above says how a type maps to a FieldMap. **Registration** says the
worker's serialize/deserialize functions speak that type, so the library
converts on either side and the FieldMap never reaches the worker. Every
library has it; each spells it the way its type system wants:

| Language | Model path | Model-less path |
|----------|-----------|-----------------|
| **Go** | `Type{Model: &T{}, Formats: …}` | `Model: nil` → `func(*FieldMap)` pairs |
| **Rust** | `Format::model::<M>().serializer(…).deserializer(…)` | `Format::new()` |
| **Python** | `Type(M, {"binary": Format(ser, deser)})` | `Type(None, …)`, or the plain format dict |
| **Node** | `type(M, {binary: {serialize, deserialize}})` | the plain format record |
| **C#** | `TypeEntry.Model<M>(…)` | `TypeEntry.Formats(…)` |
| **Java** | `TypeEntry.model(M.class, …)` | `TypeEntry.formats(…)` |
| **C++** | `model_format<M>(marshal, unmarshal)` | a plain `FormatPair` |
| **Elixir** | `%WorkerLib.Type{model: M, formats: …}` | `model: nil`, or the plain format map |
| **PHP** | `new Serify\Type(M::class, …)` | `new Type(null, …)`, or the plain format array |

The model-less path is not legacy: a type with no natural struct needs it, and
the audit fixtures are exactly that — they mutate a FieldMap on purpose.

Two things about *how* the two paths are told apart, because they decide
whether a language needs a unit test for its resolver:

- **Statically, in Go, Rust, C#, Java and C++** — separate factories, separate
  types, so a registration serify cannot resolve does not compile.
- **At run time, in Python, Node, Elixir and PHP** — an `isinstance`/shape
  check. Those four have unit tests for it (`lib/*/test*`), because an
  unresolved (type, format) is reported **SKIPPED**: a resolver that
  understands neither shape yields a *green* run made entirely of SKIPs, which
  reads exactly like a worker that honestly does not implement the type.
  Elixir is the sharpest case — a struct *is* a map, so `is_map(%Type{})` is
  true and the clause order in `resolve_registered/3` is load-bearing.

C++ has its own failure mode instead: `model_format` is a template, so an
overload nothing calls is never compiled. `lib/cpp/test/model_format_test.cpp`
exists first of all to name both of them.

## How a run works (the data flow)

- The `serify` CLI reads a **directory** of case files and auto-detects each worker's language from marker files in its directory. Case files are **YAML** (`.yaml`), loaded by `internal/config` (`loadTypeFile`): each file holds a `fields:` (or `variants:`) section, a `formats:` list, an optional `import:` list for reusable named types, and `cases:`. The type name is the filename (without `.yaml`). Case data is decoded **schema-directed** (`casedata.go`): 64/128-bit integer fields are parsed exactly from their literal text (bare or quoted, any size), and loading rejects unknown fields, bad enum variants, float forms in integer fields, and out-of-range values.
- It iterates **type × format**. For each `(type, format)` it re-inits every worker, then for each case runs: serialize on all workers → compare each non-reference worker's bytes against the reference's → deserialize the reference's bytes on every worker → compare decoded data. Optional: full N×N cross-deserialize matrix (`--full-matrix`).
- Global test id is **`type/format/case`** (`config.TestIDFmt`); it appears in the report and is sent as the request id.
- **Test data travels encoded, not raw** (`protocol.EncodeData` / `encodeValue`): 64/128-bit ints → decimal strings, `f32`/`f64` → IEEE-754 little-endian hex, `bytes` → hex. Workers decode this into a `FieldMap`, apply their own byte encoding, and return hex. Keep encode (runner) and decode (worker libs) in lockstep when adding types.

## Worker detection and configuration

**Language is always auto-detected** from marker files in the worker directory — it is a derived property, not a configuration value:

| Marker file | Language |
|-------------|----------|
| `go.mod` or `*.go` | `go` |
| `Cargo.toml` | `rust` |
| `pom.xml` | `java` |
| `mix.exs` | `elixir` |
| `*.csproj` | `csharp` |
| `package.json` | `node` |
| `*.py` | `python` |
| `*.cpp` | `cpp` |
| `composer.json` or `*.php` | `php` |

Each detected language has **default build/run commands** (defined in `internal/builder/defaults.go`). A `worker.yaml` is **optional** — if present, its `build:` and/or `run:` fields override the defaults. There is no `language:` field.

## Output is one data layer, two views (`internal/report`)

The terminal table is **not** a separate code path — it's a view of the same data the CSV holds. `Report.Records()` flattens results into a flat `[]Record` (columns `test_id,type,format,case,language,operation,status,detail`); that slice is the single source of truth. `WriteCSV`/`ReadCSV` dump/parse it, and `RenderTable` (lipgloss static table) renders it. So: inspecting the CSV verifies the run; the table is just a readable rendering of identical data. `serify run --csv <path>` writes the CSV; `serify table <path>` re-renders any saved CSV through the same `RenderTable`. The table groups rows by case (`type/case`) with formats adjacent; only serialize/deserialize rows form the grid (matrix rows stay in the CSV). Use lipgloss (not tablewriter — removed) for any new tables.

## Strictness rules

- **Case files** are YAML: one type per file, named after the file. Reusable-only types (e.g. `address`, `money`) have a `fields:` (or `variants:`) but no `cases:` and are pulled in by other files via `import:` — there is **no implicit registry**; every cross-file type reference must be imported explicitly.
- A **tested type** (one with `cases:`) **must declare `formats:`** — `config` rejects it otherwise.
- **A type file declares either a `fields:` (record) or a `variants:` (sum), never both** — the section name is the declaration, there is no flag, and `checkSections` enforces exactly one. `sum<...>` is a *wire* spelling only: `FieldType.String()` renders it for workers, and `typeOf` rejects it as input with a message pointing at `variants:`. Under `variants:` an entry with **no type** is a unit variant; the same untyped entry under `fields:` is a missing-type error. A sum referenced as a field *is* that field's type — no wrapping struct — so `transparent:` (which inlines a one-field record) is rejected on a sum. As the type under test the sum is carried under the single key `value` (`sumValueKey`), because a schema's top level is a field list and a bare sum is not one; the worker libraries' model bindings hardcode the same name. `checkKeys` rejects any top-level key outside the allowed set, so a stale `sum:`/`schema:` or a misspelled `feilds:` is a load error rather than a silently-empty type.
- **64/128-bit integer case values are decoded schema-directed from their literal text** (`internal/config/casedata.go`): bare literals of any size and quoted decimal strings are both exact; float forms (`1e3`) and out-of-range values are load-time errors. There is no quoting requirement.
- The **`bind` message requires a non-empty `type` AND `format`**. Empty values are errors on both the runner side (`internal/worker`) and inside each worker library — do not re-add single-type/single-format auto-selection.
- **Startup is `ping`, not a binding message.** The worker answers with `protocol_version` and the runner requires an exact match against `protocol.ProtocolVersion`; bump that constant on any breaking wire change. Workers report no standing capability list — whether a worker implements a type is answered per (type, format) by a `SKIPPED` bind. A hardcoded list is exactly what drifted before: the Rust lib spent an unknown stretch declaring it did not support `map`/`enum`/`sum` while passing every one of those cases.
- JSON byte-parity across languages is achievable but needs care: Go's `encoding/json` vs Rust's `serde_json` differ on `[]byte`→base64 and trailing `.0` on floats. See the hand-written `to_json`/`from_json` in `examples/rust/src/customer.rs` for how the Rust example matches Go exactly. Map-key ordering is no longer a hazard serify imposes: entry order is a property of the **format**, declared per (type, format) as `oracle: bytes` (canonical — the worker sorts explicitly) or `oracle: semantic` (free — the worker emits its own map's order). See `docs/protocol.md` § Maps: the format decides, not serify.

## Self-tests: the `wrong` meta-fixture (`test/cases/wrong`)

To verify that serify itself *reports* disagreements correctly (not just that honest workers agree), there is a deliberately-faulty `wrong` type — kept **out** of `examples/cases` so it never pollutes the real conformance suite:

- `test/cases/wrong/cases/wrong.yaml` — schema is four `bool` fault directives (`<format>_serialize` / `<format>_deserialize`) plus a `langs: list<string>` payload. Each case toggles the directives.
- `test/cases/wrong/<lang>/` — faulty workers, one per language, all nine. When a format's directive is off, the worker **drops its own language name** from `langs` (go drops `"go"`, php drops `"php"`), so its output diverges from the others; otherwise it round-trips all five fields faithfully and byte-identically. The directives must round-trip (they ride in the bytes) so the deserializer can read them back. The Rust worker is a **standalone crate** (its own empty `[workspace]`) so its `worker` binary does not collide with the example worker in the main Cargo workspace.
- `test/wrong_meta_test.go` (`TestWrongWorkerErrorsAreReported`) drives the **real `serify` CLI** (`serify run … --csv`), asserts it **exits non-zero**, then reads the exported CSV (`report.ReadCSV`) and checks it cell-by-cell against the outcome each case's flags predict.

## Audit mode: the `audit` meta-fixture (`test/cases/audit`)

The `--audit` CLI flag enables unsafe-behaviour detection inside workers. It sends `"audit": true` in the bind message; each worker library then runs additional checks and reports findings as warnings. Findings are **warnings, not failures** — they appear in the report/CSV with `StatusWarn` and do NOT cause a non-zero exit.

Six unsafe behaviours are detected:

| Detection | When | How |
|-----------|------|-----|
| **Serialize mutation** | Serializer mutates the input struct/FieldMap it was supposed to only read | Snapshot FieldMap before serialization, compare after |
| **Serialize instability** | Serializer produces different output on repeat calls | Call serializer twice, compare hex |
| **Output zero-copy** | Serializer's returned `[]byte` aliases model fields (dangerous if buffer is reused) | XOR-flip returned buffer after serialization, re-extract model, compare, flip back |
| **Deserialize zero-copy** | Deserialized fields alias the input buffer (dangerous if reused) | XOR-flip input buffer after deserialization, check which FieldMap entries change, restore |
| **Input-buffer mutation** | Deserializer modifies the raw input bytes | Snapshot buffer before deserialization, compare after |
| **Deserialize instability** | Deserializer produces different result on repeat calls | Re-deserialize from a fresh clone of the input, diff FieldMaps |

Key files:

| File | Purpose |
|------|---------|
| `lib/go/serify/auditcheck.go` | Go lib: `DetectZeroCopy`, `SnapshotFieldMap`, `CompareFieldMaps`, `DetectInputMutation`, `serializeAuditHolder` |
| `lib/go/serify/auditcheck_test.go` | Go library unit tests for audit detection functions |
| `lib/go/serify/run.go` | Go library: NDJSON loop audit logic (stability, mutation, zero-copy, input-mutation) |
| `lib/go/serify/suite.go` | Go lib: `buildSerializer` returns `*serializeAuditHolder` for mutation detection |
| `lib/rust/serify/src/lib.rs` | Rust lib: `detect_zero_copy`, `detect_output_zero_copy`, `field_map_diffs`, `json_field_diffs`; audit in serialize/deserialize handlers |
| `lib/python/serify.py` | Python lib: `_detect_zero_copy`, `_collect_bytes_snaps`, `_dict_diffs`; audit in serialize/deserialize handlers |
| `lib/node/src/workerlib.ts` | Node lib: `detectZeroCopy`, `collectByteSnaps`, `dictDiffs`; audit in serialize/deserialize handlers |
| `lib/csharp/Serify.cs` | C# lib: `DetectZeroCopy`, `CollectByteSnaps`, `DictDiffs`; audit in serialize/deserialize handlers |
| `lib/cpp/serify.hpp` | C++ lib: `detect_zero_copy_cpp`, `collect_byte_snaps`, `dict_diffs`; audit in serialize/deserialize handlers |
| `lib/java/src/main/java/io/serify/WorkerLib.java` | Java lib: `detectZeroCopy`, `collectByteSnaps`, `dictDiffs`; audit in serialize/deserialize handlers |
| `lib/elixir/lib/serify.ex` | Elixir lib: `detect_zero_copy`, `collect_bin_snaps`, `dict_diffs`; audit in serialize/deserialize handlers |
| `lib/php/src/Worker.php` | PHP lib: `detectZeroCopy`, `collectByteSnaps`, `dictDiffs`; audit in serialize/deserialize handlers |
| `internal/protocol/protocol.go` | `BindRequest.Audit`, `Response.Audit`, `AuditReport` type |
| `internal/worker/worker.go` | `Bind` passes audit flag to the worker |
| `internal/orchestrate/orchestrate.go` | `Options.Audit`; collects audit results into report |
| `internal/report/report.go` | `StatusWarn`, `OpAudit*` constants, `Warnings` slice |
| `cmd/serify/run.go` | `--audit` flag |
| `test/cases/audit/` | Meta-test fixture with deliberately-broken workers in all nine languages (clean, mutating, value-mutating, zero-copy, list-zero-copy, unstable, deser-unstable, input-mutating, output-zero-copy formats) |
| `test/audit_meta_test.go` | Integration test asserting the expected audit warnings per language |

**Audit support by language:**

| Language | Audit support |
|----------|---------------|
| Go | ✅ Full (mutation, stability, output-ZC, zero-copy, input-mutation, deser-stability) |
| Rust | ✅ Full (mutation via unsafe, stability, output-ZC, zero-copy, input-mutation, deser-stability) |
| Python | ✅ Full (mutation, stability, zero-copy, input-mutation, deser-stability; output-ZC only for mutable returns — bytearray/memoryview) |
| Node | ✅ Full (mutation, stability, output-ZC, zero-copy, input-mutation, deser-stability) |
| C# | ✅ Full (mutation, stability, output-ZC, zero-copy, input-mutation, deser-stability) |
| Java | ✅ Full (mutation, stability, output-ZC, zero-copy, input-mutation, deser-stability) |
| C++ | ✅ Full except output-ZC (FieldValue only holds owning containers — output aliasing is impossible) |
| Elixir | ✅ Full except output-ZC (BEAM binaries are immutable — output aliasing is impossible) |
| PHP | ✅ Full except output-ZC (strings are copy-on-write values — output aliasing is impossible) |

Each language wires the same detections into its NDJSON loop. The XOR-flip active overwrite for zero-copy/output-ZC detection, the double-serialize/deserialize for stability, and the before/after comparison for mutation work identically across languages — except output-ZC, which only applies where the language's memory model lets a model field mutably alias the output buffer.

**This is a published 0.x API.** The nine libraries are on their registries, so a breaking change is now a version bump rather than a free edit — but 0.x is exactly the promise that breaking changes are still allowed. Design decisions should stay clean and appropriate, not burdened by backward-compatibility concerns; compatibility shims still have no upside. What changed is only that a break must ride a release, not that it must be avoided.

## Commands

Go module is `github.com/chengxilo/serify` (Go 1.25). Rust is a Cargo workspace (`Cargo.toml`); `lib/` holds the per-language libraries (formerly `sdks/`), with the Go library at `lib/go/serify` (Thrift-style: import path ends in the package name).

**Releases:** pushing a `v*` tag ships all ten artifacts, and `.github/workflows/release.yml` is the whole procedure — there is no release doc, deliberately; its header comment carries the external setup (registry tokens, PyPI trusted publisher, Maven namespace) that cannot be re-derived from the repo. goreleaser (`.goreleaser.yaml`) builds the CLI, then one job per registry publishes the libraries. Two things are easy to get wrong from inside the code: the version lives in eight manifests that nothing keeps in sync (the CLI is not one of them — it reads its version from the tag), and `serify-derive` must reach crates.io before `serify`, which names it by version. `scripts/check-versions.sh` enforces the first and is the release's gate: it fails the run before any upload, which is all that stands between a forgotten bump and five registries carrying one version while the sixth carries another, unfixably.

Also Thrift-style, the npm and Composer **publish** manifests live at the repo root — `package.json` and `composer.json` are the source of truth for entry points and autoload paths; don't restate their contents elsewhere. `npm test` runs the node library tests; `prepack` (not `prepare` — installs must stay side-effect-free so fresh clones work) builds `lib/node/dist` at publish time. In-repo node workers link the library via `file:` deps on `lib/node` (which keeps a minimal `private: true` manifest as the link target) and build it themselves via `npx tsc -p node_modules/@chengxilo/serify` in their build commands. PHP workers `require_once` `lib/php/src/*.php` directly.

The CLI is `./cmd/serify`; the importable Go library (package `serify`) lives at `lib/go/serify`. The repo root has no Go package — build/run/test the CLI and library by their paths, not `.`.

```bash
# Build the CLI
go build -o serify ./cmd/serify    # building the whole tree: go build ./...

# Run conformance tests (--ref overrides the suite's reference_language; --cases defaults to "cases")
go run ./cmd/serify run --ref rust --cases examples/cases examples/go examples/rust
#   --no-build       skip building workers (workers must already be built)
#   --startup-timeout  seconds for a worker's start-up ping (default 60). Separate
#                      from --timeout because a cold `dotnet run` / JVM / BEAM start
#                      is a runtime cost, not a slow response — sharing one number
#                      meant either false-failing csharp or blunting hang detection.
#   --audit          enable unsafe-behaviour auditing (mutation, zero-copy, stability)
#   --full-matrix    N×N cross-language deserialize matrix
#   --output table|json|junit
#   --csv <path>     also dump the full results as CSV (canonical data)
#   --known-failures <dir>  per-language <lang>.yaml of expected failures; a listed
#                      FAIL becomes XFAIL (exit 0), an unexpected pass becomes XPASS
#   --expect-skips <dir>  per-language <lang>.yaml declaring the coverage a worker is
#                      allowed to skip. Any *other* SKIP fails the run — this is the
#                      only thing standing between a worker that honestly declines a
#                      type and one that quietly declines everything.

# Re-render a saved results CSV as the same table (offline, no workers)
go run ./cmd/serify table out.csv

# Regenerate the checked-in .schemas/*.schema.json for every case dir in the repo
scripts/gen-schemas.sh
scripts/gen-schemas.sh --check   # what CI runs: fails if the schemas are stale

# Tests
go test ./lib/go/serify                                 # Go library (package serify) tests
go test ./internal/...                                  # runner unit tests
go test ./test/...                                      # integration + CLI + meta-tests (builds go+rust workers)

# Rust workspace (lib + examples). A harmless rlib name-collision warning is expected.
cargo build --release
```

Seven of the nine libraries have unit tests of their own, one CI job each
(`test-go`, `test-rust`, `test-python`, `test-node`, `test-cpp`, `test-elixir`,
`test-php`). Each runs with the tooling that language already has, and none
added a dependency:

```bash
(cd lib/rust && cargo test)
(cd lib/python && pytest -q)          # plus: mypy --strict lib/python/serify.py
npm test                              # node, from the repo root
(cd lib/elixir && mix deps.get && mix test)
php lib/php/test/worker_test.php      # no PHPUnit, no composer; exits non-zero on failure
g++ -O2 -std=c++17 -Ilib/cpp -o /tmp/t lib/cpp/test/model_format_test.cpp && /tmp/t
```

C# and Java have none: their libraries' testable surface is checked by the
compiler (see "Registering a model with the suite"), and everything else about
them is covered by the conformance and meta-test runs.

Notes:
- The `.schemas/` JSON Schemas are generated from the `fields:`/`variants:` sections of the case files and are **checked in**. Enable the hook that keeps them in sync once per clone: `git config core.hooksPath .githooks`. It regenerates and stages them on any commit that touches a case file or the generator.
- `serify run` **builds workers by default**, so it needs the relevant toolchains (go, cargo, etc.) on PATH; pass `--no-build` to skip.
- On Windows the builder rewrites manifest commands (`worker`→`worker.exe`, `python`/`python3`→`py`); keep `worker.yaml` commands in the unix form.
- The Go tree is `gofmt`-clean and CI enforces it (`gofmt -l .` must be empty). The example workers in the other languages are hand-aligned on purpose and are deliberately outside any formatter.
