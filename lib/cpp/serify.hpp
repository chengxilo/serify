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

#pragma once
// workerlib.hpp - C++ single-header workerlib for the cross-language serialization framework.
// Usage: #include "workerlib.hpp"  then implement serialize/deserialize and call serify::run().

#include <array>
#include <cassert>
#include <cstdint>
#include <cstring>
#include <functional>
#include <iostream>
#include <map>
#include <unordered_map>
#include <memory>
#include <optional>
#include <set>
#include <sstream>
#include <stdexcept>
#include <string>
#include <variant>
#include <type_traits>
#include <vector>

namespace serify {

// Forward declarations for recursive types

class FieldMap;
using StructPtr   = std::shared_ptr<FieldMap>;
using ListStruct  = std::vector<StructPtr>;

// FieldValue variant

using Bytes      = std::vector<uint8_t>;
using ListString = std::vector<std::string>;
// A list<uint8> is byte-for-byte a Bytes, and std::variant cannot hold the same
// alternative twice, so the two share one alternative rather than colliding.
using ListU8     = Bytes;
using ListU16    = std::vector<uint16_t>;
using ListU32    = std::vector<uint32_t>;
using ListU64    = std::vector<uint64_t>;
using ListU128   = std::vector<unsigned __int128>;
using ListI8     = std::vector<int8_t>;
using ListI16    = std::vector<int16_t>;
using ListI32    = std::vector<int32_t>;
using ListI64    = std::vector<int64_t>;
using ListI128   = std::vector<__int128>;
using ListF32    = std::vector<float>;
using ListF64    = std::vector<double>;
using ListBool   = std::vector<bool>;
using ListBytes  = std::vector<Bytes>;

// u128/i128: __int128 is a GCC/Clang extension. The standard library has no
// std::stoull / std::to_string for it, so serify_u128_from_str / serify_u128_to_str
// below do the decimal conversion by hand. Parsing these as uint64_t (as this library
// used to) silently truncates every value above 2^64.
using u128 = unsigned __int128;
using i128 = __int128;

// Decimal <-> 128-bit conversion. The standard library provides neither for
// __int128, so both directions are done by hand.
inline u128 serify_u128_from_str(const std::string& s) {
    u128 n = 0;
    for (char c : s) {
        if (c < '0' || c > '9') throw std::runtime_error("invalid u128: " + s);
        n = n * 10 + (u128)(c - '0');
    }
    return n;
}

inline i128 serify_i128_from_str(const std::string& s) {
    bool neg = !s.empty() && s[0] == '-';
    u128 mag = serify_u128_from_str(neg ? s.substr(1) : s);
    // Negate in the unsigned domain so that the most negative value (-2^127,
    // whose magnitude does not fit in i128) round-trips correctly.
    return neg ? (i128)(~mag + 1) : (i128)mag;
}

inline std::string serify_u128_to_str(u128 n) {
    if (n == 0) return "0";
    std::string out;
    while (n > 0) { out += char('0' + (int)(n % 10)); n /= 10; }
    std::reverse(out.begin(), out.end());
    return out;
}

inline std::string serify_i128_to_str(i128 n) {
    if (n < 0) {
        u128 mag = ~(u128)n + 1;   // works for -2^127 too
        return "-" + serify_u128_to_str(mag);
    }
    return serify_u128_to_str((u128)n);
}


using FieldValue = std::variant<
    uint8_t, uint16_t, uint32_t, uint64_t,
    int8_t,  int16_t,  int32_t,  int64_t,
    u128,    i128,
    float,   double,
    bool,
    std::string,
    Bytes,
    ListString,
    ListU16,
    ListU32,
    ListU64,
    ListU128,
    ListI8,
    ListI16,
    ListI32,
    ListI64,
    ListI128,
    ListF32,
    ListF64,
    ListBool,
    ListBytes,
    std::optional<std::string>,
    StructPtr,                          // struct
    ListStruct,                         // list<struct>
    std::optional<StructPtr>,           // optional<struct>
    // A null optional<T> for every T that has no std::optional alternative of
    // its own. Without it an optional<uint32> had nowhere to put "no value" and
    // the field was simply dropped.
    std::monostate                      // null
>;

// Map fields are stored separately to avoid recursive variant definition.
//
// unordered_map, not map. A schema `map<K,V>` is unordered, and every other
// serify library holds one in its language's unordered type (Go map, Rust
// HashMap, C# Dictionary, Python dict). This was a std::map only because the
// protocol used to require workers to emit map entries in UTF-8 key order, and
// std::map handed that over for free — so C++ satisfied a rule it never had to
// implement. That rule is gone (docs/protocol.md § Maps: the format decides),
// and with it the reason to impose an ordering the type does not have. A worker
// whose format really is canonical over maps now sorts explicitly, the same as
// every other language has always had to.
using MapStore = std::unordered_map<std::string, FieldValue>;

// One arm of a sum: a tag and its decoded payload (null for a unit variant).
// The payload is held behind a shared_ptr so Variant does not have to be one of
// FieldValue's alternatives, which would make the variant recursive.
struct Variant {
    std::string tag;
    std::shared_ptr<FieldValue> value;

    bool is_unit() const { return value == nullptr; }
    template<typename T> T payload() const {
        if (!value) throw std::runtime_error("variant \"" + tag + "\" has no payload");
        return std::get<T>(*value);
    }
};

// FieldMap

class FieldMap {
    std::map<std::string, FieldValue> _f;
    std::map<std::string, MapStore>  _maps;
    std::map<std::string, Variant>   _variants;
public:
    template<typename T> T get(const std::string& k) const {
        auto it = _f.find(k);
        if (it == _f.end()) throw std::runtime_error("field not found: " + k);
        return std::get<T>(it->second);
    }
    template<typename T> void set(const std::string& k, T v) { _f[k] = std::move(v); }
    bool has(const std::string& k) const {
        return _f.count(k) > 0 || _maps.count(k) > 0 || _variants.count(k) > 0;
    }
    bool has_map(const std::string& k) const { return _maps.count(k) > 0; }
    bool has_variant(const std::string& k) const { return _variants.count(k) > 0; }
    const std::map<std::string, FieldValue>& raw() const { return _f; }
    // Store an already-built FieldValue, for the codec paths that carry a value
    // whose alternative is only known at run time (a list element, say).
    void set_raw(const std::string& k, FieldValue v) { _f[k] = std::move(v); }
    const std::map<std::string, MapStore>& maps() const { return _maps; }
    const std::map<std::string, Variant>& variants() const { return _variants; }

    uint8_t  get_u8 (const std::string& k) const { return get<uint8_t>(k); }
    uint16_t get_u16(const std::string& k) const { return get<uint16_t>(k); }
    uint32_t get_u32(const std::string& k) const { return get<uint32_t>(k); }
    uint64_t get_u64(const std::string& k) const { return get<uint64_t>(k); }
    u128 get_u128(const std::string& k) const { return get<u128>(k); }
    i128 get_i128(const std::string& k) const { return get<i128>(k); }
    int8_t   get_i8 (const std::string& k) const { return get<int8_t>(k); }
    int16_t  get_i16(const std::string& k) const { return get<int16_t>(k); }
    int32_t  get_i32(const std::string& k) const { return get<int32_t>(k); }
    int64_t  get_i64(const std::string& k) const { return get<int64_t>(k); }
    float    get_f32(const std::string& k) const { return get<float>(k); }
    double   get_f64(const std::string& k) const { return get<double>(k); }
    bool     get_bool(const std::string& k) const { return get<bool>(k); }
    std::string get_string(const std::string& k) const { return get<std::string>(k); }
    Bytes       get_bytes(const std::string& k)  const { return get<Bytes>(k); }
    ListString  get_list_string(const std::string& k) const { return get<ListString>(k); }
    ListU8      get_list_u8(const std::string& k) const { return get<ListU8>(k); }
    ListU16     get_list_u16(const std::string& k) const { return get<ListU16>(k); }
    ListU32     get_list_u32(const std::string& k) const { return get<ListU32>(k); }
    ListU64     get_list_u64(const std::string& k) const { return get<ListU64>(k); }
    ListU128    get_list_u128(const std::string& k) const { return get<ListU128>(k); }
    ListI8      get_list_i8(const std::string& k) const { return get<ListI8>(k); }
    ListI16     get_list_i16(const std::string& k) const { return get<ListI16>(k); }
    ListI32     get_list_i32(const std::string& k) const { return get<ListI32>(k); }
    ListI64     get_list_i64(const std::string& k) const { return get<ListI64>(k); }
    ListI128    get_list_i128(const std::string& k) const { return get<ListI128>(k); }
    ListF32     get_list_f32(const std::string& k) const { return get<ListF32>(k); }
    ListF64     get_list_f64(const std::string& k) const { return get<ListF64>(k); }
    ListBool    get_list_bool(const std::string& k) const { return get<ListBool>(k); }
    ListBytes   get_list_bytes(const std::string& k) const { return get<ListBytes>(k); }
    std::optional<std::string> get_optional_string(const std::string& k) const {
        return get<std::optional<std::string>>(k);
    }
    StructPtr get_struct(const std::string& k) const { return get<StructPtr>(k); }
    ListStruct get_list_struct(const std::string& k) const { return get<ListStruct>(k); }
    std::optional<StructPtr> get_optional_struct(const std::string& k) const {
        return get<std::optional<StructPtr>>(k);
    }

