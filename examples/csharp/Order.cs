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

// `OrderRecord` mirrors examples/cases/order.yaml — a placed order.
//
// `LineItem` mirrors the reusable line_item.yaml it imports, which itself nests
// money — so any type using it exercises struct-inside-struct. `Address` and
// `Money` come from Customer.cs, as they do in the Go worker.
//
// Between them the fields cover the four composite types nothing else in the
// suite exercises end to end: an `enum`, a `list<struct>`, a
// `map<string,struct>` and an `optional<struct>`. BillingAddress is the suite's
// only optional<struct>, so it is what proves the binding's nested-model path
// hands back a null as well as a value.
//
// An enum needs nothing from the binding: it travels as its variant *name*, so
// the property is a plain string. The u8 ordinal in the layout is this worker's
// own choice, which is why Statuses has to match the case file's declaration
// order.
//
// Go is the --ref language and owns the layout; see examples/go/wire.go.

using System;
using System.Buffers.Binary;
using System.Collections.Generic;
using System.IO;
using Serify;

[SerifyModel]
internal sealed class LineItem
{
    [SerifyField] public string Sku { get; set; } = "";
    [SerifyField("product_name")] public string ProductName { get; set; } = "";
    [SerifyField] public ushort Quantity { get; set; }
    [SerifyField("unit_price")] public Money UnitPrice { get; set; } = new();
    [SerifyField("discount_pct")] public byte DiscountPct { get; set; }
    [SerifyField("gift_wrap")] public bool GiftWrap { get; set; }

    public void Pack(MemoryStream ms)
    {
        Wire.WriteLenPrefixed(ms, Sku);
        Wire.WriteLenPrefixed(ms, ProductName);
        Span<byte> buf = stackalloc byte[2];
        BinaryPrimitives.WriteUInt16LittleEndian(buf, Quantity);
        ms.Write(buf);
        UnitPrice.Pack(ms);
        ms.WriteByte(DiscountPct);
        ms.WriteByte((byte)(GiftWrap ? 1 : 0));
    }

    public static LineItem Unpack(ReadOnlySpan<byte> d, ref int off)
    {
        var it = new LineItem
        {
            Sku = Wire.ReadLenString(d, ref off),
            ProductName = Wire.ReadLenString(d, ref off),
        };
        it.Quantity = BinaryPrimitives.ReadUInt16LittleEndian(d[off..]);
        off += 2;
        it.UnitPrice = Money.Unpack(d, ref off);
        it.DiscountPct = d[off];
        it.GiftWrap = d[off + 1] != 0;
        off += 2;
        return it;
    }
}

[SerifyModel]
internal sealed class OrderRecord
{
    /// <summary>Declaration order of the `status` enum in examples/cases/order.yaml.</summary>
    private static readonly string[] Statuses =
        ["pending", "paid", "shipped", "delivered", "cancelled"];

    [SerifyField("order_id")] public ulong OrderId { get; set; }
    [SerifyField("customer_id")] public ulong CustomerId { get; set; }
    [SerifyField("created_at")] public long CreatedAt { get; set; }
    [SerifyField] public string Status { get; set; } = "";
    [SerifyField] public LineItem[] Items { get; set; } = Array.Empty<LineItem>();
    [SerifyField] public Money Subtotal { get; set; } = new();
    [SerifyField] public Dictionary<string, Money> Adjustments { get; set; } = new();
    [SerifyField] public Money Total { get; set; } = new();
    [SerifyField("shipping_address")] public Address ShippingAddress { get; set; } = new();
    [SerifyField("billing_address")] public Address? BillingAddress { get; set; }
    [SerifyField("coupon_codes")] public string[] CouponCodes { get; set; } = Array.Empty<string>();
    [SerifyField("tracking_number")] public string? TrackingNumber { get; set; }

