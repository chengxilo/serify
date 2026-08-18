# serify — C# worker library

C# / .NET worker library for [serify](https://github.com/chengxilo/serify), a
cross-language serialization conformance harness.

You define a schema and test cases once; each target language ships a small
**worker** program that serializes and deserializes the data, and the `serify`
CLI drives every worker through the same cases and compares their output
**byte-for-byte**. This package is what a C# worker references.

## Install

```bash
dotnet add package Serify
```

## Usage

`[SerifyModel]` plus one `[SerifyField]` per property is the binding. You supply
the byte layout and register it per format:

```csharp
using System.Collections.Generic;
using Serify;

[SerifyModel]
internal sealed class UserRecord
{
    [SerifyField("user_id")] public ulong UserId { get; set; }
    [SerifyField] public string Username { get; set; } = "";
    [SerifyField] public float Score { get; set; }

    public byte[] Marshal() { /* your byte layout */ }
    public static UserRecord Unmarshal(byte[] data) { /* its inverse */ }
}

internal static class Program
{
    private static void Main()
    {
        Serify.Worker.RunSuite(new Dictionary<string, TypeEntry>
        {
            ["user"] = TypeEntry.Model<UserRecord>(new()
            {
                ["binary"] = (u => u.Marshal(), UserRecord.Unmarshal),
            }),
        });
    }
}
```

`[SerifyField("key")]` renames a property's schema key; a bare `[SerifyField]`
uses the property name. Nested classes work by attributing the nested type.

`RunSuite` takes over stdin/stdout to speak the NDJSON protocol the CLI drives,
so a worker's own output must go to stderr.

## Documentation

Full documentation, the protocol spec, and reference workers for all nine
languages live in the [main repository](https://github.com/chengxilo/serify).

## License

Apache-2.0
