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

//! The two reusable models — examples/cases/address.yaml and
//! examples/cases/money.yaml — shared by more than one type in the suite.
//!
//! `#[derive(SerifyModel)]` is the whole binding: the derive maps each field
//! onto its FieldMap slot by name, and nesting one of these inside another model
//! needs nothing further.

use serify::SerifyModel;

#[derive(SerifyModel)]
pub struct Address {
    pub recipient: String,
    pub street: String,
    pub city: String,
    pub country: String,
    pub postal_code: String,
}

#[derive(SerifyModel)]
pub struct Money {
    pub currency: String,
    pub amount_minor: i64,
}
