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

//! The two types that cross the socket, and the only two the suite tests.

use serify::SerifyModel;

use crate::op::Op;
use crate::result::ApiResult;
use crate::wire::Reader;

/// One request frame: cases/request.yaml.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub struct Request {
    pub request_id: u32,
    pub op: Op,
}

/// One response frame: cases/response.yaml. `request_id` echoes the request's,
/// so a client with several in flight can tell them apart.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub struct Response {
    pub request_id: u32,
    pub result: ApiResult,
}

impl Request {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = self.request_id.to_le_bytes().to_vec();
        self.op.put(&mut buf)?;
        Ok(buf)
    }

    pub fn unmarshal(data: &[u8]) -> Result<Self, String> {
        let mut r = Reader::new(data);
        Ok(Self {
            request_id: r.u32()?,
            op: Op::read(&mut r)?,
        })
    }
}

impl Response {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = self.request_id.to_le_bytes().to_vec();
        self.result.put(&mut buf)?;
        Ok(buf)
    }

    pub fn unmarshal(data: &[u8]) -> Result<Self, String> {
        let mut r = Reader::new(data);
        Ok(Self {
            request_id: r.u32()?,
            result: ApiResult::read(&mut r)?,
        })
    }
}
