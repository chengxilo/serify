// Copyright 2026 Chengxi Luo
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

using System;
using System.Buffers.Binary;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.Json;

namespace Serify;

// FieldMap

public sealed class FieldMap
{
    private readonly Dictionary<string, object?> _fields = new();

    public byte     GetU8(string key)   => (byte)_fields[key]!;
    public ushort   GetU16(string key)  => (ushort)_fields[key]!;
    public uint     GetU32(string key)  => (uint)_fields[key]!;
    public ulong    GetU64(string key)  => (ulong)_fields[key]!;
    public UInt128  GetU128(string key) => (UInt128)_fields[key]!;
    public sbyte    GetI8(string key)   => (sbyte)_fields[key]!;
    public short    GetI16(string key)  => (short)_fields[key]!;
    public int      GetI32(string key)  => (int)_fields[key]!;
    public long     GetI64(string key)  => (long)_fields[key]!;
    public Int128   GetI128(string key) => (Int128)_fields[key]!;
    public float    GetF32(string key)  => (float)_fields[key]!;
    public double   GetF64(string key)  => (double)_fields[key]!;
    public bool     GetBool(string key) => (bool)_fields[key]!;
    public string   GetString(string key) => (string)_fields[key]!;
    public byte[]   GetBytes(string key)  => (byte[])_fields[key]!;
    public string[] GetListString(string key) => (string[])_fields[key]!;
    public byte[]   GetListU8(string key) => (byte[])_fields[key]!;
    public int[]    GetListI32(string key) => (int[])_fields[key]!;
    public long[]   GetListI64(string key) => (long[])_fields[key]!;
    public UInt128[] GetListU128(string key) => (UInt128[])_fields[key]!;
    public Int128[]  GetListI128(string key) => (Int128[])_fields[key]!;
    public bool[]   GetListBool(string key) => (bool[])_fields[key]!;
    public string?  GetOptionalString(string key) => _fields.TryGetValue(key, out var v) ? (string?)v : null;
    public FieldMap GetStruct(string key) => (FieldMap)_fields[key]!;
    public FieldMap? GetOptionalStruct(string key) => _fields.TryGetValue(key, out var v) ? (FieldMap?)v : null;
    public FieldMap[] GetListStruct(string key) => (FieldMap[])_fields[key]!;
    public uint[]   GetListU32(string key) => (uint[])_fields[key]!;
    public ulong[]  GetListU64(string key) => (ulong[])_fields[key]!;
    public float[]  GetListF32(string key) => (float[])_fields[key]!;
    public double[] GetListF64(string key) => (double[])_fields[key]!;
    public ushort[] GetListU16(string key) => (ushort[])_fields[key]!;
    public sbyte[]  GetListI8(string key) => (sbyte[])_fields[key]!;
    public short[]  GetListI16(string key) => (short[])_fields[key]!;
    public byte[][] GetListBytes(string key) => (byte[][])_fields[key]!;
    public Dictionary<string, object?> GetMap(string key) =>
        _fields.TryGetValue(key, out var v) ? (Dictionary<string, object?>?)v ?? new() : new();

    public void SetU8(string key, byte v)     => _fields[key] = v;
    public void SetU16(string key, ushort v)  => _fields[key] = v;
    public void SetU32(string key, uint v)    => _fields[key] = v;
    public void SetU64(string key, ulong v)   => _fields[key] = v;
    // 128-bit values use .NET's native UInt128/Int128. Parsing them as ulong/long
    // (as this library used to) silently truncates everything above 2^64.
    public void SetU128(string key, UInt128 v) => _fields[key] = v;
    public void SetI8(string key, sbyte v)    => _fields[key] = v;
    public void SetI16(string key, short v)   => _fields[key] = v;
    public void SetI32(string key, int v)     => _fields[key] = v;
    public void SetI64(string key, long v)    => _fields[key] = v;
    public void SetI128(string key, Int128 v) => _fields[key] = v;
    public void SetF32(string key, float v)   => _fields[key] = v;
    public void SetF64(string key, double v)  => _fields[key] = v;
    public void SetBool(string key, bool v)   => _fields[key] = v;
    public void SetString(string key, string v) => _fields[key] = v;
    public void SetBytes(string key, byte[] v)  => _fields[key] = v;
    public void SetListString(string key, string[] v) => _fields[key] = v;
    public void SetOptionalString(string key, string? v) => _fields[key] = v;
    public void SetStruct(string key, FieldMap v) => _fields[key] = v;
    public void SetOptionalStruct(string key, FieldMap? v) => _fields[key] = v;
    public void SetListStruct(string key, FieldMap[] v) => _fields[key] = v;
    public void SetListU32(string key, uint[] v)   => _fields[key] = v;
    public void SetListU64(string key, ulong[] v)  => _fields[key] = v;
    public void SetListU8(string key, byte[] v)    => _fields[key] = v;
    public void SetListI32(string key, int[] v)    => _fields[key] = v;
    public void SetListI64(string key, long[] v)   => _fields[key] = v;
    public void SetListU128(string key, UInt128[] v) => _fields[key] = v;
    public void SetListI128(string key, Int128[] v)  => _fields[key] = v;
    public void SetListF32(string key, float[] v)  => _fields[key] = v;
    public void SetListF64(string key, double[] v) => _fields[key] = v;
    public void SetListBool(string key, bool[] v)  => _fields[key] = v;
    public void SetListU16(string key, ushort[] v) => _fields[key] = v;
    public void SetListI8(string key, sbyte[] v)   => _fields[key] = v;
    public void SetListI16(string key, short[] v)  => _fields[key] = v;
    public void SetListBytes(string key, byte[][] v) => _fields[key] = v;
    public void SetMap(string key, Dictionary<string, object?> v) => _fields[key] = v;

