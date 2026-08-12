/**
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

/**
 * `CustomerRecord` mirrors examples/cases/customer.yaml — a store account.
 *
 * `Address` and `Money` mirror the reusable address.yaml and money.yaml it
 * imports; order.ts reuses both, as the Go worker does.
 *
 * This is the only type in the suite carrying two formats, and the second one
 * is the point: `binary` is a layout written by hand below, `json` goes through
 * `JSON.stringify`, so the two fail in completely different ways. Both declare
 * `oracle: semantic`, so what has to match is the decoded value rather than the
 * bytes — Go's encoder HTML-escapes `<`, `>` and `&` and always escapes
 * U+2028/U+2029, and under a byte oracle every worker would have to reproduce
 * that.
 *
 * It is also the first model here with nested structs, and `model:` on those
 * fields is what the binding needs to hand them back as `Address` and `Money`
 * rather than as raw FieldMaps: TypeScript has erased the property type by the
 * time the value arrives, the same erasure that makes `@Serify.sum` list its
 * arms.
 *
 * Go is the --ref language and owns the layout; see examples/go/wire.go.
 */

import { Serify } from '@chengxilo/serify';

import { lenPrefixed, lenPrefixedStr } from './wire';

/** A cursor over the input buffer, so the readers below stay one-liners. */
export class Reader {
  off = 0;
  constructor(readonly buf: Buffer) {}

  u8(): number { return this.buf[this.off++]; }
  u32(): number { const v = this.buf.readUInt32LE(this.off); this.off += 4; return v; }

  bytes(): Buffer {
    const n = this.u32();
    const b = Buffer.from(this.buf.subarray(this.off, this.off + n)); // copy: a subarray aliases
    this.off += n;
    return b;
  }

  str(): string { return this.bytes().toString('utf8'); }
}

@Serify.Model()
export class Address {
  @Serify.field() recipient = '';
  @Serify.field() street = '';
  @Serify.field() city = '';
  @Serify.field() country = '';
  @Serify.field() postal_code = '';

  /** A struct is its fields back to back, in schema order — nothing frames it. */
  pack(): Buffer {
    return Buffer.concat([
      lenPrefixedStr(this.recipient), lenPrefixedStr(this.street),
      lenPrefixedStr(this.city), lenPrefixedStr(this.country),
      lenPrefixedStr(this.postal_code),
    ]);
  }

  static unpack(r: Reader): Address {
    const a = new Address();
    a.recipient = r.str();
    a.street = r.str();
    a.city = r.str();
    a.country = r.str();
    a.postal_code = r.str();
    return a;
  }
}

@Serify.Model()
export class Money {
  @Serify.field() currency = '';
  @Serify.field() amount_minor = 0n;

  pack(): Buffer {
    const amt = Buffer.alloc(8);
    amt.writeBigInt64LE(this.amount_minor, 0);
    return Buffer.concat([lenPrefixedStr(this.currency), amt]);
  }

  static unpack(r: Reader): Money {
    const m = new Money();
    m.currency = r.str();
    m.amount_minor = r.buf.readBigInt64LE(r.off);
    r.off += 8;
    return m;
  }
}

@Serify.Model()
export class CustomerRecord {
  @Serify.field() customer_id = 0n;
  @Serify.field() email = '';
  @Serify.field() display_name = '';
  @Serify.field() age = 0;
  @Serify.field() email_verified = false;
  @Serify.field() fraud_score = 0;
  @Serify.field() loyalty_points = 0;
  @Serify.field() signup_ts = 0n;
  @Serify.field() avatar_sha256: Buffer = Buffer.alloc(0);
  @Serify.field() pin: number[] = [0, 0, 0, 0];
  @Serify.field() referral_code: string | null = null;
  @Serify.field({ model: Money }) store_credit = new Money();
  @Serify.field({ model: Address }) shipping_addresses: Address[] = [];
  @Serify.field({ model: Address }) address_book = new Map<string, Address>();
  @Serify.field() wishlist_skus: string[] = [];
  @Serify.field() preferences = new Map<string, string>();

