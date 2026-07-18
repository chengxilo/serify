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

//! Byte-level primitives shared by the models in this worker.
//!
//! The Go worker (examples/go) is the --ref language and owns the layout these
//! reproduce; see the layout comment at the top of examples/go/wire.go.

pub fn append_len_str(buf: &mut Vec<u8>, s: &str) {
    buf.extend_from_slice(&(s.len() as u32).to_le_bytes());
    buf.extend_from_slice(s.as_bytes());
}

pub fn read_len_str(data: &[u8], pos: &mut usize) -> Result<String, String> {
    if *pos + 4 > data.len() {
        return Err("truncated".into());
    }
    let n = u32::from_le_bytes(data[*pos..*pos + 4].try_into().unwrap()) as usize;
    *pos += 4;
    if *pos + n > data.len() {
        return Err("truncated".into());
    }
    let s = String::from_utf8(data[*pos..*pos + n].to_vec()).map_err(|e| e.to_string())?;
    *pos += n;
    Ok(s)
}

/// Take the next `$n` bytes of `$data`, advancing the cursor `$p`, and return
/// early with a "truncated" error if they are not there.
macro_rules! read_bytes {
    ($data:expr, $p:ident, $n:expr) => {{
        if $p + $n > $data.len() {
            return Err("truncated".into());
        }
        let s = &$data[$p..$p + $n];
        $p += $n;
        s
    }};
}

pub(crate) use read_bytes;
