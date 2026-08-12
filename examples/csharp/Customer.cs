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

// `CustomerRecord` mirrors examples/cases/customer.yaml — a store account.
//
// `Address` and `Money` mirror the reusable address.yaml and money.yaml it
// imports; Order.cs reuses both, as the Go worker does.
//
// This is the only type in the suite carrying two formats, and the second one
// is the point: `binary` is a layout written by hand below, `json` goes through
// System.Text.Json, so the two fail in completely different ways. Both declare
// `oracle: semantic`, so what has to match the reference is the decoded value
// rather than the bytes — Go's encoder HTML-escapes `<`, `>` and `&` and always
// escapes U+2028/U+2029, and under a byte oracle every worker would have to
// reproduce that quirk.
//
// It is also the first C# model with nested structs. Nothing here says so: a
// property typed with another [SerifyModel] is a struct, an array of them a
// list<struct>, and a Dictionary of them a map<K,struct>.
//
// Go is the --ref language and owns the byte layout; see examples/go/wire.go.

using System;
using System.Buffers.Binary;
using System.Collections.Generic;
using System.IO;
using System.Text.Json;
using System.Text.Json.Nodes;
using Serify;

[SerifyModel]
internal sealed class Address
{
    [SerifyField] public string Recipient { get; set; } = "";
    [SerifyField] public string Street { get; set; } = "";
    [SerifyField] public string City { get; set; } = "";
    [SerifyField] public string Country { get; set; } = "";
    [SerifyField("postal_code")] public string PostalCode { get; set; } = "";

    /// <summary>A struct is its fields back to back, in schema order.</summary>
    public void Pack(MemoryStream ms)
    {
        Wire.WriteLenPrefixed(ms, Recipient);
        Wire.WriteLenPrefixed(ms, Street);
        Wire.WriteLenPrefixed(ms, City);
        Wire.WriteLenPrefixed(ms, Country);
        Wire.WriteLenPrefixed(ms, PostalCode);
    }

    public static Address Unpack(ReadOnlySpan<byte> d, ref int off) => new()
    {
        Recipient = Wire.ReadLenString(d, ref off),
        Street = Wire.ReadLenString(d, ref off),
        City = Wire.ReadLenString(d, ref off),
        Country = Wire.ReadLenString(d, ref off),
        PostalCode = Wire.ReadLenString(d, ref off),
    };

    public JsonObject ToJson() => new()
    {
        ["recipient"] = Recipient, ["street"] = Street, ["city"] = City,
        ["country"] = Country, ["postal_code"] = PostalCode,
    };

    public static Address FromJson(JsonNode? n) => new()
    {
        Recipient = (string?)n!["recipient"] ?? "",
        Street = (string?)n["street"] ?? "",
        City = (string?)n["city"] ?? "",
        Country = (string?)n["country"] ?? "",
        PostalCode = (string?)n["postal_code"] ?? "",
    };
}

[SerifyModel]
internal sealed class Money
{
    [SerifyField] public string Currency { get; set; } = "";
    [SerifyField("amount_minor")] public long AmountMinor { get; set; }

    public void Pack(MemoryStream ms)
    {
        Wire.WriteLenPrefixed(ms, Currency);
        Span<byte> buf = stackalloc byte[8];
        BinaryPrimitives.WriteInt64LittleEndian(buf, AmountMinor);
        ms.Write(buf);
    }

    public static Money Unpack(ReadOnlySpan<byte> d, ref int off)
    {
        var m = new Money { Currency = Wire.ReadLenString(d, ref off) };
        m.AmountMinor = BinaryPrimitives.ReadInt64LittleEndian(d[off..]);
        off += 8;
        return m;
    }

    public JsonObject ToJson() => new() { ["currency"] = Currency, ["amount_minor"] = AmountMinor };

    public static Money FromJson(JsonNode? n) => new()
    {
        Currency = (string?)n!["currency"] ?? "",
        AmountMinor = (long)n["amount_minor"]!,
    };
}

