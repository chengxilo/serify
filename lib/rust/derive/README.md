# serify-derive

`#[derive(SerifyModel)]` for the [`serify`](https://crates.io/crates/serify)
crate — the Rust worker library of
[serify](https://github.com/chengxilo/serify), a cross-language serialization
conformance harness.

You do not depend on this crate directly. Add `serify` instead; it re-exports
the macro:

```toml
[dependencies]
serify = "0.1"
```

## License

Apache-2.0