    void set_u8 (const std::string& k, uint8_t v)  { _f[k] = v; }
    void set_u16(const std::string& k, uint16_t v) { _f[k] = v; }
    void set_u32(const std::string& k, uint32_t v) { _f[k] = v; }
    void set_u64(const std::string& k, uint64_t v) { _f[k] = v; }
    void set_u128(const std::string& k, u128 v) { _f[k] = v; }
    void set_list_u128(const std::string& k, ListU128 v) { _f[k] = std::move(v); }
    void set_list_i128(const std::string& k, ListI128 v) { _f[k] = std::move(v); }
    void set_i128(const std::string& k, i128 v) { _f[k] = v; }
    void set_i8 (const std::string& k, int8_t v)   { _f[k] = v; }
    void set_i16(const std::string& k, int16_t v)  { _f[k] = v; }
    void set_i32(const std::string& k, int32_t v)  { _f[k] = v; }
    void set_i64(const std::string& k, int64_t v)  { _f[k] = v; }
    void set_f32(const std::string& k, float v)    { _f[k] = v; }
    void set_f64(const std::string& k, double v)   { _f[k] = v; }
    void set_bool(const std::string& k, bool v)    { _f[k] = v; }
    void set_string(const std::string& k, std::string v) { _f[k] = std::move(v); }
    void set_bytes(const std::string& k, Bytes v)        { _f[k] = std::move(v); }
    void set_list_string(const std::string& k, ListString v) { _f[k] = std::move(v); }
    void set_list_u8(const std::string& k, ListU8 v) { _f[k] = std::move(v); }
    void set_list_u16(const std::string& k, ListU16 v) { _f[k] = std::move(v); }
    void set_list_u32(const std::string& k, ListU32 v) { _f[k] = std::move(v); }
    void set_list_u64(const std::string& k, ListU64 v) { _f[k] = std::move(v); }
    void set_list_i8(const std::string& k, ListI8 v) { _f[k] = std::move(v); }
    void set_list_i16(const std::string& k, ListI16 v) { _f[k] = std::move(v); }
    void set_list_i32(const std::string& k, ListI32 v) { _f[k] = std::move(v); }
    void set_list_i64(const std::string& k, ListI64 v) { _f[k] = std::move(v); }
    void set_list_f32(const std::string& k, ListF32 v) { _f[k] = std::move(v); }
    void set_list_f64(const std::string& k, ListF64 v) { _f[k] = std::move(v); }
    void set_list_bool(const std::string& k, ListBool v) { _f[k] = std::move(v); }
    void set_list_bytes(const std::string& k, ListBytes v) { _f[k] = std::move(v); }
    void set_optional_string(const std::string& k, std::optional<std::string> v) { _f[k] = std::move(v); }
    void set_struct(const std::string& k, StructPtr v) { _f[k] = std::move(v); }
    void set_list_struct(const std::string& k, ListStruct v) { _f[k] = std::move(v); }
    void set_optional_struct(const std::string& k, std::optional<StructPtr> v) { _f[k] = std::move(v); }
    void set_map(const std::string& k, MapStore v) { _maps[k] = std::move(v); }
    const MapStore& get_map(const std::string& k) const {
        static const MapStore empty;
        auto it = _maps.find(k);
        return it != _maps.end() ? it->second : empty;
    }

    // Store a sum value: the active variant's tag and payload.
    void set_variant(const std::string& k, std::string tag, FieldValue v) {
        _variants[k] = Variant{std::move(tag), std::make_shared<FieldValue>(std::move(v))};
    }
    // Store a unit variant (no payload).
    void set_variant(const std::string& k, std::string tag) {
        _variants[k] = Variant{std::move(tag), nullptr};
    }
    // Return the sum value stored at k.
    const Variant& get_variant(const std::string& k) const {
        auto it = _variants.find(k);
        if (it == _variants.end()) throw std::runtime_error("sum field not found: " + k);
        return it->second;
    }
};

// Simple JSON value type (enough for our protocol)

struct Json {
    // Raw is a number this writer must emit exactly as spelled. JSON's only
    // number is the double, which cannot hold a 64-bit integer: max uint64
    // rounds up to 2^64 and comes back out of range. A worker with a u64 or i64
    // field builds one of these from the integer's own decimal form instead.
    enum Kind { Null, Bool, Number, String_, Array, Object, Raw } kind;
    bool b{};
    double num{};
    std::string str;
    std::vector<Json> arr;
    std::vector<std::pair<std::string,Json>> obj;

    static Json null_()            { return {Null}; }
    static Json bool_(bool b)      { Json j{Bool}; j.b = b; return j; }
    static Json num_(double n)     { Json j{Number}; j.num = n; return j; }
    static Json str_(std::string s){ Json j{String_}; j.str = std::move(s); return j; }
    static Json arr_()             { return {Array}; }
    static Json obj_()             { return {Object}; }
    static Json raw_(std::string text){ Json j{Raw}; j.str = std::move(text); return j; }

    void push(Json v) { arr.push_back(std::move(v)); }
    void set(std::string k, Json v) { obj.push_back({std::move(k), std::move(v)}); }

    const Json* get(const std::string& k) const {
        for (auto& p : obj) if (p.first == k) return &p.second;
        return nullptr;
    }
    std::string as_str() const { return str; }
    double as_num() const { return num; }
    // The other half of Raw. The parser keeps every number's source text here,
    // so a field whose value does not survive a double can be read from the
    // token rather than from `num`.
    const std::string& as_token() const { return str; }
    bool as_bool() const { return b; }
    bool is_null() const { return kind == Null; }
    bool is_obj()  const { return kind == Object; }
    bool is_arr()  const { return kind == Array; }
};

// JSON serializer

static void json_write(std::ostream& o, const Json& j) {
    switch (j.kind) {
        case Json::Null:   o << "null"; break;
        case Json::Raw:    o << j.str; break;
        case Json::Bool:   o << (j.b ? "true" : "false"); break;
        case Json::Number: {
            // A whole number prints as an integer so it does not come out as
            // "1e+06", but only when it actually fits: converting a double
            // outside long long's range is undefined, and customer's max
            // float32 (3.4e38) is outside it.
            constexpr double kLLMax = 9223372036854775808.0; // 2^63, exclusive
            if (j.num > -kLLMax && j.num < kLLMax && j.num == (double)(long long)j.num) {
                o << (long long)j.num;
                break;
            }
            // Default ostream precision is 6 significant digits, which silently
            // rounds anything needing more: max float32 went out as
            // 3.40282e+38 and came back a different float32. 17 is what an
            // IEEE-754 double needs to round-trip.
            const auto saved = o.precision(17);
            o << j.num;
            o.precision(saved);
            break;
        }
        case Json::String_: {
            o << '"';
            for (unsigned char c : j.str) {
                if (c == '"')  o << "\\\"";
                else if (c == '\\') o << "\\\\";
                else if (c == '\n') o << "\\n";
                else if (c == '\r') o << "\\r";
                else if (c == '\t') o << "\\t";
                else if (c < 0x20) { o << "\\u00"; char buf[3]; snprintf(buf,3,"%02x",c); o << buf; }
                else o << (char)c;
            }
            o << '"';
            break;
        }
        case Json::Array:
            o << '[';
            for (size_t i = 0; i < j.arr.size(); i++) {
                if (i) o << ',';
                json_write(o, j.arr[i]);
            }
            o << ']';
            break;
        case Json::Object:
            o << '{';
            for (size_t i = 0; i < j.obj.size(); i++) {
                if (i) o << ',';
                json_write(o, Json::str_(j.obj[i].first));
                o << ':';
                json_write(o, j.obj[i].second);
            }
            o << '}';
            break;
    }
}

static void emit(const Json& j) {
    json_write(std::cout, j);
    std::cout << '\n';
    std::cout.flush();
}

// JSON parser (minimal, enough for protocol messages)

// parse_hex4 reads the 4 hex digits of a \uXXXX escape. On entry p points at the
// 'u'; on exit it points at the last hex digit, so the caller's p++ moves past it.
inline uint32_t parse_hex4(const char*& p) {
    uint32_t v = 0;
    for (int i = 0; i < 4; i++) {
        char c = *(++p);
        v <<= 4;
        if      (c >= '0' && c <= '9') v |= (uint32_t)(c - '0');
        else if (c >= 'a' && c <= 'f') v |= (uint32_t)(c - 'a' + 10);
        else if (c >= 'A' && c <= 'F') v |= (uint32_t)(c - 'A' + 10);
        else throw std::runtime_error("invalid \\u escape");
    }
    return v;
}

inline void append_utf8(std::string& s, uint32_t cp) {
    if (cp < 0x80) {
        s += (char)cp;
    } else if (cp < 0x800) {
        s += (char)(0xC0 | (cp >> 6));
        s += (char)(0x80 | (cp & 0x3F));
    } else if (cp < 0x10000) {
        s += (char)(0xE0 | (cp >> 12));
        s += (char)(0x80 | ((cp >> 6) & 0x3F));
        s += (char)(0x80 | (cp & 0x3F));
    } else {
        s += (char)(0xF0 | (cp >> 18));
        s += (char)(0x80 | ((cp >> 12) & 0x3F));
        s += (char)(0x80 | ((cp >> 6) & 0x3F));
        s += (char)(0x80 | (cp & 0x3F));
    }
}

struct Parser {
    const char* p;
    const char* end;

