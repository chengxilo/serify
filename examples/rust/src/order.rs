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

//! `OrderRecord` mirrors examples/cases/order.yaml — a placed order.
//!
//! `LineItem` mirrors the reusable line_item.yaml it imports, which itself nests
//! money — so any type using it exercises struct-inside-struct. `Address` and
//! `Money` come from common.rs, as they do in the Go worker.
//!
//! Between them the fields cover the four composite types nothing else in the
//! suite exercises end to end: an `enum`, a `list<struct>`, a
//! `map<string,struct>` and an `optional<struct>`.
//!
//! An enum needs nothing from the derive: it travels as its variant *name*, so
//! the field is a plain `String`. The u8 ordinal in the layout is this worker's
//! own choice, which is why STATUSES has to match the case file's declaration
//! order.
//!
//! Go is the --ref language and owns the layout; see examples/go/wire.go.

use std::collections::HashMap;

use serify::SerifyModel;

use crate::common::{append_address, append_money, read_address, read_money, Address, Money};
use crate::wire::{append_len_str, read_bytes, read_len_str};

/// Declaration order of the `status` enum in examples/cases/order.yaml.
const STATUSES: [&str; 5] = ["pending", "paid", "shipped", "delivered", "cancelled"];

#[derive(SerifyModel)]
pub struct LineItem {
    sku: String,
    product_name: String,
    quantity: u16,
    unit_price: Money,
    discount_pct: u8,
    gift_wrap: bool,
}

#[derive(SerifyModel)]
pub struct OrderRecord {
    order_id: u64,
    customer_id: u64,
    created_at: i64,
    status: String,
    items: Vec<LineItem>,
    subtotal: Money,
    adjustments: HashMap<String, Money>,
    total: Money,
    shipping_address: Address,
    billing_address: Option<Address>,
    coupon_codes: Vec<String>,
    tracking_number: Option<String>,
}

impl OrderRecord {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = Vec::new();

        buf.extend_from_slice(&self.order_id.to_le_bytes());
        buf.extend_from_slice(&self.customer_id.to_le_bytes());
        buf.extend_from_slice(&self.created_at.to_le_bytes());

        // enum: a u8 ordinal, the variant's position in the case file.
        let ord = STATUSES
            .iter()
            .position(|s| *s == self.status)
            .ok_or_else(|| format!("unknown order status {:?}", self.status))?;
        buf.push(ord as u8);

        buf.extend_from_slice(&(self.items.len() as u32).to_le_bytes());
        for it in &self.items {
            append_len_str(&mut buf, &it.sku);
            append_len_str(&mut buf, &it.product_name);
            buf.extend_from_slice(&it.quantity.to_le_bytes());
            append_money(&mut buf, &it.unit_price);
            buf.push(it.discount_pct);
            buf.push(it.gift_wrap as u8);
        }

        append_money(&mut buf, &self.subtotal);

        // Entry order is the HashMap's own — deliberately not sorted. A map is
        // unordered, so order declares `oracle: semantic` and the decoded value
        // is what gets compared. See docs/protocol.md.
        buf.extend_from_slice(&(self.adjustments.len() as u32).to_le_bytes());
        for (k, m) in &self.adjustments {
            append_len_str(&mut buf, k);
            append_money(&mut buf, m);
        }

        append_money(&mut buf, &self.total);
        append_address(&mut buf, &self.shipping_address);

        // optional<struct>: a presence flag, then the struct's fields inline.
        match &self.billing_address {
            None => buf.push(0),
            Some(a) => {
                buf.push(1);
                append_address(&mut buf, a);
            }
        }

        buf.extend_from_slice(&(self.coupon_codes.len() as u32).to_le_bytes());
        for c in &self.coupon_codes {
            append_len_str(&mut buf, c);
        }

        match &self.tracking_number {
            None => buf.push(0),
            Some(s) => {
                buf.push(1);
                append_len_str(&mut buf, s);
            }
        }

        Ok(buf)
    }

    pub fn unmarshal(data: &[u8]) -> Result<Self, String> {
        let mut p = 0usize;

        let order_id = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let customer_id = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let created_at = i64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());

        let ord = read_bytes!(data, p, 1)[0] as usize;
        let status = STATUSES
            .get(ord)
            .ok_or_else(|| format!("status ordinal {ord} is out of range"))?
            .to_string();

        let n = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap());
        let mut items = Vec::with_capacity(n as usize);
        for _ in 0..n {
            let sku = read_len_str(data, &mut p)?;
            let product_name = read_len_str(data, &mut p)?;
            let quantity = u16::from_le_bytes(read_bytes!(data, p, 2).try_into().unwrap());
            let unit_price = read_money(data, &mut p)?;
            let tail = read_bytes!(data, p, 2);
            items.push(LineItem {
                sku,
                product_name,
                quantity,
                unit_price,
                discount_pct: tail[0],
                gift_wrap: tail[1] != 0,
            });
        }

        let subtotal = read_money(data, &mut p)?;

        let n = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap());
        let mut adjustments = HashMap::with_capacity(n as usize);
        for _ in 0..n {
            let k = read_len_str(data, &mut p)?;
            adjustments.insert(k, read_money(data, &mut p)?);
        }

        let total = read_money(data, &mut p)?;
        let shipping_address = read_address(data, &mut p)?;

        let billing_address = if read_bytes!(data, p, 1)[0] == 0 {
            None
        } else {
            Some(read_address(data, &mut p)?)
        };

        let n = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap());
        let mut coupon_codes = Vec::with_capacity(n as usize);
        for _ in 0..n {
            coupon_codes.push(read_len_str(data, &mut p)?);
        }

        let tracking_number = if read_bytes!(data, p, 1)[0] == 0 {
            None
        } else {
            Some(read_len_str(data, &mut p)?)
        };

        Ok(OrderRecord {
            order_id,
            customer_id,
            created_at,
            status,
            items,
            subtotal,
            adjustments,
            total,
            shipping_address,
            billing_address,
            coupon_codes,
            tracking_number,
        })
    }
}
