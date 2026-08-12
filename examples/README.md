# Serify Worker Examples

This directory contains example workers for the Serify conformance test
framework. Each subdirectory contains a worker implementation in a different
programming language.

## Layout

Every worker is split the same way, so that the part you would actually write
for your own project is easy to tell apart from the part serify needs:

| File                          | What it is                                                                                                                                    |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `worker.*` (`main.go`)        | The worker itself. It names the types it can handle and hands serify one serializer/deserializer pair per format. Nothing else.                  |
| one file per model (`ledger.*`, `signals.*`, …) | Stand-ins for the types an application already owns: the struct, its schema binding (a derive, tags, attributes or macros), and its byte layout. |
| `wire.*`                      | Byte-level primitives shared by those models — length prefixes and the like.                                                                    |

Which models a worker carries varies, because a worker is only obliged to
implement what it claims. `ledger`, `notification` and `signals` are in all nine;
`telemetry` in all but elixir; `customer` in all nine; `order` in go alone.

Go is the `--ref` language and owns the byte layout every other worker has to
reproduce; the conventions are documented at the top of
[`go/wire.go`](go/wire.go).

A type a worker does not register is reported to the runner as SKIPPED rather
than silently passing, so a partial worker is honest about what it covers. A SKIP
does not change the exit code, though, so the gaps above are declared in
[`cases/expected_skips/`](cases/expected_skips/): any skip that is *not* declared
there fails the run under `--expect-skips`.
