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

// NotificationRecord mirrors examples/cases/notification.yaml, whose `channel`
// field is a `sum`.
//
// std::variant is C++'s sum type, and it supplies the arms. The one thing it
// cannot supply is their names — C++ has no reflection — so SERIFY_SUM names
// them, and that single line is the whole binding. No converter, and no way to
// build a notification carrying two targets at once.
//
// Each alternative *is* its payload, so no wrapper structs are needed:
// std::monostate is the unit variant, std::string and uint64_t are scalar
// payloads, and Money is a struct payload.
//
// Go is the --ref language and owns the byte layout; see examples/go/wire.go.

#pragma once

#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <string>
#include <variant>
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

using Channel = std::variant<
    std::monostate,  // arity 0 — a unit variant
    std::string,     // arity 1 — a scalar payload
    uint64_t,        // arity 1 — a payload that exceeds 2^53
    Money            // arity N — a struct payload
>;

SERIFY_SUM(Channel, "silent", "sms", "push", "invoice")

struct NotificationRecord {
    uint32_t notification_id{};
    Channel  channel;
    bool     urgent{};
};

SERIFY_TO(NotificationRecord,
    SERIFY_FIELD(notification_id, u32)
    SERIFY_FIELD_SUM(channel, Channel)
    SERIFY_FIELD(urgent, bool)
)

SERIFY_FROM(NotificationRecord,
    SERIFY_FROM_FIELD(notification_id, u32)
    SERIFY_FROM_FIELD_SUM(channel, Channel)
    SERIFY_FROM_FIELD(urgent, bool)
)

inline std::vector<uint8_t> notification_marshal(const NotificationRecord& n) {
    std::vector<uint8_t> out;
    put_le<uint32_t>(out, n.notification_id, 4);

    // The tag ordinal is the alternative's position in the case file's sum,
    // which is its index in the variant above — so it needs no lookup table.
    // The schema tag *names* are the binding's business, and never appear here.
    out.push_back(static_cast<uint8_t>(n.channel.index()));
    switch (n.channel.index()) {
        case 0: break;  // a unit variant is nothing but its tag
        case 1: put_str(out, std::get<std::string>(n.channel)); break;
        case 2: put_le<uint64_t>(out, std::get<uint64_t>(n.channel), 8); break;
        case 3: {
            const auto& m = std::get<Money>(n.channel);
            put_str(out, m.currency);
            put_le<uint64_t>(out, static_cast<uint64_t>(m.amount_minor), 8);
            break;
        }
        default: throw std::runtime_error("unhandled channel alternative");
    }

    out.push_back(n.urgent ? 1 : 0);
    return out;
}

inline NotificationRecord notification_unmarshal(const std::vector<uint8_t>& data) {
    NotificationRecord n{};
    size_t off = 0;
    n.notification_id = take_le<uint32_t>(data, off, 4);

    switch (const uint8_t ord = data[off++]; ord) {
        case 0: n.channel.emplace<std::monostate>(); break;
        case 1: n.channel.emplace<std::string>(take_len_str(data, off)); break;
        case 2: n.channel.emplace<uint64_t>(take_le<uint64_t>(data, off, 8)); break;
        case 3: {
            Money m{};
            m.currency = take_len_str(data, off);
            m.amount_minor = static_cast<int64_t>(take_le<uint64_t>(data, off, 8));
            n.channel.emplace<Money>(std::move(m));
            break;
        }
        default: throw std::runtime_error("unknown channel ordinal " + std::to_string(ord));
    }

    n.urgent = data[off] != 0;
    return n;
}