    private static void WriteCount(MemoryStream ms, int n)
    {
        Span<byte> buf = stackalloc byte[4];
        BinaryPrimitives.WriteUInt32LittleEndian(buf, (uint)n);
        ms.Write(buf);
    }

    private static int ReadCount(ReadOnlySpan<byte> d, ref int off)
    {
        var n = (int)BinaryPrimitives.ReadUInt32LittleEndian(d[off..]);
        off += 4;
        return n;
    }

    public byte[] Marshal()
    {
        var ms = new MemoryStream();
        Span<byte> buf = stackalloc byte[8];

        BinaryPrimitives.WriteUInt64LittleEndian(buf, OrderId);
        ms.Write(buf);
        BinaryPrimitives.WriteUInt64LittleEndian(buf, CustomerId);
        ms.Write(buf);
        BinaryPrimitives.WriteInt64LittleEndian(buf, CreatedAt);
        ms.Write(buf);

        // enum: a u8 ordinal, the variant's position in the case file.
        var ord = Array.IndexOf(Statuses, Status);
        if (ord < 0) throw new InvalidOperationException($"unknown order status \"{Status}\"");
        ms.WriteByte((byte)ord);

        WriteCount(ms, Items.Length);
        foreach (var it in Items) it.Pack(ms);

        Subtotal.Pack(ms);

        // Entry order is the dictionary's own — deliberately not sorted. A map
        // is unordered, so order declares `oracle: semantic` and the decoded
        // value is what gets compared. See docs/protocol.md.
        WriteCount(ms, Adjustments.Count);
        foreach (var (k, m) in Adjustments) { Wire.WriteLenPrefixed(ms, k); m.Pack(ms); }

        Total.Pack(ms);
        ShippingAddress.Pack(ms);

        // optional<struct>: a presence flag, then the struct's fields inline.
        if (BillingAddress is null) { ms.WriteByte(0); }
        else { ms.WriteByte(1); BillingAddress.Pack(ms); }

        WriteCount(ms, CouponCodes.Length);
        foreach (var c in CouponCodes) Wire.WriteLenPrefixed(ms, c);

        if (TrackingNumber is null) { ms.WriteByte(0); }
        else { ms.WriteByte(1); Wire.WriteLenPrefixed(ms, TrackingNumber); }

        return ms.ToArray();
    }

    public static OrderRecord Unmarshal(byte[] data)
    {
        var o = new OrderRecord();
        ReadOnlySpan<byte> d = data;

        o.OrderId = BinaryPrimitives.ReadUInt64LittleEndian(d);
        o.CustomerId = BinaryPrimitives.ReadUInt64LittleEndian(d[8..]);
        o.CreatedAt = BinaryPrimitives.ReadInt64LittleEndian(d[16..]);

        var ord = d[24];
        if (ord >= Statuses.Length) throw new InvalidOperationException($"status ordinal {ord} is out of range");
        o.Status = Statuses[ord];
        var off = 25;

        o.Items = new LineItem[ReadCount(d, ref off)];
        for (var i = 0; i < o.Items.Length; i++) o.Items[i] = LineItem.Unpack(d, ref off);

        o.Subtotal = Money.Unpack(d, ref off);

        o.Adjustments = new Dictionary<string, Money>();
        for (var n = ReadCount(d, ref off); n > 0; n--)
        {
            var k = Wire.ReadLenString(d, ref off);
            o.Adjustments[k] = Money.Unpack(d, ref off);
        }

        o.Total = Money.Unpack(d, ref off);
        o.ShippingAddress = Address.Unpack(d, ref off);

        o.BillingAddress = d[off++] == 0 ? null : Address.Unpack(d, ref off);

        o.CouponCodes = new string[ReadCount(d, ref off)];
        for (var i = 0; i < o.CouponCodes.Length; i++) o.CouponCodes[i] = Wire.ReadLenString(d, ref off);

        o.TrackingNumber = d[off++] == 0 ? null : Wire.ReadLenString(d, ref off);

        return o;
    }
}
