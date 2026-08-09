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

use std::collections::HashMap;
use serify::{Format, SerifyModel, Suite, Type, run_suite};
use serde_json::Value;

#[derive(SerifyModel)]
struct Point {
    x:    i32,
    y:    i32,
    z:    i32,
    name: String,
}

#[derive(SerifyModel)]
struct Tag {
    name:   String,
    weight: u32,
}

#[derive(SerifyModel)]
struct AllTypes {
    uint8:      u8,
    uint16:     u16,
    uint32:     u32,
    uint64:     u64,
    int8:       i8,
    int16:      i16,
    int32:      i32,
    int64:      i64,
    float32:    f32,
    float64:    f64,
    bool:       bool,
    string:     String,
    bytes:      Vec<u8>,
    list:       Vec<String>,
    optional:   Option<String>,
    array:      [u32; 4],
    #[serify(rename = "struct")]
    point:      Point,
    map:        HashMap<String, u32>,
    map_struct: HashMap<String, Tag>,
    status:     String,
}

// Declaration order of the enum in all_types.yaml. The binary layout stores the
// ordinal, not the name — the usual reason a worker cares that a field is an enum.
const STATUS_VARIANTS: [&str; 5] = ["pending", "paid", "shipped", "delivered", "cancelled"];

fn status_ordinal(s: &str) -> Result<u8, String> {
    STATUS_VARIANTS.iter().position(|v| *v == s)
        .map(|i| i as u8)
        .ok_or_else(|| format!("unknown status {:?}", s))
}

fn status_name(i: u8) -> Result<String, String> {
    STATUS_VARIANTS.get(i as usize)
        .map(|s| s.to_string())
        .ok_or_else(|| format!("status ordinal {} out of range", i))
}

fn append_len_str(buf: &mut Vec<u8>, s: &str) {
    buf.extend_from_slice(&(s.len() as u32).to_le_bytes());
    buf.extend_from_slice(s.as_bytes());
}

fn read_len_str(data: &[u8], pos: &mut usize) -> Result<String, String> {
    if *pos + 4 > data.len() { return Err("truncated".into()); }
    let n = u32::from_le_bytes(data[*pos..*pos+4].try_into().unwrap()) as usize;
    *pos += 4;
    if *pos + n > data.len() { return Err("truncated".into()); }
    let s = String::from_utf8(data[*pos..*pos+n].to_vec()).map_err(|e| e.to_string())?;
    *pos += n;
    Ok(s)
}

impl AllTypes {
    fn marshal(&self) -> Result<Vec<u8>, String> {
        let mut buf = Vec::new();

        buf.push(self.uint8);
        buf.extend_from_slice(&self.uint16.to_le_bytes());
        buf.extend_from_slice(&self.uint32.to_le_bytes());
        buf.extend_from_slice(&self.uint64.to_le_bytes());
        buf.push(self.int8 as u8);
        buf.extend_from_slice(&self.int16.to_le_bytes());
        buf.extend_from_slice(&self.int32.to_le_bytes());
        buf.extend_from_slice(&self.int64.to_le_bytes());
        buf.extend_from_slice(&self.float32.to_bits().to_le_bytes());
        buf.extend_from_slice(&self.float64.to_le_bytes());

        buf.push(if self.bool { 1 } else { 0 });

        append_len_str(&mut buf, &self.string);

        buf.extend_from_slice(&(self.bytes.len() as u32).to_le_bytes());
        buf.extend_from_slice(&self.bytes);

        buf.extend_from_slice(&(self.list.len() as u32).to_le_bytes());
        for s in &self.list { append_len_str(&mut buf, s); }

        match &self.optional {
            None    => buf.push(0),
            Some(s) => { buf.push(1); append_len_str(&mut buf, s); }
        }

        for c in self.array { buf.extend_from_slice(&c.to_le_bytes()); }

        buf.extend_from_slice(&self.point.x.to_le_bytes());
        buf.extend_from_slice(&self.point.y.to_le_bytes());
        buf.extend_from_slice(&self.point.z.to_le_bytes());
        append_len_str(&mut buf, &self.point.name);

        let mut map_keys: Vec<&String> = self.map.keys().collect();
        map_keys.sort();
        buf.extend_from_slice(&(map_keys.len() as u32).to_le_bytes());
        for k in &map_keys {
            append_len_str(&mut buf, k);
            buf.extend_from_slice(&self.map[*k].to_le_bytes());
        }

        let mut ms_keys: Vec<&String> = self.map_struct.keys().collect();
        ms_keys.sort();
        buf.extend_from_slice(&(ms_keys.len() as u32).to_le_bytes());
        for k in &ms_keys {
            append_len_str(&mut buf, k);
            append_len_str(&mut buf, &self.map_struct[*k].name);
            buf.extend_from_slice(&self.map_struct[*k].weight.to_le_bytes());
        }

        buf.push(status_ordinal(&self.status)?);

        Ok(buf)
    }

