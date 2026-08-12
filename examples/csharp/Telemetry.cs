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

// `TelemetryFrame` mirrors examples/cases/telemetry.yaml — one reading from a
// field device.
//
// This is the type that covers the corners the other examples do not: a
// `uint128` address, two differently shaped fixed arrays, the suite's only
// `optional<scalar>`, a `map<string,uint64>`, and float cases running through
// NaN, ±Inf and negative zero. Only `binary` is declared, because NaN and Inf
// have no JSON spelling.
//
// C# is the language where the declared types carry the most: every schema
// width has an exact counterpart, down to UInt128 and the nullable float that
// spells `optional<float32>`, so the property types alone say what the wire
// holds and the layout below only has to say in what order.
//
// Go is the --ref language and owns that layout; see examples/go/wire.go.

using System;
using System.Buffers.Binary;
using System.Collections.Generic;
using System.IO;
using Serify;

[SerifyModel]
internal sealed class TelemetryFrame
{
    [SerifyField("device_id")] public ulong DeviceId { get; set; }
    [SerifyField] public UInt128 Ipv6 { get; set; }
    [SerifyField("local_ip")] public byte[] LocalIp { get; set; } = new byte[4];
    [SerifyField] public string Firmware { get; set; } = "";
    [SerifyField("boot_count")] public ushort BootCount { get; set; }
    [SerifyField("rssi_dbm")] public sbyte RssiDbm { get; set; }
    [SerifyField("temperature_dc")] public short TemperatureDc { get; set; }
    [SerifyField("clock_drift_ms")] public int ClockDriftMs { get; set; }
    [SerifyField("battery_volts")] public float BatteryVolts { get; set; }
    [SerifyField] public double Latitude { get; set; }
    [SerifyField] public double Longitude { get; set; }
    [SerifyField("humidity_pct")] public float? HumidityPct { get; set; }
    [SerifyField("accel_mg")] public short[] AccelMg { get; set; } = new short[3];
    [SerifyField("visible_cells")] public uint[] VisibleCells { get; set; } = Array.Empty<uint>();
    [SerifyField("packet_counts")] public Dictionary<string, ulong> PacketCounts { get; set; } = new();
    [SerifyField("gps_fix")] public bool GpsFix { get; set; }
    [SerifyField] public byte[] Signature { get; set; } = Array.Empty<byte>();

    public byte[] Marshal()
    {
        var ms = new MemoryStream();
        Span<byte> buf = stackalloc byte[16];

        BinaryPrimitives.WriteUInt64LittleEndian(buf, DeviceId);
        ms.Write(buf[..8]);

        // uint128 is unsigned, so the same 16 little-endian bytes int128 uses
        // serve here with no sign to re-apply on the way back.
        BinaryPrimitives.WriteUInt128LittleEndian(buf, Ipv6);
        ms.Write(buf);

        // array<T,N> carries no count: N is fixed by the schema.
        ms.Write(LocalIp);

        Wire.WriteLenPrefixed(ms, Firmware);

        BinaryPrimitives.WriteUInt16LittleEndian(buf, BootCount);
        ms.Write(buf[..2]);
        ms.WriteByte((byte)RssiDbm);
        BinaryPrimitives.WriteInt16LittleEndian(buf, TemperatureDc);
        ms.Write(buf[..2]);
        BinaryPrimitives.WriteInt32LittleEndian(buf, ClockDriftMs);
        ms.Write(buf[..4]);
        BinaryPrimitives.WriteSingleLittleEndian(buf, BatteryVolts);
        ms.Write(buf[..4]);
        BinaryPrimitives.WriteDoubleLittleEndian(buf, Latitude);
        ms.Write(buf[..8]);
        BinaryPrimitives.WriteDoubleLittleEndian(buf, Longitude);
        ms.Write(buf[..8]);

        // optional<float32>: a presence flag, then the value if present.
        if (HumidityPct is null) { ms.WriteByte(0); }
        else
        {
            ms.WriteByte(1);
            BinaryPrimitives.WriteSingleLittleEndian(buf, HumidityPct.Value);
            ms.Write(buf[..4]);
        }

        foreach (var v in AccelMg) { BinaryPrimitives.WriteInt16LittleEndian(buf, v); ms.Write(buf[..2]); }

        BinaryPrimitives.WriteUInt32LittleEndian(buf, (uint)VisibleCells.Length);
        ms.Write(buf[..4]);
        foreach (var v in VisibleCells) { BinaryPrimitives.WriteUInt32LittleEndian(buf, v); ms.Write(buf[..4]); }

        // Entry order is the dictionary's own — deliberately not sorted. A map
        // is unordered, so telemetry declares `oracle: semantic` and the decoded
        // value is what gets compared. See docs/protocol.md.
        BinaryPrimitives.WriteUInt32LittleEndian(buf, (uint)PacketCounts.Count);
        ms.Write(buf[..4]);
        foreach (var (k, v) in PacketCounts)
        {
            Wire.WriteLenPrefixed(ms, k);
            BinaryPrimitives.WriteUInt64LittleEndian(buf, v);
            ms.Write(buf[..8]);
        }

        ms.WriteByte((byte)(GpsFix ? 1 : 0));
        Wire.WriteLenPrefixed(ms, Signature);

        return ms.ToArray();
    }

