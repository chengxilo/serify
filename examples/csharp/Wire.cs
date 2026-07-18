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

// Byte-level primitives shared by the models in this worker.
//
// Go is the --ref language and owns the layout these reproduce; see the comment
// at the top of examples/go/wire.go.

using System;
using System.Buffers.Binary;
using System.IO;
using System.Text;

internal static class Wire
{
    internal static void WriteLenPrefixed(MemoryStream ms, ReadOnlySpan<byte> body)
    {
        Span<byte> len = stackalloc byte[4];
        BinaryPrimitives.WriteUInt32LittleEndian(len, (uint)body.Length);
        ms.Write(len);
        ms.Write(body);
    }

    internal static void WriteLenPrefixed(MemoryStream ms, string s) =>
        WriteLenPrefixed(ms, Encoding.UTF8.GetBytes(s));

    internal static ReadOnlySpan<byte> ReadLenPrefixed(ReadOnlySpan<byte> data, ref int off)
    {
        int n = (int)BinaryPrimitives.ReadUInt32LittleEndian(data[off..]);
        off += 4;
        var body = data.Slice(off, n);
        off += n;
        return body;
    }

    internal static string ReadLenString(ReadOnlySpan<byte> data, ref int off) =>
        Encoding.UTF8.GetString(ReadLenPrefixed(data, ref off));
}