    fn unmarshal(data: &[u8]) -> Result<Self, String> {
        let mut p = 0usize;
        macro_rules! read_bytes {
            ($n:expr) => {{
                if p + $n > data.len() { return Err("truncated".into()); }
                let s = &data[p..p+$n];
                p += $n;
                s
            }};
        }

        let uint8     = read_bytes!(1)[0];
        let uint16    = u16::from_le_bytes(read_bytes!(2).try_into().unwrap());
        let uint32    = u32::from_le_bytes(read_bytes!(4).try_into().unwrap());
        let uint64    = u64::from_le_bytes(read_bytes!(8).try_into().unwrap());
        let int8      = read_bytes!(1)[0] as i8;
        let int16     = i16::from_le_bytes(read_bytes!(2).try_into().unwrap());
        let int32     = i32::from_le_bytes(read_bytes!(4).try_into().unwrap());
        let int64     = i64::from_le_bytes(read_bytes!(8).try_into().unwrap());
        let float32   = f32::from_bits(u32::from_le_bytes(read_bytes!(4).try_into().unwrap()));
        let float64   = f64::from_le_bytes(read_bytes!(8).try_into().unwrap());
        let bool_val  = read_bytes!(1)[0] != 0;
        let string    = read_len_str(data, &mut p)?;

        let meta_len  = u32::from_le_bytes(read_bytes!(4).try_into().unwrap()) as usize;
        let bytes     = read_bytes!(meta_len).to_vec();

        let list_len  = u32::from_le_bytes(read_bytes!(4).try_into().unwrap()) as usize;
        let mut list  = Vec::with_capacity(list_len);
        for _ in 0..list_len { list.push(read_len_str(data, &mut p)?); }

        let optional  = if read_bytes!(1)[0] == 0 { None } else { Some(read_len_str(data, &mut p)?) };

        let mut array = [0u32; 4];
        for v in array.iter_mut() { *v = u32::from_le_bytes(read_bytes!(4).try_into().unwrap()); }

        let point_x    = i32::from_le_bytes(read_bytes!(4).try_into().unwrap());
        let point_y    = i32::from_le_bytes(read_bytes!(4).try_into().unwrap());
        let point_z    = i32::from_le_bytes(read_bytes!(4).try_into().unwrap());
        let point_name = read_len_str(data, &mut p)?;

        let map_len   = u32::from_le_bytes(read_bytes!(4).try_into().unwrap()) as usize;
        let mut map   = HashMap::with_capacity(map_len);
        for _ in 0..map_len {
            let k = read_len_str(data, &mut p)?;
            let v = u32::from_le_bytes(read_bytes!(4).try_into().unwrap());
            map.insert(k, v);
        }

        let ms_len    = u32::from_le_bytes(read_bytes!(4).try_into().unwrap()) as usize;
        let mut map_struct = HashMap::with_capacity(ms_len);
        for _ in 0..ms_len {
            let k      = read_len_str(data, &mut p)?;
            let name   = read_len_str(data, &mut p)?;
            let weight = u32::from_le_bytes(read_bytes!(4).try_into().unwrap());
            map_struct.insert(k, Tag { name, weight });
        }

        let status = status_name(read_bytes!(1)[0])?;

        Ok(Self {
            uint8, uint16, uint32, uint64,
            int8, int16, int32, int64,
            float32, float64,
            bool: bool_val, string, bytes, list, optional, array,
            point: Point { x: point_x, y: point_y, z: point_z, name: point_name },
            map, map_struct, status,
        })
    }
}

// --- JSON format -------------------------------------------------------------
// Must match Go's encoding/json byte-for-byte: []byte→base64, sorted map keys,
// floats without trailing .0 for integer values.

const B64: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

fn base64_encode(data: &[u8]) -> String {
    let mut out = String::with_capacity(data.len().div_ceil(3) * 4);
    for chunk in data.chunks(3) {
        let b0 = chunk[0] as u32;
        let b1 = *chunk.get(1).unwrap_or(&0) as u32;
        let b2 = *chunk.get(2).unwrap_or(&0) as u32;
        let n = (b0 << 16) | (b1 << 8) | b2;
        out.push(B64[((n >> 18) & 63) as usize] as char);
        out.push(B64[((n >> 12) & 63) as usize] as char);
        out.push(if chunk.len() > 1 { B64[((n >> 6) & 63) as usize] as char } else { '=' });
        out.push(if chunk.len() > 2 { B64[(n & 63) as usize] as char } else { '=' });
    }
    out
}

