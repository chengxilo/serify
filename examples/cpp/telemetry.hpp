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

// TelemetryFrame mirrors examples/cases/telemetry.yaml — one reading from a
// field device.
//
// This is the type that covers the corners the other examples do not: a
// uint128 address, two differently shaped fixed arrays, the suite's only
// optional<scalar>, a map<string,uint64>, and float cases running through NaN,
// ±Inf and negative zero. Only `binary` is declared, because NaN and Inf have
// no JSON spelling.
//
// It is also the first user of SERIFY_FIELD_MAP_SCALAR and its FROM counterpart.
// Both macros existed with no caller anywhere in the tree, which for a macro
// means they were never even expanded, let alone run.
//
// Go is the --ref language and owns the byte layout; see examples/go/wire.go.

#pragma once

#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <cstring>
#include <optional>
#include <string>
#include <unordered_map>

struct TelemetryFrame {
    uint64_t device_id{};
    serify::u128 ipv6{};
    serify::ListU8 local_ip;   // array<uint8,4> — no count on the wire
    std::string firmware;
    uint16_t boot_count{};
    int8_t rssi_dbm{};
    int16_t temperature_dc{};
    int32_t clock_drift_ms{};
    float battery_volts{};
    double latitude{};
    double longitude{};
    std::optional<float> humidity_pct;
    serify::ListI16 accel_mg;  // array<int16,3>
    serify::ListU32 visible_cells;
    std::unordered_map<std::string, uint64_t> packet_counts;
    bool gps_fix{};
    serify::Bytes signature;
};

SERIFY_TO(TelemetryFrame,
    SERIFY_FIELD(device_id, u64)
    SERIFY_FIELD(ipv6, u128)
    SERIFY_FIELD(local_ip, list_u8)
    SERIFY_FIELD(firmware, string)
    SERIFY_FIELD(boot_count, u16)
    SERIFY_FIELD(rssi_dbm, i8)
    SERIFY_FIELD(temperature_dc, i16)
    SERIFY_FIELD(clock_drift_ms, i32)
    SERIFY_FIELD(battery_volts, f32)
    SERIFY_FIELD(latitude, f64)
    SERIFY_FIELD(longitude, f64)
    SERIFY_FIELD_OPTIONAL(humidity_pct, f32)
    SERIFY_FIELD(accel_mg, list_i16)
    SERIFY_FIELD(visible_cells, list_u32)
    SERIFY_FIELD_MAP_SCALAR(packet_counts, uint64_t);
    SERIFY_FIELD(gps_fix, bool)
    SERIFY_FIELD(signature, bytes)
)

SERIFY_FROM(TelemetryFrame,
    SERIFY_FROM_FIELD(device_id, u64)
    SERIFY_FROM_FIELD(ipv6, u128)
    SERIFY_FROM_FIELD(local_ip, list_u8)
    SERIFY_FROM_FIELD(firmware, string)
    SERIFY_FROM_FIELD(boot_count, u16)
    SERIFY_FROM_FIELD(rssi_dbm, i8)
    SERIFY_FROM_FIELD(temperature_dc, i16)
    SERIFY_FROM_FIELD(clock_drift_ms, i32)
    SERIFY_FROM_FIELD(battery_volts, f32)
    SERIFY_FROM_FIELD(latitude, f64)
    SERIFY_FROM_FIELD(longitude, f64)
    SERIFY_FROM_FIELD_OPTIONAL(humidity_pct, f32)
    SERIFY_FROM_FIELD(accel_mg, list_i16)
    SERIFY_FROM_FIELD(visible_cells, list_u32)
    SERIFY_FROM_FIELD_MAP_SCALAR(packet_counts, uint64_t)
    SERIFY_FROM_FIELD(gps_fix, bool)
    SERIFY_FROM_FIELD(signature, bytes)
)

