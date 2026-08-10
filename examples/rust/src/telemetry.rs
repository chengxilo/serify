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

//! `TelemetryFrame` mirrors examples/cases/telemetry.yaml — one reading from a
//! field device.
//!
//! This is the type that could not be written until recently: its `humidity_pct`
//! is an `optional<float32>`, and the derive recognised only `Option<String>`, so
//! `Option<f32>` failed to compile. Between them its fields also cover the
//! suite's only `uint128`, two differently shaped fixed arrays, a
//! `map<string,uint64>`, and float values including NaN, ±Inf and negative zero.
//!
//! Go is the --ref language and owns the byte layout; see examples/go/wire.go.

use std::collections::HashMap;

use serify::SerifyModel;

use crate::wire::{append_len_str, read_bytes, read_len_str};

#[derive(SerifyModel)]
pub struct TelemetryFrame {
    device_id: u64,
    ipv6: u128,
    local_ip: [u8; 4],
    firmware: String,
    boot_count: u16,
    rssi_dbm: i8,
    temperature_dc: i16,
    clock_drift_ms: i32,
    battery_volts: f32,
    latitude: f64,
    longitude: f64,
    humidity_pct: Option<f32>,
    accel_mg: [i16; 3],
    visible_cells: Vec<u32>,
    packet_counts: HashMap<String, u64>,
    gps_fix: bool,
    signature: Vec<u8>,
}

impl TelemetryFrame {
    pub fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = Vec::new();
        buf.extend_from_slice(&self.device_id.to_le_bytes());
        buf.extend_from_slice(&self.ipv6.to_le_bytes());
        buf.extend_from_slice(&self.local_ip);
        append_len_str(&mut buf, &self.firmware);
        buf.extend_from_slice(&self.boot_count.to_le_bytes());
        buf.extend_from_slice(&self.rssi_dbm.to_le_bytes());
        buf.extend_from_slice(&self.temperature_dc.to_le_bytes());
        buf.extend_from_slice(&self.clock_drift_ms.to_le_bytes());
        buf.extend_from_slice(&self.battery_volts.to_le_bytes());
        buf.extend_from_slice(&self.latitude.to_le_bytes());
        buf.extend_from_slice(&self.longitude.to_le_bytes());

        // optional<float32>: a presence flag, then the value if present.
        match self.humidity_pct {
            None => buf.push(0),
            Some(v) => {
                buf.push(1);
                buf.extend_from_slice(&v.to_le_bytes());
            }
        }

        for v in self.accel_mg.iter() {
            buf.extend_from_slice(&v.to_le_bytes());
        }

        buf.extend_from_slice(&(self.visible_cells.len() as u32).to_le_bytes());
        for v in self.visible_cells.iter() {
            buf.extend_from_slice(&v.to_le_bytes());
        }

        let keys: Vec<&String> = self.packet_counts.keys().collect();
        buf.extend_from_slice(&(keys.len() as u32).to_le_bytes());
        for k in keys {
            append_len_str(&mut buf, k);
            buf.extend_from_slice(&self.packet_counts[k].to_le_bytes());
        }

        buf.push(self.gps_fix as u8);
        buf.extend_from_slice(&(self.signature.len() as u32).to_le_bytes());
        buf.extend_from_slice(&self.signature);

        Ok(buf)
    }

    pub fn unmarshal(data: &[u8]) -> Result<Self, String> {
        let mut p = 0usize;

        let device_id = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let ipv6 = u128::from_le_bytes(read_bytes!(data, p, 16).try_into().unwrap());
        let mut local_ip = [0u8; 4];
        local_ip.copy_from_slice(read_bytes!(data, p, 4));
        let firmware = read_len_str(data, &mut p)?;
        let boot_count = u16::from_le_bytes(read_bytes!(data, p, 2).try_into().unwrap());
        let rssi_dbm = i8::from_le_bytes(read_bytes!(data, p, 1).try_into().unwrap());
        let temperature_dc = i16::from_le_bytes(read_bytes!(data, p, 2).try_into().unwrap());
        let clock_drift_ms = i32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap());
        let battery_volts = f32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap());
        let latitude = f64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
        let longitude = f64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());

        let humidity_pct = if read_bytes!(data, p, 1)[0] == 0 {
            None
        } else {
            Some(f32::from_le_bytes(
                read_bytes!(data, p, 4).try_into().unwrap(),
            ))
        };

        let mut accel_mg = [0i16; 3];
        for a in accel_mg.iter_mut() {
            *a = i16::from_le_bytes(read_bytes!(data, p, 2).try_into().unwrap());
        }

        let cell_count = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let mut visible_cells = Vec::with_capacity(cell_count);
        for _ in 0..cell_count {
            visible_cells.push(u32::from_le_bytes(
                read_bytes!(data, p, 4).try_into().unwrap(),
            ));
        }

        let entry_count = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let mut packet_counts = HashMap::with_capacity(entry_count);
        for _ in 0..entry_count {
            let k = read_len_str(data, &mut p)?;
            let v = u64::from_le_bytes(read_bytes!(data, p, 8).try_into().unwrap());
            packet_counts.insert(k, v);
        }

        let gps_fix = read_bytes!(data, p, 1)[0] != 0;
        let sig_len = u32::from_le_bytes(read_bytes!(data, p, 4).try_into().unwrap()) as usize;
        let signature = read_bytes!(data, p, sig_len).to_vec();
        let _ = p; // read_bytes! advances the cursor; this is the last field.

        Ok(TelemetryFrame {
            device_id,
            ipv6,
            local_ip,
            firmware,
            boot_count,
            rssi_dbm,
            temperature_dc,
            clock_drift_ms,
            battery_volts,
            latitude,
            longitude,
            humidity_pct,
            accel_mg,
            visible_cells,
            packet_counts,
            gps_fix,
            signature,
        })
    }
}
