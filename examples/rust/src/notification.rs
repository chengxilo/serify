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

//! `NotificationRecord` mirrors examples/cases/notification.yaml, whose
//! `channel` field is a `oneof`.

use serify::SerifyModel;

use crate::common::Money;
use crate::wire::{append_len_str, read_len_str};

/// Channel is the `oneof` from the case file. Rust has a native sum type, so it
/// maps straight onto an enum — and because a sum type is exactly what `oneof`
/// describes, the same derive handles it. Variant names become schema tags in
/// snake_case; no converter, no marker trait, and no way to build a notification
/// carrying two targets at once.
#[derive(SerifyModel)]
pub enum Channel {
    Silent,         // arity 0 — a unit variant
    Sms(String),    // arity 1 — a scalar payload
    Push(u64),      // arity 1 — a payload that exceeds 2^53
    Invoice(Money), // arity N — a struct payload
}

#[derive(SerifyModel)]
pub struct NotificationRecord {
    notification_id: u32,
    channel: Channel,
    urgent: bool,
}

impl NotificationRecord {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = Vec::new();
        buf.extend_from_slice(&self.notification_id.to_le_bytes());

        // The tag ordinal is the variant's position in the case file's oneof,
        // which is this enum's declaration order. The schema tag *names* are the
        // derive's business, and never appear here.
        match &self.channel {
            Channel::Silent => buf.push(0), // a unit variant is nothing but its tag
            Channel::Sms(s) => {
                buf.push(1);
                append_len_str(&mut buf, s);
            }
            Channel::Push(n) => {
                buf.push(2);
                buf.extend_from_slice(&n.to_le_bytes());
            }
            Channel::Invoice(m) => {
                buf.push(3);
                append_len_str(&mut buf, &m.currency);
                buf.extend_from_slice(&m.amount_minor.to_le_bytes());
            }
        }

        buf.push(self.urgent as u8);
        Ok(buf)
    }

    pub fn unmarshal(data: &[u8]) -> Result<Self, String> {
        if data.len() < 5 {
            return Err("truncated".into());
        }
        let notification_id = u32::from_le_bytes(data[0..4].try_into().unwrap());
        let mut p = 5usize;

        let channel = match data[4] {
            0 => Channel::Silent,
            1 => Channel::Sms(read_len_str(data, &mut p)?),
            2 => {
                if p + 8 > data.len() {
                    return Err("truncated".into());
                }
                let n = u64::from_le_bytes(data[p..p + 8].try_into().unwrap());
                p += 8;
                Channel::Push(n)
            }
            3 => {
                let currency = read_len_str(data, &mut p)?;
                if p + 8 > data.len() {
                    return Err("truncated".into());
                }
                let amount_minor = i64::from_le_bytes(data[p..p + 8].try_into().unwrap());
                p += 8;
                Channel::Invoice(Money {
                    currency,
                    amount_minor,
                })
            }
            ord => return Err(format!("unknown channel ordinal {ord}")),
        };

        if p >= data.len() {
            return Err("truncated".into());
        }
        Ok(Self {
            notification_id,
            channel,
            urgent: data[p] != 0,
        })
    }
}
