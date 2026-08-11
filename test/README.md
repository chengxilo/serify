# tests

This directory contains integration and meta-tests for the `serify` CLI.
Each test builds the real `serify` binary and drives fixture workers through
it end-to-end.

## Structure

```
test/
  main_test.go       — TestMain: builds all fixture workers
  helpers_test.go    — shared grid helpers (resultGrid, readResultGrid, assertCell)
  cli_test.go        — basic CLI integration (run, csv+table, ref-required, junit)
  run_flags_test.go  — flag coverage (full-matrix, json output, build-only, flag validation,
                       malformed cases, bad run command)
  faults_test.go     — error/timeout/crash/known-failures coverage
  wrong_meta_test.go — byte-diff detection via the `wrong` fixture
  audit_meta_test.go — audit warning detection via the `audit` fixture
  cases/             — fixture worker directories
```

## Case directories

| Directory | Purpose |
|-----------|---------|
| `cases/happy/` | Correct workers for the happy path (all_types, point, tag) |
| `cases/wrong/` | Deliberately-faulty workers (see below) |
| `cases/invalid_schema/` | Type with no `formats:` — exercises config validation |
| `cases/audit/` | Audit meta-fixture (clean, mutating, zero-copy, unstable formats) |

All four fixtures carry a worker in **every one of the nine languages**, and
`requireWorkers` treats a missing toolchain as a failure rather than a skip — so
these tests need the full toolchain set, not just go and cargo.

## Fault formats (wrong worker)

The `wrong` worker registers four formats beyond `binary` and `json`:

| Format | Behaviour |
|--------|-----------|
| `err_ser` | Serializer returns `errors.New("injected serialize error")` |
| `err_deser` | Deserializer returns `errors.New("injected deserialize error")` |
| `hang` | Serializer sleeps 3 s then marshals normally |
| `crash` | Serializer calls `os.Exit(3)` / `std::process::exit(3)` |

Corresponding case directories (`cases_error/`, `cases_hang/`, `cases_crash/`)
each contain a `wrong.yaml` that uses only those formats. They live outside the
main `cases/` dir because `config.LoadSuite` loads every yaml in the `--cases`
directory, and mixing fault formats into a normal run would cause spurious errors.

`known_failures/` holds the checked-in `--known-failures` fixture for the
`cases_error` suite (one `<lang>.yaml` per worker). `TestCLI_KnownFailures`
runs the suite with and without it, asserting FAIL→XFAIL, exit-code 0, XPASS
for the op that succeeds despite its entry, and that SKIPs are untouched.
It is not listed in `scripts/gen-schemas.sh` because it is not a case suite.

## Toolchain gating

Tests that start workers call `requireWorkers(t, case)` which fails the test if
the required language toolchain isn't available (e.g. `cargo` not on PATH).

## Multi-language tests

`examples/test/` drives all example workers (`examples/<lang>/`) through the
shared conformance suite; `examples/test/example_test.go` documents itself.
Multi-language builds may need network (first `npm install` / `mix deps.get` /
`composer install`). serify always invokes the build command and lets each
language's build tool decide what to recompile, so repeat runs are cheap without
serify caching anything itself.

## Running

```bash
# Run integration tests (needs every one of the nine toolchains on PATH)
go test ./test/... -v

# Run multi-language conformance (example workers — needs all toolchains)
go test ./examples/test/... -v
```
