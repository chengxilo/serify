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

// Happy-path C++ worker: `all_types` in binary and json.
//
// Go is the --ref language and owns both byte layouts; see
// test/cases/happy/go/type.go. The json format must match Go's encoding/json
// byte-for-byte (with SetEscapeHTML(false)): schema field order, map keys in
// byte order (std::map gives that for free), []byte as base64, floats in
// shortest form without a trailing .0, and U+2028/U+2029 escaped (Go escapes
// those unconditionally).
//
// The library's JSON parser stores numbers as doubles, which cannot hold a u64,
// so this worker carries its own tiny parser that keeps number tokens as raw
// strings and converts with stoull/stoll.

#include "serify.hpp"

#include <cstdint>
#include <cstdio>
#include <cstring>
#include <map>
#include <stdexcept>
#include <string>
#include <vector>

using namespace serify;

static const std::vector<std::string> kStatusVariants = {
    "pending", "paid", "shipped", "delivered", "cancelled"};

static uint8_t status_ordinal(const std::string& s) {
    for (size_t i = 0; i < kStatusVariants.size(); i++) {
        if (kStatusVariants[i] == s) return (uint8_t)i;
    }
    throw std::runtime_error("unknown status \"" + s + "\"");
}

// --- binary format -----------------------------------------------------------

template <typename U>
static void put_le(std::vector<uint8_t>& out, U u, size_t nbytes) {
    for (size_t i = 0; i < nbytes; ++i) {
        out.push_back(static_cast<uint8_t>(u >> (8 * i)));
    }
}

template <typename U>
static U take_le(const std::vector<uint8_t>& data, size_t& off, size_t nbytes) {
    if (off + nbytes > data.size()) throw std::runtime_error("truncated");
    U u = 0;
    for (size_t i = 0; i < nbytes; ++i) {
        u |= static_cast<U>(data[off + i]) << (8 * i);
    }
    off += nbytes;
    return u;
}

static void put_len_str(std::vector<uint8_t>& out, const std::string& s) {
    put_le<uint32_t>(out, (uint32_t)s.size(), 4);
    out.insert(out.end(), s.begin(), s.end());
}

static std::string take_len_str(const std::vector<uint8_t>& data, size_t& off) {
    const size_t n = take_le<uint32_t>(data, off, 4);
    if (off + n > data.size()) throw std::runtime_error("truncated");
    std::string s(reinterpret_cast<const char*>(data.data() + off), n);
    off += n;
    return s;
}

