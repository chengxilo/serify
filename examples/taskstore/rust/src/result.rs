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

//! What the server answers with: the `sum` in cases/result.yaml.

use serify::SerifyModel;

use crate::model::{ApiError, Task, TaskPage};
use crate::wire::Reader;

/// The arms are named for the outcome rather than the operation, because
/// several operations share one: create, read and update all answer `Found`.
///
/// The enum is `ApiResult` rather than `Result` only to leave that name to
/// `std`; the schema tags come from the *variant* names, so the type's own name
/// never reaches the wire.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub enum ApiResult {
    Accepted,          // a delete that worked: nothing to send back
    Found(Task),       // the task as it now stands
    Listing(TaskPage), // a page of tasks plus the total
    Failed(ApiError),  // the request did not work
}

impl ApiResult {
    pub fn put(&self, buf: &mut Vec<u8>) -> Result<(), String> {
        match self {
            ApiResult::Accepted => buf.push(0),
            ApiResult::Found(t) => {
                buf.push(1);
                t.put(buf)?;
            }
            ApiResult::Listing(p) => {
                buf.push(2);
                p.put(buf)?;
            }
            ApiResult::Failed(e) => {
                buf.push(3);
                e.put(buf)?;
            }
        }
        Ok(())
    }

    pub fn read(r: &mut Reader) -> Result<Self, String> {
        match r.u8()? {
            0 => Ok(ApiResult::Accepted),
            1 => Ok(ApiResult::Found(Task::read(r)?)),
            2 => Ok(ApiResult::Listing(TaskPage::read(r)?)),
            3 => Ok(ApiResult::Failed(ApiError::read(r)?)),
            tag => Err(format!("unknown result tag {tag}")),
        }
    }
}