    /// <summary>
    /// Stores a sum value: the active variant's tag and payload (null for a unit variant).
    /// </summary>
    public void SetVariant(string key, string tag, object? value = null) => _fields[key] = new Variant(tag, value);

    /// <summary>
    /// Returns the sum value stored at key.
    /// </summary>
    public Variant GetVariant(string key)
    {
        if (_fields.TryGetValue(key, out var v) && v is Variant variant) return variant;
        throw new InvalidOperationException($"field \"{key}\" is not a variant");
    }

    internal Dictionary<string, object?> Fields => _fields;
}

/// <summary>
/// One arm of a sum: a tag and its decoded payload (null for a unit variant).
/// A sum field stores a Variant.
/// </summary>
public sealed record Variant(string Tag, object? Value = null);

// Schema

public sealed class SchemaField
{
    public string Name   { get; }
    public string Type   { get; }
    public IReadOnlyList<SchemaField> Fields { get; }
    public IReadOnlyList<SchemaVariant> Variants { get; }
    public IReadOnlyDictionary<string, string> Tags { get; }

    public SchemaField(string name, string type,
        IReadOnlyList<SchemaField>? fields = null,
        IReadOnlyDictionary<string, string>? tags = null,
        IReadOnlyList<SchemaVariant>? variants = null)
    {
        Name     = name;
        Type     = type;
        Fields   = fields   ?? Array.Empty<SchemaField>();
        Variants = variants ?? Array.Empty<SchemaVariant>();
        Tags     = tags     ?? new Dictionary<string, string>();
    }

    /// <summary>
    /// Returns the sum arm named <paramref name="tag"/>.
    /// </summary>
    public SchemaVariant FindVariant(string tag)
    {
        foreach (var sv in Variants)
            if (sv.Name == tag) return sv;
        throw new InvalidOperationException($"unknown variant \"{tag}\"");
    }

    public static IReadOnlyList<SchemaField> ParseArray(JsonElement arr)
    {
        var result = new List<SchemaField>();
        foreach (var el in arr.EnumerateArray())
        {
            var name   = el.GetProperty("name").GetString()!;
            var type   = el.GetProperty("type").GetString()!;
            var fields = el.TryGetProperty("fields", out var fe) && fe.ValueKind == JsonValueKind.Array
                ? ParseArray(fe)
                : (IReadOnlyList<SchemaField>)Array.Empty<SchemaField>();
            var tags = new Dictionary<string, string>();
            if (el.TryGetProperty("tags", out var te) && te.ValueKind == JsonValueKind.Object)
                foreach (var kv in te.EnumerateObject())
                    tags[kv.Name] = kv.Value.GetString() ?? "";
            var variants = el.TryGetProperty("variants", out var ve) && ve.ValueKind == JsonValueKind.Array
                ? ParseVariants(ve)
                : (IReadOnlyList<SchemaVariant>)Array.Empty<SchemaVariant>();
            result.Add(new SchemaField(name, type, fields, tags, variants));
        }
        return result;
    }

    private static IReadOnlyList<SchemaVariant> ParseVariants(JsonElement arr)
    {
        var result = new List<SchemaVariant>();
        foreach (var el in arr.EnumerateArray())
        {
            var name = el.GetProperty("name").GetString()!;
            SchemaField? payload = null;
            if (el.TryGetProperty("payload", out var pe) && pe.ValueKind == JsonValueKind.Object)
            {
                // Reuse the field parser by wrapping the payload in a one-element array.
                using var doc = JsonDocument.Parse($"[{pe.GetRawText()}]");
                payload = ParseArray(doc.RootElement)[0];
            }
            result.Add(new SchemaVariant(name, payload));
        }
        return result;
    }
}

/// <summary>
/// One arm of a sum: a tag and its payload schema (null for a unit variant).
/// </summary>
public sealed record SchemaVariant(string Name, SchemaField? Payload);

// Worker - protocol runner

public static class Worker
{
    /// <summary>
    /// The protocol revision this library speaks. The runner requires an exact
    /// match and refuses to start a worker reporting anything else.
    /// </summary>
    private const int ProtocolVersion = 2;

    // --- audit helpers --------------------------------------------------------

    /// <summary>
    /// Recursively walks a FieldMap and collects snapshots of every byte[] value.
    /// </summary>
    /// <remarks>
    /// This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions in a Suite and run() handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
    /// </remarks>
    public static void CollectByteSnaps(FieldMap fm, List<(FieldMap fm, string key, object orig)> snaps)
    {
        foreach (var key in fm.Fields.Keys.OrderBy(k => k))
        {
            var v = fm.Fields[key];
            if (v is byte[] bytes)
            {
                snaps.Add((fm, key, CloneBytes(bytes)));
            }
            else if (v is FieldMap nested)
            {
                CollectByteSnaps(nested, snaps);
            }
            else if (v is FieldMap[] arr)
            {
                foreach (var item in arr) CollectByteSnaps(item, snaps);
            }
            else if (v is Dictionary<string, object?> dict)
            {
                foreach (var item in dict.Values)
                    if (item is FieldMap nestedFm) CollectByteSnaps(nestedFm, snaps);
            }
            else if (v is Variant variant)
            {
                // The variant itself cannot alias, but its payload can: snapshot
                // the whole field so a zero-copy payload shows up as a change.
                if (variant.Value is byte[] payload)
                    snaps.Add((fm, key, new Variant(variant.Tag, CloneBytes(payload))));
                else if (variant.Value is FieldMap payloadFm)
                    CollectByteSnaps(payloadFm, snaps);
            }
        }
    }

    private static byte[] CloneBytes(byte[] src)
    {
        var clone = new byte[src.Length];
        Array.Copy(src, clone, src.Length);
        return clone;
    }

