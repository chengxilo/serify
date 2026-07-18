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

// SignalCapture mirrors examples/cases/signals.yaml, which uses every scalar the
// schema allows as a list element.
//
// C# has a distinct array type for each of them, so every field is the obvious
// T[] and the binding needs nothing declared beyond [SerifyField]. .NET's native
// Int128/UInt128 mean even the 128-bit lists are ordinary arrays.
//
// Go is the --ref language and owns the byte layout; see examples/go/wire.go.
// Each list is a u32 element count followed by its elements, little-endian.

using System;
using System.Buffers.Binary;
using System.IO;
using Serify;

[SerifyModel]
internal sealed class SignalCapture
{
    // Declaration order of the `mode` enum; the index is the wire ordinal.
    private static readonly string[] Modes = { "idle", "active", "fault", "calibrating" };

    [SerifyField("capture_id")] public ulong CaptureId { get; set; }
    [SerifyField] public bool[] Flags { get; set; } = Array.Empty<bool>();
    [SerifyField("raw_frame")] public byte[] RawFrame { get; set; } = Array.Empty<byte>();
    [SerifyField("port_numbers")] public ushort[] PortNumbers { get; set; } = Array.Empty<ushort>();
    [SerifyField("sample_counts")] public uint[] SampleCounts { get; set; } = Array.Empty<uint>();
    [SerifyField("byte_totals")] public ulong[] ByteTotals { get; set; } = Array.Empty<ulong>();
    [SerifyField("trim_offsets")] public sbyte[] TrimOffsets { get; set; } = Array.Empty<sbyte>();
    [SerifyField("drift_deltas")] public short[] DriftDeltas { get; set; } = Array.Empty<short>();
    [SerifyField("temperatures_c")] public int[] TemperaturesC { get; set; } = Array.Empty<int>();
    [SerifyField("timestamps_ns")] public long[] TimestampsNs { get; set; } = Array.Empty<long>();
    [SerifyField] public UInt128[] Counters { get; set; } = Array.Empty<UInt128>();
    [SerifyField] public Int128[] Balances { get; set; } = Array.Empty<Int128>();
    [SerifyField] public float[] Gains { get; set; } = Array.Empty<float>();
    [SerifyField] public double[] Voltages { get; set; } = Array.Empty<double>();
    [SerifyField("channel_names")] public string[] ChannelNames { get; set; } = Array.Empty<string>();
    [SerifyField] public byte[][] Payloads { get; set; } = Array.Empty<byte[]>();
    [SerifyField] public byte[] Checksum { get; set; } = new byte[4];
    [SerifyField] public short[] Window { get; set; } = new short[3];
    [SerifyField("dropped_frames")] public uint? DroppedFrames { get; set; }
    [SerifyField] public string Mode { get; set; } = "";

    /// <summary>Writes a list's u32 element count. Every list carries one, even when empty.</summary>
    private static void WriteCount(MemoryStream ms, int n)
    {
        Span<byte> b = stackalloc byte[4];
        BinaryPrimitives.WriteUInt32LittleEndian(b, (uint)n);
        ms.Write(b);
    }

    private static int ReadCount(ReadOnlySpan<byte> data, ref int off)
    {
        var n = (int)BinaryPrimitives.ReadUInt32LittleEndian(data[off..]);
        off += 4;
        return n;
    }

    public byte[] Marshal()
    {
        var ms = new MemoryStream();
        Span<byte> buf = stackalloc byte[16];

        BinaryPrimitives.WriteUInt64LittleEndian(buf, CaptureId);
        ms.Write(buf[..8]);

        WriteCount(ms, Flags.Length);
        foreach (var v in Flags) ms.WriteByte((byte)(v ? 1 : 0));

        WriteCount(ms, RawFrame.Length);
        ms.Write(RawFrame);

        WriteCount(ms, PortNumbers.Length);
        foreach (var v in PortNumbers) { BinaryPrimitives.WriteUInt16LittleEndian(buf, v); ms.Write(buf[..2]); }

        WriteCount(ms, SampleCounts.Length);
        foreach (var v in SampleCounts) { BinaryPrimitives.WriteUInt32LittleEndian(buf, v); ms.Write(buf[..4]); }

        WriteCount(ms, ByteTotals.Length);
        foreach (var v in ByteTotals) { BinaryPrimitives.WriteUInt64LittleEndian(buf, v); ms.Write(buf[..8]); }

        WriteCount(ms, TrimOffsets.Length);
        foreach (var v in TrimOffsets) ms.WriteByte((byte)v);

        WriteCount(ms, DriftDeltas.Length);
        foreach (var v in DriftDeltas) { BinaryPrimitives.WriteInt16LittleEndian(buf, v); ms.Write(buf[..2]); }

        WriteCount(ms, TemperaturesC.Length);
        foreach (var v in TemperaturesC) { BinaryPrimitives.WriteInt32LittleEndian(buf, v); ms.Write(buf[..4]); }

        WriteCount(ms, TimestampsNs.Length);
        foreach (var v in TimestampsNs) { BinaryPrimitives.WriteInt64LittleEndian(buf, v); ms.Write(buf[..8]); }

        WriteCount(ms, Counters.Length);
        foreach (var v in Counters) { BinaryPrimitives.WriteUInt128LittleEndian(buf, v); ms.Write(buf); }

        WriteCount(ms, Balances.Length);
        foreach (var v in Balances) { BinaryPrimitives.WriteInt128LittleEndian(buf, v); ms.Write(buf); }

        WriteCount(ms, Gains.Length);
        foreach (var v in Gains) { BinaryPrimitives.WriteSingleLittleEndian(buf, v); ms.Write(buf[..4]); }

        WriteCount(ms, Voltages.Length);
        foreach (var v in Voltages) { BinaryPrimitives.WriteDoubleLittleEndian(buf, v); ms.Write(buf[..8]); }

        WriteCount(ms, ChannelNames.Length);
        foreach (var v in ChannelNames) Wire.WriteLenPrefixed(ms, v);

        WriteCount(ms, Payloads.Length);
        foreach (var v in Payloads) Wire.WriteLenPrefixed(ms, v);

        // array<T,N> carries no count: N is fixed by the schema.
        ms.Write(Checksum);
        foreach (var v in Window) { BinaryPrimitives.WriteInt16LittleEndian(buf, v); ms.Write(buf[..2]); }

        // optional<uint32>: a presence flag, then the value if present.
        if (DroppedFrames is null) { ms.WriteByte(0); }
        else { ms.WriteByte(1); BinaryPrimitives.WriteUInt32LittleEndian(buf, DroppedFrames.Value); ms.Write(buf[..4]); }

        // enum: a u8 ordinal, the variant's position in the case file.
        ms.WriteByte((byte)Array.IndexOf(Modes, Mode));

        return ms.ToArray();
    }

