# serify — Rust worker library

Rust worker library for [serify](https://github.com/chengxilo/serify), a
cross-language serialization conformance harness.

You define a schema and test cases once; each target language ships a small
**worker** program that serializes and deserializes the data, and the `serify`
CLI drives every worker through the same cases and compares their output
**byte-for-byte**. This crate is what a Rust worker depends on.

## Install

```toml
[dependencies]
serify = "0.1"
```

## Usage

`#[derive(SerifyModel)]` generates the field-map conversion from the struct
definition. You supply the byte layout and register it per format:

```rust
use serify::{run_suite, Format, SerifyModel, Suite, Type};

#[derive(SerifyModel)]
struct UserRecord {
    user_id: u64,
    username: String,
    #[serify(rename = "email_addr")]
    email: String,
}

// marshal/unmarshal are your byte layout.

fn main() {
    run_suite(Suite::new().with_type(
        "user",
        Type::new().with_format(
            "binary",
            Format::model::<UserRecord>()
                .serializer(UserRecord::marshal)
                .deserializer(UserRecord::unmarshal),
        ),
    ));
}
```

Field names map to schema keys as written; `#[serify(rename = "key")]` renames
one and `#[serify(rename_all = "snake_case")]` renames the lot. Nested structs
work by deriving on the nested type.

The `worker` feature (on by default) pulls in the NDJSON runtime. Turn it off
with `default-features = false` to use the model-binding types alone.

`run_suite` takes over stdin/stdout to speak the NDJSON protocol the CLI
drives, so a worker's own output must go to stderr.

## Documentation

Full documentation, the protocol spec, and reference workers for all nine
languages live in the [main repository](https://github.com/chengxilo/serify).

## License

Apache-2.0
