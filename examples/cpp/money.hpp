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

// Money mirrors the reusable examples/cases/money.yaml.
//
// It lives in its own header because three case files import it — notification
// (as its struct-payload variant), customer and order — and SERIFY_TO expands
// to a non-template function, so a second definition in a second header is a
// redefinition, not an overload.

#pragma once

#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <string>
#include <vector>

struct Money {
    std::string currency;
    int64_t amount_minor{};
};

SERIFY_TO(Money,
    SERIFY_FIELD(currency, string)
    SERIFY_FIELD(amount_minor, i64)
)

SERIFY_FROM(Money,
    SERIFY_FROM_FIELD(currency, string)
    SERIFY_FROM_FIELD(amount_minor, i64)
)

// A struct is its fields back to back, in schema order — nothing frames it, so
// these take the surrounding buffer rather than owning one.
inline void money_pack(std::vector<uint8_t>& out, const Money& m) {
    put_str(out, m.currency);
    put_le<uint64_t>(out, static_cast<uint64_t>(m.amount_minor), 8);
}

inline Money money_unpack(const std::vector<uint8_t>& data, size_t& off) {
    Money m;
    m.currency = take_len_str(data, off);
    m.amount_minor = static_cast<int64_t>(take_le<uint64_t>(data, off, 8));
    return m;
}