    /// <summary>
    /// Compares two dictionaries and returns the keys whose values differ.
    /// </summary>
    /// <remarks>
    /// This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions in a Suite and run() handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
    /// </remarks>
    public static string[] DictDiffs(Dictionary<string, object?> before, Dictionary<string, object?> after)
    {
        var diffs = new List<string>();
        var allKeys = new SortedSet<string>(before.Keys);
        foreach (var k in after.Keys) allKeys.Add(k);
        foreach (var k in allKeys)
        {
            var bv = before.TryGetValue(k, out var b) ? b : null;
            var av = after.TryGetValue(k, out var a) ? a : null;
            if (!Equals(bv, av)) diffs.Add(k);
        }
        return diffs.ToArray();
    }

    /// <summary>
    /// Detects whether any byte[] fields in the FieldMap alias the input buffer by XOR-flipping the buffer and checking for changes, then restoring the original values.
    /// </summary>
    /// <remarks>
    /// This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this directly — register your serialize/deserialize functions in a Suite and run() handles audit automatically. Call this directly only if you want audit-style checks outside the runner.
    /// </remarks>
    public static string[] DetectZeroCopy(FieldMap fm, byte[] buf)
    {
        if (buf.Length == 0) return Array.Empty<string>();

        var snaps = new List<(FieldMap fm, string key, object orig)>();
        CollectByteSnaps(fm, snaps);

        // XOR-flip
        for (int i = 0; i < buf.Length; i++) buf[i] ^= 0xFF;

        var aliased = new List<string>();
        foreach (var (targetFm, key, orig) in snaps)
        {
            switch (orig)
            {
                case byte[] origBytes
                    when targetFm.Fields[key] is byte[] cur && !origBytes.AsSpan().SequenceEqual(cur):
                    aliased.Add(key);
                    break;
                case Variant origVariant
                    when targetFm.Fields[key] is Variant curVariant
                         && origVariant.Value is byte[] origPayload
                         && curVariant.Value is byte[] curPayload
                         && !origPayload.AsSpan().SequenceEqual(curPayload):
                    aliased.Add(key);
                    break;
            }
        }

        // Restore
        foreach (var (targetFm, key, orig) in snaps)
            targetFm.Fields[key] = orig;

        return aliased.ToArray();
    }

    /// <summary>Single-type worker: handles whatever type/format the runner asks for.</summary>
    public static void Run(Func<FieldMap, byte[]> serialize, Func<byte[], FieldMap>? deserialize = null)
        => RunSuite(new Dictionary<string, Dictionary<string, (Func<FieldMap, byte[]>?, Func<byte[], FieldMap>?)>>(),
                    (_, _) => (serialize, deserialize));

