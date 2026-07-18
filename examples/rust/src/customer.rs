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

//! `CustomerRecord` mirrors examples/cases/customer.yaml — the widest type in
//! the suite, and the only one carrying two formats.
//!
//! `#[derive(SerifyModel)]` covers the whole shape, nested struct, list<struct>,
//! map<string,struct>, map<string,string>, optional and array included. Both
//! layouts below are hand-written, which is the part a conformance worker exists
//! to exercise.

use std::collections::HashMap;

use serde_json::Value;
use serify::SerifyModel;

use crate::common::{Address, Money};
use crate::wire::{append_len_str, read_bytes, read_len_str};

#[derive(SerifyModel)]
pub struct CustomerRecord {
    customer_id: u64,
    email: String,
    display_name: String,
    age: u8,
    email_verified: bool,
    fraud_score: f32,
    loyalty_points: u32,
    signup_ts: i64,
    avatar_sha256: Vec<u8>,
    pin: [u8; 4],
    referral_code: Option<String>,
    store_credit: Money,
    shipping_addresses: Vec<Address>,
    address_book: HashMap<String, Address>,
    wishlist_skus: Vec<String>,
    preferences: HashMap<String, String>,
}

// --- binary format -----------------------------------------------------------

fn append_address(buf: &mut Vec<u8>, a: &Address) {
    append_len_str(buf, &a.recipient);
    append_len_str(buf, &a.street);
    append_len_str(buf, &a.city);
    append_len_str(buf, &a.country);
    append_len_str(buf, &a.postal_code);
}

fn read_address(data: &[u8], p: &mut usize) -> Result<Address, String> {
    Ok(Address {
        recipient: read_len_str(data, p)?,
        street: read_len_str(data, p)?,
        city: read_len_str(data, p)?,
        country: read_len_str(data, p)?,
        postal_code: read_len_str(data, p)?,
    })
}

impl CustomerRecord {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = Vec::new();

        buf.extend_from_slice(&self.customer_id.to_le_bytes());
        append_len_str(&mut buf, &self.email);
        append_len_str(&mut buf, &self.display_name);
        buf.push(self.age);
        buf.push(self.email_verified as u8);
        buf.extend_from_slice(&self.fraud_score.to_bits().to_le_bytes());
        buf.extend_from_slice(&self.loyalty_points.to_le_bytes());
        buf.extend_from_slice(&self.signup_ts.to_le_bytes());

        buf.extend_from_slice(&(self.avatar_sha256.len() as u32).to_le_bytes());
        buf.extend_from_slice(&self.avatar_sha256);

        buf.extend_from_slice(&self.pin);

        match &self.referral_code {
            None => buf.push(0),
            Some(s) => {
                buf.push(1);
                append_len_str(&mut buf, s);
            }
        }

        append_len_str(&mut buf, &self.store_credit.currency);
        buf.extend_from_slice(&self.store_credit.amount_minor.to_le_bytes());

        buf.extend_from_slice(&(self.shipping_addresses.len() as u32).to_le_bytes());
        for a in &self.shipping_addresses {
            append_address(&mut buf, a);
        }

        let mut book_keys: Vec<&String> = self.address_book.keys().collect();
        book_keys.sort();
        buf.extend_from_slice(&(book_keys.len() as u32).to_le_bytes());
        for k in &book_keys {
            append_len_str(&mut buf, k);
            append_address(&mut buf, &self.address_book[*k]);
        }

        buf.extend_from_slice(&(self.wishlist_skus.len() as u32).to_le_bytes());
        for s in &self.wishlist_skus {
            append_len_str(&mut buf, s);
        }

        let mut pref_keys: Vec<&String> = self.preferences.keys().collect();
        pref_keys.sort();
        buf.extend_from_slice(&(pref_keys.len() as u32).to_le_bytes());
        for k in &pref_keys {
            append_len_str(&mut buf, k);
            append_len_str(&mut buf, &self.preferences[*k]);
        }

