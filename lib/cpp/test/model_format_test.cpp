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

// Unit tests for the C++ worker library's model registration. Build and run:
//
//     g++ -std=c++17 -Ilib/cpp -o t lib/cpp/test/model_format_test.cpp && ./t
//
// No framework: the library is a single header with no dependencies, and this
// keeps its tests the same. Exits non-zero if any check fails.
//
// Half of what this file does is simply *instantiate* the model_format
// overloads. They are templates, so an overload nothing calls is never
// compiled — the serialize-only one has no caller anywhere in the repo, and
// would rot unnoticed without a line here that names it.

#include "serify.hpp"

#include <cstdint>
#include <cstdio>
#include <stdexcept>
#include <string>
#include <vector>

using namespace serify;

// A model with a byte layout of its own: one u32, little-endian.
struct Rec {
    uint32_t n = 0;
    std::string label;
};

SERIFY_TO(Rec,
    SERIFY_FIELD(n, u32)
    SERIFY_FIELD(label, string)
)
SERIFY_FROM(Rec,
    SERIFY_FROM_FIELD(n, u32)
    SERIFY_FROM_FIELD(label, string)
)

static std::vector<uint8_t> rec_marshal(const Rec& r) {
    return {static_cast<uint8_t>(r.n & 0xFF), static_cast<uint8_t>((r.n >> 8) & 0xFF),
            static_cast<uint8_t>((r.n >> 16) & 0xFF), static_cast<uint8_t>((r.n >> 24) & 0xFF)};
}

static Rec rec_unmarshal(const std::vector<uint8_t>& d) {
    Rec r{};
    r.n = static_cast<uint32_t>(d[0]) | (static_cast<uint32_t>(d[1]) << 8) |
          (static_cast<uint32_t>(d[2]) << 16) | (static_cast<uint32_t>(d[3]) << 24);
    r.label = "from-bytes";
    return r;
}

static int failures = 0;

static void check(bool ok, const char* what) {
    std::printf("%-4s %s\n", ok ? "ok" : "FAIL", what);
    if (!ok) failures++;
}

static void run_tests() {
    // A model-backed format converts FieldMap <-> model on both sides: the
    // worker's own marshal/unmarshal go in unwrapped and never see a FieldMap.
    FormatPair pair = model_format<Rec>(rec_marshal, rec_unmarshal);

    FieldMap fm;
    fm.set_u32("n", 7);
    fm.set_string("label", "in");
    auto bytes = pair.serialize(fm);
    check(bytes == std::vector<uint8_t>({7, 0, 0, 0}),
          "model serializer receives the model, not the FieldMap");

    FieldMap back = pair.deserialize({9, 0, 0, 0});
    check(back.get_u32("n") == 9, "model deserializer's result is mapped back to a FieldMap");
    check(back.get_string("label") == "from-bytes", "every bound field comes back, not just the first");

    // The serialize-only overload. Naming it here is the point: it is a
    // template, so without this line nothing in the repo ever compiles it.
    FormatPair ser_only = model_format<Rec>(rec_marshal);
    check(ser_only.serialize(fm) == std::vector<uint8_t>({7, 0, 0, 0}),
          "the serialize-only overload serializes");
    check(!ser_only.deserialize,
          "the serialize-only overload leaves deserialize unset, so the runner reports it unsupported");

    // A plain FormatPair is the model-less path, which the audit fixtures need:
    // the functions take and return the FieldMap itself.
    FormatPair raw{
        [](const FieldMap& f) {
            auto s = f.get_string("label");
            return std::vector<uint8_t>(s.begin(), s.end());
        },
        [](const std::vector<uint8_t>& d) {
            FieldMap f;
            f.set_string("label", std::string(d.begin(), d.end()));
            return f;
        },
    };
    check(raw.serialize(fm) == std::vector<uint8_t>({'i', 'n'}),
          "a plain FormatPair still hands the serializer the FieldMap itself");
}

int main() {
    // FieldMap getters throw on a missing key, and a broken conversion is
    // exactly the thing that leaves one missing. Catching here turns that into
    // a reported failure instead of an abort with no test output.
    try {
        run_tests();
    } catch (const std::exception& e) {
        std::printf("FAIL threw: %s\n", e.what());
        failures++;
    }

    std::printf(failures == 0 ? "\nall tests passed\n" : "\n%d test(s) failed\n", failures);
    return failures == 0 ? 0 : 1;
}