    /// <summary>
    /// Multi-type worker. A (type, format) that is not registered is reported to the
    /// runner as SKIPPED rather than failing the run.
    /// </summary>
    public static void RunSuite(
        Dictionary<string, Dictionary<string, (Func<FieldMap, byte[]>?, Func<byte[], FieldMap>?)>> suite,
        Func<string, string, (Func<FieldMap, byte[]>?, Func<byte[], FieldMap>?)?>? resolve = null)
    {
        resolve ??= (t, f) =>
            suite.TryGetValue(t, out var formats) && formats.TryGetValue(f, out var pair)
                ? pair
                : null;
        Func<FieldMap, byte[]>? serialize = null;
        Func<byte[], FieldMap>? deserialize = null;
        Console.InputEncoding  = Encoding.UTF8;
        Console.OutputEncoding = Encoding.UTF8;

        IReadOnlyList<SchemaField> schema = Array.Empty<SchemaField>();
        bool auditEnabled = false;
        string? line;

        while ((line = Console.ReadLine()) != null)
        {
            line = line.Trim();
            if (line.Length == 0) continue;

            using var doc = JsonDocument.Parse(line);
            var root = doc.RootElement;
            var op   = root.GetProperty("op").GetString() ?? "";
            var id   = root.TryGetProperty("id", out var idEl) ? idEl.GetString() ?? "" : "";

            switch (op)
            {
                case "ping":
                    // Health check: report liveness and the protocol revision this
                    // library speaks. Binds nothing.
                    Emit(new { op = "ping", status = "OK", protocol_version = ProtocolVersion });
                    break;

                case "bind":
                {
                    schema = root.TryGetProperty("schema", out var schemaEl)
                        ? SchemaField.ParseArray(schemaEl)
                        : Array.Empty<SchemaField>();
                    auditEnabled = root.TryGetProperty("audit", out var aEl) && aEl.GetBoolean();

                    var typeName = root.TryGetProperty("type", out var tEl) ? tEl.GetString() ?? "" : "";
                    var formatName = root.TryGetProperty("format", out var fEl) ? fEl.GetString() ?? "" : "";
                    var pair = resolve(typeName, formatName);
                    if (pair is null)
                    {
                        serialize = null;
                        deserialize = null;
                        Emit(new { op = "bind", status = "SKIPPED" });
                        break;
                    }
                    (serialize, deserialize) = pair.Value;
                    Emit(new { op = "bind" });
                    break;
                }

                case "serialize":
                    try
                    {
                        if (serialize == null)
                        {
                            if (deserialize != null)
                                Emit(new { id, op = "serialize", status = "SKIPPED", reason = "direction not registered" });
                            else
                                Emit(new { id, op = "serialize", status = "ERROR", error = "no serializer configured (call bind first)" });
                            break;
                        }
                        var fm  = DecodeFieldMap(root.GetProperty("data"), schema);

                        var before = auditEnabled ? EncodeFieldMap(fm, schema) : null;

                        var rawBytes = serialize(fm);
                        var hex = Convert.ToHexString(rawBytes).ToLowerInvariant();

                        var audit = new Dictionary<string, object>();
                        if (auditEnabled && before != null)
                        {
                            // Mutation
                            var after = EncodeFieldMap(fm, schema);
                            var diffs = DictDiffs(before, after);
                            if (diffs.Length > 0) audit["mutations"] = diffs;

                            // Output zero-copy
                            var zcBefore = EncodeFieldMap(fm, schema);
                            for (int i = 0; i < rawBytes.Length; i++) rawBytes[i] ^= 0xFF;
                            var zcAfter = EncodeFieldMap(fm, schema);
                            var zcDiffs = DictDiffs(zcBefore, zcAfter);
                            for (int i = 0; i < rawBytes.Length; i++) rawBytes[i] ^= 0xFF;
                            if (zcDiffs.Length > 0) audit["output_zero_copy_fields"] = zcDiffs;

                            // Stability
                            try
                            {
                                var b2 = serialize(fm);
                                if (hex != Convert.ToHexString(b2).ToLowerInvariant())
                                    audit["stable"] = false;
                            }
                            catch
                            {
                                audit["stable"] = false;
                            }
                        }

                        if (audit.Count > 0)
                            Emit(new { id, op = "serialize", status = "OK", hex, audit });
                        else
                            Emit(new { id, op = "serialize", status = "OK", hex });
                    }
                    catch (Exception e) { Emit(new { id, op = "serialize", status = "ERROR", error = e.Message }); }
                    break;

                case "deserialize":
                    try
                    {
                        if (deserialize == null)
                        {
                            if (serialize != null)
                                Emit(new { id, op = "deserialize", status = "SKIPPED", reason = "direction not registered" });
                            else
                                Emit(new { id, op = "deserialize", status = "ERROR", error = "no deserializer configured (call bind first)" });
                            break;
                        }
                        var bytes = Convert.FromHexString(root.GetProperty("hex").GetString()!);
                        var bufSnapshot = auditEnabled ? (byte[])bytes.Clone() : null;

                        var fm = deserialize(bytes);

                        var audit = new Dictionary<string, object>();
                        if (auditEnabled && bufSnapshot != null)
                        {
                            // Deserialize stability
                            try
                            {
                                var bufClone = (byte[])bufSnapshot.Clone();
                                var fm2 = deserialize(bufClone);
                                var data1 = EncodeFieldMap(fm, schema);
                                var data2 = EncodeFieldMap(fm2, schema);
                                var dsDiffs = DictDiffs(data1, data2);
                                if (dsDiffs.Length > 0) audit["deser_stable"] = false;
                            }
                            catch
                            {
                                audit["deser_stable"] = false;
                            }

                            // Input-buffer mutation
                            if (!bufSnapshot.AsSpan().SequenceEqual(bytes))
                                audit["input_mutated"] = true;

                            // Zero-copy
                            var zc = DetectZeroCopy(fm, bytes);
                            if (zc.Length > 0) audit["zero_copy_fields"] = zc;
                        }

                        var data = EncodeFieldMap(fm, schema);
                        if (audit.Count > 0)
                            Emit(new { id, op = "deserialize", status = "OK", data, audit });
                        else
                            Emit(new { id, op = "deserialize", status = "OK", data });
                    }
                    catch (Exception e) { Emit(new { id, op = "deserialize", status = "ERROR", error = e.Message }); }
                    break;

                case "exit":
                    Environment.Exit(0);
                    break;
            }
        }
    }


    public static FieldMap DecodeFieldMap(JsonElement data, IReadOnlyList<SchemaField> schema)
    {
        var fm = new FieldMap();
        foreach (var sf in schema)
        {
            if (!data.TryGetProperty(sf.Name, out var el)) continue;
            DecodeField(fm, sf, el);
        }
        return fm;
    }

    private static void DecodeField(FieldMap fm, SchemaField sf, JsonElement el)
    {
        var name = sf.Name;
        var type = sf.Type;

        switch (type)
        {
            case "uint8":  fm.SetU8(name,  el.GetByte());   break;
            case "uint16": fm.SetU16(name, el.GetUInt16()); break;
            case "uint32": fm.SetU32(name, el.GetUInt32()); break;
            case "uint64":  fm.SetU64(name,  ulong.Parse(el.GetString()!));   break;
            case "uint128": fm.SetU128(name, UInt128.Parse(el.GetString()!)); break;
            case "int8":  fm.SetI8(name,  el.GetSByte());  break;
            case "int16": fm.SetI16(name, el.GetInt16());  break;
            case "int32": fm.SetI32(name, el.GetInt32());  break;
            case "int64":  fm.SetI64(name,  long.Parse(el.GetString()!));   break;
            case "int128": fm.SetI128(name, Int128.Parse(el.GetString()!)); break;
            case "float32":
            {
                var b = Convert.FromHexString(el.GetString()!);
                fm.SetF32(name, BinaryPrimitives.ReadSingleLittleEndian(b));
                break;
            }
            case "float64":
            {
                var b = Convert.FromHexString(el.GetString()!);
                fm.SetF64(name, BinaryPrimitives.ReadDoubleLittleEndian(b));
                break;
            }
            case "bool":   fm.SetBool(name, el.GetBoolean()); break;
            case "string": fm.SetString(name, el.GetString()!); break;
            case "bytes":  fm.SetBytes(name, Convert.FromHexString(el.GetString()!)); break;
            case "struct":
                fm.SetStruct(name, DecodeFieldMap(el, sf.Fields));
                break;
            default:
                if (type.StartsWith("list<"))   { DecodeList(fm, sf, type[5..^1], el); break; }
                if (type.StartsWith("optional<")) { DecodeOptional(fm, sf, type[9..^1], el); break; }
                if (type.StartsWith("array<"))
                {
                    // An array<T,N> is a list whose length the schema fixes, so it
                    // shares DecodeList outright and adds only the length check. A
                    // separate representation is what pinned array<T,N> to uint.
                    var (aElem, aLen) = SplitArrayType(type);
                    DecodeList(fm, sf, aElem, el);
                    var got = ((System.Collections.IEnumerable)fm.Fields[name]!).Cast<object?>().Count();
                    if (got != aLen)
                        throw new InvalidOperationException($"array {name}: expected {aLen} elements, got {got}");
                    break;
                }
                // enum<a,b,c>: the variant name travels as a string.
                if (type.StartsWith("enum<")) { fm.SetString(name, el.GetString()!); return; }
                if (type.StartsWith("sum<")) { DecodeSum(fm, sf, el); return; }
                if (type.StartsWith("map<"))
                {
                    var (_, valType) = SplitMapTypes(type);
                    fm.SetMap(name, DecodeMap(valType, sf.Fields, el));
                    break;
                }
                // Breaking out silently left the field absent from the FieldMap,
                // which surfaces far downstream as a missing value rather than as
                // "this library does not know that type".
                throw new InvalidOperationException($"unknown type \"{type}\"");
        }
    }

