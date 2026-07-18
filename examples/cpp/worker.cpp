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

// C++ example worker.
//
// This file is the whole worker: it names the types it can handle, one
// serializer/deserializer pair per format. Everything else lives in the headers
// beside it — those stand in for the types an application already owns, each
// carrying its own schema binding and byte layout.
//
// Types this worker does not register (customer, order, telemetry) are reported
// to the runner as SKIPPED.

#include "serify.hpp"

#include "ledger.hpp"
#include "notification.hpp"
#include "signals.hpp"

#include <cstdint>
#include <vector>

using namespace serify;

// One (serializer, deserializer) pair for a model that carries its own byte
// layout: FieldMap in, model, bytes out — and back.
template <typename Model, std::vector<uint8_t> (*Marshal)(const Model&),
          Model (*Unmarshal)(const std::vector<uint8_t>&)>
static FormatPair binary() {
    return FormatPair{
        [](const FieldMap& fm) {
            Model m{};
            from_field_map(fm, m);
            return Marshal(m);
        },
        [](const std::vector<uint8_t>& data) { return to_field_map(Unmarshal(data)); },
    };
}

int main() {
    SuiteMap suite;
    suite["ledger"]["binary"] = binary<LedgerEntry, ledger_marshal, ledger_unmarshal>();
    suite["signals"]["binary"] = binary<SignalCapture, signals_marshal, signals_unmarshal>();
    suite["notification"]["binary"] =
        binary<NotificationRecord, notification_marshal, notification_unmarshal>();
    run_suite(suite);
    return 0;
}