static std::vector<uint8_t> binary_serialize(const FieldMap& fm) {
    std::vector<uint8_t> out;

    out.push_back(fm.get_u8("uint8"));
    put_le<uint16_t>(out, fm.get_u16("uint16"), 2);
    put_le<uint32_t>(out, fm.get_u32("uint32"), 4);
    put_le<uint64_t>(out, fm.get_u64("uint64"), 8);
    out.push_back((uint8_t)fm.get_i8("int8"));
    put_le<uint16_t>(out, (uint16_t)fm.get_i16("int16"), 2);
    put_le<uint32_t>(out, (uint32_t)fm.get_i32("int32"), 4);
    put_le<uint64_t>(out, (uint64_t)fm.get_i64("int64"), 8);

    float f32 = fm.get_f32("float32");
    uint32_t f32bits;
    memcpy(&f32bits, &f32, 4);
    put_le<uint32_t>(out, f32bits, 4);
    double f64 = fm.get_f64("float64");
    uint64_t f64bits;
    memcpy(&f64bits, &f64, 8);
    put_le<uint64_t>(out, f64bits, 8);

    out.push_back(fm.get_bool("bool") ? 1 : 0);
    put_len_str(out, fm.get_string("string"));

    const auto bytes = fm.get_bytes("bytes");
    put_le<uint32_t>(out, (uint32_t)bytes.size(), 4);
    out.insert(out.end(), bytes.begin(), bytes.end());

    const auto list = fm.get_list_string("list");
    put_le<uint32_t>(out, (uint32_t)list.size(), 4);
    for (const auto& s : list) put_len_str(out, s);

    const auto opt = fm.get_optional_string("optional");
    if (!opt.has_value()) {
        out.push_back(0);
    } else {
        out.push_back(1);
        put_len_str(out, *opt);
    }

    for (uint32_t n : fm.get_list_u32("array")) put_le<uint32_t>(out, n, 4);

    const auto p = fm.get_struct("struct");
    put_le<uint32_t>(out, (uint32_t)p->get_i32("x"), 4);
    put_le<uint32_t>(out, (uint32_t)p->get_i32("y"), 4);
    put_le<uint32_t>(out, (uint32_t)p->get_i32("z"), 4);
    put_len_str(out, p->get_string("name"));

    const auto& m = fm.get_map("map");  // std::map: keys already byte-sorted
    put_le<uint32_t>(out, (uint32_t)m.size(), 4);
    for (const auto& [k, v] : m) {
        put_len_str(out, k);
        put_le<uint32_t>(out, std::get<uint32_t>(v), 4);
    }

    const auto& ms = fm.get_map("map_struct");
    put_le<uint32_t>(out, (uint32_t)ms.size(), 4);
    for (const auto& [k, v] : ms) {
        const auto t = std::get<StructPtr>(v);
        put_len_str(out, k);
        put_len_str(out, t->get_string("name"));
        put_le<uint32_t>(out, t->get_u32("weight"), 4);
    }

    out.push_back(status_ordinal(fm.get_string("status")));
    return out;
}

static FieldMap binary_deserialize(const std::vector<uint8_t>& data) {
    FieldMap fm;
    size_t off = 0;

    fm.set_u8("uint8", take_le<uint8_t>(data, off, 1));
    fm.set_u16("uint16", take_le<uint16_t>(data, off, 2));
    fm.set_u32("uint32", take_le<uint32_t>(data, off, 4));
    fm.set_u64("uint64", take_le<uint64_t>(data, off, 8));
    fm.set_i8("int8", (int8_t)take_le<uint8_t>(data, off, 1));
    fm.set_i16("int16", (int16_t)take_le<uint16_t>(data, off, 2));
    fm.set_i32("int32", (int32_t)take_le<uint32_t>(data, off, 4));
    fm.set_i64("int64", (int64_t)take_le<uint64_t>(data, off, 8));

    uint32_t f32bits = take_le<uint32_t>(data, off, 4);
    float f32;
    memcpy(&f32, &f32bits, 4);
    fm.set_f32("float32", f32);
    uint64_t f64bits = take_le<uint64_t>(data, off, 8);
    double f64;
    memcpy(&f64, &f64bits, 8);
    fm.set_f64("float64", f64);

    fm.set_bool("bool", take_le<uint8_t>(data, off, 1) != 0);
    fm.set_string("string", take_len_str(data, off));

    const size_t nbytes = take_le<uint32_t>(data, off, 4);
    if (off + nbytes > data.size()) throw std::runtime_error("truncated");
    fm.set_bytes("bytes", Bytes(data.begin() + off, data.begin() + off + nbytes));
    off += nbytes;

    const size_t nlist = take_le<uint32_t>(data, off, 4);
    ListString list;
    for (size_t i = 0; i < nlist; i++) list.push_back(take_len_str(data, off));
    fm.set_list_string("list", std::move(list));

    if (take_le<uint8_t>(data, off, 1) == 0) {
        fm.set_optional_string("optional", std::nullopt);
    } else {
        fm.set_optional_string("optional", take_len_str(data, off));
    }

    // array<uint32,4> is carried as the list<uint32> it is.
    ListU32 arr(4);
    for (auto& v : arr) v = take_le<uint32_t>(data, off, 4);
    fm.set_list_u32("array", arr);

    auto p = std::make_shared<FieldMap>();
    p->set_i32("x", (int32_t)take_le<uint32_t>(data, off, 4));
    p->set_i32("y", (int32_t)take_le<uint32_t>(data, off, 4));
    p->set_i32("z", (int32_t)take_le<uint32_t>(data, off, 4));
    p->set_string("name", take_len_str(data, off));
    fm.set_struct("struct", p);

    MapStore m;
    const size_t nmap = take_le<uint32_t>(data, off, 4);
    for (size_t i = 0; i < nmap; i++) {
        std::string k = take_len_str(data, off);
        m[k] = take_le<uint32_t>(data, off, 4);
    }
    fm.set_map("map", std::move(m));

    MapStore ms;
    const size_t nms = take_le<uint32_t>(data, off, 4);
    for (size_t i = 0; i < nms; i++) {
        std::string k = take_len_str(data, off);
        auto t = std::make_shared<FieldMap>();
        t->set_string("name", take_len_str(data, off));
        t->set_u32("weight", take_le<uint32_t>(data, off, 4));
        ms[k] = t;
    }
    fm.set_map("map_struct", std::move(ms));

    const uint8_t ord = take_le<uint8_t>(data, off, 1);
    if (ord >= kStatusVariants.size()) throw std::runtime_error("status ordinal out of range");
    fm.set_string("status", kStatusVariants[ord]);
    return fm;
}