    /// <summary>
    /// Decodes a sum wire value {tag: payload} (payload null for a unit variant)
    /// into a Variant, decoding the payload per the variant's own schema.
    /// </summary>
    private static void DecodeSum(FieldMap fm, SchemaField sf, JsonElement el)
    {
        var props = el.EnumerateObject().ToArray();
        if (props.Length != 1)
            throw new InvalidOperationException($"sum must name exactly one variant, got {props.Length}");
        var tag = props[0].Name;
        var sv  = sf.FindVariant(tag);
        if (sv.Payload is null) { fm.SetVariant(sf.Name, tag); return; }
        var tmp = new FieldMap();
        DecodeField(tmp, sv.Payload, props[0].Value);
        fm.SetVariant(sf.Name, tag, tmp.Fields[sv.Payload.Name]);
    }

    /// <summary>Splits "array&lt;T,N&gt;" into its element type and length.</summary>
    private static (string, int) SplitArrayType(string type)
    {
        var inner = type[6..^1];
        var comma = inner.LastIndexOf(',');
        return (inner[..comma].Trim(), int.Parse(inner[(comma + 1)..].Trim()));
    }

    /// <summary>Element types a list may carry: every scalar, plus a nested struct.</summary>
    private static readonly HashSet<string> ListElems = new()
    {
        "uint8", "uint16", "uint32", "uint64", "uint128",
        "int8", "int16", "int32", "int64", "int128",
        "float32", "float64", "bool", "string", "bytes",
        "struct",
    };

    /// <summary>
    /// Decodes every element through DecodeField, so a list supports exactly the
    /// element types a bare field does. This used to carry its own switch, which
    /// is why uint16/int8/int16/float64/bytes were declarable in a case file and
    /// accepted by `serify validate`, but threw once a worker actually ran. The
    /// switch below only names the array each element type packs into.
    /// </summary>
    private static void DecodeList(FieldMap fm, SchemaField sf, string elem, JsonElement el)
    {
        if (!ListElems.Contains(elem))
            throw new InvalidOperationException($"unsupported list element type \"{elem}\"");

        var elemSf = new SchemaField("e", elem, sf.Fields, sf.Tags);
        var vals = new List<object?>();
        var i = 0;
        foreach (var item in el.EnumerateArray())
        {
            var tmp = new FieldMap();
            try { DecodeField(tmp, elemSf, item); }
            catch (Exception e) { throw new InvalidOperationException($"[{i}]: {e.Message}", e); }
            vals.Add(tmp.Fields["e"]);
            i++;
        }

        fm.Fields[sf.Name] = elem switch
        {
            "uint8"   => vals.Cast<byte>().ToArray(),
            "uint16"  => vals.Cast<ushort>().ToArray(),
            "uint32"  => vals.Cast<uint>().ToArray(),
            "uint64"  => vals.Cast<ulong>().ToArray(),
            "int8"    => vals.Cast<sbyte>().ToArray(),
            "int16"   => vals.Cast<short>().ToArray(),
            "int32"   => vals.Cast<int>().ToArray(),
            "int64"   => vals.Cast<long>().ToArray(),
            "uint128" => vals.Cast<UInt128>().ToArray(),
            "int128"  => vals.Cast<Int128>().ToArray(),
            "float32" => vals.Cast<float>().ToArray(),
            "float64" => vals.Cast<double>().ToArray(),
            "bool"    => vals.Cast<bool>().ToArray(),
            "string"  => vals.Cast<string>().ToArray(),
            "bytes"   => vals.Cast<byte[]>().ToArray(),
            "struct"  => vals.Cast<FieldMap>().ToArray(),
            _ => throw new InvalidOperationException($"unsupported list element type \"{elem}\""),
        };
    }

    private static void DecodeOptional(FieldMap fm, SchemaField sf, string elem, JsonElement el)
    {
        if (el.ValueKind == JsonValueKind.Null)
        {
            switch (elem)
            {
                case "string": fm.SetOptionalString(sf.Name, null); break;
                case "struct": fm.SetOptionalStruct(sf.Name, null); break;
                default: fm.Fields[sf.Name] = null; break;
            }
            return;
        }
        switch (elem)
        {
            case "string": fm.SetOptionalString(sf.Name, el.GetString()); break;
            case "struct":
                fm.SetOptionalStruct(sf.Name, DecodeFieldMap(el, sf.Fields));
                break;
            default:
            {
                var inner = new SchemaField(sf.Name, elem, sf.Fields, sf.Tags);
                var tmp = new FieldMap();
                DecodeField(tmp, inner, el);
                if (tmp.Fields.TryGetValue(sf.Name, out var v)) fm.Fields[sf.Name] = v;
                break;
            }
        }
    }


