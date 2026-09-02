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

//! The conformance worker: serify's entire footprint in the Rust follower.
//!
//! It registers the two types that cross the socket and hands each the very
//! functions client.rs calls — `Request::marshal` is not a test double.
//!
//! There is no converter, and there is no arm list. A serify `sum` is a
//! sum-of-products and so is a Rust enum, so `#[derive(SerifyModel)]` on `Op`
//! and `ApiResult` says everything the binding needs. Compare go/main.go, where
//! the same nine arms are written out twice by hand.

use serify::{run_suite, Format, Suite, Type};
use taskstore::message::{Request, Response};

fn main() {
    run_suite(
        Suite::new()
            .with_type(
                "request",
                Type::new().with_format(
                    "binary",
                    Format::model::<Request>()
                        .serializer(Request::marshal)
                        .deserializer(Request::unmarshal),
                ),
            )
            .with_type(
                "response",
                Type::new().with_format(
                    "binary",
                    Format::model::<Response>()
                        .serializer(Response::marshal)
                        .deserializer(Response::unmarshal),
                ),
            ),
    );
}