// --- json format ---------------------------------------------------------

static const char* kB64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

static std::string base64_encode(const Bytes& data) {
    std::string out;
    for (size_t i = 0; i < data.size(); i += 3) {
        uint32_t b0 = data[i];
        uint32_t b1 = i + 1 < data.size() ? data[i + 1] : 0;
        uint32_t b2 = i + 2 < data.size() ? data[i + 2] : 0;
        uint32_t n = (b0 << 16) | (b1 << 8) | b2;
        out += kB64[(n >> 18) & 63];
        out += kB64[(n >> 12) & 63];
        out += i + 1 < data.size() ? kB64[(n >> 6) & 63] : '=';
        out += i + 2 < data.size() ? kB64[n & 63] : '=';
    }
    return out;
}

static Bytes base64_decode(const std::string& s) {
    auto val = [](char c) -> uint32_t {
        if (c >= 'A' && c <= 'Z') return (uint32_t)(c - 'A');
        if (c >= 'a' && c <= 'z') return (uint32_t)(c - 'a' + 26);
        if (c >= '0' && c <= '9') return (uint32_t)(c - '0' + 52);
        if (c == '+') return 62;
        if (c == '/') return 63;
        throw std::runtime_error("invalid base64");
    };
    if (s.size() % 4 != 0) throw std::runtime_error("base64 length");
    Bytes out;
    for (size_t i = 0; i < s.size(); i += 4) {
        bool p2 = s[i + 2] == '=', p3 = s[i + 3] == '=';
        uint32_t n = (val(s[i]) << 18) | (val(s[i + 1]) << 12) |
                     ((p2 ? 0 : val(s[i + 2])) << 6) | (p3 ? 0 : val(s[i + 3]));
        out.push_back((uint8_t)(n >> 16));
        if (!p2) out.push_back((uint8_t)(n >> 8));
        if (!p3) out.push_back((uint8_t)n);
    }
    return out;
}

// Go's encoding/json string escaping with SetEscapeHTML(false): only \n, \r,
// \t are named (\b and \f become \u00xx), and U+2028/U+2029 are escaped
// unconditionally (UTF-8: E2 80 A8 / E2 80 A9).
static std::string go_str(const std::string& s) {
    std::string out = "\"";
    for (size_t i = 0; i < s.size(); i++) {
        unsigned char c = (unsigned char)s[i];
        if (c == '"') out += "\\\"";
        else if (c == '\\') out += "\\\\";
        else if (c == '\n') out += "\\n";
        else if (c == '\r') out += "\\r";
        else if (c == '\t') out += "\\t";
        else if (c < 0x20) {
            char buf[8];
            snprintf(buf, sizeof buf, "\\u%04x", c);
            out += buf;
        } else if (c == 0xE2 && i + 2 < s.size() && (unsigned char)s[i + 1] == 0x80 &&
                   ((unsigned char)s[i + 2] == 0xA8 || (unsigned char)s[i + 2] == 0xA9)) {
            out += (unsigned char)s[i + 2] == 0xA8 ? "\\u2028" : "\\u2029";
            i += 2;
        } else {
            out += (char)c;
        }
    }
    return out + "\"";
}

