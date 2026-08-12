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

// CustomerRecord mirrors examples/cases/customer.yaml — a store account.
//
// Address mirrors the reusable address.yaml it imports; Money lives in
// money.hpp, which notification.hpp already needed. order.hpp will reuse both,
// as the Go worker does.
//
// This is the first user anywhere of SERIFY_FIELD_STRUCT,
// SERIFY_FIELD_LIST_STRUCT and SERIFY_FIELD_MAP_STRUCT, and of their FROM
// counterparts. All six existed with no caller, which for a macro means they
// had never been expanded, let alone run — the same state telemetry.hpp found
// SERIFY_FIELD_MAP_SCALAR in.
//
// The `json` format goes through serify::Json, the library's own value type —
// it already has to parse and write JSON to speak the protocol, so a C++ worker
// needs no dependency for this. Both formats declare `oracle: semantic`, so
// what has to match the reference is the decoded value rather than the bytes;
// Go's encoder HTML-escapes `<`, `>` and `&` and always escapes U+2028/U+2029,
// and under a byte oracle every worker would have to reproduce that quirk.
//
// The two 64-bit fields go out as Json::raw_ and come back off the parsed
// token, because a JSON number is a double and max uint64 does not survive one.
//
// Go is the --ref language and owns the byte layout; see examples/go/wire.go.

#pragma once

#include "money.hpp"
#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <optional>
#include <sstream>
#include <string>
#include <unordered_map>
#include <vector>

// base64, for the one `bytes` field. That is the form the reference worker's
// []byte marshals to, and the semantic oracle decodes our output with it.
inline std::string b64_encode(const std::vector<uint8_t>& in) {
    static const char* T = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    std::string out;
    out.reserve(((in.size() + 2) / 3) * 4);
    for (size_t i = 0; i < in.size(); i += 3) {
        const size_t n = in.size() - i;
        const uint32_t v = (uint32_t(in[i]) << 16)
                         | (n > 1 ? uint32_t(in[i + 1]) << 8 : 0)
                         | (n > 2 ? uint32_t(in[i + 2]) : 0);
        out += T[(v >> 18) & 63];
        out += T[(v >> 12) & 63];
        out += n > 1 ? T[(v >> 6) & 63] : '=';
        out += n > 2 ? T[v & 63] : '=';
    }
    return out;
}

inline std::vector<uint8_t> b64_decode(const std::string& s) {
    auto sextet = [](char c) -> int {
        if (c >= 'A' && c <= 'Z') return c - 'A';
        if (c >= 'a' && c <= 'z') return c - 'a' + 26;
        if (c >= '0' && c <= '9') return c - '0' + 52;
        if (c == '+') return 62;
        if (c == '/') return 63;
        return -1; // '=' padding, and anything else
    };
    std::vector<uint8_t> out;
    uint32_t acc = 0;
    int bits = 0;
    for (char c : s) {
        const int v = sextet(c);
        if (v < 0) continue;
        acc = (acc << 6) | uint32_t(v);
        bits += 6;
        if (bits >= 8) {
            bits -= 8;
            out.push_back(uint8_t((acc >> bits) & 0xFF));
        }
    }
    return out;
}

struct Address {
    std::string recipient;
    std::string street;
    std::string city;
    std::string country;
    std::string postal_code;
};

SERIFY_TO(Address,
    SERIFY_FIELD(recipient, string)
    SERIFY_FIELD(street, string)
    SERIFY_FIELD(city, string)
    SERIFY_FIELD(country, string)
    SERIFY_FIELD(postal_code, string)
)

SERIFY_FROM(Address,
    SERIFY_FROM_FIELD(recipient, string)
    SERIFY_FROM_FIELD(street, string)
    SERIFY_FROM_FIELD(city, string)
    SERIFY_FROM_FIELD(country, string)
    SERIFY_FROM_FIELD(postal_code, string)
)

// A struct is its fields back to back, in schema order — nothing frames it, so
// these take the surrounding buffer rather than owning one.
inline void address_pack(std::vector<uint8_t>& out, const Address& a) {
    put_str(out, a.recipient);
    put_str(out, a.street);
    put_str(out, a.city);
    put_str(out, a.country);
    put_str(out, a.postal_code);
}

