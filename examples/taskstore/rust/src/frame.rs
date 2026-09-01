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

//! Framing: a u32 little-endian byte length, then that many bytes.
//!
//! Outside the conformance suite on purpose. serify tests the contents of a
//! message; the frame header is in none of the cases, so a follower may read
//! its socket however it likes as long as it agrees on what is inside.

use std::io::{Read, Write};

/// Caps a single message, so a bad or hostile length prefix is not an
/// allocation of up to 4 GiB from a header nobody has validated.
pub const MAX_FRAME_LEN: u32 = 1 << 20;

pub fn write_frame(w: &mut impl Write, payload: &[u8]) -> std::io::Result<()> {
    if payload.len() as u32 > MAX_FRAME_LEN {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidInput,
            format!("message of {} bytes exceeds the {MAX_FRAME_LEN} limit", payload.len()),
        ));
    }
    let mut framed = (payload.len() as u32).to_le_bytes().to_vec();
    framed.extend_from_slice(payload);
    w.write_all(&framed)
}

/// Reads one length-prefixed message. `Ok(None)` means the stream ended cleanly
/// between messages — the other side hanging up, not a failure.
pub fn read_frame(r: &mut impl Read) -> std::io::Result<Option<Vec<u8>>> {
    let mut header = [0u8; 4];
    match r.read_exact(&mut header) {
        Ok(()) => {}
        Err(e) if e.kind() == std::io::ErrorKind::UnexpectedEof => return Ok(None),
        Err(e) => return Err(e),
    }
    let n = u32::from_le_bytes(header);
    if n > MAX_FRAME_LEN {
        return Err(std::io::Error::new(
            std::io::ErrorKind::InvalidData,
            format!("frame header claims {n} bytes, over the {MAX_FRAME_LEN} limit"),
        ));
    }
    let mut payload = vec![0u8; n as usize];
    r.read_exact(&mut payload)?;
    Ok(Some(payload))
}
