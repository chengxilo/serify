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

// NotificationRecord mirrors examples/cases/notification.yaml, whose `channel`
// field is a `oneof`.
//
// C# has no discriminated union keyword, but an abstract record with a private
// constructor and nested sealed records is the closed hierarchy that stands in
// for one — only the four types below can be a Channel — and that is all the
// binding needs. No converter, no registration.
//
// Go is the --ref language and owns the byte layout; see examples/go/wire.go.

using System;
using System.Buffers.Binary;
using System.IO;
using Serify;

/// <summary>The `oneof` from the case file.</summary>
internal abstract record Channel
{
    internal sealed record Silent : Channel;                            // arity 0 — a unit variant
    internal sealed record Sms(string Value) : Channel;                 // arity 1 — a scalar payload
    internal sealed record Push(ulong Value) : Channel;                 // arity 1 — exceeds 2^53
    internal sealed record Invoice(string Currency, long AmountMinor)   // arity N — a struct payload
        : Channel;

    private Channel() { }   // only the nested records above can derive
}

[SerifyModel]
internal sealed class NotificationRecord
{
    [SerifyField] public uint NotificationId { get; set; }
    [SerifyField] public Channel Channel { get; set; } = new Channel.Silent();
    [SerifyField] public bool Urgent { get; set; }

    public byte[] Marshal()
    {
        using var ms = new MemoryStream();
        Span<byte> buf = stackalloc byte[8];

        BinaryPrimitives.WriteUInt32LittleEndian(buf, NotificationId);
        ms.Write(buf[..4]);

        // The tag ordinal is the variant's position in the case file's oneof,
        // which is the declaration order of the four arms above. The schema tag
        // *names* are the binding's business, and never appear here.
        switch (Channel)
        {
            case Channel.Silent:
                ms.WriteByte(0);    // a unit variant is nothing but its tag
                break;
            case Channel.Sms(var s):
                ms.WriteByte(1);
                Wire.WriteLenPrefixed(ms, s);
                break;
            case Channel.Push(var n):
                ms.WriteByte(2);
                BinaryPrimitives.WriteUInt64LittleEndian(buf, n);
                ms.Write(buf);
                break;
            case Channel.Invoice(var currency, var amount):
                ms.WriteByte(3);
                Wire.WriteLenPrefixed(ms, currency);
                BinaryPrimitives.WriteInt64LittleEndian(buf, amount);
                ms.Write(buf);
                break;
        }

        ms.WriteByte(Urgent ? (byte)1 : (byte)0);
        return ms.ToArray();
    }

    public static NotificationRecord Unmarshal(byte[] data)
    {
        ReadOnlySpan<byte> span = data;
        var n = new NotificationRecord { NotificationId = BinaryPrimitives.ReadUInt32LittleEndian(span) };
        int off = 5;

        n.Channel = span[4] switch
        {
            0 => new Channel.Silent(),
            1 => new Channel.Sms(Wire.ReadLenString(span, ref off)),
            2 => new Channel.Push(ReadU64(span, ref off)),
            3 => new Channel.Invoice(Wire.ReadLenString(span, ref off), (long)ReadU64(span, ref off)),
            var ordinal => throw new InvalidOperationException($"unknown channel ordinal {ordinal}"),
        };

        n.Urgent = span[off] != 0;
        return n;
    }

    private static ulong ReadU64(ReadOnlySpan<byte> span, ref int off)
    {
        var v = BinaryPrimitives.ReadUInt64LittleEndian(span[off..]);
        off += 8;
        return v;
    }
}
