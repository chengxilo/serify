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

//! `SignalCapture` mirrors examples/cases/signals.yaml, which uses every scalar
//! the schema allows as a list element. Rust needs no special handling for any
//! of them: each field is the obvious `Vec<T>`, and `#[derive(SerifyModel)]`
//! maps it. `list<uint8>` and `bytes` are both `Vec<u8>` here, which is why the
//! library carries them as the same FieldMap value.
//!
//! Go is the --ref language and owns the byte layout; see examples/go/wire.go.
//! Each list is a u32 element count followed by its elements, little-endian.

use serify::SerifyModel;

use crate::wire::{append_len_str, read_bytes, read_len_str};

/// The `mode` enum from signals.yaml. An enum field maps straight onto a Rust
/// enum: the default `#[derive(SerifyModel)]` carries it as a payload-less
/// Variant — the same shape a `oneof` uses — so no `str`/`repr` marker is
/// needed. The wire form is a u8 ordinal in declaration order; the byte layout
/// (Go's to own) is mapped by hand in marshal/unmarshal below.
#[derive(SerifyModel, Clone, Copy, PartialEq)]
enum Mode {
    Idle,
    Active,
    Fault,
    Calibrating,
}

#[derive(SerifyModel)]
pub struct SignalCapture {
    capture_id: u64,
    flags: Vec<bool>,
    raw_frame: Vec<u8>,
    port_numbers: Vec<u16>,
    sample_counts: Vec<u32>,
    byte_totals: Vec<u64>,
    trim_offsets: Vec<i8>,
    drift_deltas: Vec<i16>,
    temperatures_c: Vec<i32>,
    timestamps_ns: Vec<i64>,
    counters: Vec<u128>,
    balances: Vec<i128>,
    gains: Vec<f32>,
    voltages: Vec<f64>,
    channel_names: Vec<String>,
    payloads: Vec<Vec<u8>>,
    checksum: [u8; 4],
    window: [i16; 3],
    dropped_frames: Option<u32>,
    mode: Mode,
}

/// Write a list's u32 element count. Every list carries one, even when empty.
fn append_count(buf: &mut Vec<u8>, n: usize) {
    buf.extend_from_slice(&(n as u32).to_le_bytes());
}

/// Read a list's u32 element count.
fn read_count(data: &[u8], p: &mut usize) -> Result<usize, String> {
    if *p + 4 > data.len() {
        return Err("truncated".into());
    }
    let n = u32::from_le_bytes(data[*p..*p + 4].try_into().unwrap()) as usize;
    *p += 4;
    Ok(n)
}

/// Emit `count`, then each element through `f`.
macro_rules! write_list {
    ($buf:expr, $items:expr, $f:expr) => {{
        append_count($buf, $items.len());
        for x in $items.iter() {
            $f($buf, x);
        }
    }};
}

/// Read a count, then that many fixed-width elements of `$n` bytes each.
macro_rules! read_list {
    ($data:expr, $p:ident, $n:expr, $conv:expr) => {{
        let count = read_count($data, &mut $p)?;
        let mut out = Vec::with_capacity(count);
        for _ in 0..count {
            let b = read_bytes!($data, $p, $n);
            out.push($conv(b.try_into().unwrap()));
        }
        out
    }};
}

impl SignalCapture {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = Vec::new();
        buf.extend_from_slice(&self.capture_id.to_le_bytes());

