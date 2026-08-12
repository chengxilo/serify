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
// This worker registers every type in the suite.

#include "serify.hpp"

#include "customer.hpp"
#include "ledger.hpp"
#include "notification.hpp"
#include "order.hpp"
#include "signals.hpp"
#include "telemetry.hpp"

using namespace serify;

int main() {
    // model_format converts FieldMap <-> model around each pair, so marshal and
    // unmarshal below are the model's own functions, unwrapped.
    SuiteMap suite;
    suite["ledger"]["binary"] = model_format<LedgerEntry>(ledger_marshal, ledger_unmarshal);
    suite["customer"]["binary"] =
        model_format<CustomerRecord>(customer_marshal, customer_unmarshal);
    suite["customer"]["json"] =
        model_format<CustomerRecord>(customer_to_json, customer_from_json);
    suite["order"]["binary"] = model_format<OrderRecord>(order_marshal, order_unmarshal);
    suite["signals"]["binary"] = model_format<SignalCapture>(signals_marshal, signals_unmarshal);
    suite["notification"]["binary"] =
        model_format<NotificationRecord>(notification_marshal, notification_unmarshal);
    suite["telemetry"]["binary"] = model_format<TelemetryFrame>(telemetry_marshal, telemetry_unmarshal);
    run_suite(suite);
    return 0;
}
