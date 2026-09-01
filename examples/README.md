# Serify Worker Examples

This directory holds three different kinds of example, and it is worth knowing
which one you are reading.

**Every type** — [`appdata/`](appdata/): a `cases/` directory plus one worker
directory per language (`appdata/go/`, `appdata/rust/`, …). These are the data
shapes an application already owns — orders, customers, money, telemetry — and
it is the suite the libraries themselves are validated against: between them the
cases exercise every corner of the schema language, every scalar width, `list`,
`array`, `map`, `optional`, `enum`, `sum`, nested structs and the boundary
values of each. Read it to find out how a feature is expressed in your language.
The rest of this file describes it.

**A project** — [`taskstore/`](taskstore/). A small CRUD server over TCP: Go
writes the server, the other eight languages write clients, and serify checks
that every client agrees with it byte for byte. One plausible feature, the
smallest schema that expresses it, and a program you can run. Read it to find
out what using serify on real work looks like — and for the cross-language
notes this suite has no reason to make, such as which bindings can be kept out
of your own codec and what a `sum` costs in each language.

**One idea** — [`audit/`](audit/). Three codecs over one byte layout, two of
them unsafe in ways the bytes cannot show: one aliases the buffer it decodes
from, one empties the caller's data while serializing it. A conformance run
passes all three; `serify run --audit` finds both. Read it to see what the
audit flag is actually for, or if you are about to make a codec faster.

## Layout

Every worker is split the same way, so that the part you would actually write
for your own project is easy to tell apart from the part serify needs:

| File                          | What it is                                                                                                                                    |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `worker.*` (`main.go`)        | The worker itself. It names the types it can handle and hands serify one serializer/deserializer pair per format. Nothing else.                  |
| one file per model (`ledger.*`, `signals.*`, …) | Stand-ins for the types an application already owns: the struct, its schema binding (a derive, tags, attributes or macros), and its byte layout. |
| `wire.*`                      | Byte-level primitives shared by those models — length prefixes and the like.                                                                    |

Every worker carries every model, with one exception: `telemetry` is absent from
elixir, because the BEAM has no NaN and no infinity and telemetry's float cases
carry both. A worker is only obliged to implement what it claims, so that gap is
declared rather than hidden — see below.

Go is the `--ref` language and owns the byte layout every other worker has to
reproduce; the conventions are documented at the top of
[`appdata/go/wire.go`](appdata/go/wire.go).

A type a worker does not register is reported to the runner as SKIPPED rather
than silently passing, so a partial worker is honest about what it covers. A SKIP
does not change the exit code, though, so the one remaining gap is declared in
[`appdata/cases/expected_skips/`](appdata/cases/expected_skips/): any skip that
is *not* declared there fails the run under `--expect-skips`.
