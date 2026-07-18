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

//! The Rust half of the `audit` meta-test. Provides eight formats (value-mutating
//! is intentionally omitted to exercise the bind-SKIPPED path):
//!
//!   clean            – correct round-trip (control group, no warnings)
//!   mutating         – serializer zeroes Value via unsafe mutation
//!   zero-copy        – deserializer aliases Payload from the input buffer
//!   list-zero-copy   – deserializer aliases Tags via String::from_raw_parts
//!   unstable         – serializer appends counter → non-deterministic output
//!   deser-unstable   – deserializer adds 1 on second call
//!   input-mutating   – deserializer modifies input buffer after parsing
//!   output-zero-copy – serializer returns sub-slice aliasing payload

use serify::{FieldMap, Format, Suite, Type, run_suite};
use std::sync::atomic::{AtomicU8, Ordering};

// --- common binary helpers -------------------------------------------------

fn marshal_inner(fm: &FieldMap) -> Result<Vec<u8>, String> {
    let value   = fm.get_u32("value").ok_or("missing value")?;
    let tag     = fm.get_string("tag").ok_or("missing tag")?;
    let payload = fm.get_bytes("payload").ok_or("missing payload")?;
    let tags    = fm.get_list_string("tags").ok_or("missing tags")?;

    let mut buf = Vec::new();
    buf.extend_from_slice(&value.to_le_bytes());
    buf.push(tag.len() as u8);
    buf.extend_from_slice(tag.as_bytes());
    buf.extend_from_slice(&(payload.len() as u32).to_le_bytes());
    buf.extend_from_slice(payload);
    buf.push(tags.len() as u8);
    for t in tags {
        buf.push(t.len() as u8);
        buf.extend_from_slice(t.as_bytes());
    }
    Ok(buf)
}

fn unmarshal_inner(data: &[u8], copy_payload: bool) -> Result<FieldMap, String> {
    if data.len() < 5 { return Err("truncated".into()); }

    let mut pos = 0usize;
    let value = u32::from_le_bytes(data[pos..pos+4].try_into().unwrap());
    pos += 4;

    let tag_len = data[pos] as usize;
    pos += 1;
    if pos + tag_len > data.len() { return Err("truncated".into()); }
    let tag = String::from_utf8(data[pos..pos+tag_len].to_vec())
        .map_err(|e| e.to_string())?;
    pos += tag_len;

    if pos + 4 > data.len() { return Err("truncated".into()); }
    let payload_len = u32::from_le_bytes(data[pos..pos+4].try_into().unwrap()) as usize;
    pos += 4;
    if pos + payload_len > data.len() { return Err("truncated".into()); }

    let payload: Vec<u8> = if copy_payload {
        data[pos..pos+payload_len].to_vec()
    } else {
        unsafe {
            Vec::from_raw_parts(
                data.as_ptr().add(pos) as *mut u8,
                payload_len,
                0,
            )
        }
    };
    pos += payload_len;

    if pos >= data.len() { return Err("truncated".into()); }
    let tags_count = data[pos] as usize;
    pos += 1;
    let mut tags: Vec<String> = Vec::with_capacity(tags_count);
    for _ in 0..tags_count {
        if pos >= data.len() { return Err("truncated".into()); }
        let tl = data[pos] as usize;
        pos += 1;
        if pos + tl > data.len() { return Err("truncated".into()); }
        tags.push(String::from_utf8(data[pos..pos+tl].to_vec())
            .map_err(|e| e.to_string())?);
        pos += tl;
    }

    let mut fm = FieldMap::new();
    fm.set_u32("value", value);
    fm.set_string("tag", tag);
    fm.set_bytes("payload", payload);
    fm.set_list_string("tags", tags);
    Ok(fm)
}

// --- format: clean ----------------------------------------------------------

fn marshal_clean(fm: &FieldMap) -> Result<Vec<u8>, String> {
    marshal_inner(fm)
}

fn unmarshal_clean(data: &[u8]) -> Result<FieldMap, String> {
    unmarshal_inner(data, true)
}

// --- format: mutating -------------------------------------------------------

#[allow(invalid_reference_casting)]
unsafe fn mutate_fm_set_value_zero(fm: &FieldMap) {
    let fm_mut = &mut *((fm as *const FieldMap) as *mut FieldMap);
    fm_mut.set_u32("value", 0);
}

fn marshal_mutating(fm: &FieldMap) -> Result<Vec<u8>, String> {
    let data = marshal_inner(fm)?;
    unsafe { mutate_fm_set_value_zero(fm); }
    Ok(data)
}

// NOTE: value-mutating is intentionally NOT registered — the Rust worker
// returns SKIP for it, exercising the runner's bind-SKIPPED handling.

// --- format: zero-copy ------------------------------------------------------

fn unmarshal_zero_copy(data: &[u8]) -> Result<FieldMap, String> {
    unmarshal_inner(data, false)
}

// --- format: list-zero-copy -------------------------------------------------