fn base64_decode(s: &str) -> Result<Vec<u8>, String> {
    fn val(c: u8) -> Result<u32, String> {
        match c {
            b'A'..=b'Z' => Ok((c - b'A') as u32),
            b'a'..=b'z' => Ok((c - b'a' + 26) as u32),
            b'0'..=b'9' => Ok((c - b'0' + 52) as u32),
            b'+' => Ok(62), b'/' => Ok(63),
            _ => Err(format!("invalid base64 char {:?}", c as char)),
        }
    }
    let s = s.as_bytes();
    if s.len() % 4 != 0 { return Err("base64 length not a multiple of 4".into()); }
    let mut out = Vec::with_capacity(s.len() / 4 * 3);
    for chunk in s.chunks(4) {
        let c2pad = chunk[2] == b'=';
        let c3pad = chunk[3] == b'=';
        let v0 = val(chunk[0])?; let v1 = val(chunk[1])?;
        let v2 = if c2pad { 0 } else { val(chunk[2])? };
        let v3 = if c3pad { 0 } else { val(chunk[3])? };
        let n = (v0 << 18) | (v1 << 12) | (v2 << 6) | v3;
        out.push((n >> 16) as u8);
        if !c2pad { out.push((n >> 8) as u8); }
        if !c3pad { out.push(n as u8); }
    }
    Ok(out)
}

// The Go worker turns off encoding/json's HTML escaping with SetEscapeHTML(false),
// so <, > and & go out raw and serde_json matches. U+2028/U+2029 are a different
// story: Go escapes those *unconditionally* (SetEscapeHTML(false) does not affect
// them — they break JSONP, see encoding/json's encode.go), so there is no way to
// turn it off on the Go side and every other language has to reproduce it.
fn jstr(s: &str) -> String {
    serde_json::to_string(s)
        .unwrap()
        .replace('\u{2028}', "\\u2028")
        .replace('\u{2029}', "\\u2029")
}

fn to_json(a: &AllTypes) -> Vec<u8> {
    let mut s = std::string::String::new();
    s.push('{');
    s.push_str(&format!("\"uint8\":{}", a.uint8));
    s.push_str(&format!(",\"uint16\":{}", a.uint16));
    s.push_str(&format!(",\"uint32\":{}", a.uint32));
    s.push_str(&format!(",\"uint64\":{}", a.uint64));
    s.push_str(&format!(",\"int8\":{}", a.int8));
    s.push_str(&format!(",\"int16\":{}", a.int16));
    s.push_str(&format!(",\"int32\":{}", a.int32));
    s.push_str(&format!(",\"int64\":{}", a.int64));
    s.push_str(&format!(",\"float32\":{}", a.float32));
    s.push_str(&format!(",\"float64\":{}", a.float64));
    s.push_str(&format!(",\"bool\":{}", if a.bool { "true" } else { "false" }));
    s.push_str(",\"string\":");
    s.push_str(&jstr(&a.string));
    s.push_str(",\"bytes\":\"");
    s.push_str(&base64_encode(&a.bytes));
    s.push('"');
    s.push_str(",\"list\":[");
    for (i, t) in a.list.iter().enumerate() {
        if i > 0 { s.push(','); }
        s.push_str(&jstr(t));
    }
    s.push(']');
    s.push_str(",\"optional\":");
    match &a.optional {
        Some(p) => s.push_str(&jstr(p)),
        None => s.push_str("null"),
    }
    s.push_str(",\"array\":[");
    for (i, c) in a.array.iter().enumerate() {
        if i > 0 { s.push(','); }
        s.push_str(&c.to_string());
    }
    s.push(']');
    s.push_str(",\"struct\":{");
    s.push_str(&format!("\"x\":{},\"y\":{},\"z\":{},\"name\":", a.point.x, a.point.y, a.point.z));
    s.push_str(&jstr(&a.point.name));
    s.push('}');

    s.push_str(",\"map\":{");
    let mut map_keys: Vec<&std::string::String> = a.map.keys().collect();
    map_keys.sort();
    for (i, k) in map_keys.iter().enumerate() {
        if i > 0 { s.push(','); }
        s.push_str(&jstr(k));
        s.push(':');
        s.push_str(&a.map[*k].to_string());
    }
    s.push('}');

    s.push_str(",\"map_struct\":{");
    let mut ms_keys: Vec<&std::string::String> = a.map_struct.keys().collect();
    ms_keys.sort();
    for (i, k) in ms_keys.iter().enumerate() {
        if i > 0 { s.push(','); }
        let t = &a.map_struct[*k];
        s.push_str(&jstr(k));
        s.push_str(":{\"name\":");
        s.push_str(&jstr(&t.name));
        s.push_str(",\"weight\":");
        s.push_str(&t.weight.to_string());
        s.push('}');
    }
    s.push('}');

    s.push_str(",\"status\":");
    s.push_str(&jstr(&a.status));

    s.push('}');
    s.into_bytes()
}