        write_list!(&mut buf, self.flags, |b: &mut Vec<u8>, x: &bool| b
            .push(*x as u8));
        write_list!(&mut buf, self.raw_frame, |b: &mut Vec<u8>, x: &u8| b
            .push(*x));
        write_list!(&mut buf, self.port_numbers, |b: &mut Vec<u8>, x: &u16| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.sample_counts, |b: &mut Vec<u8>, x: &u32| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.byte_totals, |b: &mut Vec<u8>, x: &u64| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.trim_offsets, |b: &mut Vec<u8>, x: &i8| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.drift_deltas, |b: &mut Vec<u8>, x: &i16| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.temperatures_c, |b: &mut Vec<u8>, x: &i32| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.timestamps_ns, |b: &mut Vec<u8>, x: &i64| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.counters, |b: &mut Vec<u8>, x: &u128| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.balances, |b: &mut Vec<u8>, x: &i128| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.gains, |b: &mut Vec<u8>, x: &f32| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(&mut buf, self.voltages, |b: &mut Vec<u8>, x: &f64| b
            .extend_from_slice(&x.to_le_bytes()));
        write_list!(
            &mut buf,
            self.channel_names,
            |b: &mut Vec<u8>, x: &String| append_len_str(b, x)
        );
        write_list!(&mut buf, self.payloads, |b: &mut Vec<u8>, x: &Vec<u8>| {
            b.extend_from_slice(&(x.len() as u32).to_le_bytes());
            b.extend_from_slice(x);
        });

        // array<T,N> carries no count: N is fixed by the schema.
        buf.extend_from_slice(&self.checksum);
        for v in self.window.iter() {
            buf.extend_from_slice(&v.to_le_bytes());
        }

        // optional<uint32>: a presence flag, then the value if present.
        match self.dropped_frames {
            None => buf.push(0),
            Some(v) => {
                buf.push(1);
                buf.extend_from_slice(&v.to_le_bytes());
            }
        }

        // enum: a u8 ordinal, the variant's position in the case file.
        let ord: u8 = match self.mode {
            Mode::Idle => 0,
            Mode::Active => 1,
            Mode::Fault => 2,
            Mode::Calibrating => 3,
        };
        buf.push(ord);

        Ok(buf)
    }

    // read_bytes! advances the cursor after every field, including the last one,
    // where nothing reads it again. Keeping the macro uniform is worth more than
    // special-casing the final field.
    #[allow(unused_assignments)]
    pub fn unmarshal(data: &[u8]) -> Result<Self, String> {
        let mut p = 0usize;
        let capture_id = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());

        let flag_count = read_count(data, &mut p)?;
        let mut flags = Vec::with_capacity(flag_count);
        for _ in 0..flag_count {
            flags.push(read_bytes!(data, p, 1)[0] != 0);
        }

        let raw_count = read_count(data, &mut p)?;
        let raw_frame = read_bytes!(data, p, raw_count).to_vec();

        let port_numbers = read_list!(data, p, 2, u16::from_le_bytes);
        let sample_counts = read_list!(data, p, 4, u32::from_le_bytes);
        let byte_totals = read_list!(data, p, 8, u64::from_le_bytes);
        let trim_offsets = read_list!(data, p, 1, i8::from_le_bytes);
        let drift_deltas = read_list!(data, p, 2, i16::from_le_bytes);
        let temperatures_c = read_list!(data, p, 4, i32::from_le_bytes);
        let timestamps_ns = read_list!(data, p, 8, i64::from_le_bytes);
        let counters = read_list!(data, p, 16, u128::from_le_bytes);
        let balances = read_list!(data, p, 16, i128::from_le_bytes);
        let gains = read_list!(data, p, 4, f32::from_le_bytes);
        let voltages = read_list!(data, p, 8, f64::from_le_bytes);

        let name_count = read_count(data, &mut p)?;
        let mut channel_names = Vec::with_capacity(name_count);
        for _ in 0..name_count {
            channel_names.push(read_len_str(data, &mut p)?);
        }

        let payload_count = read_count(data, &mut p)?;
        let mut payloads = Vec::with_capacity(payload_count);
        for _ in 0..payload_count {
            let n = read_count(data, &mut p)?;
            payloads.push(read_bytes!(data, p, n).to_vec());
        }

        let mut checksum = [0u8; 4];
        checksum.copy_from_slice(read_bytes!(data, p, 4));
        let mut window = [0i16; 3];
        for w in window.iter_mut() {
            *w = i16::from_le_bytes(read_bytes!(data, p, 2).try_into().unwrap());
        }

        let dropped_frames = if read_bytes!(data, p, 1)[0] == 0 {
            None
        } else {
            Some(u32::from_le_bytes(
                read_bytes!(data, p, 4).try_into().unwrap(),
            ))
        };

        let ord = read_bytes!(data, p, 1)[0];
        let mode = match ord {
            0 => Mode::Idle,
            1 => Mode::Active,
            2 => Mode::Fault,
            3 => Mode::Calibrating,
            other => return Err(format!("unknown signal mode ordinal {other}")),
        };

        Ok(SignalCapture {
            capture_id,
            flags,
            raw_frame,
            port_numbers,
            sample_counts,
            byte_totals,
            trim_offsets,
            drift_deltas,
            temperatures_c,
            timestamps_ns,
            counters,
            balances,
            gains,
            voltages,
            channel_names,
            payloads,
            checksum,
            window,
            dropped_frames,
            mode,
        })
    }
}