    public static Dictionary<string, object?> EncodeFieldMap(FieldMap fm, IReadOnlyList<SchemaField> schema)
    {
        var out_ = new Dictionary<string, object?>();
        foreach (var sf in schema)
        {
            if (!fm.Fields.TryGetValue(sf.Name, out var v)) continue;
            out_[sf.Name] = EncodeField(sf, v);
        }
        return out_;
    }

    private static object? EncodeField(SchemaField sf, object? v)
    {
        var type = sf.Type;
        return type switch
        {
            "uint8" or "uint16" or "uint32" or "int8" or "int16" or "int32" => v,
            "uint64"  => ((ulong)v!).ToString(),
            "uint128" => ((UInt128)v!).ToString(),
            "int64"  => ((long)v!).ToString(),
            "int128" => ((Int128)v!).ToString(),
            "float32" => EncodeF32((float)v!),
            "float64" => EncodeF64((double)v!),
            "bool" => v,
            "string" => v,
            "bytes" => Convert.ToHexString((byte[])v!).ToLowerInvariant(),
            "struct" => EncodeFieldMap((FieldMap)v!, sf.Fields),
            _ when type.StartsWith("list<")    => EncodeList(sf, type[5..^1], v),
            _ when type.StartsWith("optional<") => EncodeOptional(sf, type[9..^1], v),
            _ when type.StartsWith("array<")   => EncodeList(sf, SplitArrayType(type).Item1, v),
            _ when type.StartsWith("enum<") => v,
            _ when type.StartsWith("sum<") => EncodeSum(sf, v),
            _ when type.StartsWith("map<") => EncodeMap(SplitMapTypes(type).Item2, sf.Fields, v),
            _ => throw new InvalidOperationException($"unknown type \"{type}\""),
        };
    }

    /// <summary>
    /// Inverse of DecodeSum: a Variant becomes {tag: payload}.
    /// </summary>
    private static object? EncodeSum(SchemaField sf, object? v)
    {
        if (v is not Variant variant)
            throw new InvalidOperationException($"expected a Variant for sum field \"{sf.Name}\"");
        var sv = sf.FindVariant(variant.Tag);
        return new Dictionary<string, object?>
        {
            [variant.Tag] = sv.Payload is null ? null : EncodeField(sv.Payload, variant.Value),
        };
    }

    /// <summary>
    /// Inverse of DecodeList: every element goes back out through EncodeField, so
    /// the two directions cannot cover different element types. The old version
    /// fell through to returning the value untouched for anything it did not name.
    /// </summary>
    private static object? EncodeList(SchemaField sf, string elem, object? v)
    {
        if (!ListElems.Contains(elem))
            throw new InvalidOperationException($"unsupported list element type \"{elem}\"");
        if (elem == "struct")
            return ((FieldMap[])v!).Select(fm => EncodeFieldMap(fm, sf.Fields)).ToArray();

        var elemSf = new SchemaField("e", elem, sf.Fields, sf.Tags);
        return ((System.Collections.IEnumerable)v!).Cast<object?>()
            .Select(x => EncodeField(elemSf, x)).ToArray();
    }

    private static object? EncodeOptional(SchemaField sf, string elem, object? v)
    {
        if (v is null) return null;
        return elem switch
        {
            "string" => v,
            "struct" => EncodeFieldMap((FieldMap)v!, sf.Fields),
            _ => EncodeField(new SchemaField(sf.Name, elem, sf.Fields, sf.Tags), v),
        };
    }

    private static string EncodeF32(float f)
    {
        var b = new byte[4];
        BinaryPrimitives.WriteSingleLittleEndian(b, f);
        return Convert.ToHexString(b).ToLowerInvariant();
    }

    private static string EncodeF64(double d)
    {
        var b = new byte[8];
        BinaryPrimitives.WriteDoubleLittleEndian(b, d);
        return Convert.ToHexString(b).ToLowerInvariant();
    }

    private static (string, string) SplitMapTypes(string type)
    {
        var inner = type[4..^1]; // strip "map<" and ">"
        int depth = 0;
        for (int i = 0; i < inner.Length; i++)
        {
            if (inner[i] == '<' || inner[i] == '[') depth++;
            else if (inner[i] == '>' || inner[i] == ']') depth--;
            else if (inner[i] == ',' && depth == 0)
                return (inner[..i].Trim(), inner[(i+1)..].Trim());
        }
        return ("", inner.Trim());
    }

    private static Dictionary<string, object?> DecodeMap(string valType,
        IReadOnlyList<SchemaField> nestedSchema, JsonElement el)
    {
        var m = new Dictionary<string, object?>();
        foreach (var kv in el.EnumerateObject())
        {
            m[kv.Name] = valType switch
            {
                "struct" => DecodeFieldMap(kv.Value, nestedSchema),
                "uint8"  => kv.Value.GetByte(),
                "uint16" => kv.Value.GetUInt16(),
                "uint32" => kv.Value.GetUInt32(),
                "uint64"  => ulong.Parse(kv.Value.GetString()!),
                "uint128" => UInt128.Parse(kv.Value.GetString()!),
                "int8"  => kv.Value.GetSByte(),
                "int16" => kv.Value.GetInt16(),
                "int32" => kv.Value.GetInt32(),
                "int64"  => long.Parse(kv.Value.GetString()!),
                "int128" => Int128.Parse(kv.Value.GetString()!),
                "float32" => BinaryPrimitives.ReadSingleLittleEndian(
                    Convert.FromHexString(kv.Value.GetString()!)),
                "float64" => BinaryPrimitives.ReadDoubleLittleEndian(
                    Convert.FromHexString(kv.Value.GetString()!)),
                "bytes" => Convert.FromHexString(kv.Value.GetString()!),
                "bool" => kv.Value.GetBoolean(),
                "string" => kv.Value.GetString(),
                _ => (object?)kv.Value,
            };
        }
        return m;
    }

