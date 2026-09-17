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


//! The Rust half of the audit example.
//!
//! Go leads and owns the byte layout; see the comment at the top of
//! examples/audit/go/wire.go. This worker reproduces it under two of the
//! suite's three formats — see the note on `handoff` at the bottom of this file
//! for the third, and README.md for the walkthrough.
//!
//! The codecs speak the `Frame` model and never see a `FieldMap`. Audit sees
//! through it: `ModelFormat` keeps the model instance each call used and
//! re-derives its state at every probe point.

use serify::{run_suite, Format, SerifyModel, Suite, Type};

/// The model, mirroring examples/audit/go/codec.go's `Frame`.
#[derive(SerifyModel)]
pub struct Frame {
    stream_id: u32,
    title: String,
    tags: Vec<String>,
    payload: Vec<u8>,
}

// --- shared encoder ---------------------------------------------------------

fn marshal_frame(f: &Frame) -> Result<Vec<u8>, String> {
    let mut buf = Vec::new();
    buf.extend_from_slice(&f.stream_id.to_le_bytes());
    put_str(&mut buf, &f.title);
    buf.extend_from_slice(&(f.tags.len() as u32).to_le_bytes());
    for t in &f.tags {
        put_str(&mut buf, t);
    }
    buf.extend_from_slice(&(f.payload.len() as u32).to_le_bytes());
    buf.extend_from_slice(&f.payload);
    Ok(buf)
}

fn put_str(buf: &mut Vec<u8>, s: &str) {
    buf.extend_from_slice(&(s.len() as u32).to_le_bytes());
    buf.extend_from_slice(s.as_bytes());
}

/// Read a u32 length at `pos` and return it with the offset of the body, having
/// checked that the body is actually there.
fn read_len(data: &[u8], pos: usize) -> Result<(usize, usize), String> {
    if pos + 4 > data.len() {
        return Err("truncated".into());
    }
    let n = u32::from_le_bytes(data[pos..pos + 4].try_into().unwrap()) as usize;
    let body = pos + 4;
    if body + n > data.len() {
        return Err("truncated".into());
    }
    Ok((n, body))
}

// --- format: safe -----------------------------------------------------------

/// Copies every field out of the input buffer. The decoded frame owns its
/// memory and outlives the buffer it came from.
fn unmarshal_safe(data: &[u8]) -> Result<Frame, String> {
    decode(data, false)
}

// --- format: fast -----------------------------------------------------------

/// Points every string and the payload straight at the input buffer instead of
/// copying. On a hot path that is a real optimization: this decoder does no
/// allocation for the frame's contents at all.
///
/// It also quietly changes the contract. The returned frame is only valid while
/// `data` is unmodified and alive, so a caller that reuses a read buffer now
/// corrupts frames that already looked decoded. Nothing about the bytes says
/// so, which is why audit exists.
fn unmarshal_fast(data: &[u8]) -> Result<Frame, String> {
    decode(data, true)
}

/// Walks the layout once; `alias` picks copying or aliasing for the three fields
/// that can be either, so the two formats visibly agree about the bytes.
///
/// The aliasing arms build a `String`/`Vec` over memory they do not own, with
/// capacity 0 so that dropping them frees nothing — correct only while the
/// caller keeps its promise about the buffer.
fn decode(data: &[u8], alias: bool) -> Result<Frame, String> {
    if data.len() < 4 {
        return Err("truncated".into());
    }
    let stream_id = u32::from_le_bytes(data[0..4].try_into().unwrap());
    let mut pos = 4usize;

    let title = take_string(data, &mut pos, alias)?;

    if pos + 4 > data.len() {
        return Err("truncated".into());
    }
    let count = u32::from_le_bytes(data[pos..pos + 4].try_into().unwrap()) as usize;
    pos += 4;
    let mut tags = Vec::with_capacity(count.min(1024));
    for _ in 0..count {
        tags.push(take_string(data, &mut pos, alias)?);
    }

    let (n, body) = read_len(data, pos)?;
    let payload: Vec<u8> = if n == 0 {
        Vec::new()
    } else if alias {
        unsafe { Vec::from_raw_parts(data.as_ptr().add(body) as *mut u8, n, 0) }
    } else {
        data[body..body + n].to_vec()
    };
    Ok(Frame {
        stream_id,
        title,
        tags,
        payload,
    })
}

/// Reads one length-prefixed string, either copying it or aliasing the buffer.
///
/// The zero-length guard is load-bearing: an empty string has nothing to alias,
/// which is why the `empty` case reports no finding.
fn take_string(data: &[u8], pos: &mut usize, alias: bool) -> Result<String, String> {
    let (n, body) = read_len(data, *pos)?;
    *pos = body + n;
    if n == 0 {
        return Ok(String::new());
    }
    if alias {
        return Ok(unsafe { String::from_raw_parts(data.as_ptr().add(body) as *mut u8, n, 0) });
    }
    String::from_utf8(data[body..body + n].to_vec()).map_err(|e| e.to_string())
}

// --- format: handoff, which this worker does not implement ------------------
//
// The Go worker's `handoff` serializer writes the frame and then scrubs the
// payload buffer it was handed — a plausible "the bytes are out, release the
// buffer" optimization that silently edits the caller's data.
//
// It cannot be written here: a serify serializer in Rust receives `&Frame`, so
// the bug needs an `unsafe` `&`→`&mut` cast (which is what the meta-fixture in
// test/cases/audit/rust does). This worker declines the format instead, and
// serify reports SKIPPED for `handoff`/rust. The test in ../test asserts that
// exact cell, so the skip cannot spread to a format this worker does implement.
//
// Go hands the serializer a `*Frame`, so the same bug is one accepted line.

fn main() {
    run_suite(
        Suite::new().with_type(
            "frame",
            Type::new()
                .with_format(
                    "safe",
                    Format::model::<Frame>()
                        .serializer(marshal_frame)
                        .deserializer(unmarshal_safe),
                )
                .with_format(
                    "fast",
                    Format::model::<Frame>()
                        .serializer(marshal_frame)
                        .deserializer(unmarshal_fast),
                ),
            // `handoff` is deliberately absent — see the note above.
        ),
    );
}