fn from_json(data: &[u8]) -> Result<AllTypes, String> {
    let v: Value = serde_json::from_slice(data).map_err(|e| e.to_string())?;
    let f = |k: &str| -> Result<&Value, String> {
        v.get(k).ok_or_else(|| format!("missing field {}", k))
    };

    let uint8    = f("uint8")?.as_u64().ok_or("uint8")? as u8;
    let uint16   = f("uint16")?.as_u64().ok_or("uint16")? as u16;
    let uint32   = f("uint32")?.as_u64().ok_or("uint32")? as u32;
    let uint64   = f("uint64")?.as_u64().ok_or("uint64")?;
    let int8     = f("int8")?.as_i64().ok_or("int8")? as i8;
    let int16    = f("int16")?.as_i64().ok_or("int16")? as i16;
    let int32    = f("int32")?.as_i64().ok_or("int32")? as i32;
    let int64    = f("int64")?.as_i64().ok_or("int64")?;
    let float32  = f("float32")?.as_f64().ok_or("float32")? as f32;
    let float64  = f("float64")?.as_f64().ok_or("float64")?;
    let bool_val = f("bool")?.as_bool().ok_or("bool")?;
    let string   = f("string")?.as_str().ok_or("string")?.to_string();
    let bytes    = base64_decode(f("bytes")?.as_str().ok_or("bytes")?)?;
    let list     = f("list")?.as_array().ok_or("list")?
        .iter().map(|x| x.as_str().unwrap_or_default().to_string()).collect();
    let optional = match f("optional")? {
        Value::Null => None,
        x => Some(x.as_str().ok_or("optional")?.to_string()),
    };
    let arr = f("array")?.as_array().ok_or("array")?;
    let mut array = [0u32; 4];
    for (i, slot) in array.iter_mut().enumerate() {
        *slot = arr.get(i).and_then(|x| x.as_u64()).ok_or("array elem")? as u32;
    }
    let st = f("struct")?;
    let point = Point {
        x: st.get("x").and_then(|x| x.as_i64()).ok_or("struct.x")? as i32,
        y: st.get("y").and_then(|x| x.as_i64()).ok_or("struct.y")? as i32,
        z: st.get("z").and_then(|x| x.as_i64()).ok_or("struct.z")? as i32,
        name: st.get("name").and_then(|x| x.as_str()).ok_or("struct.name")?.to_string(),
    };
    let mut map = HashMap::new();
    for (k, val) in f("map")?.as_object().ok_or("map")? {
        map.insert(k.clone(), val.as_u64().ok_or("map value")? as u32);
    }
    let mut map_struct = HashMap::new();
    for (k, val) in f("map_struct")?.as_object().ok_or("map_struct")? {
        map_struct.insert(k.clone(), Tag {
            name: val.get("name").and_then(|x| x.as_str()).ok_or("tag.name")?.to_string(),
            weight: val.get("weight").and_then(|x| x.as_u64()).ok_or("tag.weight")? as u32,
        });
    }

    let status = f("status")?.as_str().ok_or("status")?.to_string();

    Ok(AllTypes {
        uint8, uint16, uint32, uint64,
        int8, int16, int32, int64,
        float32, float64,
        bool: bool_val, string, bytes, list, optional, array,
        point, map, map_struct, status,
    })
}

fn main() {
    run_suite(Suite::new()
        .with_type("all_types",
            Type::new()
                .with_format("binary", Format::model::<AllTypes>()
                    .serializer(AllTypes::marshal)
                    .deserializer(AllTypes::unmarshal),
                )
                .with_format("json", Format::model::<AllTypes>()
                    .serializer(|a| Ok(to_json(a)))
                    .deserializer(from_json),
                ),
        ),
    );
}