inline Address address_unpack(const std::vector<uint8_t>& data, size_t& off) {
    Address a;
    a.recipient = take_len_str(data, off);
    a.street = take_len_str(data, off);
    a.city = take_len_str(data, off);
    a.country = take_len_str(data, off);
    a.postal_code = take_len_str(data, off);
    return a;
}

struct CustomerRecord {
    uint64_t customer_id{};
    std::string email;
    std::string display_name;
    uint8_t age{};
    bool email_verified{};
    float fraud_score{};
    uint32_t loyalty_points{};
    int64_t signup_ts{};
    serify::Bytes avatar_sha256;
    serify::ListU8 pin;  // array<uint8,4> — no count on the wire
    std::optional<std::string> referral_code;
    Money store_credit;
    std::vector<Address> shipping_addresses;
    std::unordered_map<std::string, Address> address_book;
    std::vector<std::string> wishlist_skus;
    std::unordered_map<std::string, std::string> preferences;
};

SERIFY_TO(CustomerRecord,
    SERIFY_FIELD(customer_id, u64)
    SERIFY_FIELD(email, string)
    SERIFY_FIELD(display_name, string)
    SERIFY_FIELD(age, u8)
    SERIFY_FIELD(email_verified, bool)
    SERIFY_FIELD(fraud_score, f32)
    SERIFY_FIELD(loyalty_points, u32)
    SERIFY_FIELD(signup_ts, i64)
    SERIFY_FIELD(avatar_sha256, bytes)
    SERIFY_FIELD(pin, list_u8)
    SERIFY_FIELD(referral_code, optional_string)
    SERIFY_FIELD_STRUCT(store_credit, Money);
    SERIFY_FIELD_LIST_STRUCT(shipping_addresses, Address);
    SERIFY_FIELD_MAP_STRUCT(address_book, Address);
    SERIFY_FIELD(wishlist_skus, list_string)
    SERIFY_FIELD_MAP_SCALAR(preferences, std::string);
)

SERIFY_FROM(CustomerRecord,
    SERIFY_FROM_FIELD(customer_id, u64)
    SERIFY_FROM_FIELD(email, string)
    SERIFY_FROM_FIELD(display_name, string)
    SERIFY_FROM_FIELD(age, u8)
    SERIFY_FROM_FIELD(email_verified, bool)
    SERIFY_FROM_FIELD(fraud_score, f32)
    SERIFY_FROM_FIELD(loyalty_points, u32)
    SERIFY_FROM_FIELD(signup_ts, i64)
    SERIFY_FROM_FIELD(avatar_sha256, bytes)
    SERIFY_FROM_FIELD(pin, list_u8)
    SERIFY_FROM_FIELD(referral_code, optional_string)
    SERIFY_FROM_FIELD_STRUCT(store_credit, Money)
    SERIFY_FROM_FIELD_LIST_STRUCT(shipping_addresses, Address)
    SERIFY_FROM_FIELD_MAP_STRUCT(address_book, Address)
    SERIFY_FROM_FIELD(wishlist_skus, list_string)
    SERIFY_FROM_FIELD_MAP_SCALAR(preferences, std::string)
)

