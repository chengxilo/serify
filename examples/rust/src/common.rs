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

//! The two reusable models — examples/cases/address.yaml and
//! examples/cases/money.yaml — shared by more than one type in the suite.
//!
//! `#[derive(SerifyModel)]` is the whole binding: the derive maps each field
//! onto its FieldMap slot by name, and nesting one of these inside another model
//! needs nothing further.

use serify::SerifyModel;

use crate::wire::{append_len_str, read_len_str};

#[derive(SerifyModel)]
pub struct Address {
    pub recipient: String,
    pub street: String,
    pub city: String,
    pub country: String,
    pub postal_code: String,
}

#[derive(SerifyModel)]
pub struct Money {
    pub currency: String,
    pub amount_minor: i64,
}

// A struct is its fields back to back, in schema order — nothing frames it, so
// these take the surrounding buffer rather than owning one. They live here
// rather than in customer.rs because order.rs needs the same two.

pub fn append_address(buf: &mut Vec<u8>, a: &Address) {
    append_len_str(buf, &a.recipient);
    append_len_str(buf, &a.street);
    append_len_str(buf, &a.city);
    append_len_str(buf, &a.country);
    append_len_str(buf, &a.postal_code);
}

pub fn read_address(data: &[u8], p: &mut usize) -> Result<Address, String> {
    Ok(Address {
        recipient: read_len_str(data, p)?,
        street: read_len_str(data, p)?,
        city: read_len_str(data, p)?,
        country: read_len_str(data, p)?,
        postal_code: read_len_str(data, p)?,
    })
}

pub fn append_money(buf: &mut Vec<u8>, m: &Money) {
    append_len_str(buf, &m.currency);
    buf.extend_from_slice(&m.amount_minor.to_le_bytes());
}

pub fn read_money(data: &[u8], p: &mut usize) -> Result<Money, String> {
    let currency = read_len_str(data, p)?;
    if *p + 8 > data.len() {
        return Err("truncated".into());
    }
    let amount_minor = i64::from_le_bytes(data[*p..*p + 8].try_into().unwrap());
    *p += 8;
    Ok(Money {
        currency,
        amount_minor,
    })
}