    public static TelemetryFrame Unmarshal(byte[] data)
    {
        var t = new TelemetryFrame();
        ReadOnlySpan<byte> d = data;
        var off = 0;

        t.DeviceId = BinaryPrimitives.ReadUInt64LittleEndian(d[off..]);
        off += 8;
        t.Ipv6 = BinaryPrimitives.ReadUInt128LittleEndian(d[off..]);
        off += 16;

        t.LocalIp = d.Slice(off, 4).ToArray();
        off += 4;

        t.Firmware = Wire.ReadLenString(d, ref off);

        t.BootCount = BinaryPrimitives.ReadUInt16LittleEndian(d[off..]);
        t.RssiDbm = (sbyte)d[off + 2];
        t.TemperatureDc = BinaryPrimitives.ReadInt16LittleEndian(d[(off + 3)..]);
        t.ClockDriftMs = BinaryPrimitives.ReadInt32LittleEndian(d[(off + 5)..]);
        t.BatteryVolts = BinaryPrimitives.ReadSingleLittleEndian(d[(off + 9)..]);
        t.Latitude = BinaryPrimitives.ReadDoubleLittleEndian(d[(off + 13)..]);
        t.Longitude = BinaryPrimitives.ReadDoubleLittleEndian(d[(off + 21)..]);
        off += 29;

        if (d[off] == 0) { t.HumidityPct = null; off += 1; }
        else { t.HumidityPct = BinaryPrimitives.ReadSingleLittleEndian(d[(off + 1)..]); off += 5; }

        t.AccelMg = new short[3];
        for (var i = 0; i < 3; i++) t.AccelMg[i] = BinaryPrimitives.ReadInt16LittleEndian(d[(off + i * 2)..]);
        off += 6;

        var cells = (int)BinaryPrimitives.ReadUInt32LittleEndian(d[off..]);
        off += 4;
        t.VisibleCells = new uint[cells];
        for (var i = 0; i < cells; i++) t.VisibleCells[i] = BinaryPrimitives.ReadUInt32LittleEndian(d[(off + i * 4)..]);
        off += cells * 4;

        var entries = (int)BinaryPrimitives.ReadUInt32LittleEndian(d[off..]);
        off += 4;
        t.PacketCounts = new Dictionary<string, ulong>(entries);
        for (var i = 0; i < entries; i++)
        {
            var k = Wire.ReadLenString(d, ref off);
            t.PacketCounts[k] = BinaryPrimitives.ReadUInt64LittleEndian(d[off..]);
            off += 8;
        }

        t.GpsFix = d[off] != 0;
        off += 1;

        t.Signature = Wire.ReadLenPrefixed(d, ref off).ToArray();

        return t;
    }
}