inline std::vector<uint8_t> customer_marshal(const CustomerRecord& c) {
    std::vector<uint8_t> out;

    put_le<uint64_t>(out, c.customer_id, 8);
    put_str(out, c.email);
    put_str(out, c.display_name);

    out.push_back(c.age);
    out.push_back(c.email_verified ? 1 : 0);

    uint32_t fbits;
    std::memcpy(&fbits, &c.fraud_score, 4);
    put_le<uint32_t>(out, fbits, 4);
    put_le<uint32_t>(out, c.loyalty_points, 4);
    put_le<uint64_t>(out, static_cast<uint64_t>(c.signup_ts), 8);

    put_len_prefixed(out, c.avatar_sha256.data(), c.avatar_sha256.size());

    // array<T,N> carries no count: N is fixed by the schema.
    out.insert(out.end(), c.pin.begin(), c.pin.end());

    // optional<string>: a presence flag, then the value if present. An empty
    // string is present, which is why the flag cannot be inferred from it.
    if (!c.referral_code.has_value()) {
        out.push_back(0);
    } else {
        out.push_back(1);
        put_str(out, *c.referral_code);
    }

    money_pack(out, c.store_credit);

    put_le<uint32_t>(out, static_cast<uint32_t>(c.shipping_addresses.size()), 4);
    for (const auto& a : c.shipping_addresses) address_pack(out, a);

    // Entry order is the unordered_map's own — deliberately not sorted. A map is
    // unordered, so customer declares `oracle: semantic` and the decoded value
    // is what gets compared. See docs/protocol.md.
    put_le<uint32_t>(out, static_cast<uint32_t>(c.address_book.size()), 4);
    for (const auto& [k, a] : c.address_book) {
        put_str(out, k);
        address_pack(out, a);
    }

    put_le<uint32_t>(out, static_cast<uint32_t>(c.wishlist_skus.size()), 4);
    for (const auto& s : c.wishlist_skus) put_str(out, s);

    put_le<uint32_t>(out, static_cast<uint32_t>(c.preferences.size()), 4);
    for (const auto& [k, v] : c.preferences) {
        put_str(out, k);
        put_str(out, v);
    }

    return out;
}

inline CustomerRecord customer_unmarshal(const std::vector<uint8_t>& data) {
    CustomerRecord c{};
    size_t off = 0;

    c.customer_id = take_le<uint64_t>(data, off, 8);
    c.email = take_len_str(data, off);
    c.display_name = take_len_str(data, off);

    c.age = take_le<uint8_t>(data, off, 1);
    c.email_verified = take_le<uint8_t>(data, off, 1) != 0;

    uint32_t fbits = take_le<uint32_t>(data, off, 4);
    std::memcpy(&c.fraud_score, &fbits, 4);
    c.loyalty_points = take_le<uint32_t>(data, off, 4);
    c.signup_ts = static_cast<int64_t>(take_le<uint64_t>(data, off, 8));

    c.avatar_sha256 = take_len_prefixed(data, off);

    c.pin.assign(data.begin() + static_cast<std::ptrdiff_t>(off),
                 data.begin() + static_cast<std::ptrdiff_t>(off + 4));
    off += 4;

    if (take_le<uint8_t>(data, off, 1) == 0) {
        c.referral_code = std::nullopt;
    } else {
        c.referral_code = take_len_str(data, off);
    }

    c.store_credit = money_unpack(data, off);

    for (uint32_t n = take_le<uint32_t>(data, off, 4); n > 0; n--) {
        c.shipping_addresses.push_back(address_unpack(data, off));
    }

    for (uint32_t n = take_le<uint32_t>(data, off, 4); n > 0; n--) {
        std::string k = take_len_str(data, off);
        c.address_book[k] = address_unpack(data, off);
    }

    for (uint32_t n = take_le<uint32_t>(data, off, 4); n > 0; n--) {
        c.wishlist_skus.push_back(take_len_str(data, off));
    }

    for (uint32_t n = take_le<uint32_t>(data, off, 4); n > 0; n--) {
        std::string k = take_len_str(data, off);
        c.preferences[k] = take_len_str(data, off);
    }

    return c;
}

inline serify::Json address_json(const Address& a) {
    auto j = serify::Json::obj_();
    j.set("recipient", serify::Json::str_(a.recipient));
    j.set("street", serify::Json::str_(a.street));
    j.set("city", serify::Json::str_(a.city));
    j.set("country", serify::Json::str_(a.country));
    j.set("postal_code", serify::Json::str_(a.postal_code));
    return j;
}

inline Address address_from_json(const serify::Json& j) {
    Address a;
    a.recipient = j.get("recipient")->as_str();
    a.street = j.get("street")->as_str();
    a.city = j.get("city")->as_str();
    a.country = j.get("country")->as_str();
    a.postal_code = j.get("postal_code")->as_str();
    return a;
}