    private static Dictionary<string, object?> EncodeMap(string valType,
        IReadOnlyList<SchemaField> nestedSchema, object? v)
    {
        var src = (Dictionary<string, object?>)v!;
        var out_ = new Dictionary<string, object?>();
        foreach (var k in src.Keys.OrderBy(k => k))
        {
            var item = src[k];
            out_[k] = valType switch
            {
                "struct" => EncodeFieldMap((FieldMap)item!, nestedSchema),
                "uint64"  => ((ulong)item!).ToString(),
                "uint128" => ((UInt128)item!).ToString(),
                "int64"  => ((long)item!).ToString(),
                "int128" => ((Int128)item!).ToString(),
                "float32" => EncodeF32((float)item!),
                "float64" => EncodeF64((double)item!),
                "bytes" => Convert.ToHexString((byte[])item!).ToLowerInvariant(),
                _ => item,
            };
        }
        return out_;
    }

    private static void Emit(object obj)
    {
        Console.WriteLine(JsonSerializer.Serialize(obj));
    }
}

// --- Model attributes --------------------------------------------------------

/// <summary>Marks a class as a Serify model, enabling ToFieldMap / FromFieldMap.</summary>
[AttributeUsage(AttributeTargets.Class, Inherited = false)]
public sealed class SerifyModelAttribute : Attribute { }

/// <summary>Maps a property to a serify schema key.  Defaults to the property name.</summary>
[AttributeUsage(AttributeTargets.Property | AttributeTargets.Field, Inherited = false)]
public sealed class SerifyFieldAttribute : Attribute
{
    public string? Name { get; }
    public SerifyFieldAttribute() { }
    public SerifyFieldAttribute(string name) => Name = name;
}

/// <summary>Reflection-based helpers for [SerifyModel] / [SerifyField] classes.</summary>
public static class SerifyModel
{
    /// <summary>
    /// Default schema key for a member: <c>EntryId</c> becomes <c>entry_id</c>.
    /// C# members are PascalCase and schema keys are snake_case, so bridging the
    /// two is the library's job — pass a name to [SerifyField] to override.
    /// </summary>
    private static string DefaultKey(string memberName)
    {
        var sb = new StringBuilder(memberName.Length + 4);
        for (int i = 0; i < memberName.Length; i++)
        {
            char c = memberName[i];
            if (char.IsUpper(c))
            {
                if (i != 0) sb.Append('_');
                sb.Append(char.ToLowerInvariant(c));
            }
            else sb.Append(c);
        }
        return sb.ToString();
    }

    /// <summary>Convert a [SerifyModel] instance to a FieldMap.</summary>
    public static FieldMap ToFieldMap<T>(T obj) where T : class
    {
        var fm = new FieldMap();
        foreach (var prop in typeof(T).GetProperties(System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.Instance))
        {
            if (!Attribute.IsDefined(prop, typeof(SerifyFieldAttribute))) continue;
            var key = (Attribute.GetCustomAttribute(prop, typeof(SerifyFieldAttribute)) as SerifyFieldAttribute)?.Name ?? DefaultKey(prop.Name);
            var val = prop.GetValue(obj);
            // A closed record hierarchy is C#'s sum type, so a member declared
            // with one is a sum — see ToVariant.
            if (SumArms(prop.PropertyType) is { } arms) fm.Fields[key] = ToVariant(arms, key, val);
            else SetFieldMapValue(fm, key, val);
        }
        foreach (var fld in typeof(T).GetFields(System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.Instance))
        {
            if (!Attribute.IsDefined(fld, typeof(SerifyFieldAttribute))) continue;
            var key = (Attribute.GetCustomAttribute(fld, typeof(SerifyFieldAttribute)) as SerifyFieldAttribute)?.Name ?? DefaultKey(fld.Name);
            var val = fld.GetValue(obj);
            SetFieldMapValue(fm, key, val);
        }
        return fm;
    }

    /// <summary>Create a [SerifyModel] instance from a FieldMap.</summary>
    public static T FromFieldMap<T>(FieldMap fm) where T : class, new()
    {
        var obj = new T();
        foreach (var prop in typeof(T).GetProperties(System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.Instance))
        {
            if (!Attribute.IsDefined(prop, typeof(SerifyFieldAttribute))) continue;
            var key = (Attribute.GetCustomAttribute(prop, typeof(SerifyFieldAttribute)) as SerifyFieldAttribute)?.Name ?? DefaultKey(prop.Name);
            if (fm.Fields.TryGetValue(key, out var val))
            {
                if (!prop.CanWrite) continue;
                if (SumArms(prop.PropertyType) is { } arms && val is Variant variant)
                    prop.SetValue(obj, FromVariant(arms, prop.PropertyType, variant));
                else
                    prop.SetValue(obj, ConvertValue(val, prop.PropertyType));
            }
        }
        foreach (var fld in typeof(T).GetFields(System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.Instance))
        {
            if (!Attribute.IsDefined(fld, typeof(SerifyFieldAttribute))) continue;
            var key = (Attribute.GetCustomAttribute(fld, typeof(SerifyFieldAttribute)) as SerifyFieldAttribute)?.Name ?? DefaultKey(fld.Name);
            if (fm.Fields.TryGetValue(key, out var val))
                fld.SetValue(obj, ConvertValue(val, fld.FieldType));
        }
        return obj;
    }

    // ── sum ───────────────────────────────────────────────────────────────
    //
    // C# has no discriminated union keyword, but an abstract record with a
    // private constructor and nested sealed records is the closed hierarchy that
    // stands in for one — and it is all the binding needs: the nested types name
    // the arms and each arm's primary-constructor parameters give its payload.
    // No converter, no registration.
    //
    // The arity rule is the same one every serify binding uses, because a sum
    // is a sum-of-products:
    //
    //     0 parameters -> a unit variant, no payload
    //     1 parameter  -> that parameter's value is the payload
    //     N parameters -> the payload is a struct holding the N parameters

