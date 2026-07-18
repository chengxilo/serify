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

// LedgerEntry mirrors examples/cases/ledger.yaml.
//
// The SERIFY_* macros are the entire schema binding — nothing here calls a
// get_*/set_* accessor. Everything else is the byte layout, which is the part a
// conformance worker exists to exercise.
//
// __int128 is a GCC/Clang extension, but it shifts like any other integer, so
// the two int128 fields need no special handling.
//
// Go is the --ref language and owns the layout; see examples/go/wire.go.

#pragma once

#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

struct LedgerEntry {
    uint64_t entry_id{};
    uint64_t block_number{};
    int64_t  block_time{};
    serify::Bytes tx_hash;
    std::string account;
    std::string asset;
    serify::i128 amount_base_units{};
    serify::i128 balance_after{};
    bool confirmed{};
    std::optional<std::string> memo;
};

SERIFY_TO(LedgerEntry,
    SERIFY_FIELD(entry_id, u64)
    SERIFY_FIELD(block_number, u64)
    SERIFY_FIELD(block_time, i64)
    SERIFY_FIELD(tx_hash, bytes)
    SERIFY_FIELD(account, string)
    SERIFY_FIELD(asset, string)
    SERIFY_FIELD(amount_base_units, i128)
    SERIFY_FIELD(balance_after, i128)
    SERIFY_FIELD(confirmed, bool)
    SERIFY_FIELD(memo, optional_string)
)

SERIFY_FROM(LedgerEntry,
    SERIFY_FROM_FIELD(entry_id, u64)
    SERIFY_FROM_FIELD(block_number, u64)
    SERIFY_FROM_FIELD(block_time, i64)
    SERIFY_FROM_FIELD(tx_hash, bytes)
    SERIFY_FROM_FIELD(account, string)
    SERIFY_FROM_FIELD(asset, string)
    SERIFY_FROM_FIELD(amount_base_units, i128)
    SERIFY_FROM_FIELD(balance_after, i128)
    SERIFY_FROM_FIELD(confirmed, bool)
    SERIFY_FROM_FIELD(memo, optional_string)
)

inline std::vector<uint8_t> ledger_marshal(const LedgerEntry& e) {
    std::vector<uint8_t> out;
    put_le<uint64_t>(out, e.entry_id, 8);
    put_le<uint64_t>(out, e.block_number, 8);
    put_le<uint64_t>(out, static_cast<uint64_t>(e.block_time), 8);

    put_len_prefixed(out, e.tx_hash.data(), e.tx_hash.size());
    put_str(out, e.account);
    put_str(out, e.asset);

    put_le<serify::u128>(out, static_cast<serify::u128>(e.amount_base_units), 16);
    put_le<serify::u128>(out, static_cast<serify::u128>(e.balance_after), 16);

    out.push_back(e.confirmed ? 1 : 0);
    if (!e.memo.has_value()) {
        out.push_back(0);
    } else {
        out.push_back(1);
        put_str(out, *e.memo);
    }
    return out;
}

inline LedgerEntry ledger_unmarshal(const std::vector<uint8_t>& data) {
    LedgerEntry e{};
    size_t off = 0;

    e.entry_id     = take_le<uint64_t>(data, off, 8);
    e.block_number = take_le<uint64_t>(data, off, 8);
    e.block_time   = static_cast<int64_t>(take_le<uint64_t>(data, off, 8));

    e.tx_hash = take_len_prefixed(data, off);
    e.account = take_len_str(data, off);
    e.asset   = take_len_str(data, off);

    e.amount_base_units = static_cast<serify::i128>(take_le<serify::u128>(data, off, 16));
    e.balance_after     = static_cast<serify::i128>(take_le<serify::u128>(data, off, 16));

    e.confirmed = data[off] != 0;
    const bool has_memo = data[off + 1] != 0;
    off += 2;
    if (has_memo) e.memo = take_len_str(data, off);

    return e;
}
