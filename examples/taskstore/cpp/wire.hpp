/**
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */


// Byte-level primitives. Go owns the layout these reproduce; the conventions
// are documented at the top of go/api/wire.go.

#pragma once

#include <cstdint>
#include <optional>
#include <stdexcept>
#include <string>
#include <vector>

namespace taskstore {

/// A cursor over one message's bytes.
class Reader {
public:
    explicit Reader(const std::vector<uint8_t>& data) : data_(data) {}

    uint8_t u8() { return take(1)[0]; }

    uint32_t u32() {
        const uint8_t* p = take(4);
        return static_cast<uint32_t>(p[0]) | (static_cast<uint32_t>(p[1]) << 8) |
               (static_cast<uint32_t>(p[2]) << 16) | (static_cast<uint32_t>(p[3]) << 24);
    }

    uint64_t u64() {
        const uint8_t* p = take(8);
        uint64_t v = 0;
        for (int i = 7; i >= 0; --i) v = (v << 8) | p[i];
        return v;
    }

    int64_t i64() { return static_cast<int64_t>(u64()); }

    bool boolean() { return u8() != 0; }

    std::string str() {
        const uint32_t n = u32();
        const uint8_t* p = take(n);
        return std::string(reinterpret_cast<const char*>(p), n);
    }

    /// An enum arrives as an ordinal into a fixed list, so a value outside the
    /// declared variants has no encoding and cannot be read back.
    std::string enum_of(const std::vector<std::string>& variants) {
        const uint8_t ord = u8();
        if (ord >= variants.size())
            throw std::runtime_error("enum ordinal " + std::to_string(ord) + " out of range");
        return variants[ord];
    }

    std::vector<std::string> tags() {
        std::vector<std::string> out;
        const uint32_t n = u32();
        out.reserve(n);
        for (uint32_t i = 0; i < n; ++i) out.push_back(str());
        return out;
    }

    std::optional<int64_t> optional_i64() {
        if (u8() == 0) return std::nullopt;
        return i64();
    }

private:
    const uint8_t* take(size_t n) {
        if (off_ + n > data_.size()) throw std::runtime_error("truncated message");
        const uint8_t* p = data_.data() + off_;
        off_ += n;
        return p;
    }

    const std::vector<uint8_t>& data_;
    size_t off_ = 0;
};

inline void put_u32(std::vector<uint8_t>& out, uint32_t v) {
    for (int i = 0; i < 4; ++i) out.push_back(static_cast<uint8_t>(v >> (8 * i)));
}

inline void put_u64(std::vector<uint8_t>& out, uint64_t v) {
    for (int i = 0; i < 8; ++i) out.push_back(static_cast<uint8_t>(v >> (8 * i)));
}

inline void put_str(std::vector<uint8_t>& out, const std::string& s) {
    put_u32(out, static_cast<uint32_t>(s.size()));
    out.insert(out.end(), s.begin(), s.end());
}

inline void put_enum(std::vector<uint8_t>& out, const std::vector<std::string>& variants,
                     const std::string& name) {
    for (size_t i = 0; i < variants.size(); ++i) {
        if (variants[i] == name) {
            out.push_back(static_cast<uint8_t>(i));
            return;
        }
    }
    throw std::runtime_error("\"" + name + "\" is not a declared variant");
}

inline void put_tags(std::vector<uint8_t>& out, const std::vector<std::string>& tags) {
    put_u32(out, static_cast<uint32_t>(tags.size()));
    for (const auto& t : tags) put_str(out, t);
}

inline void put_optional_i64(std::vector<uint8_t>& out, const std::optional<int64_t>& v) {
    if (!v.has_value()) {
        out.push_back(0);
        return;
    }
    out.push_back(1);
    put_u64(out, static_cast<uint64_t>(*v));
}

}  // namespace taskstore