[SerifyModel]
internal sealed class CustomerRecord
{
    [SerifyField("customer_id")] public ulong CustomerId { get; set; }
    [SerifyField] public string Email { get; set; } = "";
    [SerifyField("display_name")] public string DisplayName { get; set; } = "";
    [SerifyField] public byte Age { get; set; }
    [SerifyField("email_verified")] public bool EmailVerified { get; set; }
    [SerifyField("fraud_score")] public float FraudScore { get; set; }
    [SerifyField("loyalty_points")] public uint LoyaltyPoints { get; set; }
    [SerifyField("signup_ts")] public long SignupTs { get; set; }
    [SerifyField("avatar_sha256")] public byte[] AvatarSha256 { get; set; } = Array.Empty<byte>();
    [SerifyField] public byte[] Pin { get; set; } = new byte[4];
    [SerifyField("referral_code")] public string? ReferralCode { get; set; }
    [SerifyField("store_credit")] public Money StoreCredit { get; set; } = new();
    [SerifyField("shipping_addresses")] public Address[] ShippingAddresses { get; set; } = Array.Empty<Address>();
    [SerifyField("address_book")] public Dictionary<string, Address> AddressBook { get; set; } = new();
    [SerifyField("wishlist_skus")] public string[] WishlistSkus { get; set; } = Array.Empty<string>();
    [SerifyField] public Dictionary<string, string> Preferences { get; set; } = new();

    private static void WriteCount(MemoryStream ms, int n)
    {
        Span<byte> buf = stackalloc byte[4];
        BinaryPrimitives.WriteUInt32LittleEndian(buf, (uint)n);
        ms.Write(buf);
    }

    public byte[] Marshal()
    {
        var ms = new MemoryStream();
        Span<byte> buf = stackalloc byte[8];

        BinaryPrimitives.WriteUInt64LittleEndian(buf, CustomerId);
        ms.Write(buf);
        Wire.WriteLenPrefixed(ms, Email);
        Wire.WriteLenPrefixed(ms, DisplayName);

        ms.WriteByte(Age);
        ms.WriteByte((byte)(EmailVerified ? 1 : 0));
        BinaryPrimitives.WriteSingleLittleEndian(buf, FraudScore);
        ms.Write(buf[..4]);
        BinaryPrimitives.WriteUInt32LittleEndian(buf, LoyaltyPoints);
        ms.Write(buf[..4]);
        BinaryPrimitives.WriteInt64LittleEndian(buf, SignupTs);
        ms.Write(buf);

        Wire.WriteLenPrefixed(ms, AvatarSha256);

        // array<T,N> carries no count: N is fixed by the schema.
        ms.Write(Pin);

        // optional<string>: a presence flag, then the value if present. An empty
        // string is present, which is why the flag cannot be inferred from it.
        if (ReferralCode is null) { ms.WriteByte(0); }
        else { ms.WriteByte(1); Wire.WriteLenPrefixed(ms, ReferralCode); }

        StoreCredit.Pack(ms);

        WriteCount(ms, ShippingAddresses.Length);
        foreach (var a in ShippingAddresses) a.Pack(ms);

        // Entry order is the dictionary's own — deliberately not sorted. A map
        // is unordered, so customer declares `oracle: semantic` and the decoded
        // value is what gets compared. See docs/protocol.md.
        WriteCount(ms, AddressBook.Count);
        foreach (var (k, a) in AddressBook) { Wire.WriteLenPrefixed(ms, k); a.Pack(ms); }

        WriteCount(ms, WishlistSkus.Length);
        foreach (var s in WishlistSkus) Wire.WriteLenPrefixed(ms, s);

        WriteCount(ms, Preferences.Count);
        foreach (var (k, v) in Preferences)
        {
            Wire.WriteLenPrefixed(ms, k);
            Wire.WriteLenPrefixed(ms, v);
        }

        return ms.ToArray();
    }

    public static CustomerRecord Unmarshal(byte[] data)
    {
        var c = new CustomerRecord();
        ReadOnlySpan<byte> d = data;
        var off = 0;

        c.CustomerId = BinaryPrimitives.ReadUInt64LittleEndian(d);
        off += 8;
        c.Email = Wire.ReadLenString(d, ref off);
        c.DisplayName = Wire.ReadLenString(d, ref off);

        c.Age = d[off];
        c.EmailVerified = d[off + 1] != 0;
        c.FraudScore = BinaryPrimitives.ReadSingleLittleEndian(d[(off + 2)..]);
        c.LoyaltyPoints = BinaryPrimitives.ReadUInt32LittleEndian(d[(off + 6)..]);
        c.SignupTs = BinaryPrimitives.ReadInt64LittleEndian(d[(off + 10)..]);
        off += 18;

        c.AvatarSha256 = Wire.ReadLenPrefixed(d, ref off).ToArray();

        c.Pin = d.Slice(off, 4).ToArray();
        off += 4;

        if (d[off++] == 0) { c.ReferralCode = null; }
        else { c.ReferralCode = Wire.ReadLenString(d, ref off); }

        c.StoreCredit = Money.Unpack(d, ref off);

        c.ShippingAddresses = new Address[ReadCount(d, ref off)];
        for (var i = 0; i < c.ShippingAddresses.Length; i++) c.ShippingAddresses[i] = Address.Unpack(d, ref off);

        c.AddressBook = new Dictionary<string, Address>();
        for (var n = ReadCount(d, ref off); n > 0; n--)
        {
            var k = Wire.ReadLenString(d, ref off);
            c.AddressBook[k] = Address.Unpack(d, ref off);
        }

        c.WishlistSkus = new string[ReadCount(d, ref off)];
        for (var i = 0; i < c.WishlistSkus.Length; i++) c.WishlistSkus[i] = Wire.ReadLenString(d, ref off);

        c.Preferences = new Dictionary<string, string>();
        for (var n = ReadCount(d, ref off); n > 0; n--)
        {
            var k = Wire.ReadLenString(d, ref off);
            c.Preferences[k] = Wire.ReadLenString(d, ref off);
        }

        return c;
    }

