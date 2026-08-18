# serify — Elixir worker library

Elixir worker library for [serify](https://github.com/chengxilo/serify), a
cross-language serialization conformance harness.

You define a schema and test cases once; each target language ships a small
**worker** program that serializes and deserializes the data, and the `serify`
CLI drives every worker through the same cases and compares their output
**byte-for-byte**. This package is what an Elixir worker depends on.

Requires OTP 27 or newer.

## Install

```elixir
def deps do
  [{:serify, "~> 0.1"}]
end
```

## Usage

`use WorkerLib.Serify.Model` plus one `serify_field` per field is the binding.
You supply the byte layout and register it per format:

```elixir
defmodule UserRecord do
  use WorkerLib.Serify.Model

  defstruct [:user_id, :username, :score]

  serify_field(:user_id, :u64)
  serify_field(:username, :string)
  serify_field(:score, :f32, key: "score_pct")

  # marshal/1 and unmarshal/1 are your byte layout — Elixir's bitstring syntax
  # makes one whole wire format read as a single <<...>> literal.
end

defmodule Worker do
  def main(_args) do
    WorkerLib.run_suite(%{
      "user" => %WorkerLib.Type{
        model: UserRecord,
        formats: %{"binary" => {&UserRecord.marshal/1, &UserRecord.unmarshal/1}}
      }
    })
  end
end
```

`key:` renames a field's schema key. The macro generates `to_field_map/1` and
`from_field_map/1` at compile time.

`run_suite` takes over stdin/stdout to speak the NDJSON protocol the CLI drives,
so a worker's own output must go to stderr.

## Documentation

Full documentation, the protocol spec, and reference workers for all nine
languages live in the [main repository](https://github.com/chengxilo/serify).

## License

Apache-2.0
