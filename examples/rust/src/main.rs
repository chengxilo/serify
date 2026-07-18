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

use serify::{run_suite, Format, SerifyModel, Suite, Type};

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
                        Format::new()
                            .serializer(|fm| CustomerRecord::from_field_map(fm)?.marshal())
                            .deserializer(|data| {
                                CustomerRecord::unmarshal(data).map(|c| c.to_field_map())
                            }),
                    )
                    .with_format(
                        "json",
                        Format::new()
                            .serializer(|fm| Ok(CustomerRecord::from_field_map(fm)?.to_json()))
                            .deserializer(|data| {
                                CustomerRecord::from_json(data).map(|c| c.to_field_map())
                            }),
                    ),
            )
            .with_type(
                "ledger",
                Type::new().with_format(
                    "binary",
                    Format::new()
                        .serializer(|fm| LedgerEntry::from_field_map(fm)?.marshal())
                        .deserializer(|data| {
                            LedgerEntry::unmarshal(data).map(|l| l.to_field_map())
                        }),
                ),
            )
            .with_type(
                "telemetry",
                Type::new().with_format(
                    "binary",
                    Format::new()
                        .serializer(|fm| TelemetryFrame::from_field_map(fm)?.marshal())
                        .deserializer(|data| {
                            TelemetryFrame::unmarshal(data).map(|t| t.to_field_map())
                        }),
                ),
            )
            .with_type(
                "signals",
                Type::new().with_format(
                    "binary",
                    Format::new()
                        .serializer(|fm| SignalCapture::from_field_map(fm)?.marshal())
                        .deserializer(|data| {
                            SignalCapture::unmarshal(data).map(|s| s.to_field_map())
                        }),
                ),
            )
            .with_type(
                "notification",
                Type::new().with_format(
                    "binary",
                    Format::new()
                        .serializer(|fm| NotificationRecord::from_field_map(fm)?.marshal())
                        .deserializer(|data| {
                            NotificationRecord::unmarshal(data).map(|n| n.to_field_map())
                        }),
                ),
            ),
    );
}
