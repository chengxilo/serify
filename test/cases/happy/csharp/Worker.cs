/*
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Happy-path C# worker: `all_types` in binary and json.
//
// Go is the --ref language and owns both byte layouts; see
// test/cases/happy/go/type.go. The json format must match Go's encoding/json
// byte-for-byte (with SetEscapeHTML(false)): schema field order, map keys in
// UTF-8 byte order, []byte as base64, floats in shortest form without a
// trailing .0, and U+2028/U+2029 escaped (Go escapes those unconditionally).

using System.Globalization;
using System.Text;
using System.Text.Json;
using Serify;

internal static class HappyWorker
{
    private static readonly string[] StatusVariants =
        { "pending", "paid", "shipped", "delivered", "cancelled" };

    private static byte StatusOrdinal(string s)
    {
        var i = Array.IndexOf(StatusVariants, s);
        if (i < 0) throw new ArgumentException($"unknown status \"{s}\"");
        return (byte)i;
    }

    // UTF-8 byte order (== code-point order), not UTF-16 code-unit order,
    // which string.CompareOrdinal would use and which misplaces astral keys.
    private static int ByteCompare(string a, string b)
    {
        var ab = Encoding.UTF8.GetBytes(a);
        var bb = Encoding.UTF8.GetBytes(b);
        var n = Math.Min(ab.Length, bb.Length);
        for (var i = 0; i < n; i++)
        {
            if (ab[i] != bb[i]) return ab[i] - bb[i];
        }
        return ab.Length - bb.Length;
    }

    // --- binary format -------------------------------------------------------

    private static void WriteLenStr(BinaryWriter w, string s)
    {
        var b = Encoding.UTF8.GetBytes(s);
        w.Write((uint)b.Length);
        w.Write(b);
    }

    private static string ReadLenStr(BinaryReader r)
        => Encoding.UTF8.GetString(r.ReadBytes((int)r.ReadUInt32()));

    private static byte[] BinarySerialize(FieldMap fm)
    {
        using var ms = new MemoryStream();
        using var w = new BinaryWriter(ms); // BinaryWriter is little-endian

        w.Write(fm.GetU8("uint8"));
        w.Write(fm.GetU16("uint16"));
        w.Write(fm.GetU32("uint32"));
        w.Write(fm.GetU64("uint64"));
        w.Write(fm.GetI8("int8"));
        w.Write(fm.GetI16("int16"));
        w.Write(fm.GetI32("int32"));
        w.Write(fm.GetI64("int64"));
        w.Write(fm.GetF32("float32"));
        w.Write(fm.GetF64("float64"));
        w.Write((byte)(fm.GetBool("bool") ? 1 : 0));
        WriteLenStr(w, fm.GetString("string"));

        var raw = fm.GetBytes("bytes");
        w.Write((uint)raw.Length);
        w.Write(raw);

        var list = fm.GetListString("list");
        w.Write((uint)list.Length);
        foreach (var s in list) WriteLenStr(w, s);

        var opt = fm.GetOptionalString("optional");
        if (opt is null) { w.Write((byte)0); }
        else { w.Write((byte)1); WriteLenStr(w, opt); }

        foreach (var n in fm.GetListU32("array")) w.Write(n);

        var p = fm.GetStruct("struct");
        w.Write(p.GetI32("x"));
        w.Write(p.GetI32("y"));
        w.Write(p.GetI32("z"));
        WriteLenStr(w, p.GetString("name"));

        var m = fm.GetMap("map");
        var keys = m.Keys.ToList();
        keys.Sort(ByteCompare);
        w.Write((uint)keys.Count);
        foreach (var k in keys) { WriteLenStr(w, k); w.Write((uint)m[k]!); }

        var ms2 = fm.GetMap("map_struct");
        var mkeys = ms2.Keys.ToList();
        mkeys.Sort(ByteCompare);
        w.Write((uint)mkeys.Count);
        foreach (var k in mkeys)
        {
            var t = (FieldMap)ms2[k]!;
            WriteLenStr(w, k);
            WriteLenStr(w, t.GetString("name"));
            w.Write(t.GetU32("weight"));
        }

        w.Write(StatusOrdinal(fm.GetString("status")));
        w.Flush();
        return ms.ToArray();
    }

    private static FieldMap BinaryDeserialize(byte[] data)
    {
        using var r = new BinaryReader(new MemoryStream(data));
        var fm = new FieldMap();
        fm.SetU8("uint8", r.ReadByte());
        fm.SetU16("uint16", r.ReadUInt16());
        fm.SetU32("uint32", r.ReadUInt32());
        fm.SetU64("uint64", r.ReadUInt64());
        fm.SetI8("int8", r.ReadSByte());
        fm.SetI16("int16", r.ReadInt16());
        fm.SetI32("int32", r.ReadInt32());
        fm.SetI64("int64", r.ReadInt64());
        fm.SetF32("float32", r.ReadSingle());
        fm.SetF64("float64", r.ReadDouble());
        fm.SetBool("bool", r.ReadByte() != 0);
        fm.SetString("string", ReadLenStr(r));
        fm.SetBytes("bytes", r.ReadBytes((int)r.ReadUInt32()));

        var listLen = (int)r.ReadUInt32();
        var list = new string[listLen];
        for (var i = 0; i < listLen; i++) list[i] = ReadLenStr(r);
        fm.SetListString("list", list);

        fm.SetOptionalString("optional", r.ReadByte() != 0 ? ReadLenStr(r) : null);

        var arr = new uint[4];
        for (var i = 0; i < 4; i++) arr[i] = r.ReadUInt32();
        fm.SetListU32("array", arr);

        var p = new FieldMap();
        p.SetI32("x", r.ReadInt32());
        p.SetI32("y", r.ReadInt32());
        p.SetI32("z", r.ReadInt32());
        p.SetString("name", ReadLenStr(r));
        fm.SetStruct("struct", p);

        var m = new Dictionary<string, object?>();
        var mapLen = (int)r.ReadUInt32();
        for (var i = 0; i < mapLen; i++)
        {
            var k = ReadLenStr(r);
            m[k] = r.ReadUInt32();
        }
        fm.SetMap("map", m);

        var ms = new Dictionary<string, object?>();
        var msLen = (int)r.ReadUInt32();
        for (var i = 0; i < msLen; i++)
        {
            var k = ReadLenStr(r);
            var t = new FieldMap();
            t.SetString("name", ReadLenStr(r));
            t.SetU32("weight", r.ReadUInt32());
            ms[k] = t;
        }
        fm.SetMap("map_struct", ms);

        var ord = r.ReadByte();
        if (ord >= StatusVariants.Length)
            throw new ArgumentException($"status ordinal {ord} out of range");
        fm.SetString("status", StatusVariants[ord]);
        return fm;
    }

    // --- json format ---------------------------------------------------------

    // Go's encoding/json string escaping with SetEscapeHTML(false): only \n,
    // \r, \t are named (\b and \f become \u00xx), and U+2028/U+2029 are
    // escaped unconditionally.
    private static string GoStr(string s)
    {
        var sb = new StringBuilder("\"");
        foreach (var ch in s)
        {
            switch (ch)
            {
                case '"': sb.Append("\\\""); break;
                case '\\': sb.Append("\\\\"); break;
                case '\n': sb.Append("\\n"); break;
                case '\r': sb.Append("\\r"); break;
                case '\t': sb.Append("\\t"); break;
                case '\u2028': sb.Append("\\u2028"); break;
                case '\u2029': sb.Append("\\u2029"); break;
                default:
                    if (ch < 0x20) sb.Append($"\\u{(int)ch:x4}");
                    else sb.Append(ch);
                    break;
            }
        }
        return sb.Append('"').ToString();
    }

    // .NET Core 3.0+ default double/float ToString is shortest round-trip,
    // which matches Go's format for the value range these cases use — as long
    // as the invariant culture keeps the decimal point a point.
    private static string GoF64(double v) => v.ToString(CultureInfo.InvariantCulture);
    private static string GoF32(float v) => v.ToString(CultureInfo.InvariantCulture);

    private static byte[] JsonSerialize(FieldMap fm)
    {
        var sb = new StringBuilder("{");
        sb.Append($"\"uint8\":{fm.GetU8("uint8")}");
        sb.Append($",\"uint16\":{fm.GetU16("uint16")}");
        sb.Append($",\"uint32\":{fm.GetU32("uint32")}");
        sb.Append($",\"uint64\":{fm.GetU64("uint64")}");
        sb.Append($",\"int8\":{fm.GetI8("int8")}");
        sb.Append($",\"int16\":{fm.GetI16("int16")}");
        sb.Append($",\"int32\":{fm.GetI32("int32")}");
        sb.Append($",\"int64\":{fm.GetI64("int64")}");
        sb.Append($",\"float32\":{GoF32(fm.GetF32("float32"))}");
        sb.Append($",\"float64\":{GoF64(fm.GetF64("float64"))}");
        sb.Append($",\"bool\":{(fm.GetBool("bool") ? "true" : "false")}");
        sb.Append($",\"string\":{GoStr(fm.GetString("string"))}");
        sb.Append($",\"bytes\":\"{Convert.ToBase64String(fm.GetBytes("bytes"))}\"");
        sb.Append($",\"list\":[{string.Join(",", fm.GetListString("list").Select(GoStr))}]");

        var opt = fm.GetOptionalString("optional");
        sb.Append($",\"optional\":{(opt is null ? "null" : GoStr(opt))}");

        sb.Append($",\"array\":[{string.Join(",", fm.GetListU32("array"))}]");

        var p = fm.GetStruct("struct");
        sb.Append($",\"struct\":{{\"x\":{p.GetI32("x")},\"y\":{p.GetI32("y")},\"z\":{p.GetI32("z")},\"name\":{GoStr(p.GetString("name"))}}}");

        var m = fm.GetMap("map");
        var keys = m.Keys.ToList();
        keys.Sort(ByteCompare);
        sb.Append($",\"map\":{{{string.Join(",", keys.Select(k => $"{GoStr(k)}:{m[k]}"))}}}");

        var ms = fm.GetMap("map_struct");
        var mkeys = ms.Keys.ToList();
        mkeys.Sort(ByteCompare);
        sb.Append($",\"map_struct\":{{{string.Join(",", mkeys.Select(k =>
        {
            var t = (FieldMap)ms[k]!;
            return $"{GoStr(k)}:{{\"name\":{GoStr(t.GetString("name"))},\"weight\":{t.GetU32("weight")}}}";
        }))}}}");

        sb.Append($",\"status\":{GoStr(fm.GetString("status"))}");
        sb.Append('}');
        return Encoding.UTF8.GetBytes(sb.ToString());
    }

    private static FieldMap JsonDeserialize(byte[] data)
    {
        using var doc = JsonDocument.Parse(data);
        var v = doc.RootElement;

        var fm = new FieldMap();
        fm.SetU8("uint8", v.GetProperty("uint8").GetByte());
        fm.SetU16("uint16", v.GetProperty("uint16").GetUInt16());
        fm.SetU32("uint32", v.GetProperty("uint32").GetUInt32());
        fm.SetU64("uint64", v.GetProperty("uint64").GetUInt64());
        fm.SetI8("int8", v.GetProperty("int8").GetSByte());
        fm.SetI16("int16", v.GetProperty("int16").GetInt16());
        fm.SetI32("int32", v.GetProperty("int32").GetInt32());
        fm.SetI64("int64", v.GetProperty("int64").GetInt64());
        fm.SetF32("float32", v.GetProperty("float32").GetSingle());
        fm.SetF64("float64", v.GetProperty("float64").GetDouble());
        fm.SetBool("bool", v.GetProperty("bool").GetBoolean());
        fm.SetString("string", v.GetProperty("string").GetString()!);
        fm.SetBytes("bytes", Convert.FromBase64String(v.GetProperty("bytes").GetString()!));
        fm.SetListString("list",
            v.GetProperty("list").EnumerateArray().Select(x => x.GetString()!).ToArray());

        var opt = v.GetProperty("optional");
        fm.SetOptionalString("optional",
            opt.ValueKind == JsonValueKind.Null ? null : opt.GetString());

        fm.SetListU32("array",
            v.GetProperty("array").EnumerateArray().Select(x => x.GetUInt32()).ToArray());

        var st = v.GetProperty("struct");
        var p = new FieldMap();
        p.SetI32("x", st.GetProperty("x").GetInt32());
        p.SetI32("y", st.GetProperty("y").GetInt32());
        p.SetI32("z", st.GetProperty("z").GetInt32());
        p.SetString("name", st.GetProperty("name").GetString()!);
        fm.SetStruct("struct", p);

        var m = new Dictionary<string, object?>();
        foreach (var prop in v.GetProperty("map").EnumerateObject())
            m[prop.Name] = prop.Value.GetUInt32();
        fm.SetMap("map", m);

        var ms = new Dictionary<string, object?>();
        foreach (var prop in v.GetProperty("map_struct").EnumerateObject())
        {
            var t = new FieldMap();
            t.SetString("name", prop.Value.GetProperty("name").GetString()!);
            t.SetU32("weight", prop.Value.GetProperty("weight").GetUInt32());
            ms[prop.Name] = t;
        }
        fm.SetMap("map_struct", ms);

        fm.SetString("status", v.GetProperty("status").GetString()!);
        return fm;
    }

    private static void Main()
    {
        Serify.Worker.RunSuite(new Dictionary<string, Dictionary<string, (Func<FieldMap, byte[]>, Func<byte[], FieldMap>)>>
        {
            ["all_types"] = new()
            {
                ["binary"] = (BinarySerialize, BinaryDeserialize),
                ["json"] = (JsonSerialize, JsonDeserialize),
            },
        });
    }
}
