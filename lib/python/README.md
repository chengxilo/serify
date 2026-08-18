# serify — Python worker library

Python worker library for [serify](https://github.com/chengxilo/serify), a
cross-language serialization conformance harness.

You define a schema and test cases once; each target language ships a small
**worker** program that serializes and deserializes the data, and the `serify`
CLI drives every worker through the same cases and compares their output
**byte-for-byte**. This package is what a Python worker imports.

## Install

```bash
pip install serify
```

## Usage

`@serify_model` reads the dataclass annotations and generates the field-map
conversion. You supply the byte layout and register it per format:

```python
from dataclasses import dataclass, field

from serify import Format, Type, run_suite, serify_model


@serify_model
@dataclass
class UserRecord:
    user_id: int
    username: str
    score: float
    email: str = field(default="", metadata={"serify": "email_addr"})

    def marshal(self) -> bytes:
        ...  # your byte layout

    @classmethod
    def unmarshal(cls, data: bytes) -> "UserRecord":
        ...  # its inverse


if __name__ == "__main__":
    run_suite({
        "user": Type(UserRecord, {
            "binary": Format(UserRecord.marshal, UserRecord.unmarshal),
        }),
    })
```

Field names map to schema keys as written; `metadata={"serify": "key"}` renames
one. Nested structs work by decorating the nested dataclass, and `dict[str, X]`
maps to `map<string,X>`.

`run_suite` takes over stdin/stdout to speak the NDJSON protocol the CLI drives,
so a worker's own output must go to stderr.

## Documentation

Full documentation, the protocol spec, and reference workers for all nine
languages live in the [main repository](https://github.com/chengxilo/serify).

## License

Apache-2.0
