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


// Framing: a u32 little-endian byte length, then that many bytes.
//
// Outside the conformance suite on purpose. serify tests the contents of a
// message; the frame header is in none of the cases, so a follower may read its
// socket however it likes as long as it agrees on what is inside.

using System;
using System.Buffers.Binary;
using System.IO;

internal static class Frame
{
    /// <summary>Caps a single message, so a bad length prefix is not a 4 GiB allocation.</summary>
    internal const uint MaxFrameLen = 1 << 20;

    internal static void Write(Stream stream, byte[] payload)
    {
        if (payload.Length > MaxFrameLen)
            throw new InvalidOperationException(
                $"message of {payload.Length} bytes exceeds the {MaxFrameLen} limit");

        var header = new byte[4];
        BinaryPrimitives.WriteUInt32LittleEndian(header, (uint)payload.Length);
        stream.Write(header);
        stream.Write(payload);
        stream.Flush();
    }

    internal static byte[] Read(Stream stream)
    {
        var header = new byte[4];
        ReadExact(stream, header, "frame header");
        uint n = BinaryPrimitives.ReadUInt32LittleEndian(header);
        if (n > MaxFrameLen)
            throw new InvalidDataException($"frame header claims {n} bytes, over the {MaxFrameLen} limit");

        var payload = new byte[n];
        if (n > 0) ReadExact(stream, payload, "frame body");
        return payload;
    }

    private static void ReadExact(Stream stream, byte[] dst, string what)
    {
        int got = 0;
        while (got < dst.Length)
        {
            int n = stream.Read(dst, got, dst.Length - got);
            if (n <= 0) throw new EndOfStreamException($"connection closed inside the {what}");
            got += n;
        }
    }
}