  marshal(): Buffer {
    const head = Buffer.alloc(8);
    head.writeBigUInt64LE(this.customer_id, 0);

    const scalars = Buffer.alloc(18);
    scalars.writeUInt8(this.age, 0);
    scalars.writeUInt8(this.email_verified ? 1 : 0, 1);
    scalars.writeFloatLE(this.fraud_score, 2);
    scalars.writeUInt32LE(this.loyalty_points, 6);
    scalars.writeBigInt64LE(this.signup_ts, 10);

    const parts: Buffer[] = [
      head,
      lenPrefixedStr(this.email),
      lenPrefixedStr(this.display_name),
      scalars,
      lenPrefixed(this.avatar_sha256),
      // array<T,N> carries no count: N is fixed by the schema.
      Buffer.from(this.pin),
    ];

    // optional<string>: a presence flag, then the value if present. An empty
    // string is present, which is why the flag cannot be inferred from it.
    if (this.referral_code === null) parts.push(Buffer.from([0]));
    else parts.push(Buffer.from([1]), lenPrefixedStr(this.referral_code));

    parts.push(this.store_credit.pack());

    parts.push(count(this.shipping_addresses.length));
    for (const a of this.shipping_addresses) parts.push(a.pack());

    // Entry order is the Map's own — deliberately not sorted. A map is
    // unordered, so customer declares `oracle: semantic` and the decoded value
    // is what gets compared. See docs/protocol.md.
    parts.push(count(this.address_book.size));
    for (const [k, a] of this.address_book) parts.push(lenPrefixedStr(k), a.pack());

    parts.push(count(this.wishlist_skus.length));
    for (const s of this.wishlist_skus) parts.push(lenPrefixedStr(s));

    parts.push(count(this.preferences.size));
    for (const [k, v] of this.preferences) parts.push(lenPrefixedStr(k), lenPrefixedStr(v));

    return Buffer.concat(parts);
  }

  static unmarshal(data: Buffer): CustomerRecord {
    const c = new CustomerRecord();
    const r = new Reader(data);

    c.customer_id = data.readBigUInt64LE(0);
    r.off = 8;
    c.email = r.str();
    c.display_name = r.str();

    c.age = data.readUInt8(r.off);
    c.email_verified = data.readUInt8(r.off + 1) !== 0;
    c.fraud_score = data.readFloatLE(r.off + 2);
    c.loyalty_points = data.readUInt32LE(r.off + 6);
    c.signup_ts = data.readBigInt64LE(r.off + 10);
    r.off += 18;

    c.avatar_sha256 = r.bytes();

    c.pin = [...data.subarray(r.off, r.off + 4)];
    r.off += 4;

    c.referral_code = r.u8() === 0 ? null : r.str();

    c.store_credit = Money.unpack(r);

    c.shipping_addresses = [];
    for (let n = r.u32(); n > 0; n--) c.shipping_addresses.push(Address.unpack(r));

    c.address_book = new Map();
    for (let n = r.u32(); n > 0; n--) c.address_book.set(r.str(), Address.unpack(r));

    c.wishlist_skus = [];
    for (let n = r.u32(); n > 0; n--) c.wishlist_skus.push(r.str());

    c.preferences = new Map();
    for (let n = r.u32(); n > 0; n--) c.preferences.set(r.str(), r.str());

    return c;
  }

