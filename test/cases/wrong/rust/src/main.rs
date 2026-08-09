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

//! The Rust half of the `wrong` meta-test. Mirrors the Go worker's byte/JSON
//! layout exactly (so faithful output is byte-identical across languages), but
//! drops its OWN language ("rust") from `langs` when the active format's fault
//! directive is disabled, so its output diverges and serify must report it.

use serde_json::Value;
use serify::{run_suite, Format, SerifyModel, Suite, Type};

/// The language this worker drops from `langs` when corrupting.
const SELF_LANG: &str = "rust";

#[derive(SerifyModel)]
struct Wrong {
    binary_serialize: bool,
    binary_deserialize: bool,
    json_serialize: bool,
    json_deserialize: bool,
    langs: Vec<String>,
}

fn upper_self(langs: &[String]) -> Vec<String> {
    langs.iter()
        .map(|l| {
            if l == SELF_LANG {
                l.to_uppercase()
            } else {
                l.clone()
            }
        })
        .collect()
}

impl Wrong {
    // -- binary: [bs][bd][js][jd] then u32 len + (u32 len + utf8)* --------------

    fn marshal_binary(&self) -> Result<Vec<u8>, String> {
        let langs = if self.binary_serialize {
            self.langs.clone()
        } else {
            upper_self(&self.langs)
        };
        let mut buf = vec![
            self.binary_serialize as u8,
            self.binary_deserialize as u8,
            self.json_serialize as u8,
            self.json_deserialize as u8,
        ];
        buf.extend_from_slice(&(langs.len() as u32).to_le_bytes());
        for s in &langs {
            buf.extend_from_slice(&(s.len() as u32).to_le_bytes());
            buf.extend_from_slice(s.as_bytes());
        }
        Ok(buf)
    }

    fn unmarshal_binary(data: &[u8]) -> Result<Self, String> {
        if data.len() < 4 {
            return Err("truncated: missing flags".into());
        }
        let binary_serialize = data[0] != 0;
        let binary_deserialize = data[1] != 0;
        let json_serialize = data[2] != 0;
        let json_deserialize = data[3] != 0;

        let mut p = 4usize;
        let read_u32 = |p: &mut usize| -> Result<usize, String> {
            if *p + 4 > data.len() {
                return Err("truncated: u32".into());
            }
            let n = u32::from_le_bytes(data[*p..*p + 4].try_into().unwrap()) as usize;
            *p += 4;
            Ok(n)
        };
        let count = read_u32(&mut p)?;
        let mut langs = Vec::with_capacity(count);
        for _ in 0..count {
            let n = read_u32(&mut p)?;
            if p + n > data.len() {
                return Err("truncated: string".into());
            }
            let s = String::from_utf8(data[p..p + n].to_vec()).map_err(|e| e.to_string())?;
            p += n;
            langs.push(s);
        }
        if !binary_deserialize {
            langs = upper_self(&langs);
        }
        Ok(Wrong {
            binary_serialize,
            binary_deserialize,
            json_serialize,
            json_deserialize,
            langs,
        })
    }

    // -- json: matches Go's encoding/json (struct field order, compact) ---------

    fn to_json(&self) -> Vec<u8> {
        let langs = if self.json_serialize {
            self.langs.clone()
        } else {
            upper_self(&self.langs)
        };
        let mut s = String::from("{");
        s.push_str("\"binary_serialize\":");
        s.push_str(jbool(self.binary_serialize));
        s.push_str(",\"binary_deserialize\":");
        s.push_str(jbool(self.binary_deserialize));
        s.push_str(",\"json_serialize\":");
        s.push_str(jbool(self.json_serialize));
        s.push_str(",\"json_deserialize\":");
        s.push_str(jbool(self.json_deserialize));
        s.push_str(",\"langs\":[");
        for (i, l) in langs.iter().enumerate() {
            if i > 0 {
                s.push(',');
            }
            s.push_str(&serde_json::to_string(l).unwrap());
        }
        s.push_str("]}");
        s.into_bytes()
    }

    fn from_json(data: &[u8]) -> Result<Self, String> {
        let v: Value = serde_json::from_slice(data).map_err(|e| e.to_string())?;
        let getb = |k: &str| -> Result<bool, String> {
            v.get(k).and_then(Value::as_bool).ok_or_else(|| format!("missing bool {k}"))
        };
        let json_deserialize = getb("json_deserialize")?;
        let langs: Vec<String> = v
            .get("langs")
            .and_then(Value::as_array)
            .ok_or("missing langs")?
            .iter()
            .map(|x| x.as_str().unwrap_or_default().to_string())
            .collect();
        let langs = if json_deserialize { langs } else { upper_self(&langs) };
        Ok(Wrong {
            binary_serialize: getb("binary_serialize")?,
            binary_deserialize: getb("binary_deserialize")?,
            json_serialize: getb("json_serialize")?,
            json_deserialize,
            langs,
        })
    }
}

fn jbool(b: bool) -> &'static str {
    if b {
        "true"
    } else {
        "false"
    }
}

fn main() {
    run_suite(
        Suite::new().with_type(
            "wrong",
            Type::new()
                .with_format(
                    "binary",
                    Format::model::<Wrong>()
                        .serializer(Wrong::marshal_binary)
                        .deserializer(Wrong::unmarshal_binary),
                )
                .with_format(
                    "json",
                    Format::model::<Wrong>()
                        .serializer(|w| Ok(w.to_json()))
                        .deserializer(Wrong::from_json),
                )
                .with_format(
                    "err_ser",
                    Format::new()
                        .serializer(|_fm| Err("injected serialize error".into()))
                        .deserializer(|data| Wrong::unmarshal_binary(data).map(|w| w.to_field_map())),
                )
                .with_format(
                    "err_deser",
                    Format::new()
                        .serializer(|fm| Wrong::from_field_map(fm)?.marshal_binary())
                        .deserializer(|_data| Err("injected deserialize error".into())),
                )
                .with_format(
                    "hang",
                    Format::new()
                        .serializer(|fm| {
                            std::thread::sleep(std::time::Duration::from_secs(3));
                            Wrong::from_field_map(fm)?.marshal_binary()
                        })
                        .deserializer(|data| Wrong::unmarshal_binary(data).map(|w| w.to_field_map())),
                )
                .with_format(
                    "crash",
                    Format::new()
                        .serializer(|_fm| -> Result<Vec<u8>, String> {
                            std::process::exit(3);
                        })
                        .deserializer(|data| Wrong::unmarshal_binary(data).map(|w| w.to_field_map())),
                ),
        ),
    );
}
