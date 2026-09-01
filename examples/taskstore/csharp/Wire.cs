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


// Byte-level primitives. Go owns the layout these reproduce; the conventions
// are documented at the top of go/api/wire.go.

using System;
using System.Buffers.Binary;
using System.Collections.Generic;
using System.IO;
using System.Text;

/// <summary>A cursor over one message's bytes.</summary>
internal ref struct Reader
{
    private readonly ReadOnlySpan<byte> _data;
    private int _off;

    internal Reader(ReadOnlySpan<byte> data)
    {
        _data = data;
        _off = 0;
    }

    private ReadOnlySpan<byte> Take(int n)
    {
        if (_off + n > _data.Length)
            throw new InvalidDataException($"truncated: want {n} bytes at offset {_off}");
        var span = _data.Slice(_off, n);
        _off += n;
        return span;
    }

    internal byte U8() => Take(1)[0];

    internal uint U32() => BinaryPrimitives.ReadUInt32LittleEndian(Take(4));

    internal ulong U64() => BinaryPrimitives.ReadUInt64LittleEndian(Take(8));

    internal long I64() => BinaryPrimitives.ReadInt64LittleEndian(Take(8));

    internal bool Bool() => U8() != 0;

    internal string Str() => Encoding.UTF8.GetString(Take((int)U32()));

    /// <summary>
    /// An enum arrives as an ordinal into a fixed list, so a value outside the
    /// declared variants has no encoding and cannot be read back.
    /// </summary>
    internal string Enum(string[] variants)
    {
        byte ord = U8();
        if (ord >= variants.Length)
            throw new InvalidDataException($"enum ordinal {ord} out of range");
        return variants[ord];
    }

    internal string[] Tags()
    {
        uint n = U32();
        var tags = new string[n];
        for (uint i = 0; i < n; i++) tags[i] = Str();
        return tags;
    }

    internal long? OptionalI64() => U8() == 0 ? null : I64();
}

internal static class Wire
{
    internal static void PutU32(MemoryStream ms, uint v)
    {
        Span<byte> buf = stackalloc byte[4];
        BinaryPrimitives.WriteUInt32LittleEndian(buf, v);
        ms.Write(buf);
    }

    internal static void PutU64(MemoryStream ms, ulong v)
    {
        Span<byte> buf = stackalloc byte[8];
        BinaryPrimitives.WriteUInt64LittleEndian(buf, v);
        ms.Write(buf);
    }

    internal static void PutStr(MemoryStream ms, string s)
    {
        var body = Encoding.UTF8.GetBytes(s);
        PutU32(ms, (uint)body.Length);
        ms.Write(body);
    }

    internal static void PutEnum(MemoryStream ms, string[] variants, string name)
    {
        int ord = Array.IndexOf(variants, name);
        if (ord < 0)
            throw new InvalidOperationException($"\"{name}\" is not one of {string.Join(", ", variants)}");
        ms.WriteByte((byte)ord);
    }

    internal static void PutTags(MemoryStream ms, IReadOnlyList<string> tags)
    {
        PutU32(ms, (uint)tags.Count);
        foreach (var t in tags) PutStr(ms, t);
    }

    internal static void PutOptionalI64(MemoryStream ms, long? v)
    {
        if (v is null)
        {
            ms.WriteByte(0);
            return;
        }
        ms.WriteByte(1);
        PutU64(ms, unchecked((ulong)v.Value));
    }
}