    void skip_ws() { while (p < end && (*p==' '||*p=='\t'||*p=='\n'||*p=='\r')) p++; }

    std::string parse_string() {
        assert(*p == '"'); p++;
        std::string s;
        while (p < end && *p != '"') {
            if (*p == '\\') { p++;
                switch (*p) {
                    case '"': s+='"'; break; case '\\': s+='\\'; break;
                    case '/': s+='/'; break;
                    case 'b': s+='\b'; break; case 'f': s+='\f'; break;
                    case 'n': s+='\n'; break; case 'r': s+='\r'; break;
                    case 't': s+='\t'; break;
                    case 'u': {
                        // \uXXXX -> UTF-8. Go's encoding/json escapes <, > and &
                        // this way by default, so schema types arrive as
                        // "optional\u003cstring\u003e"; dropping the escape here
                        // mangles every composite type name.
                        uint32_t cp = parse_hex4(p);
                        if (cp >= 0xD800 && cp <= 0xDBFF && p + 6 < end &&
                            *(p + 1) == '\\' && *(p + 2) == 'u') {
                            p += 2;                       // step onto the low surrogate's 'u'
                            uint32_t lo = parse_hex4(p);
                            cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
                        }
                        append_utf8(s, cp);
                        break;
                    }
                    default: s+=*p; break;
                }
            } else s += *p;
            p++;
        }
        if (p < end) p++;
        return s;
    }

    Json parse() {
        skip_ws();
        if (p >= end) return Json::null_();
        if (*p == '"') return Json::str_(parse_string());
        if (*p == 'n') { p+=4; return Json::null_(); }
        if (*p == 't') { p+=4; return Json::bool_(true); }
        if (*p == 'f') { p+=5; return Json::bool_(false); }
        if (*p == '[') {
            p++; auto j = Json::arr_();
            skip_ws();
            if (p<end && *p == ']') { p++; return j; }
            do { skip_ws(); j.push(parse()); skip_ws(); } while (p<end && *p==',' && (p++,true));
            if (p<end && *p==']') p++;
            return j;
        }
        if (*p == '{') {
            p++; auto j = Json::obj_();
            skip_ws();
            if (p<end && *p == '}') { p++; return j; }
            do {
                skip_ws();
                auto k = parse_string();
                skip_ws(); assert(*p == ':'); p++;
                j.set(std::move(k), parse());
                skip_ws();
            } while (p<end && *p==',' && (p++,true));
            if (p<end && *p=='}') p++;
            return j;
        }
        char* ep;
        const char* start = p;
        double n = strtod(p, &ep);
        p = ep;
        Json j = Json::num_(n);
        j.str.assign(start, static_cast<size_t>(ep - start)); // the undamaged token
        return j;
    }
};



static Json parse_json(const std::string& s) {
    Parser par{s.data(), s.data()+s.size()};
    return par.parse();
}

// Hex utilities

static std::string to_hex(const uint8_t* data, size_t len) {
    static const char* h = "0123456789abcdef";
    std::string s(len*2, '0');
    for (size_t i=0;i<len;i++) { s[2*i]=h[data[i]>>4]; s[2*i+1]=h[data[i]&0xf]; }
    return s;
}
static std::vector<uint8_t> from_hex(const std::string& s) {
    std::vector<uint8_t> out(s.size()/2);
    for (size_t i=0;i<out.size();i++) {
        auto nibble=[](char c)->uint8_t{
            return c>='0'&&c<='9'?c-'0':c>='a'&&c<='f'?c-'a'+10:c-'A'+10;
        };
        out[i]=(nibble(s[2*i])<<4)|nibble(s[2*i+1]);
    }
    return out;
}

// SchemaField - carries nested fields for struct types

struct SchemaField;

// One arm of a sum: a tag and its payload schema (empty for a unit variant).
struct SchemaVariant {
    std::string name;
    std::vector<SchemaField> payload; // 0 or 1 element; a vector keeps SchemaField incomplete here
};

struct SchemaField {
    std::string name, type;
    std::vector<SchemaField> fields; // nested schema for struct / list[struct] / optional[struct]
    std::vector<SchemaVariant> variants; // arms of a sum<...>; empty otherwise
    std::map<std::string, std::string> tags;

