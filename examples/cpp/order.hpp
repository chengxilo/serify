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

// OrderRecord mirrors examples/cases/order.yaml — a placed order.
//
// LineItem mirrors the reusable line_item.yaml it imports, which itself nests
// money — so any type using it exercises struct-inside-struct. Address comes
// from customer.hpp and Money from money.hpp, as they do in the Go worker.
//
// Between them the fields cover the four composite types nothing else in the
// suite exercises end to end: an enum, a list<struct>, a map<string,struct> and
// an optional<struct>. billing_address is the suite's only optional<struct>,
// and the C++ library had no macro for one — SERIFY_FIELD_OPTIONAL covers an
// optional scalar and SERIFY_FIELD_STRUCT a required struct, but the FieldMap
// slot for this holds a std::optional<StructPtr>, which is neither. This is
// what SERIFY_FIELD_OPTIONAL_STRUCT was added for.
//
// An enum needs nothing from the binding: it travels as its variant *name*, so
// the field is a plain std::string. The u8 ordinal in the layout is this
// worker's own choice, which is why kStatuses has to match the case file's
// declaration order.
//
// Go is the --ref language and owns the layout; see examples/go/wire.go.

#pragma once

#include "customer.hpp"  // Address, and its pack/unpack
#include "money.hpp"
#include "serify.hpp"
#include "wire.hpp"

#include <algorithm>
#include <cstdint>
#include <optional>
#include <stdexcept>
#include <string>
#include <unordered_map>
#include <vector>

// Declaration order of the `status` enum in examples/cases/order.yaml.
inline const std::vector<std::string>& order_statuses() {
    static const std::vector<std::string> kStatuses = {
        "pending", "paid", "shipped", "delivered", "cancelled"};
    return kStatuses;
}

struct LineItem {
    std::string sku;
    std::string product_name;
    uint16_t quantity{};
    Money unit_price;
    uint8_t discount_pct{};
    bool gift_wrap{};
};

SERIFY_TO(LineItem,
    SERIFY_FIELD(sku, string)
    SERIFY_FIELD(product_name, string)
    SERIFY_FIELD(quantity, u16)
    SERIFY_FIELD_STRUCT(unit_price, Money);
    SERIFY_FIELD(discount_pct, u8)
    SERIFY_FIELD(gift_wrap, bool)
)

SERIFY_FROM(LineItem,
    SERIFY_FROM_FIELD(sku, string)
    SERIFY_FROM_FIELD(product_name, string)
    SERIFY_FROM_FIELD(quantity, u16)
    SERIFY_FROM_FIELD_STRUCT(unit_price, Money)
    SERIFY_FROM_FIELD(discount_pct, u8)
    SERIFY_FROM_FIELD(gift_wrap, bool)
)

inline void line_item_pack(std::vector<uint8_t>& out, const LineItem& it) {
    put_str(out, it.sku);
    put_str(out, it.product_name);
    put_le<uint16_t>(out, it.quantity, 2);
    money_pack(out, it.unit_price);
    out.push_back(it.discount_pct);
    out.push_back(it.gift_wrap ? 1 : 0);
}

inline LineItem line_item_unpack(const std::vector<uint8_t>& data, size_t& off) {
    LineItem it;
    it.sku = take_len_str(data, off);
    it.product_name = take_len_str(data, off);
    it.quantity = take_le<uint16_t>(data, off, 2);
    it.unit_price = money_unpack(data, off);
    it.discount_pct = take_le<uint8_t>(data, off, 1);
    it.gift_wrap = take_le<uint8_t>(data, off, 1) != 0;
    return it;
}

struct OrderRecord {
    uint64_t order_id{};
    uint64_t customer_id{};
    int64_t created_at{};
    std::string status;
    std::vector<LineItem> items;
    Money subtotal;
    std::unordered_map<std::string, Money> adjustments;
    Money total;
    Address shipping_address;
    std::optional<Address> billing_address;
    std::vector<std::string> coupon_codes;
    std::optional<std::string> tracking_number;
};

SERIFY_TO(OrderRecord,
    SERIFY_FIELD(order_id, u64)
    SERIFY_FIELD(customer_id, u64)
    SERIFY_FIELD(created_at, i64)
    SERIFY_FIELD(status, string)
    SERIFY_FIELD_LIST_STRUCT(items, LineItem);
    SERIFY_FIELD_STRUCT(subtotal, Money);
    SERIFY_FIELD_MAP_STRUCT(adjustments, Money);
    SERIFY_FIELD_STRUCT(total, Money);
    SERIFY_FIELD_STRUCT(shipping_address, Address);
    SERIFY_FIELD_OPTIONAL_STRUCT(billing_address, Address)
    SERIFY_FIELD(coupon_codes, list_string)
    SERIFY_FIELD(tracking_number, optional_string)
)