    private static int ReadCount(ReadOnlySpan<byte> d, ref int off)
    {
        var n = (int)BinaryPrimitives.ReadUInt32LittleEndian(d[off..]);
        off += 4;
        return n;
    }

    /// <summary>
    /// `bytes` is base64 and the 64-bit integers are JSON numbers: that is what
    /// the reference worker's `[]byte` and `uint64` marshal to, and the semantic
    /// oracle decodes our output with it.
    /// </summary>
    public byte[] ToJson()
    {
        var addrs = new JsonArray();
        foreach (var a in ShippingAddresses) addrs.Add(a.ToJson());

        var book = new JsonObject();
        foreach (var (k, a) in AddressBook) book[k] = a.ToJson();

        var skus = new JsonArray();
        foreach (var s in WishlistSkus) skus.Add(s);

        var prefs = new JsonObject();
        foreach (var (k, v) in Preferences) prefs[k] = v;

        var pin = new JsonArray();
        foreach (var b in Pin) pin.Add(b);

        var o = new JsonObject
        {
            ["customer_id"] = CustomerId,
            ["email"] = Email,
            ["display_name"] = DisplayName,
            ["age"] = Age,
            ["email_verified"] = EmailVerified,
            ["fraud_score"] = FraudScore,
            ["loyalty_points"] = LoyaltyPoints,
            ["signup_ts"] = SignupTs,
            ["avatar_sha256"] = Convert.ToBase64String(AvatarSha256),
            ["pin"] = pin,
            ["referral_code"] = ReferralCode,
            ["store_credit"] = StoreCredit.ToJson(),
            ["shipping_addresses"] = addrs,
            ["address_book"] = book,
            ["wishlist_skus"] = skus,
            ["preferences"] = prefs,
        };
        return JsonSerializer.SerializeToUtf8Bytes(o);
    }

    public static CustomerRecord FromJson(byte[] data)
    {
        var o = JsonNode.Parse(data)!;
        var c = new CustomerRecord
        {
            CustomerId = (ulong)o["customer_id"]!,
            Email = (string?)o["email"] ?? "",
            DisplayName = (string?)o["display_name"] ?? "",
            Age = (byte)o["age"]!,
            EmailVerified = (bool)o["email_verified"]!,
            FraudScore = (float)o["fraud_score"]!,
            LoyaltyPoints = (uint)o["loyalty_points"]!,
            SignupTs = (long)o["signup_ts"]!,
            AvatarSha256 = Convert.FromBase64String((string?)o["avatar_sha256"] ?? ""),
            ReferralCode = (string?)o["referral_code"],
            StoreCredit = Money.FromJson(o["store_credit"]),
        };

        var pin = o["pin"]!.AsArray();
        c.Pin = new byte[pin.Count];
        for (var i = 0; i < pin.Count; i++) c.Pin[i] = (byte)pin[i]!;

        var addrs = o["shipping_addresses"]!.AsArray();
        c.ShippingAddresses = new Address[addrs.Count];
        for (var i = 0; i < addrs.Count; i++) c.ShippingAddresses[i] = Address.FromJson(addrs[i]);

        foreach (var kv in o["address_book"]!.AsObject()) c.AddressBook[kv.Key] = Address.FromJson(kv.Value);

        var skus = o["wishlist_skus"]!.AsArray();
        c.WishlistSkus = new string[skus.Count];
        for (var i = 0; i < skus.Count; i++) c.WishlistSkus[i] = (string?)skus[i] ?? "";

        foreach (var kv in o["preferences"]!.AsObject()) c.Preferences[kv.Key] = (string?)kv.Value ?? "";

        return c;
    }
}