    const SchemaVariant& find_variant(const std::string& tag) const {
        for (auto& sv : variants) if (sv.name == tag) return sv;
        throw std::runtime_error("unknown variant \"" + tag + "\"");
    }
};

static std::vector<SchemaField> parse_schema_fields(const Json& arr);

static std::vector<SchemaVariant> parse_schema_variants(const Json& arr) {
    std::vector<SchemaVariant> result;
    if (!arr.is_arr()) return result;
    for (auto& v : arr.arr) {
        SchemaVariant sv;
        if (auto* nm = v.get("name")) sv.name = nm->as_str();
        if (auto* pl = v.get("payload"); pl && pl->is_obj()) {
            // Reuse the field parser by wrapping the payload in a one-element array.
            Json wrapper = Json::arr_();
            wrapper.push(*pl);
            sv.payload = parse_schema_fields(wrapper);
        }
        result.push_back(std::move(sv));
    }
    return result;
}

static std::vector<SchemaField> parse_schema_fields(const Json& arr) {
    std::vector<SchemaField> result;
    if (!arr.is_arr()) return result;
    for (auto& f : arr.arr) {
        SchemaField sf;
        if (auto* nm = f.get("name")) sf.name = nm->as_str();
        if (auto* tp = f.get("type")) sf.type = tp->as_str();
        if (auto* fl = f.get("fields")) sf.fields = parse_schema_fields(*fl);
        if (auto* vs = f.get("variants")) sf.variants = parse_schema_variants(*vs);
        if (auto* tg = f.get("tags")) {
            for (auto& kv : tg->obj) sf.tags[kv.first] = kv.second.as_str();
        }
        result.push_back(std::move(sf));
    }
    return result;
}

// Split "array<T,N>" into its element type and length.
static std::pair<std::string, size_t> serify_split_array(const std::string& t) {
    std::string inner = t.substr(6, t.size() - 7);
    size_t comma = inner.rfind(',');
    if (comma == std::string::npos)
        throw std::runtime_error("array type " + t + " has no length");
    std::string elem = inner.substr(0, comma);
    while (!elem.empty() && elem.back() == ' ') elem.pop_back();
    return {elem, (size_t)std::stoul(inner.substr(comma + 1))};
}

// Element types a list may carry: every scalar, plus a nested struct.
static bool serify_is_list_elem(const std::string& e) {
    static const std::set<std::string> kElems = {
        "uint8", "uint16", "uint32", "uint64", "uint128",
        "int8", "int16", "int32", "int64", "int128",
        "float32", "float64", "bool", "string", "bytes",
        "struct",
    };
    return kElems.count(e) > 0;
}

// Decode / Encode - forward declarations

// Value type of a map<K,V> type string. The comma has to be found at nesting
// depth 0: a map's value may itself be parameterized (map<string, list<uint8>>),
// so scanning for the first plain comma splits the wrong one.
inline std::string map_value_type(const std::string& t) {
    std::string inner = t.substr(4, t.size()-5); // strip "map<" and ">"
    int depth=0; size_t comma=std::string::npos;
    for (size_t i=0;i<inner.size();i++) {
        if (inner[i]=='<'||inner[i]=='[') depth++;
        else if (inner[i]=='>'||inner[i]==']') depth--;
        else if (inner[i]==','&&depth==0) { comma=i; break; }
    }
    std::string v=(comma==std::string::npos)?inner:inner.substr(comma+1);
    while(!v.empty()&&v.front()==' ') v.erase(0,1);
    while(!v.empty()&&v.back()==' ') v.pop_back();
    return v;
}

static FieldMap decode_field_map(const Json& data, const std::vector<SchemaField>& schema);
static Json encode_field_map(const FieldMap& fm, const std::vector<SchemaField>& schema);

static FieldMap decode_field_map(const Json& data, const std::vector<SchemaField>& schema) {
    FieldMap fm;
    for (auto& sf : schema) {
        auto* el = data.get(sf.name);
        if (!el) continue;
        auto& n = sf.name; auto& t = sf.type;
        if (t=="uint8")       fm.set_u8(n,(uint8_t)el->as_num());
        else if (t=="uint16") fm.set_u16(n,(uint16_t)el->as_num());
        else if (t=="uint32") fm.set_u32(n,(uint32_t)el->as_num());
        else if (t=="uint64")  fm.set_u64(n,(uint64_t)std::stoull(el->as_str()));
        else if (t=="uint128") fm.set_u128(n, serify_u128_from_str(el->as_str()));
        else if (t=="int8")  fm.set_i8(n,(int8_t)el->as_num());
        else if (t=="int16") fm.set_i16(n,(int16_t)el->as_num());
        else if (t=="int32") fm.set_i32(n,(int32_t)el->as_num());
        else if (t=="int64")  fm.set_i64(n,(int64_t)std::stoll(el->as_str()));
        else if (t=="int128") fm.set_i128(n, serify_i128_from_str(el->as_str()));
        else if (t=="float32") {
            auto b=from_hex(el->as_str()); uint32_t bits; memcpy(&bits,b.data(),4);
            float f; memcpy(&f,&bits,4); fm.set_f32(n,f);
        }
        else if (t=="float64") {
            auto b=from_hex(el->as_str()); uint64_t bits; memcpy(&bits,b.data(),8);
            double d; memcpy(&d,&bits,8); fm.set_f64(n,d);
        }
        else if (t=="bool")   fm.set_bool(n,el->as_bool());
        else if (t=="string") fm.set_string(n,el->as_str());
        else if (t=="bytes")  fm.set_bytes(n,from_hex(el->as_str()));
        else if (t=="struct") {
            fm.set_struct(n, std::make_shared<FieldMap>(decode_field_map(*el, sf.fields)));
        }
        else if (t.rfind("list<",0)==0) {
            // Every element is decoded through this same function, as a one-field
            // record, so a list supports exactly the element types a bare field
            // does. This branch used to carry its own if/else chain with no final
            // else at all, so a list of any element type it did not name left the
            // field silently absent from the FieldMap instead of raising.
            std::string elem = t.substr(5, t.size()-6);
            if (!serify_is_list_elem(elem))
                throw std::runtime_error("unsupported list element type \"" + elem + "\"");

            if (elem=="struct") {
                ListStruct v;
                for (auto& item:el->arr)
                    v.push_back(std::make_shared<FieldMap>(decode_field_map(item, sf.fields)));
                fm.set_list_struct(n,v);
            } else {
                SchemaField esf; esf.name = "e"; esf.type = elem; esf.fields = sf.fields;
                std::vector<SchemaField> eschema{esf};
                std::vector<FieldValue> vals;
                vals.reserve(el->arr.size());
                for (auto& item : el->arr) {
                    Json holder = Json::obj_();
                    holder.set("e", item);
                    FieldMap tmp = decode_field_map(holder, eschema);
                    vals.push_back(tmp.raw().at("e"));
                }
                // Pack the decoded elements into the vector for this element type.
                #define SERIFY_PACK_LIST(LIST_T, SCALAR_T)                       \
                    { LIST_T out; out.reserve(vals.size());                      \
                      for (auto& fv : vals) out.push_back(std::get<SCALAR_T>(fv)); \
                      fm.set<LIST_T>(n, std::move(out)); }
                if      (elem=="uint8")   SERIFY_PACK_LIST(ListU8,   uint8_t)
                else if (elem=="uint16")  SERIFY_PACK_LIST(ListU16,  uint16_t)
                else if (elem=="uint32")  SERIFY_PACK_LIST(ListU32,  uint32_t)
                else if (elem=="uint64")  SERIFY_PACK_LIST(ListU64,  uint64_t)
                else if (elem=="uint128") SERIFY_PACK_LIST(ListU128, u128)
                else if (elem=="int8")    SERIFY_PACK_LIST(ListI8,   int8_t)
                else if (elem=="int16")   SERIFY_PACK_LIST(ListI16,  int16_t)
                else if (elem=="int32")   SERIFY_PACK_LIST(ListI32,  int32_t)
                else if (elem=="int64")   SERIFY_PACK_LIST(ListI64,  int64_t)
                else if (elem=="int128")  SERIFY_PACK_LIST(ListI128, i128)
                else if (elem=="float32") SERIFY_PACK_LIST(ListF32,  float)
                else if (elem=="float64") SERIFY_PACK_LIST(ListF64,  double)
                else if (elem=="bool")    SERIFY_PACK_LIST(ListBool, bool)
                else if (elem=="string")  SERIFY_PACK_LIST(ListString, std::string)
                else if (elem=="bytes")   SERIFY_PACK_LIST(ListBytes, Bytes)
                #undef SERIFY_PACK_LIST
            }
        }
        else if (t.rfind("optional<",0)==0) {
            // string and struct have their own std::optional alternatives; every
            // other element type decodes through the scalar path and carries a
            // null as std::monostate. This branch used to name only those two and
            // silently drop the field for anything else.
            std::string elem = t.substr(9, t.size()-10);
            if (el->is_null()) {
                if (elem=="string") fm.set_optional_string(n,std::nullopt);
                else if (elem=="struct") fm.set_optional_struct(n,std::nullopt);
                else fm.set_raw(n, std::monostate{});
            } else if (elem=="string") {
                fm.set_optional_string(n,el->as_str());
            } else if (elem=="struct") {
                fm.set_optional_struct(n,std::make_shared<FieldMap>(decode_field_map(*el,sf.fields)));
            } else {
                SchemaField isf; isf.name = n; isf.type = elem; isf.fields = sf.fields;
                Json holder = Json::obj_();
                holder.set(n, *el);
                FieldMap tmp = decode_field_map(holder, std::vector<SchemaField>{isf});
                fm.set_raw(n, tmp.raw().at(n));
            }
        }
        else if (t.rfind("array<",0)==0) {
            // An array<T,N> is a list whose length the schema fixes, so it shares
            // the list branch outright and adds only the length check. A separate
            // representation is what pinned array<T,N> to uint32 with N = 4 — it
            // silently truncated anything longer.
            auto [elem, want] = serify_split_array(t);
            if (el->arr.size() != want)
                throw std::runtime_error("array " + n + ": expected " + std::to_string(want) +
                                         " elements, got " + std::to_string(el->arr.size()));
            SchemaField lsf; lsf.name = n; lsf.type = "list<" + elem + ">"; lsf.fields = sf.fields;
            Json holder = Json::obj_();
            holder.set(n, *el);
            FieldMap tmp = decode_field_map(holder, std::vector<SchemaField>{lsf});
            fm.set_raw(n, tmp.raw().at(n));
        }
        // enum<a,b,c>: the variant name travels as a string.
        else if (t.rfind("enum<",0)==0) fm.set_string(n, el->as_str());
        // sum<a, b: T>: {tag: payload} on the wire (payload null for a unit
        // variant). The payload is decoded through the variant's own schema by
        // re-entering this function with a one-field schema.
        else if (t.rfind("sum<",0)==0) {
            if (el->obj.size() != 1)
                throw std::runtime_error("sum must name exactly one variant, got " +
                                         std::to_string(el->obj.size()));
            auto& tag = el->obj[0].first;
            auto& sv  = sf.find_variant(tag);
            if (sv.payload.empty()) { fm.set_variant(n, tag); }
            else {
                auto& psf = sv.payload[0];
                Json wrapper = Json::obj_();
                wrapper.set(psf.name, el->obj[0].second);
                FieldMap tmp = decode_field_map(wrapper, sv.payload);
                if (!tmp.raw().count(psf.name))
                    throw std::runtime_error("variant \"" + tag +
                                             "\": unsupported payload type " + psf.type);
                fm.set_variant(n, tag, tmp.raw().at(psf.name));
            }
        }
        else if (t.rfind("map<",0)==0) {
            std::string valType = map_value_type(t);

            MapStore m;
            for (auto& kv : el->obj) {
                if (valType=="string") m[kv.first]=kv.second.as_str();
                else if (valType=="uint32"||valType=="uint16"||valType=="uint8") m[kv.first]=(uint32_t)kv.second.as_num();
                else if (valType=="uint64")  m[kv.first]=(uint64_t)std::stoull(kv.second.as_str());
                else if (valType=="uint128") m[kv.first]=serify_u128_from_str(kv.second.as_str());
                else if (valType=="int32"||valType=="int16"||valType=="int8") m[kv.first]=(int32_t)kv.second.as_num();
                else if (valType=="int64")  m[kv.first]=(int64_t)std::stoll(kv.second.as_str());
                else if (valType=="int128") m[kv.first]=serify_i128_from_str(kv.second.as_str());
                else if (valType=="float32") {
                    auto b=from_hex(kv.second.as_str()); uint32_t bits; memcpy(&bits,b.data(),4);
                    float f; memcpy(&f,&bits,4); m[kv.first]=f;
                }
                else if (valType=="float64") {
                    auto b=from_hex(kv.second.as_str()); uint64_t bits; memcpy(&bits,b.data(),8);
                    double d; memcpy(&d,&bits,8); m[kv.first]=d;
                }
                else if (valType=="bool") m[kv.first]=kv.second.as_bool();
                else if (valType=="bytes") m[kv.first]=from_hex(kv.second.as_str());
                else if (valType=="struct")
                    m[kv.first]=std::make_shared<FieldMap>(decode_field_map(kv.second,sf.fields));
                else m[kv.first]=kv.second.as_str();
            }
            fm.set_map(n,std::move(m));
        }
        // Falling off the chain left the field absent from the FieldMap, which
        // surfaces far downstream as a missing value rather than as "this
        // library does not know that type".
        else throw std::runtime_error("unknown type \"" + t + "\"");
    }
    return fm;
}

static Json encode_field_map(const FieldMap& fm, const std::vector<SchemaField>& schema) {
    auto out = Json::obj_();
    for (auto& sf : schema) {
        if (!fm.has(sf.name)) continue;
        auto& n=sf.name; auto& t=sf.type;
        // sum values live in a separate container. Inverse of the decode
        // branch: a Variant becomes {tag: payload}.
        if (t.rfind("sum<",0)==0) {
            auto& var = fm.get_variant(n);
            auto& sv  = sf.find_variant(var.tag);
            auto obj  = Json::obj_();
            if (sv.payload.empty() || var.is_unit()) obj.set(var.tag, Json::null_());
            else {
                auto& psf = sv.payload[0];
                FieldMap tmp;
                tmp.set(psf.name, *var.value);
                obj.set(var.tag, *encode_field_map(tmp, sv.payload).get(psf.name));
            }
            out.set(n, obj);
            continue;
        }
        // Map fields live in a separate container.
        if (t.rfind("map<",0)==0) {
            auto& m = fm.maps().at(n);
            std::string valType = map_value_type(t);

            auto obj=Json::obj_();
            for (auto& [mk,mv] : m) {
                if (valType=="string") obj.set(mk,Json::str_(std::get<std::string>(mv)));
                else if (valType=="uint32"||valType=="uint16"||valType=="uint8") obj.set(mk,Json::num_(std::get<uint32_t>(mv)));
                else if (valType=="uint64")  obj.set(mk,Json::str_(std::to_string(std::get<uint64_t>(mv))));
                else if (valType=="uint128") obj.set(mk,Json::str_(serify_u128_to_str(std::get<u128>(mv))));
                else if (valType=="int32"||valType=="int16"||valType=="int8") obj.set(mk,Json::num_(std::get<int32_t>(mv)));
                else if (valType=="int64")  obj.set(mk,Json::str_(std::to_string(std::get<int64_t>(mv))));
                else if (valType=="int128") obj.set(mk,Json::str_(serify_i128_to_str(std::get<i128>(mv))));
                else if (valType=="float32") {
                    float f=std::get<float>(mv); uint32_t bits; memcpy(&bits,&f,4);
                    uint8_t b[4]; for(int i=0;i<4;i++) b[i]=(bits>>(i*8))&0xff;
                    obj.set(mk,Json::str_(to_hex(b,4)));
                }
                else if (valType=="float64") {
                    double d=std::get<double>(mv); uint64_t bits; memcpy(&bits,&d,8);
                    uint8_t b[8]; for(int i=0;i<8;i++) b[i]=(bits>>(i*8))&0xff;
                    obj.set(mk,Json::str_(to_hex(b,8)));
                }
                else if (valType=="bool") obj.set(mk,Json::bool_(std::get<bool>(mv)));
                else if (valType=="bytes") {
                    auto& bv=std::get<Bytes>(mv);
                    obj.set(mk,Json::str_(to_hex(bv.data(),bv.size())));
                }
                else if (valType=="struct") {
                    auto& sp=std::get<StructPtr>(mv);
                    obj.set(mk, sp ? encode_field_map(*sp, sf.fields) : Json::null_());
                }
                else obj.set(mk,Json::str_(std::get<std::string>(mv)));
            }
            out.set(n,obj);
            continue;
        }

        auto& v=fm.raw().at(n);
        // enum<a,b,c>: the variant name goes back out as a plain string.
        if (t.rfind("enum<",0)==0) out.set(n,Json::str_(std::get<std::string>(v)));
        else if (t=="uint8")  out.set(n,Json::num_(std::get<uint8_t>(v)));
        else if (t=="uint16") out.set(n,Json::num_(std::get<uint16_t>(v)));
        else if (t=="uint32") out.set(n,Json::num_(std::get<uint32_t>(v)));
        else if (t=="uint64")  out.set(n,Json::str_(std::to_string(std::get<uint64_t>(v))));
        else if (t=="uint128") out.set(n,Json::str_(serify_u128_to_str(std::get<u128>(v))));
        else if (t=="int8")  out.set(n,Json::num_(std::get<int8_t>(v)));
        else if (t=="int16") out.set(n,Json::num_(std::get<int16_t>(v)));
        else if (t=="int32") out.set(n,Json::num_(std::get<int32_t>(v)));
        else if (t=="int64")  out.set(n,Json::str_(std::to_string(std::get<int64_t>(v))));
        else if (t=="int128") out.set(n,Json::str_(serify_i128_to_str(std::get<i128>(v))));
        else if (t=="float32") {
            float f=std::get<float>(v); uint32_t bits; memcpy(&bits,&f,4);
            uint8_t b[4]; for(int i=0;i<4;i++) b[i]=(bits>>(i*8))&0xff;
            out.set(n,Json::str_(to_hex(b,4)));
        }
        else if (t=="float64") {
            double d=std::get<double>(v); uint64_t bits; memcpy(&bits,&d,8);
            uint8_t b[8]; for(int i=0;i<8;i++) b[i]=(bits>>(i*8))&0xff;
            out.set(n,Json::str_(to_hex(b,8)));
        }
        else if (t=="bool")   out.set(n,Json::bool_(std::get<bool>(v)));
        else if (t=="string") out.set(n,Json::str_(std::get<std::string>(v)));
        else if (t=="bytes") {
            auto& bv=std::get<Bytes>(v);
            out.set(n,Json::str_(to_hex(bv.data(),bv.size())));
        }
        else if (t=="struct") {
            auto& sp=std::get<StructPtr>(v);
            out.set(n, sp ? encode_field_map(*sp, sf.fields) : Json::null_());
        }
        else if (t.rfind("list<",0)==0) {
            // Inverse of the decode branch: every element goes back out through
            // encode_field_map as a one-field record, so the two directions
            // cannot cover different element types.
            std::string elem = t.substr(5, t.size()-6);
            if (!serify_is_list_elem(elem))
                throw std::runtime_error("unsupported list element type \"" + elem + "\"");

            Json arr = Json::arr_();
            if (elem=="struct") {
                for (auto& sp:std::get<ListStruct>(v))
                    arr.push(sp ? encode_field_map(*sp, sf.fields) : Json::null_());
            } else {
                SchemaField esf; esf.name = "e"; esf.type = elem; esf.fields = sf.fields;
                std::vector<SchemaField> eschema{esf};
                // Unpack the typed vector back into per-element FieldValues.
                std::vector<FieldValue> vals;
                #define SERIFY_UNPACK_LIST(LIST_T)                                \
                    { auto& xs = std::get<LIST_T>(v);                             \
                      vals.reserve(xs.size());                                    \
                      for (auto x : xs) vals.push_back(FieldValue(x)); }
                if      (elem=="uint8")   SERIFY_UNPACK_LIST(ListU8)
                else if (elem=="uint16")  SERIFY_UNPACK_LIST(ListU16)
                else if (elem=="uint32")  SERIFY_UNPACK_LIST(ListU32)
                else if (elem=="uint64")  SERIFY_UNPACK_LIST(ListU64)
                else if (elem=="uint128") SERIFY_UNPACK_LIST(ListU128)
                else if (elem=="int8")    SERIFY_UNPACK_LIST(ListI8)
                else if (elem=="int16")   SERIFY_UNPACK_LIST(ListI16)
                else if (elem=="int32")   SERIFY_UNPACK_LIST(ListI32)
                else if (elem=="int64")   SERIFY_UNPACK_LIST(ListI64)
                else if (elem=="int128")  SERIFY_UNPACK_LIST(ListI128)
                else if (elem=="float32") SERIFY_UNPACK_LIST(ListF32)
                else if (elem=="float64") SERIFY_UNPACK_LIST(ListF64)
                else if (elem=="bool")    SERIFY_UNPACK_LIST(ListBool)
                #undef SERIFY_UNPACK_LIST
                else if (elem=="string") {
                    auto& xs = std::get<ListString>(v);
                    vals.reserve(xs.size());
                    for (auto& x : xs) vals.push_back(FieldValue(x));
                } else if (elem=="bytes") {
                    auto& xs = std::get<ListBytes>(v);
                    vals.reserve(xs.size());
                    for (auto& x : xs) vals.push_back(FieldValue(x));
                }
                for (auto& fv : vals) {
                    FieldMap tmp; tmp.set_raw("e", fv);
                    Json one = encode_field_map(tmp, eschema);
                    if (auto* got = one.get("e")) arr.push(*got);
                }
            }
            out.set(n,arr);
        }
        else if (t.rfind("optional<",0)==0) {
            std::string elem = t.substr(9, t.size()-10);
            if (elem=="string") {
                auto& opt=std::get<std::optional<std::string>>(v);
                out.set(n, opt ? Json::str_(*opt) : Json::null_());
            } else if (elem=="struct") {
                auto& opt=std::get<std::optional<StructPtr>>(v);
                if (!opt || !*opt) out.set(n, Json::null_());
                else out.set(n, encode_field_map(**opt, sf.fields));
            } else if (std::holds_alternative<std::monostate>(v)) {
                out.set(n, Json::null_());
            } else {
                SchemaField isf; isf.name = n; isf.type = elem; isf.fields = sf.fields;
                FieldMap tmp; tmp.set_raw(n, v);
                Json one = encode_field_map(tmp, std::vector<SchemaField>{isf});
                if (auto* got = one.get(n)) out.set(n, *got);
            }
        }
        else if (t.rfind("array<",0)==0) {
            auto [elem, _want] = serify_split_array(t);
            SchemaField lsf; lsf.name = n; lsf.type = "list<" + elem + ">"; lsf.fields = sf.fields;
            FieldMap tmp; tmp.set_raw(n, v);
            Json one = encode_field_map(tmp, std::vector<SchemaField>{lsf});
            if (auto* got = one.get(n)) out.set(n, *got);
        }
        // Mirror of the decode chain: emitting nothing for an unrecognised type
        // drops the field from the response with nothing reported.
        else throw std::runtime_error("unknown type \"" + t + "\"");
    }
    return out;
}

// run()

// The protocol revision this library speaks. The runner requires an exact
// match and refuses to start a worker reporting anything else.
static const int PROTOCOL_VERSION = 2;

// --- audit helpers --------------------------------------------------------

// in_variant marks a snapshot taken from a sum payload rather than a plain
// bytes field, so the comparison knows where to read the current value from.
struct ByteSnap { FieldMap* fm; std::string key; Bytes orig; bool in_variant = false; };

inline void collect_byte_snaps(FieldMap& fm, std::vector<ByteSnap>& snaps) {
    for (auto& [k, v] : fm.raw()) {
        if (auto* b = std::get_if<Bytes>(&v)) {
            snaps.push_back({&fm, k, *b});
        } else if (auto* sp = std::get_if<StructPtr>(&v)) {
            collect_byte_snaps(**sp, snaps);
        } else if (auto* ls = std::get_if<ListStruct>(&v)) {
            for (auto& item : *ls) collect_byte_snaps(*item, snaps);
        } else if (auto* os = std::get_if<std::optional<StructPtr>>(&v)) {
            if (*os) collect_byte_snaps(***os, snaps);
        }
    }
    // Also walk map fields (stored separately).
    for (auto& [mk, m] : fm.maps()) {
        for (auto& [k, mv] : m) {
            if (auto* nested = std::get_if<StructPtr>(&mv))
                collect_byte_snaps(**nested, snaps);
        }
    }
    // …and sum fields, likewise stored separately. The variant itself cannot
    // alias, but its payload can: snapshot the payload so a zero-copy variant
    // shows up as a change.
    for (auto& [k, var] : fm.variants()) {
        if (!var.value) continue;
        if (auto* b = std::get_if<Bytes>(var.value.get())) {
            snaps.push_back({&fm, k, *b, /*in_variant=*/true});
        } else if (auto* sp = std::get_if<StructPtr>(var.value.get())) {
            collect_byte_snaps(**sp, snaps);
        }
    }
}

inline std::vector<std::string> dict_diffs(const Json& before, const Json& after) {
    std::vector<std::string> diffs;
    std::set<std::string> keys;
    if (before.is_obj()) for (auto& [k,_] : before.obj) keys.insert(k);
    if (after.is_obj())  for (auto& [k,_] : after.obj) keys.insert(k);
    for (auto& k : keys) {
        auto* bv = before.get(k);
        auto* av = after.get(k);
        // Compare via JSON serialisation.
        std::ostringstream bs, as;
        if (bv) json_write(bs, *bv);
        if (av) json_write(as, *av);
        if (bs.str() != as.str()) diffs.push_back(k);
    }
    return diffs;
}

inline std::vector<std::string> detect_zero_copy_cpp(FieldMap& fm, std::vector<uint8_t>& buf) {
    if (buf.empty()) return {};
    std::vector<ByteSnap> snaps;
    collect_byte_snaps(fm, snaps);
    for (auto& b : buf) b ^= 0xFF;
    std::vector<std::string> aliased;
    for (auto& s : snaps) {
        const Bytes* v = nullptr;
        if (s.in_variant) {
            auto& var = s.fm->get_variant(s.key);
            if (var.value) v = std::get_if<Bytes>(var.value.get());
        } else {
            v = std::get_if<Bytes>(&s.fm->raw().at(s.key));
        }
        if (v && *v != s.orig) aliased.push_back(s.key);
    }
    for (auto& s : snaps) {
        if (s.in_variant) {
            s.fm->set_variant(s.key, s.fm->get_variant(s.key).tag, s.orig);
        } else {
            s.fm->set_bytes(s.key, s.orig);
        }
    }
    return aliased;
}

// One (serialize, deserialize) pair for a single format.
using SerFn   = std::function<std::vector<uint8_t>(const FieldMap&)>;
using DeserFn = std::function<FieldMap(const std::vector<uint8_t>&)>;
struct FormatPair { SerFn serialize; DeserFn deserialize; };
// type name -> format name -> pair
using SuiteMap = std::map<std::string, std::map<std::string, FormatPair>>;

// One format whose functions speak the model M rather than a FieldMap: serify
// converts around them, using the SERIFY_TO / SERIFY_FROM binding on M.
//
//     suite["ledger"]["binary"] = model_format<LedgerEntry>(ledger_marshal,
//                                                           ledger_unmarshal);
//
// Assigning a plain FormatPair is the other path, for a type with no natural
// struct — the audit fixtures mutate a FieldMap on purpose.
//
// The conversion is baked in here, at registration, rather than resolved when
// the runner binds. That is deliberate and matches rust's Format::model::<M>():
// with no reflection there is nothing to resolve at run time, and a model whose
// binding does not compile is a compile error at the registration site, not a
// (type, format) that silently reports SKIPPED.
template <typename M, typename Ser, typename Deser>
FormatPair model_format(Ser serialize, Deser deserialize) {
    return FormatPair{
        [serialize](const FieldMap& fm) {
            return serialize(from_field_map_of(fm, static_cast<const M*>(nullptr)));
        },
        [deserialize](const std::vector<uint8_t>& data) {
            return to_field_map(deserialize(data));
        },
    };
}

// Serialize-only: the runner reports this format's deserialize direction as
// unsupported rather than skipping the type.
template <typename M, typename Ser>
FormatPair model_format(Ser serialize) {
    return FormatPair{
        [serialize](const FieldMap& fm) {
            return serialize(from_field_map_of(fm, static_cast<const M*>(nullptr)));
        },
        nullptr,
    };
}

// Multi-type worker. A (type, format) that is not registered is reported to the
// runner as SKIPPED rather than failing the run.
inline void run_suite(const SuiteMap& suite);

// Single-type worker: handles whatever type/format the runner asks for.
template<typename Ser, typename Deser>
void run(Ser serialize, Deser deserialize) {
    SuiteMap suite;
    suite["*"]["*"] = FormatPair{SerFn(serialize), DeserFn(deserialize)};
    run_suite(suite);
}

inline void run_suite(const SuiteMap& suite) {
    SerFn serialize;
    DeserFn deserialize;
    std::vector<SchemaField> schema;
    bool audit_enabled = false;
    std::string line;

    auto emit_err = [&](const std::string& id, const std::string& op,
                        const std::string& status, const std::string& err){
        auto j=Json::obj_();
        j.set("id",Json::str_(id)); j.set("op",Json::str_(op));
        j.set("status",Json::str_(status)); j.set("error",Json::str_(err));
        emit(j);
    };

    // Helper to conditionally set an audit sub-object on a response.
    auto set_audit = [](Json& r, const Json& audit) {
        if (audit.is_obj() && !audit.obj.empty()) r.set("audit", audit);
    };

    while (std::getline(std::cin, line)) {
        if (!line.empty() && line.back()=='\r') line.pop_back();
        if (line.empty()) continue;

        auto msg = parse_json(line);
        auto* op_el = msg.get("op");
        if (!op_el) continue;
        std::string op = op_el->as_str();
        std::string id;
        if (auto* el = msg.get("id")) id = el->as_str();

        if (op == "ping") {
            // Health check: report liveness and the protocol revision this
            // library speaks. Binds nothing.
            auto r = Json::obj_();
            r.set("op", Json::str_("ping"));
            r.set("status", Json::str_("OK"));
            r.set("protocol_version", Json::num_(PROTOCOL_VERSION));
            emit(r);
        }
        else if (op == "bind") {
            schema.clear();
            if (auto* s = msg.get("schema")) schema = parse_schema_fields(*s);
            if (auto* a = msg.get("audit")) audit_enabled = a->as_bool();

            std::string type_name, format_name;
            if (auto* t = msg.get("type"))   type_name   = t->as_str();
            if (auto* f = msg.get("format")) format_name = f->as_str();

            const FormatPair* pair = nullptr;
            auto ti = suite.find(type_name);
            if (ti == suite.end()) ti = suite.find("*");
            if (ti != suite.end()) {
                auto fi = ti->second.find(format_name);
                if (fi == ti->second.end()) fi = ti->second.find("*");
                if (fi != ti->second.end()) pair = &fi->second;
            }

            auto r = Json::obj_();
            r.set("op", Json::str_("bind"));
            if (pair == nullptr) {
                serialize = nullptr;
                deserialize = nullptr;
                r.set("status", Json::str_("SKIPPED"));
            } else {
                serialize = pair->serialize;
                deserialize = pair->deserialize;
            }
            emit(r);
        }
        else if (op == "serialize") {
            try {
                auto* data_el = msg.get("data");
                if (!data_el) throw std::runtime_error("missing data");
                auto fm  = decode_field_map(*data_el, schema);

                Json before = audit_enabled ? encode_field_map(fm, schema) : Json();

                auto vec = serialize(fm);
                auto hex = to_hex(vec.data(), vec.size());
                auto r   = Json::obj_();
                r.set("id",Json::str_(id)); r.set("op",Json::str_("serialize"));
                r.set("status",Json::str_("OK"));
                r.set("hex",Json::str_(hex));

                if (audit_enabled) {
                    auto audit = Json::obj_();

                    // Mutation
                    auto after = encode_field_map(fm, schema);
                    auto diffs = dict_diffs(before, after);
                    if (!diffs.empty()) {
                        auto arr = Json::arr_();
                        for (auto& d : diffs) arr.push(Json::str_(d));
                        audit.set("mutations", arr);
                    }

                    // Output zero-copy is not detectable here: FieldValue only
                    // holds owning containers (std::string, std::vector), so
                    // model fields can never alias the returned buffer.

                    // Stability
                    try {
                        auto vec2 = serialize(fm);
                        auto hex2 = to_hex(vec2.data(), vec2.size());
                        if (hex != hex2) audit.set("stable", Json::bool_(false));
                    } catch (std::exception&) {
                        audit.set("stable", Json::bool_(false));
                    }

                    set_audit(r, audit);
                }
                emit(r);
            } catch (std::exception& e) { emit_err(id,"serialize","ERROR",e.what()); }
        }
        else if (op == "deserialize") {
            try {
                auto* hex_el = msg.get("hex");
                if (!hex_el) throw std::runtime_error("missing hex");
                auto bytes = from_hex(hex_el->as_str());
                auto buf_snapshot = audit_enabled ? bytes : Bytes{};

                auto fm    = deserialize(bytes);
                auto data  = encode_field_map(fm, schema);
                auto r     = Json::obj_();
                r.set("id",Json::str_(id)); r.set("op",Json::str_("deserialize"));
                r.set("status",Json::str_("OK")); r.set("data",data);

                if (audit_enabled) {
                    auto audit = Json::obj_();

                    // Input-buffer mutation
                    if (buf_snapshot != bytes)
                        audit.set("input_mutated", Json::bool_(true));

                    // Deser-stability: re-deserialize from a clone
                    try {
                        auto bytes_clone = buf_snapshot;
                        auto fm2 = deserialize(bytes_clone);
                        auto data2 = encode_field_map(fm2, schema);
                        auto deser_diffs = dict_diffs(data, data2);
                        if (!deser_diffs.empty())
                            audit.set("deser_stable", Json::bool_(false));
                    } catch (std::exception&) {
                        audit.set("deser_stable", Json::bool_(false));
                    }

                    // Zero-copy
                    auto zc = detect_zero_copy_cpp(fm, bytes);
                    if (!zc.empty()) {
                        auto arr = Json::arr_();
                        for (auto& z : zc) arr.push(Json::str_(z));
                        audit.set("zero_copy_fields", arr);
                    }

                    set_audit(r, audit);
                }
                emit(r);
            } catch (std::exception& e) { emit_err(id,"deserialize","ERROR",e.what()); }
        }
        else if (op == "exit") {
            std::exit(0);
        }
    }
}

} // namespace serify