SERIFY_FROM(OrderRecord,
    SERIFY_FROM_FIELD(order_id, u64)
    SERIFY_FROM_FIELD(customer_id, u64)
    SERIFY_FROM_FIELD(created_at, i64)
    SERIFY_FROM_FIELD(status, string)
    SERIFY_FROM_FIELD_LIST_STRUCT(items, LineItem)
    SERIFY_FROM_FIELD_STRUCT(subtotal, Money)
    SERIFY_FROM_FIELD_MAP_STRUCT(adjustments, Money)
    SERIFY_FROM_FIELD_STRUCT(total, Money)
    SERIFY_FROM_FIELD_STRUCT(shipping_address, Address)
    SERIFY_FROM_FIELD_OPTIONAL_STRUCT(billing_address, Address);
    SERIFY_FROM_FIELD(coupon_codes, list_string)
    SERIFY_FROM_FIELD(tracking_number, optional_string)
)

inline std::vector<uint8_t> order_marshal(const OrderRecord& o) {
    std::vector<uint8_t> out;

    put_le<uint64_t>(out, o.order_id, 8);
    put_le<uint64_t>(out, o.customer_id, 8);
    put_le<uint64_t>(out, static_cast<uint64_t>(o.created_at), 8);

    // enum: a u8 ordinal, the variant's position in the case file.
    const auto& statuses = order_statuses();
    const auto it = std::find(statuses.begin(), statuses.end(), o.status);
    if (it == statuses.end()) throw std::runtime_error("unknown order status \"" + o.status + "\"");
    out.push_back(static_cast<uint8_t>(it - statuses.begin()));

    put_le<uint32_t>(out, static_cast<uint32_t>(o.items.size()), 4);
    for (const auto& li : o.items) line_item_pack(out, li);

    money_pack(out, o.subtotal);

    // Entry order is the unordered_map's own — deliberately not sorted. A map is
    // unordered, so order declares `oracle: semantic` and the decoded value is
    // what gets compared. See docs/protocol.md.
    put_le<uint32_t>(out, static_cast<uint32_t>(o.adjustments.size()), 4);
    for (const auto& [k, m] : o.adjustments) {
        put_str(out, k);
        money_pack(out, m);
    }

    money_pack(out, o.total);
    address_pack(out, o.shipping_address);

    // optional<struct>: a presence flag, then the struct's fields inline.
    if (!o.billing_address.has_value()) {
        out.push_back(0);
    } else {
        out.push_back(1);
        address_pack(out, *o.billing_address);
    }

    put_le<uint32_t>(out, static_cast<uint32_t>(o.coupon_codes.size()), 4);
    for (const auto& c : o.coupon_codes) put_str(out, c);

    if (!o.tracking_number.has_value()) {
        out.push_back(0);
    } else {
        out.push_back(1);
        put_str(out, *o.tracking_number);
    }

    return out;
}

inline OrderRecord order_unmarshal(const std::vector<uint8_t>& data) {
    OrderRecord o{};
    size_t off = 0;

    o.order_id = take_le<uint64_t>(data, off, 8);
    o.customer_id = take_le<uint64_t>(data, off, 8);
    o.created_at = static_cast<int64_t>(take_le<uint64_t>(data, off, 8));

    const auto ord = take_le<uint8_t>(data, off, 1);
    const auto& statuses = order_statuses();
    if (ord >= statuses.size()) {
        throw std::runtime_error("status ordinal " + std::to_string(ord) + " is out of range");
    }
    o.status = statuses[ord];

    for (uint32_t n = take_le<uint32_t>(data, off, 4); n > 0; n--) {
        o.items.push_back(line_item_unpack(data, off));
    }

    o.subtotal = money_unpack(data, off);

    for (uint32_t n = take_le<uint32_t>(data, off, 4); n > 0; n--) {
        std::string k = take_len_str(data, off);
        o.adjustments[k] = money_unpack(data, off);
    }

    o.total = money_unpack(data, off);
    o.shipping_address = address_unpack(data, off);

    if (take_le<uint8_t>(data, off, 1) == 0) {
        o.billing_address = std::nullopt;
    } else {
        o.billing_address = address_unpack(data, off);
    }

    for (uint32_t n = take_le<uint32_t>(data, off, 4); n > 0; n--) {
        o.coupon_codes.push_back(take_len_str(data, off));
    }

    if (take_le<uint8_t>(data, off, 1) == 0) {
        o.tracking_number = std::nullopt;
    } else {
        o.tracking_number = take_len_str(data, off);
    }

    return o;
}