        Ok(buf)
    }

    pub fn unmarshal(data: &[u8]) -> Result<Self, String> {
        let mut p = 0usize;

        let customer_id = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let email = read_len_str(data, &mut p)?;
        let display_name = read_len_str(data, &mut p)?;
        let age = read_bytes!(data, p, 1)[0];
        let email_verified = read_bytes!(data, p, 1)[0] != 0;
        let fraud_score = f32::from_bits(u32::from_le_bytes(
            read_bytes!(data, p, 4).try_into().unwrap(),
        ));
        let loyalty_points = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap());
        let signup_ts = i64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());

        let avatar_len = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let avatar_sha256 = read_bytes!(data, p, avatar_len).to_vec();

        let mut pin = [0u8; 4];
        pin.copy_from_slice(read_bytes!(data, p, 4));

        let referral_code = if read_bytes!(data, p, 1)[0] == 0 {
            None
        } else {
            Some(read_len_str(data, &mut p)?)
        };

        let currency = read_len_str(data, &mut p)?;
        let amount_minor = i64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let store_credit = Money {
            currency,
            amount_minor,
        };

        let ship_len = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let mut shipping_addresses = Vec::with_capacity(ship_len);
        for _ in 0..ship_len {
            shipping_addresses.push(read_address(data, &mut p)?);
        }

        let book_len = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let mut address_book = HashMap::with_capacity(book_len);
        for _ in 0..book_len {
            let k = read_len_str(data, &mut p)?;
            let a = read_address(data, &mut p)?;
            address_book.insert(k, a);
        }

        let sku_len = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let mut wishlist_skus = Vec::with_capacity(sku_len);
        for _ in 0..sku_len {
            wishlist_skus.push(read_len_str(data, &mut p)?);
        }

        let pref_len = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let mut preferences = HashMap::with_capacity(pref_len);
        for _ in 0..pref_len {
            let k = read_len_str(data, &mut p)?;
            let v = read_len_str(data, &mut p)?;
            preferences.insert(k, v);
        }

        Ok(Self {
            customer_id,
            email,
            display_name,
            age,
            email_verified,
            fraud_score,
            loyalty_points,
            signup_ts,
            avatar_sha256,
            pin,
            referral_code,
            store_credit,
            shipping_addresses,
            address_book,
            wishlist_skus,
            preferences,
        })
    }
}

// --- JSON format -------------------------------------------------------------
//
// The bytes must match Go's `encoding/json` output for the same struct exactly
// (serify compares byte-for-byte). Go differs from serde_json in four ways; each
// is handled below, or on the Go side:
//
//   - `[]byte` -> base64 string, and map keys sorted: done here.
//   - Floats: Go switches to exponent notation outside [1e-6, 1e21) and prints the
//     shortest round-trip form. Rust's `{}` Display never uses exponents, so
//     go_f32 below reproduces Go's rule.
//   - <, > and & are HTML-escaped by default. That is a Go-only quirk, so the Go
//     worker switches it off with SetEscapeHTML(false) (see examples/go/customer.go)
//     rather than making all eight languages reproduce it.
//   - U+2028/U+2029 are escaped *unconditionally* - SetEscapeHTML(false) does not
//     affect them, because unescaped they break JSONP. There is no way to turn
//     that off on the Go side, so jstr has to reproduce it.

const B64: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

fn base64_encode(data: &[u8]) -> String {
    let mut out = String::with_capacity(data.len().div_ceil(3) * 4);
    for chunk in data.chunks(3) {
        let b0 = chunk[0] as u32;
        let b1 = *chunk.get(1).unwrap_or(&0) as u32;
        let b2 = *chunk.get(2).unwrap_or(&0) as u32;
        let n = (b0 << 16) | (b1 << 8) | b2;
        out.push(B64[((n >> 18) & 63) as usize] as char);
        out.push(B64[((n >> 12) & 63) as usize] as char);
        out.push(if chunk.len() > 1 {
            B64[((n >> 6) & 63) as usize] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            B64[(n & 63) as usize] as char
        } else {
            '='
        });
    }
    out
}

