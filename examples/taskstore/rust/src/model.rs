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

//! The records the server stores and sends. Each mirrors the case file of the
//! same name and owns its own byte layout.
//!
//! `#[derive(SerifyModel)]` is the whole schema binding — it reads the field
//! names and types and generates the FieldMap conversion, so nothing here calls
//! an accessor. Unlike Go's inert struct tags, a derive is a real dependency:
//! see the note in Cargo.toml about `default-features = false`.

use serify::SerifyModel;

use crate::wire::{put_enum, put_optional_i64, put_str, put_tags, Reader};

/// Declaration order of the `enum<low, normal, high>` in cases/task.yaml.
/// Position is the ordinal on the wire.
pub const PRIORITIES: [&str; 3] = ["low", "normal", "high"];

/// Declaration order of the `enum<not_found, invalid, conflict>` in
/// cases/api_error.yaml.
pub const ERROR_CODES: [&str; 3] = ["not_found", "invalid", "conflict"];

/// A stored task: cases/task.yaml.
///
/// An enum needs nothing from the derive — it travels as its variant *name*, so
/// `priority` is a plain String and PRIORITIES above fixes the ordinal this
/// codec writes.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub done: bool,
    pub priority: String,
    pub tags: Vec<String>,
    pub due_at: Option<i64>,
}

/// A task the server has not assigned an id to yet: cases/draft.yaml.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub struct Draft {
    pub title: String,
    pub priority: String,
    pub tags: Vec<String>,
    pub due_at: Option<i64>,
}

/// The body of a list response: cases/task_page.yaml.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub struct TaskPage {
    pub items: Vec<Task>,
    pub total: u32,
}

/// A failed request: cases/api_error.yaml.
#[derive(SerifyModel, Clone, Debug, PartialEq)]
pub struct ApiError {
    pub code: String,
    pub message: String,
}

impl Task {
    pub fn put(&self, buf: &mut Vec<u8>) -> Result<(), String> {
        buf.extend_from_slice(&self.id.to_le_bytes());
        put_str(buf, &self.title);
        buf.push(self.done as u8);
        put_enum(buf, &PRIORITIES, &self.priority)?;
        put_tags(buf, &self.tags);
        put_optional_i64(buf, self.due_at);
        Ok(())
    }

    pub fn read(r: &mut Reader) -> Result<Self, String> {
        Ok(Self {
            id: r.u64()?,
            title: r.string()?,
            done: r.bool()?,
            priority: r.enum_of(&PRIORITIES)?,
            tags: r.tags()?,
            due_at: r.optional_i64()?,
        })
    }
}

impl Draft {
    pub fn put(&self, buf: &mut Vec<u8>) -> Result<(), String> {
        put_str(buf, &self.title);
        put_enum(buf, &PRIORITIES, &self.priority)?;
        put_tags(buf, &self.tags);
        put_optional_i64(buf, self.due_at);
        Ok(())
    }

    pub fn read(r: &mut Reader) -> Result<Self, String> {
        Ok(Self {
            title: r.string()?,
            priority: r.enum_of(&PRIORITIES)?,
            tags: r.tags()?,
            due_at: r.optional_i64()?,
        })
    }
}

impl TaskPage {
    pub fn put(&self, buf: &mut Vec<u8>) -> Result<(), String> {
        buf.extend_from_slice(&(self.items.len() as u32).to_le_bytes());
        for t in &self.items {
            t.put(buf)?;
        }
        buf.extend_from_slice(&self.total.to_le_bytes());
        Ok(())
    }

    pub fn read(r: &mut Reader) -> Result<Self, String> {
        let n = r.u32()?;
        let items = (0..n).map(|_| Task::read(r)).collect::<Result<_, _>>()?;
        Ok(Self {
            items,
            total: r.u32()?,
        })
    }
}

impl ApiError {
    pub fn put(&self, buf: &mut Vec<u8>) -> Result<(), String> {
        put_enum(buf, &ERROR_CODES, &self.code)?;
        put_str(buf, &self.message);
        Ok(())
    }

    pub fn read(r: &mut Reader) -> Result<Self, String> {
        Ok(Self {
            code: r.enum_of(&ERROR_CODES)?,
            message: r.string()?,
        })
    }
}
