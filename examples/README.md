# Serify Worker Examples

This directory contains example workers for the Serify conformance test
framework. [`appdata/`](appdata/) holds a `cases/` directory plus one worker
directory per language (`appdata/go/`, `appdata/rust/`, …); between them the
cases exercise every corner of the schema language.

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