fn base64_decode(s: &str) -> Result<Vec<u8>, String> {
    fn val(c: u8) -> Result<u32, String> {
        match c {
            b'A'..=b'Z' => Ok((c - b'A') as u32),
            b'a'..=b'z' => Ok((c - b'a' + 26) as u32),
            b'0'..=b'9' => Ok((c - b'0' + 52) as u32),
            b'+' => Ok(62),
            b'/' => Ok(63),
            _ => Err(format!("invalid base64 char {:?}", c as char)),
        }
    }
    let s = s.as_bytes();
    if !s.len().is_multiple_of(4) {
        return Err("base64 length not a multiple of 4".into());
    }
    let mut out = Vec::with_capacity(s.len() / 4 * 3);
    for chunk in s.chunks(4) {
        let c2pad = chunk[2] == b'=';
        let c3pad = chunk[3] == b'=';
        let v0 = val(chunk[0])?;
        let v1 = val(chunk[1])?;
        let v2 = if c2pad { 0 } else { val(chunk[2])? };
        let v3 = if c3pad { 0 } else { val(chunk[3])? };
        let n = (v0 << 18) | (v1 << 12) | (v2 << 6) | v3;
        out.push((n >> 16) as u8);
        if !c2pad {
            out.push((n >> 8) as u8);
        }
        if !c3pad {
            out.push(n as u8);
        }
    }
    Ok(out)
}

// go_f32 formats an f32 the way Go's encoding/json does: the shortest round-trip
// decimal, switching to exponent notation when |x| < 1e-6 or |x| >= 1e21, with an
// explicit exponent sign ("3.4028235e+38"). Rust's `{}` never uses exponents and
// its `{:e}` omits the '+', so neither form alone matches Go.
fn go_f32(f: f32) -> String {
    let abs = f.abs();
    if abs != 0.0 && !(1e-6..1e21).contains(&abs) {
        let s = format!("{:e}", f);
        return match s.split_once('e') {
            Some((mantissa, exp)) if !exp.starts_with('-') => format!("{}e+{}", mantissa, exp),
            _ => s,
        };
    }
    format!("{}", f)
}

// jstr quotes and escapes a string the way Go does (UTF-8 emitted directly).
// serde_json matches Go on everything except U+2028/U+2029; neither can appear
// inside an escape serde already produced, so patching its output is safe.
fn jstr(s: &str) -> String {
    serde_json::to_string(s)
        .unwrap()
        .replace('\u{2028}', "\\u2028")
        .replace('\u{2029}', "\\u2029")
}

fn address_to_json(a: &Address) -> String {
    format!(
        "{{\"recipient\":{},\"street\":{},\"city\":{},\"country\":{},\"postal_code\":{}}}",
        jstr(&a.recipient),
        jstr(&a.street),
        jstr(&a.city),
        jstr(&a.country),
        jstr(&a.postal_code),
    )
}

fn address_from_json(v: &Value) -> Result<Address, String> {
    let f = |k: &str| -> Result<String, String> {
        v.get(k)
            .and_then(|x| x.as_str())
            .ok_or_else(|| format!("address.{}", k))
            .map(|s| s.to_string())
    };
    Ok(Address {
        recipient: f("recipient")?,
        street: f("street")?,
        city: f("city")?,
        country: f("country")?,
        postal_code: f("postal_code")?,
    })
}

