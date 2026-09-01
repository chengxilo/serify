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


// Framing: a u32 little-endian byte length, then that many bytes.
//
// Outside the conformance suite on purpose. serify tests the contents of a
// message; the frame header is in none of the cases, so a follower may read its
// socket however it likes as long as it agrees on what is inside.
//
// POSIX sockets, so this compiles on Linux and macOS. Winsock would need a
// different include and a WSAStartup, and nothing below the frame would change.

#pragma once

#include <cstdint>
#include <stdexcept>
#include <string>
#include <vector>

#include <sys/socket.h>
#include <unistd.h>

namespace taskstore {

/// Caps a single message, so a bad length prefix is not a 4 GiB allocation.
constexpr uint32_t MAX_FRAME_LEN = 1u << 20;

inline void write_frame(int fd, const std::vector<uint8_t>& payload) {
    if (payload.size() > MAX_FRAME_LEN)
        throw std::runtime_error("message exceeds the frame limit");

    std::vector<uint8_t> framed;
    framed.reserve(4 + payload.size());
    const auto n = static_cast<uint32_t>(payload.size());
    for (int i = 0; i < 4; ++i) framed.push_back(static_cast<uint8_t>(n >> (8 * i)));
    framed.insert(framed.end(), payload.begin(), payload.end());

    size_t sent = 0;
    while (sent < framed.size()) {
        const ssize_t w = ::send(fd, framed.data() + sent, framed.size() - sent, 0);
        if (w <= 0) throw std::runtime_error("send failed");
        sent += static_cast<size_t>(w);
    }
}

inline void read_exact(int fd, uint8_t* dst, size_t n) {
    size_t got = 0;
    while (got < n) {
        const ssize_t r = ::recv(fd, dst + got, n - got, 0);
        if (r <= 0) throw std::runtime_error("connection closed mid-frame");
        got += static_cast<size_t>(r);
    }
}

inline std::vector<uint8_t> read_frame(int fd) {
    uint8_t header[4];
    read_exact(fd, header, 4);

    uint32_t n = 0;
    for (int i = 3; i >= 0; --i) n = (n << 8) | header[i];
    if (n > MAX_FRAME_LEN)
        throw std::runtime_error("frame header claims " + std::to_string(n) +
                                 " bytes, over the limit");

    std::vector<uint8_t> payload(n);
    if (n > 0) read_exact(fd, payload.data(), n);
    return payload;
}

}  // namespace taskstore
