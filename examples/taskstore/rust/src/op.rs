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

//! The operation a request asks for: the `sum` in cases/op.yaml.

use serify::SerifyModel;

use crate::model::{Draft, Task};
use crate::wire::Reader;

/// Rust has a native sum type, so `sum` maps straight onto an enum and the same
/// derive handles it: variant names become schema tags in snake_case, and a
/// request carrying two operations at once is unwritable.
///
/// This is the cheapest of the nine bindings. Go needs a hand-written converter
/// for the same five arms, because implementations of an interface cannot be
/// enumerated at run time — compare go/main.go.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub enum Op {
    ListAll,       // arity 0 — a unit variant, no payload
    Create(Draft), // arity N — a struct payload
    Read(u64),     // arity 1 — a scalar payload: the id
    Update(Task),  // arity N — the whole task, id included
    Delete(u64),
}

impl Op {
    pub fn put(&self, buf: &mut Vec<u8>) -> Result<(), String> {
        // The tag ordinal is the variant's position in the case file's sum,
        // which is this enum's declaration order. `match` is exhaustive, so a
        // sixth operation is a compile error here rather than a runtime
        // surprise — the one thing Go's sealed interface cannot give.
        match self {
            Op::ListAll => buf.push(0), // a unit variant is nothing but its tag
            Op::Create(d) => {
                buf.push(1);
                d.put(buf)?;
            }
            Op::Read(id) => {
                buf.push(2);
                buf.extend_from_slice(&id.to_le_bytes());
            }
            Op::Update(t) => {
                buf.push(3);
                t.put(buf)?;
            }
            Op::Delete(id) => {
                buf.push(4);
                buf.extend_from_slice(&id.to_le_bytes());
            }
        }
        Ok(())
    }

    pub fn read(r: &mut Reader) -> Result<Self, String> {
        match r.u8()? {
            0 => Ok(Op::ListAll),
            1 => Ok(Op::Create(Draft::read(r)?)),
            2 => Ok(Op::Read(r.u64()?)),
            3 => Ok(Op::Update(Task::read(r)?)),
            4 => Ok(Op::Delete(r.u64()?)),
            tag => Err(format!("unknown op tag {tag}")),
        }
    }
}