// Shortest decimal that round-trips, without a trailing .0 — Go's format for
// the value range these cases use. %g never emits trailing zeros, and the
// bit-exact round-trip check picks the first precision that preserves them.
static std::string go_f64(double v) {
    char buf[64];
    for (int p = 1; p <= 17; p++) {
        snprintf(buf, sizeof buf, "%.*g", p, v);
        if (strtod(buf, nullptr) == v) return buf;
    }
    return buf;
}

static std::string go_f32(float v) {
    char buf[64];
    for (int p = 1; p <= 9; p++) {
        snprintf(buf, sizeof buf, "%.*g", p, (double)v);
        if (strtof(buf, nullptr) == v) return buf;
    }
    return buf;
}

static std::vector<uint8_t> json_serialize(const FieldMap& fm) {
    std::string s = "{";
    s += "\"uint8\":" + std::to_string(fm.get_u8("uint8"));
    s += ",\"uint16\":" + std::to_string(fm.get_u16("uint16"));
    s += ",\"uint32\":" + std::to_string(fm.get_u32("uint32"));
    s += ",\"uint64\":" + std::to_string(fm.get_u64("uint64"));
    s += ",\"int8\":" + std::to_string(fm.get_i8("int8"));
    s += ",\"int16\":" + std::to_string(fm.get_i16("int16"));
    s += ",\"int32\":" + std::to_string(fm.get_i32("int32"));
    s += ",\"int64\":" + std::to_string(fm.get_i64("int64"));
    s += ",\"float32\":" + go_f32(fm.get_f32("float32"));
    s += ",\"float64\":" + go_f64(fm.get_f64("float64"));
    s += ",\"bool\":" + std::string(fm.get_bool("bool") ? "true" : "false");
    s += ",\"string\":" + go_str(fm.get_string("string"));
    s += ",\"bytes\":\"" + base64_encode(fm.get_bytes("bytes")) + "\"";

    s += ",\"list\":[";
    const auto list = fm.get_list_string("list");
    for (size_t i = 0; i < list.size(); i++) {
        if (i) s += ",";
        s += go_str(list[i]);
    }
    s += "]";

    const auto opt = fm.get_optional_string("optional");
    s += ",\"optional\":" + (opt.has_value() ? go_str(*opt) : "null");

    s += ",\"array\":[";
    const auto arr = fm.get_list_u32("array");
    for (size_t i = 0; i < arr.size(); i++) {
        if (i) s += ",";
        s += std::to_string(arr[i]);
    }
    s += "]";

    const auto p = fm.get_struct("struct");
    s += ",\"struct\":{\"x\":" + std::to_string(p->get_i32("x")) +
         ",\"y\":" + std::to_string(p->get_i32("y")) +
         ",\"z\":" + std::to_string(p->get_i32("z")) +
         ",\"name\":" + go_str(p->get_string("name")) + "}";

    s += ",\"map\":{";
    bool first = true;
    for (const auto& [k, v] : fm.get_map("map")) {
        if (!first) s += ",";
        first = false;
        s += go_str(k) + ":" + std::to_string(std::get<uint32_t>(v));
    }
    s += "}";

    s += ",\"map_struct\":{";
    first = true;
    for (const auto& [k, v] : fm.get_map("map_struct")) {
        if (!first) s += ",";
        first = false;
        const auto t = std::get<StructPtr>(v);
        s += go_str(k) + ":{\"name\":" + go_str(t->get_string("name")) +
             ",\"weight\":" + std::to_string(t->get_u32("weight")) + "}";
    }
    s += "}";

    s += ",\"status\":" + go_str(fm.get_string("status"));
    s += "}";
    return std::vector<uint8_t>(s.begin(), s.end());
}

