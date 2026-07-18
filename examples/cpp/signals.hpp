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

// SignalCapture mirrors examples/cases/signals.yaml, which uses every scalar the
// schema allows as a list element.
//
// C++ has a distinct vector type per element width, so each field names its kind
// once in the SERIFY_* block and nothing here calls an accessor directly. A
// list<uint8> and a bytes are both std::vector<uint8_t>, which is why the library
// carries them as the same FieldValue alternative.
//
// Go is the --ref language and owns the byte layout; see examples/go/wire.go.
// Each list is a u32 element count followed by its elements, little-endian.

#pragma once

#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <array>
#include <string>
#include <optional>
#include <vector>

// Declaration order of the `mode` enum; the index is the wire ordinal.
static const std::array<std::string, 4> SIGNAL_MODES = {"idle", "active", "fault", "calibrating"};

struct SignalCapture {
    uint64_t capture_id{};
    serify::ListBool   flags;
    serify::ListU8     raw_frame;
    serify::ListU16    port_numbers;
    serify::ListU32    sample_counts;
    serify::ListU64    byte_totals;
    serify::ListI8     trim_offsets;
    serify::ListI16    drift_deltas;
    serify::ListI32    temperatures_c;
    serify::ListI64    timestamps_ns;
    serify::ListU128   counters;
    serify::ListI128   balances;
    serify::ListF32    gains;
    serify::ListF64    voltages;
    serify::ListString channel_names;
    serify::ListBytes  payloads;
    serify::ListU8     checksum;
    serify::ListI16    window;
    std::optional<uint32_t> dropped_frames;
    std::string mode;
};

SERIFY_TO(SignalCapture,
    SERIFY_FIELD(capture_id, u64)
    SERIFY_FIELD(flags, list_bool)
    SERIFY_FIELD(raw_frame, list_u8)
    SERIFY_FIELD(port_numbers, list_u16)
    SERIFY_FIELD(sample_counts, list_u32)
    SERIFY_FIELD(byte_totals, list_u64)
    SERIFY_FIELD(trim_offsets, list_i8)
    SERIFY_FIELD(drift_deltas, list_i16)
    SERIFY_FIELD(temperatures_c, list_i32)
    SERIFY_FIELD(timestamps_ns, list_i64)
    SERIFY_FIELD(counters, list_u128)
    SERIFY_FIELD(balances, list_i128)
    SERIFY_FIELD(gains, list_f32)
    SERIFY_FIELD(voltages, list_f64)
    SERIFY_FIELD(channel_names, list_string)
    SERIFY_FIELD(payloads, list_bytes)
    SERIFY_FIELD(checksum, list_u8)
    SERIFY_FIELD(window, list_i16)
    SERIFY_FIELD_OPTIONAL(dropped_frames, u32)
    SERIFY_FIELD(mode, string)
)

SERIFY_FROM(SignalCapture,
    SERIFY_FROM_FIELD(capture_id, u64)
    SERIFY_FROM_FIELD(flags, list_bool)
    SERIFY_FROM_FIELD(raw_frame, list_u8)
    SERIFY_FROM_FIELD(port_numbers, list_u16)
    SERIFY_FROM_FIELD(sample_counts, list_u32)
    SERIFY_FROM_FIELD(byte_totals, list_u64)
    SERIFY_FROM_FIELD(trim_offsets, list_i8)
    SERIFY_FROM_FIELD(drift_deltas, list_i16)
    SERIFY_FROM_FIELD(temperatures_c, list_i32)
    SERIFY_FROM_FIELD(timestamps_ns, list_i64)
    SERIFY_FROM_FIELD(counters, list_u128)
    SERIFY_FROM_FIELD(balances, list_i128)
    SERIFY_FROM_FIELD(gains, list_f32)
    SERIFY_FROM_FIELD(voltages, list_f64)
    SERIFY_FROM_FIELD(channel_names, list_string)
    SERIFY_FROM_FIELD(payloads, list_bytes)
    SERIFY_FROM_FIELD(checksum, list_u8)
    SERIFY_FROM_FIELD(window, list_i16)
    SERIFY_FROM_FIELD_OPTIONAL(dropped_frames, u32)
    SERIFY_FROM_FIELD(mode, string)
)

// u32 element count, then each element written by `f`.
template <typename L, typename F>
inline void put_list(std::vector<uint8_t>& out, const L& items, F f) {
    put_le<uint32_t>(out, static_cast<uint32_t>(items.size()), 4);
    for (const auto& x : items) f(out, x);
}

// Read a u32 count, then that many fixed-width elements of `nbytes` each.
template <typename L, typename U>
inline L take_list(const std::vector<uint8_t>& data, size_t& off, size_t nbytes) {
    const size_t count = take_le<uint32_t>(data, off, 4);
    L out;
    out.reserve(count);
    for (size_t i = 0; i < count; ++i) {
        out.push_back(static_cast<typename L::value_type>(take_le<U>(data, off, nbytes)));
    }
    return out;
}

