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


// The conformance worker: serify's entire footprint in the C++ follower.
//
// It registers the two types that cross the socket and hands each the very
// functions client.cpp calls — request_marshal is not a test double.

#include "serify.hpp"

#include "message.hpp"

using namespace serify;

int main() {
    // model_format converts FieldMap <-> model around each pair, so the two
    // functions below are the codec's own, unwrapped.
    SuiteMap suite;
    suite["request"]["binary"] =
        model_format<taskstore::Request>(taskstore::request_marshal, taskstore::request_unmarshal);
    suite["response"]["binary"] =
        model_format<taskstore::Response>(taskstore::response_marshal, taskstore::response_unmarshal);
    run_suite(suite);
    return 0;
}