inline std::vector<uint8_t> telemetry_marshal(const TelemetryFrame& t) {
    std::vector<uint8_t> out;
    put_le<uint64_t>(out, t.device_id, 8);

    // uint128 is unsigned, so the same 16 little-endian bytes int128 uses serve
    // here with no sign to re-apply on the way back.
    put_le<serify::u128>(out, t.ipv6, 16);

    // array<T,N> carries no count: N is fixed by the schema.
    out.insert(out.end(), t.local_ip.begin(), t.local_ip.end());

    put_str(out, t.firmware);
    put_le<uint16_t>(out, t.boot_count, 2);
    out.push_back(static_cast<uint8_t>(t.rssi_dbm));
    put_le<uint16_t>(out, static_cast<uint16_t>(t.temperature_dc), 2);
    put_le<uint32_t>(out, static_cast<uint32_t>(t.clock_drift_ms), 4);

    uint32_t fbits;
    std::memcpy(&fbits, &t.battery_volts, 4);
    put_le<uint32_t>(out, fbits, 4);
    uint64_t dbits;
    std::memcpy(&dbits, &t.latitude, 8);
    put_le<uint64_t>(out, dbits, 8);
    std::memcpy(&dbits, &t.longitude, 8);
    put_le<uint64_t>(out, dbits, 8);

    // optional<float32>: a presence flag, then the value if present.
    if (!t.humidity_pct.has_value()) {
        out.push_back(0);
    } else {
        out.push_back(1);
        float h = *t.humidity_pct;
        std::memcpy(&fbits, &h, 4);
        put_le<uint32_t>(out, fbits, 4);
    }

    for (int16_t v : t.accel_mg) put_le<uint16_t>(out, static_cast<uint16_t>(v), 2);

    put_le<uint32_t>(out, static_cast<uint32_t>(t.visible_cells.size()), 4);
    for (uint32_t v : t.visible_cells) put_le<uint32_t>(out, v, 4);

    // Entry order is the unordered_map's own — deliberately not sorted. A map is
    // unordered, so telemetry declares `oracle: semantic` and the decoded value
    // is what gets compared. See docs/protocol.md.
    put_le<uint32_t>(out, static_cast<uint32_t>(t.packet_counts.size()), 4);
    for (const auto& [k, v] : t.packet_counts) {
        put_str(out, k);
        put_le<uint64_t>(out, v, 8);
    }

    out.push_back(t.gps_fix ? 1 : 0);
    put_len_prefixed(out, t.signature.data(), t.signature.size());

    return out;
}

inline TelemetryFrame telemetry_unmarshal(const std::vector<uint8_t>& data) {
    TelemetryFrame t{};
    size_t off = 0;

    t.device_id = take_le<uint64_t>(data, off, 8);
    t.ipv6 = take_le<serify::u128>(data, off, 16);

    t.local_ip.assign(data.begin() + off, data.begin() + off + 4);
    off += 4;

    t.firmware = take_len_str(data, off);
    t.boot_count = take_le<uint16_t>(data, off, 2);
    t.rssi_dbm = static_cast<int8_t>(take_le<uint8_t>(data, off, 1));
    t.temperature_dc = static_cast<int16_t>(take_le<uint16_t>(data, off, 2));
    t.clock_drift_ms = static_cast<int32_t>(take_le<uint32_t>(data, off, 4));

    uint32_t fbits = take_le<uint32_t>(data, off, 4);
    std::memcpy(&t.battery_volts, &fbits, 4);
    uint64_t dbits = take_le<uint64_t>(data, off, 8);
    std::memcpy(&t.latitude, &dbits, 8);
    dbits = take_le<uint64_t>(data, off, 8);
    std::memcpy(&t.longitude, &dbits, 8);

    if (take_le<uint8_t>(data, off, 1) == 0) {
        t.humidity_pct = std::nullopt;
    } else {
        fbits = take_le<uint32_t>(data, off, 4);
        float h;
        std::memcpy(&h, &fbits, 4);
        t.humidity_pct = h;
    }

    t.accel_mg.clear();
    for (int i = 0; i < 3; i++) {
        t.accel_mg.push_back(static_cast<int16_t>(take_le<uint16_t>(data, off, 2)));
    }

    uint32_t cells = take_le<uint32_t>(data, off, 4);
    t.visible_cells.clear();
    for (uint32_t i = 0; i < cells; i++) t.visible_cells.push_back(take_le<uint32_t>(data, off, 4));

    uint32_t entries = take_le<uint32_t>(data, off, 4);
    t.packet_counts.clear();
    for (uint32_t i = 0; i < entries; i++) {
        std::string k = take_len_str(data, off);
        t.packet_counts[k] = take_le<uint64_t>(data, off, 8);
    }

    t.gps_fix = take_le<uint8_t>(data, off, 1) != 0;
    t.signature = take_len_prefixed(data, off);

    return t;
}
