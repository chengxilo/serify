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


// The records the server stores and sends. Each mirrors the case file of the
// same name and owns its own byte layout.
//
// SERIFY_TO / SERIFY_FROM are the schema binding: C++ has no reflection, so the
// fields are listed once in each direction and the macros generate the free
// functions to_field_map / from_field_map. An enum needs nothing special — it
// travels as its variant *name*, so `priority` is a plain string and PRIORITIES
// fixes the ordinal this codec writes.

#pragma once

#include "serify.hpp"
#include "wire.hpp"

#include <cstdint>
#include <optional>
#include <string>
#include <vector>

namespace taskstore {

/// Declaration order of the `enum<low, normal, high>` in cases/task.yaml.
inline const std::vector<std::string> PRIORITIES = {"low", "normal", "high"};

/// Declaration order of the enum in cases/api_error.yaml.
inline const std::vector<std::string> ERROR_CODES = {"not_found", "invalid", "conflict"};

/// A stored task: cases/task.yaml.
struct Task {
    uint64_t                 id{};
    std::string              title;
    bool                     done{};
    std::string              priority = "normal";
    std::vector<std::string> tags;
    std::optional<int64_t>   due_at;
};

/// A task the server has not assigned an id to yet: cases/draft.yaml.
struct Draft {
    std::string              title;
    std::string              priority = "normal";
    std::vector<std::string> tags;
    std::optional<int64_t>   due_at;
};

/// The body of a list response: cases/task_page.yaml.
struct TaskPage {
    std::vector<Task> items;
    uint32_t          total{};
};

/// A failed request: cases/api_error.yaml.
struct ApiError {
    std::string code = "not_found";
    std::string message;
};

SERIFY_TO(Task,
    SERIFY_FIELD(id, u64)
    SERIFY_FIELD(title, string)
    SERIFY_FIELD(done, bool)
    SERIFY_FIELD(priority, string)
    SERIFY_FIELD(tags, list_string)
    SERIFY_FIELD_OPTIONAL(due_at, i64)
)
SERIFY_FROM(Task,
    SERIFY_FROM_FIELD(id, u64)
    SERIFY_FROM_FIELD(title, string)
    SERIFY_FROM_FIELD(done, bool)
    SERIFY_FROM_FIELD(priority, string)
    SERIFY_FROM_FIELD(tags, list_string)
    SERIFY_FROM_FIELD_OPTIONAL(due_at, i64)
)

SERIFY_TO(Draft,
    SERIFY_FIELD(title, string)
    SERIFY_FIELD(priority, string)
    SERIFY_FIELD(tags, list_string)
    SERIFY_FIELD_OPTIONAL(due_at, i64)
)
SERIFY_FROM(Draft,
    SERIFY_FROM_FIELD(title, string)
    SERIFY_FROM_FIELD(priority, string)
    SERIFY_FROM_FIELD(tags, list_string)
    SERIFY_FROM_FIELD_OPTIONAL(due_at, i64)
)

SERIFY_TO(TaskPage,
    SERIFY_FIELD_LIST_STRUCT(items, Task);
    SERIFY_FIELD(total, u32)
)
SERIFY_FROM(TaskPage,
    SERIFY_FROM_FIELD_LIST_STRUCT(items, Task);
    SERIFY_FROM_FIELD(total, u32)
)

SERIFY_TO(ApiError,
    SERIFY_FIELD(code, string)
    SERIFY_FIELD(message, string)
)
SERIFY_FROM(ApiError,
    SERIFY_FROM_FIELD(code, string)
    SERIFY_FROM_FIELD(message, string)
)

inline void put_task(std::vector<uint8_t>& out, const Task& t) {
    put_u64(out, t.id);
    put_str(out, t.title);
    out.push_back(t.done ? 1 : 0);
    put_enum(out, PRIORITIES, t.priority);
    put_tags(out, t.tags);
    put_optional_i64(out, t.due_at);
}

inline Task take_task(Reader& r) {
    Task t{};
    t.id       = r.u64();
    t.title    = r.str();
    t.done     = r.boolean();
    t.priority = r.enum_of(PRIORITIES);
    t.tags     = r.tags();
    t.due_at   = r.optional_i64();
    return t;
}

inline void put_draft(std::vector<uint8_t>& out, const Draft& d) {
    put_str(out, d.title);
    put_enum(out, PRIORITIES, d.priority);
    put_tags(out, d.tags);
    put_optional_i64(out, d.due_at);
}

inline Draft take_draft(Reader& r) {
    Draft d{};
    d.title    = r.str();
    d.priority = r.enum_of(PRIORITIES);
    d.tags     = r.tags();
    d.due_at   = r.optional_i64();
    return d;
}

inline void put_page(std::vector<uint8_t>& out, const TaskPage& p) {
    put_u32(out, static_cast<uint32_t>(p.items.size()));
    for (const auto& t : p.items) put_task(out, t);
    put_u32(out, p.total);
}

inline TaskPage take_page(Reader& r) {
    TaskPage p{};
    const uint32_t n = r.u32();
    p.items.reserve(n);
    for (uint32_t i = 0; i < n; ++i) p.items.push_back(take_task(r));
    p.total = r.u32();
    return p;
}

inline void put_error(std::vector<uint8_t>& out, const ApiError& e) {
    put_enum(out, ERROR_CODES, e.code);
    put_str(out, e.message);
}

inline ApiError take_error(Reader& r) {
    ApiError e{};
    e.code    = r.enum_of(ERROR_CODES);
    e.message = r.str();
    return e;
}

}  // namespace taskstore
