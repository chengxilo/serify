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


// The two sums and the two messages that carry them.
//
// std::variant is C++'s sum type and it supplies the arms; the one thing it
// cannot supply is their names, because C++ has no reflection. SERIFY_SUM names
// them, and that single line is the whole binding.
//
// Each alternative *is* its payload, so no wrapper structs are needed:
// std::monostate is the unit variant, uint64_t is the scalar payload, and Draft
// and Task are struct payloads.
//
// Note that Op holds uint64_t twice — `read` and `delete` both carry an id and
// nothing else. A std::variant may repeat an alternative type; what it may not
// do is let you reach one by type, so this file uses std::get<I> throughout.
// That reads awkwardly at first and is then exactly right: the index *is* the
// tag ordinal, so the codec below needs no lookup table at all, and the two ids
// are told apart by position in precisely the way the schema tells them apart
// by tag.

#pragma once

#include "model.hpp"
#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <stdexcept>
#include <variant>
#include <vector>

namespace taskstore {

/// The operation a request asks for: the `sum` in cases/op.yaml.
using Op = std::variant<
    std::monostate,  // list_all — arity 0, a unit variant
    Draft,           // create   — arity N, a struct payload
    uint64_t,        // read     — arity 1, the id
    Task,            // update   — arity N, the whole task
    uint64_t         // delete   — arity 1, the id
>;

SERIFY_SUM(Op, "list_all", "create", "read", "update", "delete")

/// What the server answers with: the `sum` in cases/result.yaml.
using ApiResult = std::variant<
    std::monostate,  // accepted — nothing to send back
    Task,            // found
    TaskPage,        // listing
    ApiError         // failed
>;

SERIFY_SUM(ApiResult, "accepted", "found", "listing", "failed")

/// One request frame: cases/request.yaml.
struct Request {
    uint32_t request_id{};
    Op       op;
};

/// One response frame: cases/response.yaml. request_id echoes the request's, so
/// a client with several in flight can tell them apart.
struct Response {
    uint32_t  request_id{};
    ApiResult result;
};

SERIFY_TO(Request,
    SERIFY_FIELD(request_id, u32)
    SERIFY_FIELD_SUM(op, Op)
)
SERIFY_FROM(Request,
    SERIFY_FROM_FIELD(request_id, u32)
    SERIFY_FROM_FIELD_SUM(op, Op)
)

SERIFY_TO(Response,
    SERIFY_FIELD(request_id, u32)
    SERIFY_FIELD_SUM(result, ApiResult)
)
SERIFY_FROM(Response,
    SERIFY_FROM_FIELD(request_id, u32)
    SERIFY_FROM_FIELD_SUM(result, ApiResult)
)

inline std::vector<uint8_t> request_marshal(const Request& req) {
    std::vector<uint8_t> out;
    put_u32(out, req.request_id);

    // The tag ordinal is the alternative's index in the variant above, which is
    // the declaration order of cases/op.yaml.
    out.push_back(static_cast<uint8_t>(req.op.index()));
    switch (req.op.index()) {
        case 0: break;  // a unit variant is nothing but its tag
        case 1: put_draft(out, std::get<1>(req.op)); break;
        case 2: put_u64(out, std::get<2>(req.op)); break;
        case 3: put_task(out, std::get<3>(req.op)); break;
        case 4: put_u64(out, std::get<4>(req.op)); break;
        default: throw std::runtime_error("unhandled op alternative");
    }
    return out;
}

inline Request request_unmarshal(const std::vector<uint8_t>& data) {
    Reader r(data);
    Request req{};
    req.request_id = r.u32();

    switch (const uint8_t tag = r.u8(); tag) {
        case 0: req.op.emplace<0>(); break;
        case 1: req.op.emplace<1>(take_draft(r)); break;
        case 2: req.op.emplace<2>(r.u64()); break;
        case 3: req.op.emplace<3>(take_task(r)); break;
        case 4: req.op.emplace<4>(r.u64()); break;
        default: throw std::runtime_error("unknown op tag " + std::to_string(tag));
    }
    return req;
}

inline std::vector<uint8_t> response_marshal(const Response& resp) {
    std::vector<uint8_t> out;
    put_u32(out, resp.request_id);

    out.push_back(static_cast<uint8_t>(resp.result.index()));
    switch (resp.result.index()) {
        case 0: break;
        case 1: put_task(out, std::get<1>(resp.result)); break;
        case 2: put_page(out, std::get<2>(resp.result)); break;
        case 3: put_error(out, std::get<3>(resp.result)); break;
        default: throw std::runtime_error("unhandled result alternative");
    }
    return out;
}

inline Response response_unmarshal(const std::vector<uint8_t>& data) {
    Reader r(data);
    Response resp{};
    resp.request_id = r.u32();

    switch (const uint8_t tag = r.u8(); tag) {
        case 0: resp.result.emplace<0>(); break;
        case 1: resp.result.emplace<1>(take_task(r)); break;
        case 2: resp.result.emplace<2>(take_page(r)); break;
        case 3: resp.result.emplace<3>(take_error(r)); break;
        default: throw std::runtime_error("unknown result tag " + std::to_string(tag));
    }
    return resp;
}

}  // namespace taskstore