inline std::vector<uint8_t> signals_marshal(const SignalCapture& s) {
    std::vector<uint8_t> out;
    put_le<uint64_t>(out, s.capture_id, 8);

    put_list(out, s.flags, [](std::vector<uint8_t>& o, bool v) { o.push_back(v ? 1 : 0); });
    put_list(out, s.raw_frame, [](std::vector<uint8_t>& o, uint8_t v) { o.push_back(v); });
    put_list(out, s.port_numbers, [](std::vector<uint8_t>& o, uint16_t v) { put_le<uint16_t>(o, v, 2); });
    put_list(out, s.sample_counts, [](std::vector<uint8_t>& o, uint32_t v) { put_le<uint32_t>(o, v, 4); });
    put_list(out, s.byte_totals, [](std::vector<uint8_t>& o, uint64_t v) { put_le<uint64_t>(o, v, 8); });
    put_list(out, s.trim_offsets, [](std::vector<uint8_t>& o, int8_t v) { put_le<uint8_t>(o, static_cast<uint8_t>(v), 1); });
    put_list(out, s.drift_deltas, [](std::vector<uint8_t>& o, int16_t v) { put_le<uint16_t>(o, static_cast<uint16_t>(v), 2); });
    put_list(out, s.temperatures_c, [](std::vector<uint8_t>& o, int32_t v) { put_le<uint32_t>(o, static_cast<uint32_t>(v), 4); });
    put_list(out, s.timestamps_ns, [](std::vector<uint8_t>& o, int64_t v) { put_le<uint64_t>(o, static_cast<uint64_t>(v), 8); });
    put_list(out, s.counters, [](std::vector<uint8_t>& o, serify::u128 v) { put_le<serify::u128>(o, v, 16); });
    put_list(out, s.balances, [](std::vector<uint8_t>& o, serify::i128 v) { put_le<serify::u128>(o, static_cast<serify::u128>(v), 16); });
    put_list(out, s.gains, [](std::vector<uint8_t>& o, float v) {
        uint32_t bits; memcpy(&bits, &v, 4); put_le<uint32_t>(o, bits, 4);
    });
    put_list(out, s.voltages, [](std::vector<uint8_t>& o, double v) {
        uint64_t bits; memcpy(&bits, &v, 8); put_le<uint64_t>(o, bits, 8);
    });
    put_list(out, s.channel_names, [](std::vector<uint8_t>& o, const std::string& v) { put_str(o, v); });
    put_list(out, s.payloads, [](std::vector<uint8_t>& o, const serify::Bytes& v) {
        put_len_prefixed(o, v.data(), v.size());
    });

    // array<T,N> carries no count: N is fixed by the schema.
    for (auto v : s.checksum) out.push_back(v);
    for (auto v : s.window) put_le<uint16_t>(out, static_cast<uint16_t>(v), 2);

    // optional<uint32>: a presence flag, then the value if present.
    if (!s.dropped_frames.has_value()) {
        out.push_back(0);
    } else {
        out.push_back(1);
        put_le<uint32_t>(out, *s.dropped_frames, 4);
    }

    // enum: a u8 ordinal, the variant's position in the case file.
    for (size_t i = 0; i < SIGNAL_MODES.size(); ++i) {
        if (SIGNAL_MODES[i] == s.mode) { out.push_back(static_cast<uint8_t>(i)); break; }
    }

    return out;
}

inline SignalCapture signals_unmarshal(const std::vector<uint8_t>& data) {
    SignalCapture s;
    size_t off = 0;
    s.capture_id = take_le<uint64_t>(data, off, 8);

    const size_t flag_count = take_le<uint32_t>(data, off, 4);
    s.flags.reserve(flag_count);
    for (size_t i = 0; i < flag_count; ++i) {
        s.flags.push_back(take_le<uint8_t>(data, off, 1) != 0);
    }

    s.raw_frame      = take_list<serify::ListU8,   uint8_t>(data, off, 1);
    s.port_numbers   = take_list<serify::ListU16,  uint16_t>(data, off, 2);
    s.sample_counts  = take_list<serify::ListU32,  uint32_t>(data, off, 4);
    s.byte_totals    = take_list<serify::ListU64,  uint64_t>(data, off, 8);
    s.trim_offsets   = take_list<serify::ListI8,   uint8_t>(data, off, 1);
    s.drift_deltas   = take_list<serify::ListI16,  uint16_t>(data, off, 2);
    s.temperatures_c = take_list<serify::ListI32,  uint32_t>(data, off, 4);
    s.timestamps_ns  = take_list<serify::ListI64,  uint64_t>(data, off, 8);
    s.counters       = take_list<serify::ListU128, serify::u128>(data, off, 16);
    s.balances       = take_list<serify::ListI128, serify::u128>(data, off, 16);

    const size_t gain_count = take_le<uint32_t>(data, off, 4);
    s.gains.reserve(gain_count);
    for (size_t i = 0; i < gain_count; ++i) {
        uint32_t bits = take_le<uint32_t>(data, off, 4);
        float f; memcpy(&f, &bits, 4);
        s.gains.push_back(f);
    }

    const size_t volt_count = take_le<uint32_t>(data, off, 4);
    s.voltages.reserve(volt_count);
    for (size_t i = 0; i < volt_count; ++i) {
        uint64_t bits = take_le<uint64_t>(data, off, 8);
        double d; memcpy(&d, &bits, 8);
        s.voltages.push_back(d);
    }

    const size_t name_count = take_le<uint32_t>(data, off, 4);
    s.channel_names.reserve(name_count);
    for (size_t i = 0; i < name_count; ++i) {
        s.channel_names.push_back(take_len_str(data, off));
    }

    const size_t payload_count = take_le<uint32_t>(data, off, 4);
    s.payloads.reserve(payload_count);
    for (size_t i = 0; i < payload_count; ++i) {
        s.payloads.push_back(take_len_prefixed(data, off));
    }

    for (int i = 0; i < 4; ++i) s.checksum.push_back(take_le<uint8_t>(data, off, 1));
    for (int i = 0; i < 3; ++i) s.window.push_back(static_cast<int16_t>(take_le<uint16_t>(data, off, 2)));

    if (take_le<uint8_t>(data, off, 1) == 0) {
        s.dropped_frames = std::nullopt;
    } else {
        s.dropped_frames = static_cast<uint32_t>(take_le<uint32_t>(data, off, 4));
    }

    s.mode = SIGNAL_MODES[take_le<uint8_t>(data, off, 1)];

    return s;
}
