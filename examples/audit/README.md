# audit — what conformance cannot see

Three codecs. One byte layout. Every one of them produces identical bytes for
every case, so a conformance run says they are the same and passes all three.

Two of them are not the same. One aliases the buffer it decodes from; one
empties the caller's data as a side effect of serializing it. Neither shows up
in the bytes, because neither is *about* the bytes.

`serify run --audit` is how you find them.

## The two runs

```console
$ serify run --cases examples/audit/cases examples/audit/go examples/audit/rust
```

```text
│ case id          │ format  │ operation   │  go  │ rust │
├──────────────────┼─────────┼─────────────┼──────┼──────┤
│ frame/typical    │ safe    │ serialize   │ PASS │ PASS │
│                  │         │ deserialize │ PASS │ PASS │
│                  │ fast    │ serialize   │ PASS │ PASS │
│                  │         │ deserialize │ PASS │ PASS │
│                  │ handoff │ serialize   │ PASS │ SKIP │
│                  │         │ deserialize │ PASS │ SKIP │
│ …                                                      │
╰──────────────────┴─────────┴─────────────┴──────┴──────╯
FAIL: 0  XFAIL: 0  XPASS: 0  SKIPPED: 8  WARN: 0  PASSED: 40  (exit code 0)
```

Now the same suite, the same workers, the same bytes — with one flag added:

```console
$ serify run --audit --cases examples/audit/cases examples/audit/go examples/audit/rust
```

```text
WARNINGS:
------------------------------------------------------------
[frame/fast/typical / rust / audit-zero-copy]
  zero-copy fields: payload, tags, title

[frame/fast/typical / go / audit-zero-copy]
  zero-copy fields: title, tags, payload

[frame/fast/no_payload / go / audit-zero-copy]
  zero-copy fields: title, tags

[frame/handoff/typical / go / audit-mutation]
  mutated fields: payload

[frame/handoff/typical / go / audit-stability]
  serializer produced different output on repeat call

FAIL: 0  XFAIL: 0  XPASS: 0  SKIPPED: 8  WARN: 10  PASSED: 40  (exit code 0)
```

Same forty passes. Ten findings. **Exit code 0 either way** — a warning is a
warning, and audit never fails your build for you.

## The two findings are not the same kind of thing

### `fast` is not a bug

`unmarshalFast` points the title, the tags and the payload straight at the input
buffer rather than copying them out. That is a real optimization — the decoder
allocates nothing at all for the frame's contents — and on a hot path it is the
obvious thing to do.

What it also does is take on a constraint that nothing in the format can
express: **the decoded frame is only valid while the input buffer is.** Reuse
the read buffer, hand it back to a pool, or decode the next frame over it, and
frames you already decoded change underneath you. The bug will not appear here.
It will appear in the caller, later, under load.

Audit is how you find out you made that trade. Whether to keep it is your call —
which is exactly why this is a warning and not a failure.

### `handoff` is a bug

`Frame.MarshalHandoff` writes the frame and then scrubs the payload buffer it
was handed, on the reasoning that the bytes are out and the buffer can go back
to the pool clean. It is an easy line to write: the serializer is finished with
the payload, wiping it before release looks like hygiene, and every test that
checks only the output bytes passes.

What it actually does is empty the caller's frame as a side effect of being
asked to read it.

It reports **two** findings, and the second explains the first:

```text
[frame/handoff/typical / go / audit-mutation]
  mutated fields: payload
[frame/handoff/typical / go / audit-stability]
  serializer produced different output on repeat call
```

serify serializes twice to check stability, and the second call reads a payload
the first call already wiped. A serializer that mutates its input cannot be a
stable one. That is not two bugs — it is one bug seen from both ends, and the
instability is the reason the mutation matters.

Scrubbing the buffer rather than reassigning `f.Payload = nil` is deliberate.
Both are mutations and audit reports either, but only the scrub also shows up as
instability: serify rebuilds the model from the same FieldMap for the repeat
call, so a reassigned field is restored while scribbled-on bytes — which the
model and the FieldMap share — are not. Releasing a pooled buffer looks like
this in real code, and it is the version that shows both halves of the bug.

## Rust declines `handoff`, and that is the interesting part

The `SKIP` column above is not an omission. A serify serializer in Rust receives
a shared reference — `&FieldMap` here — with no way to write through it. Writing
this bug takes an `unsafe` cast from `&` to `&mut` plus an
`#[allow(invalid_reference_casting)]` to quiet the compiler. An honest worker
will not do that, so this one declines the format and serify reports SKIPPED: a
declaration about coverage, not a failure.

Go hands the serializer a `*Frame`, so the same bug is one line the compiler
accepts without comment.

Neither language stops you from aliasing the input buffer — both do it under
`fast`, and both get caught. Only one of them lets you scribble on your caller by
accident.

## The findings depend on the data, not only on the code

Look at which cases are quiet:

| case | `fast` | `handoff` |
|------|--------|-----------|
| `typical` — title, tags, a payload | zero-copy | mutation + instability |
| `unicode` — same, non-ASCII | zero-copy | mutation + instability |
| `no_payload` — strings, empty blob | zero-copy (title, tags) | **silent** |
| `empty` — nothing at all | **silent** | **silent** |

The codecs are byte-for-byte identical across all four rows. There is simply
nothing to alias in an empty string and nothing to release in an empty buffer,
so the unsafe code runs and reports nothing.

This is the argument for running `--audit` over a real case list rather than one
hand-picked frame. Had `examples/audit/cases/frame.yaml` carried only the
`empty` case, this whole example would be green and say nothing at all.

## What the tests cover

```bash
go test ./examples/audit/test/
```

`TestAudit_ConformanceIsBlindToBoth` runs the suite **without** `--audit` and
requires every cell to pass and every audit row to be absent. It is asserting
the premise: if a codec ever drifts so that the formats stop producing identical
bytes, this example has quietly stopped making its point, and a green
conformance table would hide that.

`TestAudit_FindsWhatConformanceCannot` runs it **with** the flag and pins each
finding to the exact case and language it belongs to — including the silences,
so "audit flags this format" cannot creep in where the truth is "audit flags
this format on this data".

## The files

```text
audit/
  cases/
    _config.yaml   go leads; three formats
    frame.yaml     one type, four cases, one layout
  go/
    wire.go        the layout, shared by all three formats
    codec.go       the Frame model and the three codecs — the subject
    main.go        registration
  rust/
    src/main.rs    the same model, safe and fast; handoff deliberately absent
  test/
```

The Go worker registers a **model** — an ordinary `Frame` struct with ordinary
methods, which is what a worker author actually writes. Audit is not weakened by
that: serify snapshots the struct around the serializer and extracts a FieldMap
that shares its backing memory, so aliasing and mutation are caught through the
model exactly as they are at the raw boundary. `test/cases/audit/go` proves the
same for all nine detections.

The Rust worker registers a model too. It did not always: `ModelFormat` used to
wrap a model format as a pure conversion, so `to_field_map(&self)` cloned the
`String` and `Vec<u8>` fields and the alias was gone before `detect_zero_copy`
flipped the buffer — this worker's `fast` findings simply vanished. It now keeps
the model instance each call used and re-derives its state at every probe point,
the way Go's `buildSerializer` does, and the findings come back. See
[AGENTS.md](../../AGENTS.md).

## Related

- The six detections and which languages implement each: the **Audit mode**
  section of the [root README](../../README.md).
- The deliberately-broken workers that test serify's own detection, in all nine
  languages: `test/cases/audit/`. They are kept out of `examples/` on purpose —
  they exist to be wrong. This example exists to be plausible.
