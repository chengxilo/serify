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

//! `LedgerEntry` mirrors examples/cases/ledger.yaml. Rust has native 128-bit
//! integers, so int128 maps straight onto i128 — the Go worker has to reach for
//! math/big to hold the same range.
//!
//! `#[derive(SerifyModel)]` is the entire schema binding. Everything below it is
//! the byte layout, which is the part a conformance worker exists to exercise.

use serify::SerifyModel;

use crate::wire::{append_len_str, read_bytes, read_len_str};

#[derive(SerifyModel)]
pub struct LedgerEntry {
    entry_id: u64,
    block_number: u64,
    block_time: i64,
    tx_hash: Vec<u8>,
    account: String,
    asset: String,
    amount_base_units: i128,
    balance_after: i128,
    confirmed: bool,
    memo: Option<String>,
}

impl LedgerEntry {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = Vec::new();
        buf.extend_from_slice(&self.entry_id.to_le_bytes());
        buf.extend_from_slice(&self.block_number.to_le_bytes());
        buf.extend_from_slice(&self.block_time.to_le_bytes());

        buf.extend_from_slice(&(self.tx_hash.len() as u32).to_le_bytes());
        buf.extend_from_slice(&self.tx_hash);

        append_len_str(&mut buf, &self.account);
        append_len_str(&mut buf, &self.asset);

        // int128: 16 bytes little-endian two's complement, matching the Go worker.
        buf.extend_from_slice(&self.amount_base_units.to_le_bytes());
        buf.extend_from_slice(&self.balance_after.to_le_bytes());

        buf.push(self.confirmed as u8);
        match &self.memo {
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

        let entry_id = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let block_number = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let block_time = i64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());

        let hash_len = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let tx_hash = read_bytes!(data, p, hash_len).to_vec();

        let account = read_len_str(data, &mut p)?;
        let asset = read_len_str(data, &mut p)?;

        let amount_base_units = i128::from_le_bytes(read_bytes!(data, p, 16).try_into().unwrap());
        let balance_after = i128::from_le_bytes(read_bytes!(data, p, 16).try_into().unwrap());

        let confirmed = read_bytes!(data, p, 1)[0] != 0;
        let memo = if read_bytes!(data, p, 1)[0] == 0 {
            None
        } else {
            Some(read_len_str(data, &mut p)?)
        };

        Ok(Self {
            entry_id,
            block_number,
            block_time,
            tx_hash,
            account,
            asset,
            amount_base_units,
            balance_after,
            confirmed,
            memo,
        })
    }
}
