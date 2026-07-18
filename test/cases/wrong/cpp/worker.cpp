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

// C++ half of the `wrong` meta-test. Mirrors the Go worker's byte/JSON layout.

#include "serify.hpp"
#include <cstdint>
#include <cstdlib>
#include <string>
#include <thread>
#include <vector>

using namespace serify;

static const std::string SELF_LANG = "cpp";

static std::vector<std::string> to_upper_self(const std::vector<std::string>& langs) {
    std::vector<std::string> out;
    for (auto& s : langs) out.push_back(s == SELF_LANG ? "CPP" : s);
    return out;
}

// --- binary ----------------------------------------------------------------

template <typename U>
static void put_le(std::vector<uint8_t>& out, U u, size_t n) {
    for (size_t i = 0; i < n; i++) out.push_back((uint8_t)(u >> (8 * i)));
}

template <typename U>
static U take_le(const std::vector<uint8_t>& d, size_t& off, size_t n) {
    U u = 0;
    for (size_t i = 0; i < n; i++) u |= (U)d[off + i] << (8 * i);
    off += n;
    return u;
}

static std::vector<uint8_t> binary_serialize(const FieldMap& fm) {
    auto langs = fm.get_list_string("langs");
    if (!fm.get_bool("binary_serialize")) langs = to_upper_self(langs);

    std::vector<uint8_t> out;
    out.push_back(fm.get_bool("binary_serialize") ? 1 : 0);
    out.push_back(fm.get_bool("binary_deserialize") ? 1 : 0);
    out.push_back(fm.get_bool("json_serialize") ? 1 : 0);
    out.push_back(fm.get_bool("json_deserialize") ? 1 : 0);
    put_le<uint32_t>(out, (uint32_t)langs.size(), 4);
    for (auto& s : langs) { put_le<uint32_t>(out, (uint32_t)s.size(), 4); out.insert(out.end(), s.begin(), s.end()); }
    return out;
}

static FieldMap binary_deserialize(const std::vector<uint8_t>& data) {
    FieldMap fm;
    size_t off = 0;
    bool bs = take_le<uint8_t>(data, off, 1), bd = take_le<uint8_t>(data, off, 1);
    bool js = take_le<uint8_t>(data, off, 1), jd = take_le<uint8_t>(data, off, 1);
    fm.set_bool("binary_serialize", bs);
    fm.set_bool("binary_deserialize", bd);
    fm.set_bool("json_serialize", js);
    fm.set_bool("json_deserialize", jd);

    auto n = take_le<uint32_t>(data, off, 4);
    std::vector<std::string> langs;
    for (uint32_t i = 0; i < n; i++) {
        auto slen = take_le<uint32_t>(data, off, 4);
        langs.emplace_back(reinterpret_cast<const char*>(&data[off]), slen);
        off += slen;
    }
    fm.set_list_string("langs", bd ? langs : to_upper_self(langs));
    return fm;
}

// --- json (via the library's parser) -----------------------------------------------

static std::vector<uint8_t> json_serialize(const FieldMap& fm) {
    auto langs = fm.get_list_string("langs");
    if (!fm.get_bool("json_serialize")) langs = to_upper_self(langs);

    // Hand-roll minimal JSON to avoid pulling in a library.
    std::string s = "{\"binary_serialize\":" + std::string(fm.get_bool("binary_serialize") ? "true" : "false") +
        ",\"binary_deserialize\":" + std::string(fm.get_bool("binary_deserialize") ? "true" : "false") +
        ",\"json_serialize\":" + std::string(fm.get_bool("json_serialize") ? "true" : "false") +
        ",\"json_deserialize\":" + std::string(fm.get_bool("json_deserialize") ? "true" : "false") +
        ",\"langs\":[";
    for (size_t i = 0; i < langs.size(); i++) {
        if (i) s += ",";
        s += "\"" + langs[i] + "\"";
    }
    s += "]}";
    return std::vector<uint8_t>(s.begin(), s.end());
}

static FieldMap json_deserialize(const std::vector<uint8_t>& data) {
    // Parse enough JSON for the wrong type.
    std::string d(data.begin(), data.end());
    FieldMap fm;

    auto get_bool = [&](const std::string& key) -> bool {
        auto pos = d.find("\"" + key + "\":");
        if (pos == std::string::npos) return false;
        return d.substr(pos + key.size() + 3, 4) == "true";
    };
    fm.set_bool("binary_serialize", get_bool("binary_serialize"));
    fm.set_bool("binary_deserialize", get_bool("binary_deserialize"));
    fm.set_bool("json_serialize", get_bool("json_serialize"));
    fm.set_bool("json_deserialize", get_bool("json_deserialize"));
    bool jd = get_bool("json_deserialize");

    // Parse langs array.
    std::vector<std::string> langs;
    auto arr = d.find("\"langs\":[");
    if (arr != std::string::npos) {
        size_t p = arr + 9;
        while (p < d.size() && d[p] != ']') {
            if (d[p] == '"') { p++; auto end = d.find('"', p); langs.emplace_back(d, p, end - p); p = end + 1; }
            else p++;
        }
    }
    fm.set_list_string("langs", jd ? langs : to_upper_self(langs));
    return fm;
}

// --- fault formats ---------------------------------------------------------

static std::vector<uint8_t> err_ser(const FieldMap&) { throw std::runtime_error("injected serialize error"); }
static FieldMap err_deser(const std::vector<uint8_t>&) { throw std::runtime_error("injected deserialize error"); }
static std::vector<uint8_t> hang_ser(const FieldMap& fm) {
    std::this_thread::sleep_for(std::chrono::seconds(3));
    return binary_serialize(fm);
}
static std::vector<uint8_t> crash_ser(const FieldMap&) { std::exit(3); }

int main() {
    SuiteMap suite;
    suite["wrong"]["binary"]    = FormatPair{binary_serialize, binary_deserialize};
    suite["wrong"]["json"]      = FormatPair{json_serialize, json_deserialize};
    suite["wrong"]["err_ser"]   = FormatPair{err_ser, binary_deserialize};
    suite["wrong"]["err_deser"] = FormatPair{binary_serialize, err_deser};
    suite["wrong"]["hang"]      = FormatPair{hang_ser, binary_deserialize};
    suite["wrong"]["crash"]     = FormatPair{crash_ser, binary_deserialize};
    run_suite(suite);
    return 0;
}
