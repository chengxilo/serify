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

//! A trivial worker for the `invalid_schema` fixture. Its only job is to build:
//! the fixture's case YAML declares no `formats:`, so `serify` rejects the
//! schema during config parsing and this worker is never actually invoked.
//! Mirrors the Go `Dumb` worker (a single `id: string`, one `byte` format).

use serify::{run_suite, Format, SerifyModel, Suite, Type};

#[derive(SerifyModel)]
struct Dumb {
    id: String,
}

fn main() {
    run_suite(Suite::new().with_type(
        "invalid_schema",
        Type::new().with_format(
            "byte",
            Format::new()
                .serializer(|fm| Ok(Dumb::from_field_map(fm)?.id.into_bytes()))
                .deserializer(|data| {
                    let id = String::from_utf8(data.to_vec()).map_err(|e| e.to_string())?;
                    Ok(Dumb { id }.to_field_map())
                }),
        ),
    ));
}
