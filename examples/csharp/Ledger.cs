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

// LedgerEntry mirrors examples/cases/ledger.yaml.
//
// [SerifyModel] plus one [SerifyField] per property is the entire schema binding
// — nothing here calls a Get*/Set* accessor. Everything else is the byte layout,
// which is the part a conformance worker exists to exercise.
//
// .NET has native Int128 and BinaryPrimitives can write it directly, so the two
// int128 fields need no big-integer library — unlike Go (math/big) or Java
// (BigInteger).
//
// Go is the --ref language and owns the layout; see examples/go/wire.go.

using System;
using System.Buffers.Binary;
using System.IO;
using Serify;

[SerifyModel]
internal sealed class LedgerEntry
{
    [SerifyField] public ulong EntryId { get; set; }
    [SerifyField] public ulong BlockNumber { get; set; }
    [SerifyField] public long BlockTime { get; set; }
    [SerifyField] public byte[] TxHash { get; set; } = Array.Empty<byte>();
    [SerifyField] public string Account { get; set; } = "";
    [SerifyField] public string Asset { get; set; } = "";
    [SerifyField] public Int128 AmountBaseUnits { get; set; }
    [SerifyField] public Int128 BalanceAfter { get; set; }
    [SerifyField] public bool Confirmed { get; set; }
    [SerifyField] public string? Memo { get; set; }

    public byte[] Marshal()
    {
        var ms = new MemoryStream();
        Span<byte> buf = stackalloc byte[16];

        BinaryPrimitives.WriteUInt64LittleEndian(buf, EntryId);
        ms.Write(buf[..8]);
        BinaryPrimitives.WriteUInt64LittleEndian(buf, BlockNumber);
        ms.Write(buf[..8]);
        BinaryPrimitives.WriteInt64LittleEndian(buf, BlockTime);
        ms.Write(buf[..8]);

        Wire.WriteLenPrefixed(ms, TxHash);
        Wire.WriteLenPrefixed(ms, Account);
        Wire.WriteLenPrefixed(ms, Asset);

        BinaryPrimitives.WriteInt128LittleEndian(buf, AmountBaseUnits);
        ms.Write(buf);
        BinaryPrimitives.WriteInt128LittleEndian(buf, BalanceAfter);
        ms.Write(buf);

        ms.WriteByte(Confirmed ? (byte)1 : (byte)0);

        if (Memo is null)
        {
            ms.WriteByte(0);
        }
        else
        {
            ms.WriteByte(1);
            Wire.WriteLenPrefixed(ms, Memo);
        }

        return ms.ToArray();
    }

    public static LedgerEntry Unmarshal(byte[] data)
    {
        ReadOnlySpan<byte> span = data;
        var e = new LedgerEntry
        {
            EntryId     = BinaryPrimitives.ReadUInt64LittleEndian(span),
            BlockNumber = BinaryPrimitives.ReadUInt64LittleEndian(span[8..]),
            BlockTime   = BinaryPrimitives.ReadInt64LittleEndian(span[16..]),
        };
        int off = 24;

        // ToArray copies: a slice would alias the input buffer.
        e.TxHash  = Wire.ReadLenPrefixed(span, ref off).ToArray();
        e.Account = Wire.ReadLenString(span, ref off);
        e.Asset   = Wire.ReadLenString(span, ref off);

        e.AmountBaseUnits = BinaryPrimitives.ReadInt128LittleEndian(span[off..]);
        e.BalanceAfter    = BinaryPrimitives.ReadInt128LittleEndian(span[(off + 16)..]);
        off += 32;

        e.Confirmed = span[off] != 0;
        bool hasMemo = span[off + 1] != 0;
        off += 2;
        e.Memo = hasMemo ? Wire.ReadLenString(span, ref off) : null;

        return e;
    }
}