// Minimal JSON parser that keeps number tokens as raw strings, so 64-bit
// integers survive (the library's parser stores numbers as doubles).
namespace bigjson {

struct JV {
    enum Kind { Null, Bool, Num, Str, Arr, Obj } kind = Null;
    bool b = false;
    std::string text;  // Num: raw token; Str: decoded string
    std::vector<JV> arr;
    std::vector<std::pair<std::string, JV>> obj;

    const JV& at(const std::string& k) const {
        for (const auto& p : obj) {
            if (p.first == k) return p.second;
        }
        throw std::runtime_error("missing field " + k);
    }
};

struct Parser {
    const std::string& s;
    size_t i = 0;
    explicit Parser(const std::string& src) : s(src) {}

    void ws() { while (i < s.size() && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r')) i++; }
    char peek() { ws(); if (i >= s.size()) throw std::runtime_error("eof"); return s[i]; }
    void expect(char c) { if (peek() != c) throw std::runtime_error("expected " + std::string(1, c)); i++; }

    static void utf8_append(std::string& out, uint32_t cp) {
        if (cp < 0x80) out += (char)cp;
        else if (cp < 0x800) {
            out += (char)(0xC0 | (cp >> 6));
            out += (char)(0x80 | (cp & 0x3F));
        } else if (cp < 0x10000) {
            out += (char)(0xE0 | (cp >> 12));
            out += (char)(0x80 | ((cp >> 6) & 0x3F));
            out += (char)(0x80 | (cp & 0x3F));
        } else {
            out += (char)(0xF0 | (cp >> 18));
            out += (char)(0x80 | ((cp >> 12) & 0x3F));
            out += (char)(0x80 | ((cp >> 6) & 0x3F));
            out += (char)(0x80 | (cp & 0x3F));
        }
    }

    uint32_t hex4() {
        uint32_t v = 0;
        for (int k = 0; k < 4; k++) {
            char c = s.at(i++);
            v <<= 4;
            if (c >= '0' && c <= '9') v |= (uint32_t)(c - '0');
            else if (c >= 'a' && c <= 'f') v |= (uint32_t)(c - 'a' + 10);
            else if (c >= 'A' && c <= 'F') v |= (uint32_t)(c - 'A' + 10);
            else throw std::runtime_error("bad \\u escape");
        }
        return v;
    }

    std::string str() {
        expect('"');
        std::string out;
        while (true) {
            char c = s.at(i++);
            if (c == '"') break;
            if (c != '\\') { out += c; continue; }
            char e = s.at(i++);
            switch (e) {
                case '"': out += '"'; break;
                case '\\': out += '\\'; break;
                case '/': out += '/'; break;
                case 'b': out += '\b'; break;
                case 'f': out += '\f'; break;
                case 'n': out += '\n'; break;
                case 'r': out += '\r'; break;
                case 't': out += '\t'; break;
                case 'u': {
                    uint32_t cp = hex4();
                    if (cp >= 0xD800 && cp <= 0xDBFF && i + 1 < s.size() && s[i] == '\\' && s[i + 1] == 'u') {
                        i += 2;
                        uint32_t lo = hex4();
                        cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
                    }
                    utf8_append(out, cp);
                    break;
                }
                default: throw std::runtime_error("bad escape");
            }
        }
        return out;
    }

    JV value() {
        char c = peek();
        JV v;
        if (c == '{') {
            i++;
            v.kind = JV::Obj;
            if (peek() == '}') { i++; return v; }
            while (true) {
                std::string k = str();
                expect(':');
                v.obj.emplace_back(std::move(k), value());
                if (peek() == ',') { i++; continue; }
                expect('}');
                break;
            }
        } else if (c == '[') {
            i++;
            v.kind = JV::Arr;
            if (peek() == ']') { i++; return v; }
            while (true) {
                v.arr.push_back(value());
                if (peek() == ',') { i++; continue; }
                expect(']');
                break;
            }
        } else if (c == '"') {
            v.kind = JV::Str;
            v.text = str();
        } else if (c == 't') {
            i += 4;
            v.kind = JV::Bool;
            v.b = true;
        } else if (c == 'f') {
            i += 5;
            v.kind = JV::Bool;
        } else if (c == 'n') {
            i += 4;
            v.kind = JV::Null;
        } else {
            v.kind = JV::Num;
            size_t start = i;
            while (i < s.size() && (isdigit((unsigned char)s[i]) || strchr("+-.eE", s[i]))) i++;
            v.text = s.substr(start, i - start);
        }
        return v;
    }
};

}  // namespace bigjson

static FieldMap json_deserialize(const std::vector<uint8_t>& data) {
    const std::string text(data.begin(), data.end());
    bigjson::Parser parser(text);
    const bigjson::JV v = parser.value();

    FieldMap fm;
    fm.set_u8("uint8", (uint8_t)std::stoul(v.at("uint8").text));
    fm.set_u16("uint16", (uint16_t)std::stoul(v.at("uint16").text));
    fm.set_u32("uint32", (uint32_t)std::stoul(v.at("uint32").text));
    fm.set_u64("uint64", (uint64_t)std::stoull(v.at("uint64").text));
    fm.set_i8("int8", (int8_t)std::stol(v.at("int8").text));
    fm.set_i16("int16", (int16_t)std::stol(v.at("int16").text));
    fm.set_i32("int32", (int32_t)std::stol(v.at("int32").text));
    fm.set_i64("int64", (int64_t)std::stoll(v.at("int64").text));
    fm.set_f32("float32", strtof(v.at("float32").text.c_str(), nullptr));
    fm.set_f64("float64", strtod(v.at("float64").text.c_str(), nullptr));
    fm.set_bool("bool", v.at("bool").b);
    fm.set_string("string", v.at("string").text);
    fm.set_bytes("bytes", base64_decode(v.at("bytes").text));

    ListString list;
    for (const auto& x : v.at("list").arr) list.push_back(x.text);
    fm.set_list_string("list", std::move(list));

    const auto& opt = v.at("optional");
    if (opt.kind == bigjson::JV::Null) {
        fm.set_optional_string("optional", std::nullopt);
    } else {
        fm.set_optional_string("optional", opt.text);
    }

    ListU32 arr(4);
    const auto& ja = v.at("array").arr;
    for (size_t i = 0; i < arr.size() && i < ja.size(); i++) {
        arr[i] = (uint32_t)std::stoul(ja[i].text);
    }
    fm.set_list_u32("array", arr);

    const auto& st = v.at("struct");
    auto p = std::make_shared<FieldMap>();
    p->set_i32("x", (int32_t)std::stol(st.at("x").text));
    p->set_i32("y", (int32_t)std::stol(st.at("y").text));
    p->set_i32("z", (int32_t)std::stol(st.at("z").text));
    p->set_string("name", st.at("name").text);
    fm.set_struct("struct", p);

    MapStore m;
    for (const auto& [k, mv] : v.at("map").obj) {
        m[k] = (uint32_t)std::stoul(mv.text);
    }
    fm.set_map("map", std::move(m));

    MapStore ms;
    for (const auto& [k, tv] : v.at("map_struct").obj) {
        auto t = std::make_shared<FieldMap>();
        t->set_string("name", tv.at("name").text);
        t->set_u32("weight", (uint32_t)std::stoul(tv.at("weight").text));
        ms[k] = t;
    }
    fm.set_map("map_struct", std::move(ms));

    fm.set_string("status", v.at("status").text);
    return fm;
}

int main() {
    SuiteMap suite;
    suite["all_types"]["binary"] = FormatPair{binary_serialize, binary_deserialize};
    suite["all_types"]["json"] = FormatPair{json_serialize, json_deserialize};
    run_suite(suite);
    return 0;
}