fn unmarshal_list_zero_copy(data: &[u8]) -> Result<FieldMap, String> {
    // Parse with copies first, then replace tag strings with aliases.
    if data.len() < 5 { return Err("truncated".into()); }

    let mut pos = 0usize;
    let value = u32::from_le_bytes(data[pos..pos+4].try_into().unwrap());
    pos += 4;

    let tag_len = data[pos] as usize;
    pos += 1;
    if pos + tag_len > data.len() { return Err("truncated".into()); }
    let tag = String::from_utf8(data[pos..pos+tag_len].to_vec())
        .map_err(|e| e.to_string())?;
    pos += tag_len;

    if pos + 4 > data.len() { return Err("truncated".into()); }
    let payload_len = u32::from_le_bytes(data[pos..pos+4].try_into().unwrap()) as usize;
    pos += 4;
    if pos + payload_len > data.len() { return Err("truncated".into()); }
    let payload = data[pos..pos+payload_len].to_vec();
    pos += payload_len;

    if pos >= data.len() { return Err("truncated".into()); }
    let tags_count = data[pos] as usize;
    pos += 1;
    let mut tags: Vec<String> = Vec::with_capacity(tags_count);
    for _ in 0..tags_count {
        if pos >= data.len() { return Err("truncated".into()); }
        let tl = data[pos] as usize;
        pos += 1;
        if pos + tl > data.len() { return Err("truncated".into()); }
        // Zero-copy alias via from_raw_parts (cap=0 so Drop is a no-op).
        let s = unsafe {
            String::from_raw_parts(data.as_ptr().add(pos) as *mut u8, tl, 0)
        };
        tags.push(s);
        pos += tl;
    }

    let mut fm = FieldMap::new();
    fm.set_u32("value", value);
    fm.set_string("tag", tag);
    fm.set_bytes("payload", payload);
    fm.set_list_string("tags", tags);
    Ok(fm)
}

// --- format: unstable -------------------------------------------------------

static UNSTABLE_COUNTER: AtomicU8 = AtomicU8::new(0);

fn marshal_unstable(fm: &FieldMap) -> Result<Vec<u8>, String> {
    let mut data = marshal_inner(fm)?;
    let c = UNSTABLE_COUNTER.fetch_add(1, Ordering::SeqCst);
    data.push(c);
    Ok(data)
}

// --- format: deser-unstable -------------------------------------------------

static DESER_UNSTABLE_COUNTER: AtomicU8 = AtomicU8::new(0);

fn unmarshal_deser_unstable(data: &[u8]) -> Result<FieldMap, String> {
    let mut fm = unmarshal_inner(data, true)?;
    let c = DESER_UNSTABLE_COUNTER.fetch_add(1, Ordering::SeqCst);
    if c > 0 {
        let v = fm.get_u32("value").unwrap_or(0);
        fm.set_u32("value", v + 1);
    }
    Ok(fm)
}

// --- format: input-mutating -------------------------------------------------

fn unmarshal_input_mutating(data: &[u8]) -> Result<FieldMap, String> {
    let fm = unmarshal_inner(data, true)?;
    // Modify the input buffer after parsing. Use write_volatile to prevent
    // the compiler from optimizing away the mutation through the shared ref.
    if !data.is_empty() {
        unsafe {
            let ptr = data.as_ptr() as *mut u8;
            std::ptr::write_volatile(ptr, data[0] ^ 0xFF);
        }
    }
    Ok(fm)
}

// --- format: output-zero-copy -----------------------------------------------

fn marshal_output_zero_copy(fm: &FieldMap) -> Result<Vec<u8>, String> {
    let mut buf = marshal_inner(fm)?;
    // Make fm's internal "payload" field alias a sub-slice of buf.
    // When audit XOR-flips buf, the payload field changes and
    // detect_output_zero_copy reports it.
    let tag = fm.get_string("tag").ok_or("missing tag")?;
    let payload_off = 4 + 1 + tag.len() + 4; // value + tagLen + tag + payloadLen
    let payload_len = fm.get_bytes("payload").ok_or("missing payload")?.len();
    #[allow(invalid_reference_casting)]
    unsafe {
        let fm_mut = &mut *((fm as *const FieldMap) as *mut FieldMap);
        let aliasing = Vec::from_raw_parts(buf.as_mut_ptr().add(payload_off), payload_len, 0);
        fm_mut.set_bytes("payload", aliasing);
    }
    Ok(buf) // return FULL buffer for cross-language comparison
}

fn main() {
    run_suite(Suite::new().with_type("audit",
        Type::new()
            .with_format("clean", Format::new()
                .serializer(marshal_clean)
                .deserializer(unmarshal_clean),
            )
            .with_format("mutating", Format::new()
                .serializer(marshal_mutating)
                .deserializer(unmarshal_clean),
            )
            // value-mutating intentionally omitted → SKIP
            .with_format("zero-copy", Format::new()
                .serializer(marshal_clean)
                .deserializer(unmarshal_zero_copy),
            )
            .with_format("list-zero-copy", Format::new()
                .serializer(marshal_clean)
                .deserializer(unmarshal_list_zero_copy),
            )
            .with_format("unstable", Format::new()
                .serializer(marshal_unstable)
                .deserializer(unmarshal_clean),
            )
            .with_format("deser-unstable", Format::new()
                .serializer(marshal_clean)
                .deserializer(unmarshal_deser_unstable),
            )
            .with_format("input-mutating", Format::new()
                .serializer(marshal_clean)
                .deserializer(unmarshal_input_mutating),
            )
            .with_format("output-zero-copy", Format::new()
                .serializer(marshal_output_zero_copy)
                .deserializer(unmarshal_clean),
            ),
    ));
}