// ─── SERIFY_MODEL macros ────────────────────────────────────────────────
//
// C++ macros to generate to_field_map / from_field_map for a struct.
// Two separate blocks: one for serialize, one for deserialize.
//
//   struct User {
//       uint64_t user_id;
//       std::string name;
//       std::string email;
//   };
//
//   // --- Serialize ---------------------------------------------------
//   SERIFY_TO(User,
//       SERIFY_FIELD(user_id, u64)
//       SERIFY_FIELD(name, string)
//       SERIFY_FIELD_RENAMED(email, string, "email_addr")
//   )
//
//   // --- Deserialize -------------------------------------------------
//   SERIFY_FROM(User,
//       SERIFY_FROM_FIELD(user_id, u64)
//       SERIFY_FROM_FIELD(name, string)
//       SERIFY_FROM_FIELD_RENAMED(email, string, "email_addr")
//   )
//
//   // Reading: the destination is an out-parameter, so several models can
//   // coexist in one file.
//   User u{};
//   from_field_map(fm, u);
//
// Struct fields:
//   SERIFY_FIELD_STRUCT(address, Address)
//   SERIFY_FROM_FIELD_STRUCT(address, Address)
//
// list<struct>:
//   SERIFY_FIELD_LIST_STRUCT(items, ItemType)
//   SERIFY_FROM_FIELD_LIST_STRUCT(items, ItemType)
//
// sum:
//   using Channel = std::variant<std::monostate, std::string, uint64_t, Money>;
//   SERIFY_SUM(Channel, "silent", "sms", "push", "invoice")
//   SERIFY_FIELD_SUM(channel, Channel)
//   SERIFY_FROM_FIELD_SUM(channel, Channel)