impl CustomerRecord {
    pub fn to_json(&self) -> Vec<u8> {
        let c = self;
        let mut s = String::new();
        s.push('{');
        s.push_str(&format!("\"customer_id\":{}", c.customer_id));
        s.push_str(&format!(",\"email\":{}", jstr(&c.email)));
        s.push_str(&format!(",\"display_name\":{}", jstr(&c.display_name)));
        s.push_str(&format!(",\"age\":{}", c.age));
        s.push_str(&format!(",\"email_verified\":{}", c.email_verified));
        s.push_str(&format!(",\"fraud_score\":{}", go_f32(c.fraud_score)));
        s.push_str(&format!(",\"loyalty_points\":{}", c.loyalty_points));
        s.push_str(&format!(",\"signup_ts\":{}", c.signup_ts));
        s.push_str(&format!(
            ",\"avatar_sha256\":\"{}\"",
            base64_encode(&c.avatar_sha256)
        ));

        s.push_str(",\"pin\":[");
        for (i, b) in c.pin.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&b.to_string());
        }
        s.push(']');

        s.push_str(",\"referral_code\":");
        match &c.referral_code {
            Some(r) => s.push_str(&jstr(r)),
            None => s.push_str("null"),
        }

        s.push_str(&format!(
            ",\"store_credit\":{{\"currency\":{},\"amount_minor\":{}}}",
            jstr(&c.store_credit.currency),
            c.store_credit.amount_minor,
        ));

        s.push_str(",\"shipping_addresses\":[");
        for (i, a) in c.shipping_addresses.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&address_to_json(a));
        }
        s.push(']');

        s.push_str(",\"address_book\":{");
        let mut book_keys: Vec<&String> = c.address_book.keys().collect();
        book_keys.sort();
        for (i, k) in book_keys.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&jstr(k));
            s.push(':');
            s.push_str(&address_to_json(&c.address_book[*k]));
        }
        s.push('}');

        s.push_str(",\"wishlist_skus\":[");
        for (i, sku) in c.wishlist_skus.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&jstr(sku));
        }
        s.push(']');

        s.push_str(",\"preferences\":{");
        let mut pref_keys: Vec<&String> = c.preferences.keys().collect();
        pref_keys.sort();
        for (i, k) in pref_keys.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&jstr(k));
            s.push(':');
            s.push_str(&jstr(&c.preferences[*k]));
        }
        s.push('}');

        s.push('}');
        s.into_bytes()
    }

    pub fn from_json(data: &[u8]) -> Result<Self, String> {
        let v: Value = serde_json::from_slice(data).map_err(|e| e.to_string())?;
        let f = |k: &str| -> Result<&Value, String> {
            v.get(k).ok_or_else(|| format!("missing field {}", k))
        };

        let pin_arr = f("pin")?.as_array().ok_or("pin")?;
        let mut pin = [0u8; 4];
        for (i, slot) in pin.iter_mut().enumerate() {
            *slot = pin_arr.get(i).and_then(|x| x.as_u64()).ok_or("pin elem")? as u8;
        }

        let sc = f("store_credit")?;
        let store_credit = Money {
            currency: sc
                .get("currency")
                .and_then(|x| x.as_str())
                .ok_or("store_credit.currency")?
                .to_string(),
            amount_minor: sc
                .get("amount_minor")
                .and_then(|x| x.as_i64())
                .ok_or("store_credit.amount_minor")?,
        };

        let shipping_addresses = f("shipping_addresses")?
            .as_array()
            .ok_or("shipping_addresses")?
            .iter()
            .map(address_from_json)
            .collect::<Result<Vec<_>, _>>()?;

        let mut address_book = HashMap::new();
        for (k, val) in f("address_book")?.as_object().ok_or("address_book")? {
            address_book.insert(k.clone(), address_from_json(val)?);
        }

        let wishlist_skus = f("wishlist_skus")?
            .as_array()
            .ok_or("wishlist_skus")?
            .iter()
            .map(|x| x.as_str().unwrap_or_default().to_string())
            .collect();

        let mut preferences = HashMap::new();
        for (k, val) in f("preferences")?.as_object().ok_or("preferences")? {
            preferences.insert(
                k.clone(),
                val.as_str().ok_or("preferences value")?.to_string(),
            );
        }

        Ok(CustomerRecord {
            customer_id: f("customer_id")?.as_u64().ok_or("customer_id")?,
            email: f("email")?.as_str().ok_or("email")?.to_string(),
            display_name: f("display_name")?
                .as_str()
                .ok_or("display_name")?
                .to_string(),
            age: f("age")?.as_u64().ok_or("age")? as u8,
            email_verified: f("email_verified")?.as_bool().ok_or("email_verified")?,
            fraud_score: f("fraud_score")?.as_f64().ok_or("fraud_score")? as f32,
            loyalty_points: f("loyalty_points")?.as_u64().ok_or("loyalty_points")? as u32,
            signup_ts: f("signup_ts")?.as_i64().ok_or("signup_ts")?,
            avatar_sha256: base64_decode(f("avatar_sha256")?.as_str().ok_or("avatar_sha256")?)?,
            pin,
            referral_code: match f("referral_code")? {
                Value::Null => None,
                x => Some(x.as_str().ok_or("referral_code")?.to_string()),
            },
            store_credit,
            shipping_addresses,
            address_book,
            wishlist_skus,
            preferences,
        })
    }
}