    /// <summary>The arms of a closed record hierarchy, or null if <paramref name="t"/> is not one.</summary>
    private static Type[]? SumArms(Type t)
    {
        if (!t.IsAbstract || t.IsInterface || t == typeof(object)) return null;
        // NonPublic too: GetNestedTypes() alone returns only public nested types,
        // and an `internal` hierarchy is perfectly ordinary.
        var nested = t.GetNestedTypes(System.Reflection.BindingFlags.Public
                                      | System.Reflection.BindingFlags.NonPublic);
        var arms = Array.FindAll(nested, n => n.BaseType == t);
        return arms.Length > 0 ? arms : null;
    }

    /// <summary>Schema tag for an arm: its name in snake_case.</summary>
    private static string ArmTag(Type arm) => DefaultKey(arm.Name);

    private static System.Reflection.ParameterInfo[] ArmParams(Type arm) =>
        arm.GetConstructors()[0].GetParameters();

    /// <summary>Render the active arm of a closed hierarchy as a Variant.</summary>
    private static Variant ToVariant(Type[] arms, string key, object? val)
    {
        if (val == null) throw new InvalidOperationException($"{key} is unset");
        var arm = val.GetType();
        if (Array.IndexOf(arms, arm) < 0)
        {
            throw new InvalidOperationException(
                $"{key}: {arm.Name} is not one of {string.Join(", ", Array.ConvertAll(arms, a => a.Name))}");
        }

        var ps = ArmParams(arm);
        if (ps.Length == 0) return new Variant(ArmTag(arm), null);
        if (ps.Length == 1) return new Variant(ArmTag(arm), ArmValue(arm, val, ps[0]));

        var payload = new FieldMap();                       // N parameters -> a struct
        foreach (var p in ps) SetFieldMapValue(payload, DefaultKey(p.Name!), ArmValue(arm, val, p));
        return new Variant(ArmTag(arm), payload);
    }

    /// <summary>A positional record parameter is exposed as a property of the same name.</summary>
    private static object? ArmValue(Type arm, object val, System.Reflection.ParameterInfo p) =>
        arm.GetProperty(p.Name!)?.GetValue(val)
        ?? throw new InvalidOperationException($"{arm.Name}: no property for parameter {p.Name}");

    /// <summary>Rebuild the active arm of a closed hierarchy from a Variant.</summary>
    private static object FromVariant(Type[] arms, Type declared, Variant v)
    {
        foreach (var arm in arms)
        {
            if (ArmTag(arm) != v.Tag) continue;
            var ps = ArmParams(arm);
            var args = new object?[ps.Length];
            if (ps.Length == 1)
            {
                args[0] = ConvertValue(v.Value, ps[0].ParameterType);
            }
            else if (ps.Length > 1)
            {
                if (v.Value is not FieldMap payload)
                    throw new InvalidOperationException($"variant \"{v.Tag}\" needs a struct payload");
                for (int i = 0; i < ps.Length; i++)
                {
                    payload.Fields.TryGetValue(DefaultKey(ps[i].Name!), out var raw);
                    args[i] = ConvertValue(raw, ps[i].ParameterType);
                }
            }
            return Activator.CreateInstance(arm, args)!;
        }
        throw new InvalidOperationException(
            $"unknown variant \"{v.Tag}\" for {declared.Name} " +
            $"(declared: {string.Join(", ", Array.ConvertAll(arms, ArmTag))})");
    }

    private static void SetFieldMapValue(FieldMap fm, string key, object? val)
    {
        // A null is a field, not an absent one: it is how an optional<T> says
        // "no value", and skipping it would silently omit the key.
        if (val == null) { fm.Fields[key] = null; return; }
        switch (val)
        {
            case byte   v: fm.SetU8(key, v); break;
            case ushort v: fm.SetU16(key, v); break;
            case uint   v: fm.SetU32(key, v); break;
            case ulong  v: fm.SetU64(key, v); break;
            case sbyte  v: fm.SetI8(key, v); break;
            case short  v: fm.SetI16(key, v); break;
            case int    v: fm.SetI32(key, v); break;
            case long   v: fm.SetI64(key, v); break;
            case UInt128 v: fm.SetU128(key, v); break;
            case Int128  v: fm.SetI128(key, v); break;
            case float  v: fm.SetF32(key, v); break;
            case double v: fm.SetF64(key, v); break;
            case bool   v: fm.SetBool(key, v); break;
            case string v: fm.SetString(key, v); break;
            case byte[] v: fm.SetBytes(key, v); break;
            case FieldMap v: fm.SetStruct(key, v); break;
            case FieldMap[] v: fm.SetListStruct(key, v); break;
            // Every other array: store it as-is, since the schema — not the
            // runtime element type — decides the wire form. Naming the array
            // types individually is what left ushort[], sbyte[], short[], int[],
            // long[], UInt128[], Int128[], double[], bool[] and byte[][] falling
            // through to the ToString() default below.
            case System.Array v: fm.Fields[key] = v; break;
            case Dictionary<string, object?> v: fm.SetMap(key, v); break;
            case Variant v: fm.Fields[key] = v; break;
            default: fm.SetString(key, val.ToString() ?? ""); break;
        }
    }

    private static object? ConvertValue(object? val, Type targetType)
    {
        if (val == null) return targetType.IsValueType ? Activator.CreateInstance(targetType) : null;
        if (targetType.IsInstanceOfType(val)) return val;
        if (targetType == typeof(FieldMap) && val is FieldMap fm) return fm;
        try { return Convert.ChangeType(val, targetType); }
        catch { return val; }
    }
}