// -- sum --------------------------------------------------------------
//
// std::variant is C++'s sum type and supplies the arms. What it cannot supply
// is their *names*: C++ has no reflection, so the tags are the one thing the
// user has to write, and SERIFY_SUM is the whole of it. Everything else —
// which arm is live, how its payload is carried — comes from the alternative
// types, which the compiler already knows.
//
// Each alternative *is* the payload, so the arity rule needs no wrapper structs:
//
//     std::monostate       -> a unit variant, no payload
//     a scalar / string    -> that value is the payload
//     a type with to_field_map -> the payload is that struct

namespace serify {

/// True when `to_field_map(T)` exists — i.e. T is a model, not a scalar.
template <typename T, typename = void>
struct has_field_map : std::false_type {};
template <typename T>
struct has_field_map<T, std::void_t<decltype(to_field_map(std::declval<const T&>()))>>
    : std::true_type {};

template <typename A>
inline void sum_payload_out(FieldMap& fm, const std::string& key,
                              const std::string& tag, const A& a) {
    if constexpr (std::is_same_v<A, std::monostate>) {
        (void)a;
        fm.set_variant(key, tag);
    } else if constexpr (has_field_map<A>::value) {
        fm.set_variant(key, tag, FieldValue(std::make_shared<FieldMap>(to_field_map(a))));
    } else {
        fm.set_variant(key, tag, FieldValue(a));
    }
}

/// Walk the alternatives at compile time and write whichever one is live.
template <size_t I = 0, typename V>
inline void set_sum(FieldMap& fm, const std::string& key, const V& v,
                      const char* const* tags) {
    if constexpr (I < std::variant_size_v<V>) {
        if (v.index() == I) {
            sum_payload_out(fm, key, tags[I], std::get<I>(v));
            return;
        }
        set_sum<I + 1>(fm, key, v, tags);
    } else {
        throw std::runtime_error("sum field \"" + key + "\" is valueless");
    }
}

/// Walk the alternatives at compile time and rebuild the one the tag names.
template <size_t I = 0, typename V>
inline void get_sum(const FieldMap& fm, const std::string& key, V& v,
                      const char* const* tags) {
    if constexpr (I < std::variant_size_v<V>) {
        const Variant& var = fm.get_variant(key);
        if (var.tag == tags[I]) {
            using A = std::variant_alternative_t<I, V>;
            if constexpr (std::is_same_v<A, std::monostate>) {
                v.template emplace<I>();
            } else if constexpr (has_field_map<A>::value) {
                A a{};
                from_field_map(*var.template payload<StructPtr>(), a);
                v.template emplace<I>(std::move(a));
            } else {
                v.template emplace<I>(var.template payload<A>());
            }
            return;
        }
        get_sum<I + 1>(fm, key, v, tags);
    } else {
        throw std::runtime_error("unknown variant \"" + fm.get_variant(key).tag + "\"");
    }
}

}  // namespace serify

