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

//! Rust example worker.
//!
//! This file is the whole worker: it names the types it can handle and, for each
//! format, hands serify a serializer and a deserializer. Everything else lives
//! in the model modules beside it — those stand in for the types an application
//! already owns, each carrying `#[derive(SerifyModel)]` and its own byte layout.
//!
//! The Go worker (examples/go) is the --ref language and owns the byte layout;
//! see the layout comment at the top of examples/go/wire.go.
//!
//! `customer`, `ledger` and `notification` are implemented; `order` and
//! `telemetry` are reported as SKIPPED until this worker registers them.

mod common;
mod customer;
mod ledger;
mod notification;
mod signals;
mod telemetry;
mod wire;

use serify::{run_suite, Format, Suite, Type};

use crate::customer::CustomerRecord;
use crate::ledger::LedgerEntry;
use crate::notification::NotificationRecord;
use crate::signals::SignalCapture;
use crate::telemetry::TelemetryFrame;

fn main() {
    run_suite(
        Suite::new()
            .with_type(
                "customer",
                Type::new()
                    .with_format(
                        "binary",
                        Format::model::<CustomerRecord>()
                            .serializer(CustomerRecord::marshal)
                            .deserializer(CustomerRecord::unmarshal),
                    )
                    .with_format(
                        "json",
                        Format::model::<CustomerRecord>()
                            .serializer(|c| Ok(c.to_json()))
                            .deserializer(CustomerRecord::from_json),
                    ),
            )
            .with_type(
                "ledger",
                Type::new().with_format(
                    "binary",
                    Format::model::<LedgerEntry>()
                        .serializer(LedgerEntry::marshal)
                        .deserializer(LedgerEntry::unmarshal),
                ),
            )
            .with_type(
                "telemetry",
                Type::new().with_format(
                    "binary",
                    Format::model::<TelemetryFrame>()
                        .serializer(TelemetryFrame::marshal)
                        .deserializer(TelemetryFrame::unmarshal),
                ),
            )
            .with_type(
                "signals",
                Type::new().with_format(
                    "binary",
                    Format::model::<SignalCapture>()
                        .serializer(SignalCapture::marshal)
                        .deserializer(SignalCapture::unmarshal),
                ),
            )
            .with_type(
                "notification",
                Type::new().with_format(
                    "binary",
                    Format::model::<NotificationRecord>()
                        .serializer(NotificationRecord::marshal)
                        .deserializer(NotificationRecord::unmarshal),
                ),
            ),
    );
}