inline std::vector<uint8_t> customer_to_json(const CustomerRecord& c) {
    auto j = serify::Json::obj_();

    // raw_, not num_: a JSON number is a double, and the boundary case carries
    // max uint64 and min int64. Their decimal forms go out verbatim.
    j.set("customer_id", serify::Json::raw_(std::to_string(c.customer_id)));
    j.set("email", serify::Json::str_(c.email));
    j.set("display_name", serify::Json::str_(c.display_name));
    j.set("age", serify::Json::num_(c.age));
    j.set("email_verified", serify::Json::bool_(c.email_verified));
    j.set("fraud_score", serify::Json::num_(c.fraud_score));
    j.set("loyalty_points", serify::Json::num_(c.loyalty_points));
    j.set("signup_ts", serify::Json::raw_(std::to_string(c.signup_ts)));
    j.set("avatar_sha256", serify::Json::str_(b64_encode(c.avatar_sha256)));

    auto pin = serify::Json::arr_();
    for (uint8_t b : c.pin) pin.push(serify::Json::num_(b));
    j.set("pin", std::move(pin));

    j.set("referral_code", c.referral_code.has_value()
                               ? serify::Json::str_(*c.referral_code)
                               : serify::Json::null_());

    auto credit = serify::Json::obj_();
    credit.set("currency", serify::Json::str_(c.store_credit.currency));
    credit.set("amount_minor", serify::Json::raw_(std::to_string(c.store_credit.amount_minor)));
    j.set("store_credit", std::move(credit));

    auto addrs = serify::Json::arr_();
    for (const auto& a : c.shipping_addresses) addrs.push(address_json(a));
    j.set("shipping_addresses", std::move(addrs));

    auto book = serify::Json::obj_();
    for (const auto& [k, a] : c.address_book) book.set(k, address_json(a));
    j.set("address_book", std::move(book));

    auto skus = serify::Json::arr_();
    for (const auto& s : c.wishlist_skus) skus.push(serify::Json::str_(s));
    j.set("wishlist_skus", std::move(skus));

    auto prefs = serify::Json::obj_();
    for (const auto& [k, v] : c.preferences) prefs.set(k, serify::Json::str_(v));
    j.set("preferences", std::move(prefs));

    std::ostringstream os;
    serify::json_write(os, j);
    const std::string text = os.str();
    return std::vector<uint8_t>(text.begin(), text.end());
}

inline CustomerRecord customer_from_json(const std::vector<uint8_t>& data) {
    const serify::Json j =
        serify::parse_json(std::string(data.begin(), data.end()));
    CustomerRecord c{};

    // as_token(), not as_num(): the parser keeps every number's source text, so
    // these two are read from the digits rather than from a rounded double.
    c.customer_id = std::strtoull(j.get("customer_id")->as_token().c_str(), nullptr, 10);
    c.email = j.get("email")->as_str();
    c.display_name = j.get("display_name")->as_str();
    c.age = static_cast<uint8_t>(j.get("age")->as_num());
    c.email_verified = j.get("email_verified")->as_bool();
    // A JSON number is a double; narrowing to float is what the wire does, so a
    // float32 field holds a value float32 can actually represent.
    c.fraud_score = static_cast<float>(j.get("fraud_score")->as_num());
    c.loyalty_points = static_cast<uint32_t>(j.get("loyalty_points")->as_num());
    c.signup_ts = std::strtoll(j.get("signup_ts")->as_token().c_str(), nullptr, 10);
    c.avatar_sha256 = b64_decode(j.get("avatar_sha256")->as_str());

    for (const auto& b : j.get("pin")->arr) {
        c.pin.push_back(static_cast<uint8_t>(b.as_num()));
    }

    const serify::Json* ref = j.get("referral_code");
    if (ref->is_null()) c.referral_code = std::nullopt;
    else c.referral_code = ref->as_str();

    const serify::Json* credit = j.get("store_credit");
    c.store_credit.currency = credit->get("currency")->as_str();
    c.store_credit.amount_minor =
        std::strtoll(credit->get("amount_minor")->as_token().c_str(), nullptr, 10);

    for (const auto& a : j.get("shipping_addresses")->arr) {
        c.shipping_addresses.push_back(address_from_json(a));
    }
    for (const auto& [k, a] : j.get("address_book")->obj) {
        c.address_book[k] = address_from_json(a);
    }
    for (const auto& s : j.get("wishlist_skus")->arr) {
        c.wishlist_skus.push_back(s.as_str());
    }
    for (const auto& [k, v] : j.get("preferences")->obj) {
        c.preferences[k] = v.as_str();
    }

    return c;
}
