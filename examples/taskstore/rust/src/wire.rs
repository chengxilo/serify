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

//! Byte-level primitives. Go owns the layout these reproduce; the conventions
//! are documented at the top of go/api/wire.go.

/// A cursor over one message's bytes.
///
/// Go threads the remaining slice through every read and Python keeps an
/// offset; Rust is happiest with a borrow plus a position. The byte order and
/// the widths are the contract — the shape of the code that walks them is not.
pub struct Reader<'a> {
    data: &'a [u8],
    pos: usize,
}

impl<'a> Reader<'a> {
    pub fn new(data: &'a [u8]) -> Self {
        Self { data, pos: 0 }
    }

    fn take(&mut self, n: usize) -> Result<&'a [u8], String> {
        if self.pos + n > self.data.len() {
            return Err(format!(
                "truncated: want {n} bytes at offset {}, have {}",
                self.pos,
                self.data.len() - self.pos
            ));
        }
        let out = &self.data[self.pos..self.pos + n];
        self.pos += n;
        Ok(out)
    }

    pub fn u8(&mut self) -> Result<u8, String> {
        Ok(self.take(1)?[0])
    }

    pub fn u32(&mut self) -> Result<u32, String> {
        Ok(u32::from_le_bytes(self.take(4)?.try_into().unwrap()))
    }

    pub fn u64(&mut self) -> Result<u64, String> {
        Ok(u64::from_le_bytes(self.take(8)?.try_into().unwrap()))
    }

    pub fn i64(&mut self) -> Result<i64, String> {
        Ok(i64::from_le_bytes(self.take(8)?.try_into().unwrap()))
    }

    pub fn bool(&mut self) -> Result<bool, String> {
        Ok(self.u8()? != 0)
    }

    pub fn string(&mut self) -> Result<String, String> {
        let n = self.u32()? as usize;
        String::from_utf8(self.take(n)?.to_vec()).map_err(|e| e.to_string())
    }

    /// An enum arrives as an ordinal into a fixed list, so a value outside the
    /// declared variants has no encoding and cannot be read back.
    pub fn enum_of(&mut self, variants: &[&str]) -> Result<String, String> {
        let ord = self.u8()? as usize;
        variants
            .get(ord)
            .map(|s| s.to_string())
            .ok_or_else(|| format!("enum ordinal {ord} out of range for {variants:?}"))
    }

    pub fn tags(&mut self) -> Result<Vec<String>, String> {
        let n = self.u32()?;
        (0..n).map(|_| self.string()).collect()
    }

    pub fn optional_i64(&mut self) -> Result<Option<i64>, String> {
        if self.u8()? == 0 {
            return Ok(None);
        }
        self.i64().map(Some)
    }
}

pub fn put_str(buf: &mut Vec<u8>, s: &str) {
    buf.extend_from_slice(&(s.len() as u32).to_le_bytes());
    buf.extend_from_slice(s.as_bytes());
}

pub fn put_enum(buf: &mut Vec<u8>, variants: &[&str], name: &str) -> Result<(), String> {
    let ord = variants
        .iter()
        .position(|v| *v == name)
        .ok_or_else(|| format!("{name:?} is not one of {variants:?}"))?;
    buf.push(ord as u8);
    Ok(())
}

pub fn put_tags(buf: &mut Vec<u8>, tags: &[String]) {
    buf.extend_from_slice(&(tags.len() as u32).to_le_bytes());
    for t in tags {
        put_str(buf, t);
    }
}

pub fn put_optional_i64(buf: &mut Vec<u8>, v: Option<i64>) {
    match v {
        None => buf.push(0),
        Some(n) => {
            buf.push(1);
            buf.extend_from_slice(&n.to_le_bytes());
        }
    }
}
