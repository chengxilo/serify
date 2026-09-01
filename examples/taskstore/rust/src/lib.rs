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

//! The taskstore wire format, in Rust.
//!
//! Go leads: the layouts here were read off go/api/wire.go and reproduced,
//! which is the position a second language is in on a real project. The module
//! split mirrors the Go worker's `api/` package file for file.

pub mod frame;
pub mod message;
pub mod model;
pub mod op;
pub mod result;
pub mod wire;