/// Name the arms of a std::variant, in declaration order.
#define SERIFY_SUM(Type, ...) \
    inline const char* const* serify_sum_tags(const Type*) { \
        static const char* _tags[] = {__VA_ARGS__}; \
        static_assert(sizeof(_tags) / sizeof(_tags[0]) == std::variant_size_v<Type>, \
                      #Type ": SERIFY_SUM needs exactly one tag per variant alternative"); \
        return _tags; \
    }

#define SERIFY_FIELD_SUM(name, SumType) \
    serify::set_sum(fm, #name, obj.name, serify_sum_tags((const SumType*)nullptr));

#define SERIFY_FROM_FIELD_SUM(name, SumType) \
    if (fm.has_variant(#name)) \
        serify::get_sum(fm, #name, obj.name, serify_sum_tags((const SumType*)nullptr));

// -- to_field_map block -------------------------------------------------

#define SERIFY_TO(Type, ...) \
    inline serify::FieldMap to_field_map(const Type& obj) { \
        serify::FieldMap fm; \
        (void)obj; \
        __VA_ARGS__ \
        return fm; \
    }

#define SERIFY_FIELD(name, kind)   fm.set_##kind(#name, obj.name);
#define SERIFY_FIELD_RENAMED(name, kind, key)  fm.set_##kind(key, obj.name);

#define SERIFY_FIELD_STRUCT(name, NestedType) \
    fm.set_struct(#name, std::make_shared<serify::FieldMap>(to_field_map(obj.name)));

// optional<scalar> for a std::optional<T> field: a present value rides as the
// bare scalar (kind, e.g. u32), a null as std::monostate — matching the decoder,
// which has no std::optional alternative for scalars.
#define SERIFY_FIELD_OPTIONAL(name, kind) \
    if (obj.name.has_value()) fm.set_##kind(#name, *obj.name); \
    else fm.set_raw(#name, std::monostate{});

#define SERIFY_FIELD_LIST_STRUCT(name, NestedType) \
    do { \
        serify::ListStruct _ls_##name; \
        for (auto& _i : obj.name) \
            _ls_##name.push_back(std::make_shared<serify::FieldMap>(to_field_map(_i))); \
        fm.set_list_struct(#name, std::move(_ls_##name)); \
    } while(0)

// optional<struct> for a std::optional<T> field. Distinct from
// SERIFY_FIELD_STRUCT because the FieldMap slot holds a std::optional<StructPtr>
// rather than a bare StructPtr, and from SERIFY_FIELD_OPTIONAL because a struct
// payload is a nested FieldMap rather than a scalar alternative.
#define SERIFY_FIELD_OPTIONAL_STRUCT(name, NestedType) \
    if (obj.name.has_value()) \
        fm.set_optional_struct(#name, \
            std::make_shared<serify::FieldMap>(to_field_map(*obj.name))); \
    else fm.set_optional_struct(#name, std::nullopt);

#define SERIFY_FIELD_MAP_SCALAR(name, kind) \
    do { \
        serify::MapStore _m_##name; \
        for (auto& [k_, v_] : obj.name) _m_##name[k_] = v_; \
        fm.set_map(#name, std::move(_m_##name)); \
    } while(0)

#define SERIFY_FIELD_MAP_STRUCT(name, NestedType) \
    do { \
        serify::MapStore _m_##name; \
        for (auto& [k_, v_] : obj.name) \
            _m_##name[k_] = std::make_shared<serify::FieldMap>(to_field_map(v_)); \
        fm.set_map(#name, std::move(_m_##name)); \
    } while(0)

// -- from_field_map block -----------------------------------------------

// Takes the destination by reference rather than returning it: two models in
// one translation unit would otherwise declare functions differing only in
// return type, which C++ cannot overload.
#define SERIFY_FROM(Type, ...) \
    inline void from_field_map(const serify::FieldMap& fm, Type& obj) { \
        (void)fm; \
        (void)obj; \
        __VA_ARGS__ \
    } \
    inline Type from_field_map_of(const serify::FieldMap& fm, const Type*) { \
        Type obj{}; \
        from_field_map(fm, obj); \
        return obj; \
    }

#define SERIFY_FROM_FIELD(name, kind) \
    if (fm.has(#name)) obj.name = fm.get_##kind(#name);

#define SERIFY_FROM_FIELD_RENAMED(name, kind, key) \
    if (fm.has(key)) obj.name = fm.get_##kind(key);

#define SERIFY_FROM_FIELD_STRUCT(name, NestedType) \
    if (fm.has(#name)) { \
        from_field_map(*fm.get_struct(#name), obj.name); \
    }

#define SERIFY_FROM_FIELD_OPTIONAL(name, kind) \
    if (fm.raw().count(#name) \
        && !std::holds_alternative<std::monostate>(fm.raw().at(#name))) \
        obj.name = fm.get_##kind(#name); \
    else obj.name = std::nullopt;

#define SERIFY_FROM_FIELD_LIST_STRUCT(name, NestedType) \
    if (fm.has(#name)) { \
        for (auto& _i : fm.get_list_struct(#name)) { \
            NestedType _e{}; \
            from_field_map(*_i, _e); \
            obj.name.push_back(std::move(_e)); \
        } \
    }

#define SERIFY_FROM_FIELD_OPTIONAL_STRUCT(name, NestedType) \
    do { \
        auto _o_##name = fm.get_optional_struct(#name); \
        if (_o_##name.has_value() && *_o_##name) { \
            NestedType _v_##name{}; \
            from_field_map(**_o_##name, _v_##name); \
            obj.name = std::move(_v_##name); \
        } else { \
            obj.name = std::nullopt; \
        } \
    } while(0)

#define SERIFY_FROM_FIELD_MAP_SCALAR(name, kind) \
    if (fm.has_map(#name)) { \
        for (auto& [k_,v_] : fm.get_map(#name)) obj.name[k_] = std::get<kind>(v_); \
    }

#define SERIFY_FROM_FIELD_MAP_STRUCT(name, NestedType) \
    if (fm.has_map(#name)) { \
        for (auto& [k_,v_] : fm.get_map(#name)) { \
            from_field_map(*std::get<serify::StructPtr>(v_), obj.name[k_]); \
        } \
    }