    public static SignalCapture Unmarshal(byte[] data)
    {
        var s = new SignalCapture();
        ReadOnlySpan<byte> d = data;
        int off = 0;

        s.CaptureId = BinaryPrimitives.ReadUInt64LittleEndian(d[off..]);
        off += 8;

        var n = ReadCount(d, ref off);
        s.Flags = new bool[n];
        for (var i = 0; i < n; i++) s.Flags[i] = d[off + i] != 0;
        off += n;

        n = ReadCount(d, ref off);
        s.RawFrame = d.Slice(off, n).ToArray();
        off += n;

        n = ReadCount(d, ref off);
        s.PortNumbers = new ushort[n];
        for (var i = 0; i < n; i++) s.PortNumbers[i] = BinaryPrimitives.ReadUInt16LittleEndian(d[(off + i * 2)..]);
        off += n * 2;

        n = ReadCount(d, ref off);
        s.SampleCounts = new uint[n];
        for (var i = 0; i < n; i++) s.SampleCounts[i] = BinaryPrimitives.ReadUInt32LittleEndian(d[(off + i * 4)..]);
        off += n * 4;

        n = ReadCount(d, ref off);
        s.ByteTotals = new ulong[n];
        for (var i = 0; i < n; i++) s.ByteTotals[i] = BinaryPrimitives.ReadUInt64LittleEndian(d[(off + i * 8)..]);
        off += n * 8;

        n = ReadCount(d, ref off);
        s.TrimOffsets = new sbyte[n];
        for (var i = 0; i < n; i++) s.TrimOffsets[i] = (sbyte)d[off + i];
        off += n;

        n = ReadCount(d, ref off);
        s.DriftDeltas = new short[n];
        for (var i = 0; i < n; i++) s.DriftDeltas[i] = BinaryPrimitives.ReadInt16LittleEndian(d[(off + i * 2)..]);
        off += n * 2;

        n = ReadCount(d, ref off);
        s.TemperaturesC = new int[n];
        for (var i = 0; i < n; i++) s.TemperaturesC[i] = BinaryPrimitives.ReadInt32LittleEndian(d[(off + i * 4)..]);
        off += n * 4;

        n = ReadCount(d, ref off);
        s.TimestampsNs = new long[n];
        for (var i = 0; i < n; i++) s.TimestampsNs[i] = BinaryPrimitives.ReadInt64LittleEndian(d[(off + i * 8)..]);
        off += n * 8;

        n = ReadCount(d, ref off);
        s.Counters = new UInt128[n];
        for (var i = 0; i < n; i++) s.Counters[i] = BinaryPrimitives.ReadUInt128LittleEndian(d[(off + i * 16)..]);
        off += n * 16;

        n = ReadCount(d, ref off);
        s.Balances = new Int128[n];
        for (var i = 0; i < n; i++) s.Balances[i] = BinaryPrimitives.ReadInt128LittleEndian(d[(off + i * 16)..]);
        off += n * 16;

        n = ReadCount(d, ref off);
        s.Gains = new float[n];
        for (var i = 0; i < n; i++) s.Gains[i] = BinaryPrimitives.ReadSingleLittleEndian(d[(off + i * 4)..]);
        off += n * 4;

        n = ReadCount(d, ref off);
        s.Voltages = new double[n];
        for (var i = 0; i < n; i++) s.Voltages[i] = BinaryPrimitives.ReadDoubleLittleEndian(d[(off + i * 8)..]);
        off += n * 8;

        n = ReadCount(d, ref off);
        s.ChannelNames = new string[n];
        for (var i = 0; i < n; i++) s.ChannelNames[i] = Wire.ReadLenString(d, ref off);

        n = ReadCount(d, ref off);
        s.Payloads = new byte[n][];
        for (var i = 0; i < n; i++) s.Payloads[i] = Wire.ReadLenPrefixed(d, ref off).ToArray();

        s.Checksum = d.Slice(off, 4).ToArray();
        off += 4;
        s.Window = new short[3];
        for (var i = 0; i < 3; i++) s.Window[i] = BinaryPrimitives.ReadInt16LittleEndian(d[(off + i * 2)..]);
        off += 6;

        if (d[off] == 0) { s.DroppedFrames = null; off += 1; }
        else { s.DroppedFrames = BinaryPrimitives.ReadUInt32LittleEndian(d[(off + 1)..]); off += 5; }

        s.Mode = Modes[d[off]];
        off += 1;

        return s;
    }
}
