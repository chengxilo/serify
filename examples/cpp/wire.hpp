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

// Byte-level primitives shared by the models in this worker.
//
// Encoding is plain shift-and-mask into a std::vector<uint8_t>, which makes the
// little-endian layout explicit and independent of the host byte order.
//
// Go is the --ref language and owns the layout these reproduce; see the comment
// at the top of examples/go/wire.go.

#pragma once

#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>

// Append `nbytes` of `u` in little-endian order.
template <typename U>
inline void put_le(std::vector<uint8_t>& out, U u, size_t nbytes) {
    for (size_t i = 0; i < nbytes; ++i) {
        out.push_back(static_cast<uint8_t>(u >> (8 * i)));
    }
}

// Read `nbytes` little-endian bytes at `off` and advance it.
template <typename U>
inline U take_le(const std::vector<uint8_t>& data, size_t& off, size_t nbytes) {
    U u = 0;
    for (size_t i = 0; i < nbytes; ++i) {
        u |= static_cast<U>(data[off + i]) << (8 * i);
    }
    off += nbytes;
    return u;
}

inline void put_len_prefixed(std::vector<uint8_t>& out, const uint8_t* body, size_t n) {
    put_le<uint32_t>(out, static_cast<uint32_t>(n), 4);
    out.insert(out.end(), body, body + n);
}

inline void put_str(std::vector<uint8_t>& out, const std::string& s) {
    put_len_prefixed(out, reinterpret_cast<const uint8_t*>(s.data()), s.size());
}

inline std::vector<uint8_t> take_len_prefixed(const std::vector<uint8_t>& data, size_t& off) {
    const size_t n = take_le<uint32_t>(data, off, 4);
    std::vector<uint8_t> body(data.begin() + static_cast<std::ptrdiff_t>(off),
                              data.begin() + static_cast<std::ptrdiff_t>(off + n));
    off += n;
    return body;
}

inline std::string take_len_str(const std::vector<uint8_t>& data, size_t& off) {
    const size_t n = take_le<uint32_t>(data, off, 4);
    std::string s(reinterpret_cast<const char*>(data.data() + off), n);
    off += n;
    return s;
}
