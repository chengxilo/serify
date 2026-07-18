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
| `customer.*`, `ledger.*`, `notification.*` | Stand-ins for the types an application already owns: the struct, its schema binding (a derive, tags, attributes or macros), and its byte layout. |
| `wire.*`                      | Byte-level primitives shared by those models — length prefixes and the like.                                                                    |

Go is the `--ref` language and owns the byte layout every other worker has to
reproduce; the conventions are documented at the top of
[`go/wire.go`](go/wire.go).

A type a worker does not register is reported to the runner as SKIPPED rather
than silently passing, so a partial worker is honest about what it covers.