  /**
   * `bytes` is base64 in JSON, and the 64-bit fields are plain numbers: that is
   * what the reference worker's `[]byte` and `uint64` marshal to, and the
   * semantic oracle decodes our output with it.
   */
  toJSON(): Buffer {
    const obj = {
      customer_id: this.customer_id.toString(),
      email: this.email,
      display_name: this.display_name,
      age: this.age,
      email_verified: this.email_verified,
      fraud_score: this.fraud_score,
      loyalty_points: this.loyalty_points,
      signup_ts: this.signup_ts.toString(),
      avatar_sha256: this.avatar_sha256.toString('base64'),
      pin: this.pin,
      referral_code: this.referral_code,
      store_credit: {
        currency: this.store_credit.currency,
        amount_minor: this.store_credit.amount_minor.toString(),
      },
      shipping_addresses: this.shipping_addresses.map(addrJSON),
      address_book: Object.fromEntries([...this.address_book].map(([k, a]) => [k, addrJSON(a)])),
      wishlist_skus: this.wishlist_skus,
      preferences: Object.fromEntries(this.preferences),
    };
    return Buffer.from(unquoteBigints(JSON.stringify(obj)), 'utf8');
  }

  static fromJSON(data: Buffer): CustomerRecord {
    const c = new CustomerRecord();
    const o = JSON.parse(quoteBigints(data.toString('utf8')));

    c.customer_id = BigInt(o.customer_id);
    c.email = o.email;
    c.display_name = o.display_name;
    c.age = o.age;
    c.email_verified = o.email_verified;
    // A JSON number is a double; narrow it the way the wire does, so a float32
    // field holds a value float32 can actually represent.
    c.fraud_score = Math.fround(o.fraud_score);
    c.loyalty_points = o.loyalty_points;
    c.signup_ts = BigInt(o.signup_ts);
    c.avatar_sha256 = Buffer.from(o.avatar_sha256 ?? '', 'base64');
    c.pin = o.pin;
    c.referral_code = o.referral_code ?? null;
    c.store_credit = moneyFromJSON(o.store_credit);
    c.shipping_addresses = (o.shipping_addresses ?? []).map(addrFromJSON);
    c.address_book = new Map(
      Object.entries(o.address_book ?? {}).map(([k, v]) => [k, addrFromJSON(v)]));
    c.wishlist_skus = o.wishlist_skus ?? [];
    c.preferences = new Map(Object.entries(o.preferences ?? {}));
    return c;
  }
}

function count(n: number): Buffer {
  const b = Buffer.alloc(4);
  b.writeUInt32LE(n, 0);
  return b;
}

// 64-bit integers in JSON
//
// JSON's only number is the double, and customer's boundary case does not fit
// in one: max uint64 rounds up to 2^64 and comes back out of range. JSON.parse
// offers no way to see the undamaged token -- Node 22's JSON.rawJSON does, but
// CI builds on 20 -- so these three fields cross the text boundary as quoted
// strings, unquoted on the way out and requoted on the way in. They are the
// only 64-bit fields customer has.
//
// This rewrites the text rather than the parse tree, so a *string value*
// spelling one of these keys followed by a colon and digits would be lifted
// too. No case produces one, and the escape-hunting cases cannot: a `"` inside
// a JSON string is written `\"`.
const BIG_KEYS = 'customer_id|signup_ts|amount_minor';

function unquoteBigints(text: string): string {
  return text.replace(new RegExp(`"(${BIG_KEYS})":"(-?\\d+)"`, 'g'), '"$1":$2');
}

function quoteBigints(text: string): string {
  return text.replace(new RegExp(`"(${BIG_KEYS})":(-?\\d+)`, 'g'), '"$1":"$2"');
}

function addrJSON(a: Address) {
  return {
    recipient: a.recipient, street: a.street, city: a.city,
    country: a.country, postal_code: a.postal_code,
  };
}

function addrFromJSON(o: any): Address {
  const a = new Address();
  a.recipient = o.recipient;
  a.street = o.street;
  a.city = o.city;
  a.country = o.country;
  a.postal_code = o.postal_code;
  return a;
}

function moneyFromJSON(o: any): Money {
  const m = new Money();
  m.currency = o.currency;
  m.amount_minor = BigInt(o.amount_minor);
  return m;
}
