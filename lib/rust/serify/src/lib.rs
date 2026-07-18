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

// Lets the `#[derive(SerifyModel)]` macro's `serify::` paths resolve inside this
// crate itself (e.g. in unit tests); a no-op for external users.
extern crate self as serify;

use std::collections::HashMap;

#[cfg(feature = "worker")]
use serde_json::{json, Value};
#[cfg(feature = "worker")]
use std::io::{BufRead, Write};

// SerifyModel - trait for structs that can round-trip through a FieldMap

/// Implement this trait (or `#[derive(SerifyModel)]`) to enable automatic
/// field mapping between your struct and the schema's `FieldMap`.
pub trait SerifyModel: Sized {
    fn from_field_map(fm: &FieldMap) -> Result<Self, String>;
    fn to_field_map(&self) -> FieldMap;
}

/// How a value of this type occupies one *field* of an enclosing FieldMap.
///
/// A product type sits under its key as a nested struct; a sum type sits there
/// as a variant. The enclosing struct's derive cannot tell which — a proc macro
/// sees only the field's spelling, never its definition — so it always emits
/// `T::serify_read(fm, key)` and lets `T` decide.
///
/// `#[derive(SerifyModel)]` emits this alongside `SerifyModel`: on a struct it
/// reads and writes a nested struct, on an enum a variant. There is deliberately
/// no blanket impl — a blanket for `T: SerifyModel` would collide with the enum
/// derive's own impl, and the point is that each type answers for itself.
pub trait SerifyField: Sized {
    fn serify_read(fm: &FieldMap, key: &str) -> Result<Self, String>;
    fn serify_write(&self, fm: &mut FieldMap, key: &str);
}

/// How a value of this type occupies a *oneof payload* — a bare `FieldValue`
/// rather than a keyed slot.
///
/// Emitted by `#[derive(SerifyModel)]` for the same reason as [`SerifyField`]:
/// the enclosing enum's derive sees only a type's spelling, so the type itself
/// has to say whether it is a struct, or a newtype that is transparent to
/// whatever it wraps. As with `SerifyField` there is deliberately no blanket
/// impl — one would collide with the per-type impls the derive emits.
pub trait SerifyPayload: Sized {
    fn from_payload(v: &FieldValue) -> Result<Self, String>;
    fn to_payload(&self) -> FieldValue;
}

pub use serify_derive::SerifyModel;

/// `SerifyField` + `SerifyPayload` for the built-in scalar types, so a
/// `#[serify(transparent)]` newtype over any of them works without further
/// machinery. Written out per type rather than blanket-implemented: a blanket
/// would collide with the impls the derive emits for user types.
macro_rules! serify_scalar {
    ($($ty:ty => $variant:ident, $get:ident, $set:ident;)*) => {$(
        impl SerifyField for $ty {
            fn serify_read(fm: &FieldMap, key: &str) -> Result<Self, String> {
                fm.$get(key).map(::std::convert::Into::into)
                    .ok_or_else(|| format!("missing {key}"))
            }
            fn serify_write(&self, fm: &mut FieldMap, key: &str) {
                fm.$set(key, self.clone());
            }
        }
        impl SerifyPayload for $ty {
            fn from_payload(v: &FieldValue) -> Result<Self, String> {
                match v {
                    FieldValue::$variant(x) => Ok(x.clone().into()),
                    other => Err(format!("expected {}, got {other:?}", stringify!($ty))),
                }
            }
            fn to_payload(&self) -> FieldValue {
                FieldValue::$variant(self.clone().into())
            }
        }
    )*};
}

serify_scalar! {
    u8   => U8,   get_u8,   set_u8;
    u16  => U16,  get_u16,  set_u16;
    u32  => U32,  get_u32,  set_u32;
    u64  => U64,  get_u64,  set_u64;
    u128 => U128, get_u128, set_u128;
    i8   => I8,   get_i8,   set_i8;
    i16  => I16,  get_i16,  set_i16;
    i32  => I32,  get_i32,  set_i32;
    i64  => I64,  get_i64,  set_i64;
    i128 => I128, get_i128, set_i128;
    f32  => F32,  get_f32,  set_f32;
    f64  => F64,  get_f64,  set_f64;
    bool => Bool, get_bool, set_bool;
}

/// An `Option<T>` occupies a field exactly as `T` does, plus a null for `None`.
///
/// Delegating to `T`'s own `SerifyField` is the whole point: how a type sits in
/// a field is `T`'s decision. A `#[serify(transparent)]` newtype writes itself
/// straight into the field, a product type nests — and an enclosing `Option`
/// must not re-decide that. Hardcoding "nest it" here is what made
/// `Option<WireName>` round-trip as a struct and lose its value.
impl<T: SerifyField> SerifyField for Option<T> {
    fn serify_read(fm: &FieldMap, key: &str) -> Result<Self, String> {
        match fm.fields.get(key) {
            // Absent, and the three spellings of "present but null".
            None
            | Some(FieldValue::Null)
            | Some(FieldValue::OptionalString(None))
            | Some(FieldValue::OptionalStruct(None)) => Ok(None),
            // A present optional may be stored bare or inside the dedicated
            // variant, depending on which side built the map. Unwrap the latter
            // so `T` sees the same shape either way.
            Some(FieldValue::OptionalString(Some(v))) => {
                let mut bare = FieldMap::new();
                bare.set_string(key, v.clone());
                Ok(Some(T::serify_read(&bare, key)?))
            }
            Some(FieldValue::OptionalStruct(Some(v))) => {
                let mut bare = FieldMap::new();
                bare.set_struct(key, (**v).clone());
                Ok(Some(T::serify_read(&bare, key)?))
            }
            _ => Ok(Some(T::serify_read(fm, key)?)),
        }
    }

    fn serify_write(&self, fm: &mut FieldMap, key: &str) {
        match self {
            None => fm.set_null(key),
            Some(v) => v.serify_write(fm, key),
        }
    }
}

impl SerifyField for String {
    fn serify_read(fm: &FieldMap, key: &str) -> Result<Self, String> {
        fm.get_string(key)
            .map(str::to_string)
            .ok_or_else(|| format!("missing {key}"))
    }
    fn serify_write(&self, fm: &mut FieldMap, key: &str) {
        fm.set_string(key, self.clone());
    }
}

impl SerifyPayload for String {
    fn from_payload(v: &FieldValue) -> Result<Self, String> {
        match v {
            FieldValue::Str(s) => Ok(s.clone()),
            other => Err(format!("expected a string, got {other:?}")),
        }
    }
    fn to_payload(&self) -> FieldValue {
        FieldValue::Str(self.clone())
    }
}

impl SerifyField for Vec<u8> {
    fn serify_read(fm: &FieldMap, key: &str) -> Result<Self, String> {
        fm.get_bytes(key)
            .map(<[u8]>::to_vec)
            .ok_or_else(|| format!("missing {key}"))
    }
    fn serify_write(&self, fm: &mut FieldMap, key: &str) {
        fm.set_bytes(key, self.clone());
    }
}

impl SerifyPayload for Vec<u8> {
    fn from_payload(v: &FieldValue) -> Result<Self, String> {
        match v {
            FieldValue::Bytes(b) => Ok(b.clone()),
            other => Err(format!("expected bytes, got {other:?}")),
        }
    }
    fn to_payload(&self) -> FieldValue {
        FieldValue::Bytes(self.clone())
    }
}

// FieldValue / FieldMap

#[derive(Debug, Clone, PartialEq)]
pub enum FieldValue {
    U8(u8),
    U16(u16),
    U32(u32),
    U64(u64),
    U128(u128),
    I8(i8),
    I16(i16),
    I32(i32),
    I64(i64),
    I128(i128),
    F32(f32),
    F64(f64),
    Bool(bool),
    Str(String),
    Bytes(Vec<u8>),
    ListString(Vec<String>),
    ListU16(Vec<u16>),
    ListU32(Vec<u32>),
    ListU64(Vec<u64>),
    ListI8(Vec<i8>),
    ListI16(Vec<i16>),
    ListI32(Vec<i32>),
    ListI64(Vec<i64>),
    ListU128(Vec<u128>),
    ListI128(Vec<i128>),
    ListF32(Vec<f32>),
    ListF64(Vec<f64>),
    ListBool(Vec<bool>),
    ListBytes(Vec<Vec<u8>>),
    ListStruct(Vec<FieldMap>),
    OptionalString(Option<String>),
    Struct(Box<FieldMap>),
    OptionalStruct(Option<Box<FieldMap>>),
    Map(HashMap<String, FieldValue>),
    /// One arm of a oneof: the active variant's tag and its payload (None for a
    /// unit variant).
    Variant {
        tag: String,
        value: Option<Box<FieldValue>>,
    },
    Null,
}

#[derive(Debug, Default, Clone, PartialEq)]
pub struct FieldMap {
    fields: HashMap<String, FieldValue>,
}

impl FieldMap {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn get_u8(&self, k: &str) -> Option<u8> {
        if let Some(FieldValue::U8(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_u16(&self, k: &str) -> Option<u16> {
        if let Some(FieldValue::U16(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_u32(&self, k: &str) -> Option<u32> {
        if let Some(FieldValue::U32(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_u64(&self, k: &str) -> Option<u64> {
        if let Some(FieldValue::U64(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_i8(&self, k: &str) -> Option<i8> {
        if let Some(FieldValue::I8(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_i16(&self, k: &str) -> Option<i16> {
        if let Some(FieldValue::I16(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_i32(&self, k: &str) -> Option<i32> {
        if let Some(FieldValue::I32(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_i64(&self, k: &str) -> Option<i64> {
        if let Some(FieldValue::I64(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_u128(&self, k: &str) -> Option<u128> {
        if let Some(FieldValue::U128(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_i128(&self, k: &str) -> Option<i128> {
        if let Some(FieldValue::I128(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_f32(&self, k: &str) -> Option<f32> {
        if let Some(FieldValue::F32(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_f64(&self, k: &str) -> Option<f64> {
        if let Some(FieldValue::F64(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_bool(&self, k: &str) -> Option<bool> {
        if let Some(FieldValue::Bool(v)) = self.fields.get(k) {
            Some(*v)
        } else {
            None
        }
    }
    pub fn get_string(&self, k: &str) -> Option<&str> {
        if let Some(FieldValue::Str(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_bytes(&self, k: &str) -> Option<&[u8]> {
        if let Some(FieldValue::Bytes(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_string(&self, k: &str) -> Option<&Vec<String>> {
        if let Some(FieldValue::ListString(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    /// A `list<uint8>` and a `bytes` are the same `Vec<u8>`, and a Rust model
    /// spells both `Vec<u8>`, so they share the `Bytes` variant rather than
    /// being indistinguishable at the type level but distinct in the FieldMap.
    pub fn get_list_u8(&self, k: &str) -> Option<&Vec<u8>> {
        if let Some(FieldValue::Bytes(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_u16(&self, k: &str) -> Option<&Vec<u16>> {
        if let Some(FieldValue::ListU16(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_u32(&self, k: &str) -> Option<&Vec<u32>> {
        if let Some(FieldValue::ListU32(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_u64(&self, k: &str) -> Option<&Vec<u64>> {
        if let Some(FieldValue::ListU64(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_i8(&self, k: &str) -> Option<&Vec<i8>> {
        if let Some(FieldValue::ListI8(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_i16(&self, k: &str) -> Option<&Vec<i16>> {
        if let Some(FieldValue::ListI16(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_i32(&self, k: &str) -> Option<&Vec<i32>> {
        if let Some(FieldValue::ListI32(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_i64(&self, k: &str) -> Option<&Vec<i64>> {
        if let Some(FieldValue::ListI64(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_u128(&self, k: &str) -> Option<&Vec<u128>> {
        if let Some(FieldValue::ListU128(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_i128(&self, k: &str) -> Option<&Vec<i128>> {
        if let Some(FieldValue::ListI128(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_f32(&self, k: &str) -> Option<&Vec<f32>> {
        if let Some(FieldValue::ListF32(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_f64(&self, k: &str) -> Option<&Vec<f64>> {
        if let Some(FieldValue::ListF64(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_bool(&self, k: &str) -> Option<&Vec<bool>> {
        if let Some(FieldValue::ListBool(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_bytes(&self, k: &str) -> Option<&Vec<Vec<u8>>> {
        if let Some(FieldValue::ListBytes(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_list_struct(&self, k: &str) -> Option<&Vec<FieldMap>> {
        if let Some(FieldValue::ListStruct(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_optional_string(&self, k: &str) -> Option<Option<&str>> {
        match self.fields.get(k) {
            Some(FieldValue::OptionalString(v)) => Some(v.as_deref()),
            _ => None,
        }
    }
    pub fn get_struct(&self, k: &str) -> Option<&FieldMap> {
        if let Some(FieldValue::Struct(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
    pub fn get_optional_struct(&self, k: &str) -> Option<Option<&FieldMap>> {
        match self.fields.get(k) {
            Some(FieldValue::OptionalStruct(v)) => Some(v.as_deref()),
            _ => None,
        }
    }

    pub fn set_u8(&mut self, k: &str, v: u8) {
        self.fields.insert(k.into(), FieldValue::U8(v));
    }
    pub fn set_u16(&mut self, k: &str, v: u16) {
        self.fields.insert(k.into(), FieldValue::U16(v));
    }
    pub fn set_u32(&mut self, k: &str, v: u32) {
        self.fields.insert(k.into(), FieldValue::U32(v));
    }
    pub fn set_u64(&mut self, k: &str, v: u64) {
        self.fields.insert(k.into(), FieldValue::U64(v));
    }
    pub fn set_i8(&mut self, k: &str, v: i8) {
        self.fields.insert(k.into(), FieldValue::I8(v));
    }
    pub fn set_i16(&mut self, k: &str, v: i16) {
        self.fields.insert(k.into(), FieldValue::I16(v));
    }
    pub fn set_i32(&mut self, k: &str, v: i32) {
        self.fields.insert(k.into(), FieldValue::I32(v));
    }
    pub fn set_i64(&mut self, k: &str, v: i64) {
        self.fields.insert(k.into(), FieldValue::I64(v));
    }
    pub fn set_u128(&mut self, k: &str, v: u128) {
        self.fields.insert(k.into(), FieldValue::U128(v));
    }
    pub fn set_i128(&mut self, k: &str, v: i128) {
        self.fields.insert(k.into(), FieldValue::I128(v));
    }
    pub fn set_f32(&mut self, k: &str, v: f32) {
        self.fields.insert(k.into(), FieldValue::F32(v));
    }
    pub fn set_f64(&mut self, k: &str, v: f64) {
        self.fields.insert(k.into(), FieldValue::F64(v));
    }
    pub fn set_bool(&mut self, k: &str, v: bool) {
        self.fields.insert(k.into(), FieldValue::Bool(v));
    }
    pub fn set_string(&mut self, k: &str, v: String) {
        self.fields.insert(k.into(), FieldValue::Str(v));
    }
    pub fn set_bytes(&mut self, k: &str, v: Vec<u8>) {
        self.fields.insert(k.into(), FieldValue::Bytes(v));
    }
    pub fn set_list_string(&mut self, k: &str, v: Vec<String>) {
        self.fields.insert(k.into(), FieldValue::ListString(v));
    }
    pub fn set_list_u8(&mut self, k: &str, v: Vec<u8>) {
        self.fields.insert(k.into(), FieldValue::Bytes(v));
    }
    pub fn set_list_u16(&mut self, k: &str, v: Vec<u16>) {
        self.fields.insert(k.into(), FieldValue::ListU16(v));
    }
    pub fn set_list_u32(&mut self, k: &str, v: Vec<u32>) {
        self.fields.insert(k.into(), FieldValue::ListU32(v));
    }
    pub fn set_list_u64(&mut self, k: &str, v: Vec<u64>) {
        self.fields.insert(k.into(), FieldValue::ListU64(v));
    }
    pub fn set_list_i8(&mut self, k: &str, v: Vec<i8>) {
        self.fields.insert(k.into(), FieldValue::ListI8(v));
    }
    pub fn set_list_i16(&mut self, k: &str, v: Vec<i16>) {
        self.fields.insert(k.into(), FieldValue::ListI16(v));
    }
    pub fn set_list_i32(&mut self, k: &str, v: Vec<i32>) {
        self.fields.insert(k.into(), FieldValue::ListI32(v));
    }
    pub fn set_list_i64(&mut self, k: &str, v: Vec<i64>) {
        self.fields.insert(k.into(), FieldValue::ListI64(v));
    }
    pub fn set_list_u128(&mut self, k: &str, v: Vec<u128>) {
        self.fields.insert(k.into(), FieldValue::ListU128(v));
    }
    pub fn set_list_i128(&mut self, k: &str, v: Vec<i128>) {
        self.fields.insert(k.into(), FieldValue::ListI128(v));
    }
    pub fn set_list_f32(&mut self, k: &str, v: Vec<f32>) {
        self.fields.insert(k.into(), FieldValue::ListF32(v));
    }
    pub fn set_list_f64(&mut self, k: &str, v: Vec<f64>) {
        self.fields.insert(k.into(), FieldValue::ListF64(v));
    }
    pub fn set_list_bool(&mut self, k: &str, v: Vec<bool>) {
        self.fields.insert(k.into(), FieldValue::ListBool(v));
    }
    pub fn set_list_bytes(&mut self, k: &str, v: Vec<Vec<u8>>) {
        self.fields.insert(k.into(), FieldValue::ListBytes(v));
    }
    pub fn set_list_struct(&mut self, k: &str, v: Vec<FieldMap>) {
        self.fields.insert(k.into(), FieldValue::ListStruct(v));
    }
    pub fn set_optional_string(&mut self, k: &str, v: Option<String>) {
        self.fields.insert(k.into(), FieldValue::OptionalString(v));
    }
    /// Write an absent `optional<T>` for a T with no `Option` alternative of its
    /// own — every scalar. `get_<scalar>` already reads this back as `None`.
    pub fn set_null(&mut self, k: &str) {
        self.fields.insert(k.into(), FieldValue::Null);
    }
    pub fn set_struct(&mut self, k: &str, v: FieldMap) {
        self.fields
            .insert(k.into(), FieldValue::Struct(Box::new(v)));
    }

    /// Store a oneof value: the active variant's tag and payload (None for a
    /// unit variant).
    pub fn set_variant(&mut self, k: &str, tag: &str, value: Option<FieldValue>) {
        self.fields.insert(
            k.into(),
            FieldValue::Variant {
                tag: tag.into(),
                value: value.map(Box::new),
            },
        );
    }

    /// Read a oneof value: (tag, payload). Returns None if the field is absent
    /// or not a variant.
    pub fn get_variant(&self, k: &str) -> Option<(&str, Option<&FieldValue>)> {
        if let Some(FieldValue::Variant { tag, value }) = self.fields.get(k) {
            Some((tag, value.as_deref()))
        } else {
            None
        }
    }
    pub fn set_optional_struct(&mut self, k: &str, v: Option<FieldMap>) {
        self.fields
            .insert(k.into(), FieldValue::OptionalStruct(v.map(Box::new)));
    }
    pub fn set_map(&mut self, k: &str, v: HashMap<String, FieldValue>) {
        self.fields.insert(k.into(), FieldValue::Map(v));
    }
    pub fn get_map(&self, k: &str) -> Option<&HashMap<String, FieldValue>> {
        if let Some(FieldValue::Map(v)) = self.fields.get(k) {
            Some(v)
        } else {
            None
        }
    }
}

// SchemaField - carries nested field info for struct types

#[cfg(feature = "worker")]
#[derive(Debug, Clone, Default)]
pub struct SchemaField {
    pub name: String,
    pub typ: String,
    pub fields: Vec<SchemaField>, // nested schema for struct / list<struct> / optional<struct>
    pub variants: Vec<SchemaVariant>, // for oneof<...>
    pub tags: HashMap<String, String>,
}

/// One arm of a oneof: a tag and its payload schema (None for a unit variant).
#[cfg(feature = "worker")]
#[derive(Debug, Clone, Default)]
pub struct SchemaVariant {
    pub name: String,
    pub payload: Option<Box<SchemaField>>,
}

#[cfg(feature = "worker")]
fn parse_schema_fields(arr: &[Value]) -> Vec<SchemaField> {
    arr.iter()
        .filter_map(|f| {
            let name = f["name"].as_str()?.to_string();
            let typ = f["type"].as_str()?.to_string();
            let fields = f["fields"]
                .as_array()
                .map(|a| parse_schema_fields(a))
                .unwrap_or_default();
            let variants = f["variants"]
                .as_array()
                .map(|a| parse_schema_variants(a))
                .unwrap_or_default();
            let tags = f["tags"]
                .as_object()
                .map(|o| {
                    o.iter()
                        .filter_map(|(k, v)| Some((k.clone(), v.as_str()?.to_string())))
                        .collect()
                })
                .unwrap_or_default();
            Some(SchemaField {
                name,
                typ,
                fields,
                variants,
                tags,
            })
        })
        .collect()
}

#[cfg(feature = "worker")]
fn parse_schema_variants(arr: &[Value]) -> Vec<SchemaVariant> {
    arr.iter()
        .filter_map(|v| {
            let name = v["name"].as_str()?.to_string();
            let payload = v.get("payload").filter(|p| !p.is_null()).map(|p| {
                // A payload is one SchemaField; reuse the field parser via a 1-elem slice.
                Box::new(
                    parse_schema_fields(std::slice::from_ref(p))
                        .into_iter()
                        .next()
                        .unwrap_or_default(),
                )
            });
            Some(SchemaVariant { name, payload })
        })
        .collect()
}

// decode_field_map / encode_field_map

#[cfg(feature = "worker")]
fn map_value_type(typ: &str) -> &str {
    let inner = &typ[4..typ.len() - 1]; // strip "map<" and ">"
    let mut depth = 0i32;
    for (i, c) in inner.char_indices() {
        match c {
            '[' | '<' => depth += 1,
            ']' | '>' => depth -= 1,
            ',' if depth == 0 => return inner[i + 1..].trim(),
            _ => {}
        }
    }
    ""
}

#[cfg(feature = "worker")]
pub fn decode_field_map(data: &Value, schema: &[SchemaField]) -> Result<FieldMap, String> {
    let obj = data.as_object().ok_or("data is not an object")?;
    let mut fm = FieldMap::new();
    for sf in schema {
        let v = match obj.get(&sf.name) {
            Some(v) => v,
            None => continue,
        };
        decode_field(&mut fm, sf, v)?;
    }
    Ok(fm)
}

#[cfg(feature = "worker")]
fn decode_field(fm: &mut FieldMap, sf: &SchemaField, v: &Value) -> Result<(), String> {
    let name = sf.name.as_str();
    match sf.typ.as_str() {
        "uint8" => fm.set_u8(name, v.as_u64().ok_or("uint8")? as u8),
        "uint16" => fm.set_u16(name, v.as_u64().ok_or("uint16")? as u16),
        "uint32" => fm.set_u32(name, v.as_u64().ok_or("uint32")? as u32),
        "uint64" => {
            let s = v.as_str().ok_or("u64 must be string")?;
            fm.set_u64(name, s.parse::<u64>().map_err(|e| e.to_string())?);
        }
        // 128-bit values are parsed at full width. Folding them into the u64/i64
        // branch (as this library used to) silently truncates everything above 2^64.
        "uint128" => {
            let s = v.as_str().ok_or("u128 must be string")?;
            fm.set_u128(name, s.parse::<u128>().map_err(|e| e.to_string())?);
        }
        "int8" => fm.set_i8(name, v.as_i64().ok_or("int8")? as i8),
        "int16" => fm.set_i16(name, v.as_i64().ok_or("int16")? as i16),
        "int32" => fm.set_i32(name, v.as_i64().ok_or("int32")? as i32),
        "int64" => {
            let s = v.as_str().ok_or("i64 must be string")?;
            fm.set_i64(name, s.parse::<i64>().map_err(|e| e.to_string())?);
        }
        "int128" => {
            let s = v.as_str().ok_or("i128 must be string")?;
            fm.set_i128(name, s.parse::<i128>().map_err(|e| e.to_string())?);
        }
        "float32" => {
            let s = v.as_str().ok_or("f32 must be hex")?;
            let b = hex::decode(s).map_err(|e| e.to_string())?;
            if b.len() != 4 {
                return Err("f32: expected 4 bytes".into());
            }
            fm.set_f32(
                name,
                f32::from_bits(u32::from_le_bytes(b.try_into().unwrap())),
            );
        }
        "float64" => {
            let s = v.as_str().ok_or("f64 must be hex")?;
            let b = hex::decode(s).map_err(|e| e.to_string())?;
            if b.len() != 8 {
                return Err("f64: expected 8 bytes".into());
            }
            fm.set_f64(
                name,
                f64::from_bits(u64::from_le_bytes(b.try_into().unwrap())),
            );
        }
        "bool" => fm.set_bool(name, v.as_bool().ok_or("bool")?),
        "string" => fm.set_string(name, v.as_str().ok_or("string")?.to_string()),
        "bytes" => {
            let s = v.as_str().ok_or("bytes must be hex")?;
            fm.set_bytes(name, hex::decode(s).map_err(|e| e.to_string())?);
        }
        "struct" => {
            let nested = decode_field_map(v, &sf.fields)?;
            fm.set_struct(name, nested);
        }
        typ if typ.starts_with("list<") => {
            let elem = &typ[5..typ.len() - 1];
            decode_list(fm, sf, elem, v)?;
        }
        typ if typ.starts_with("optional<") => {
            let elem = &typ[9..typ.len() - 1];
            decode_optional(fm, sf, elem, v)?;
        }
        // An array<T,N> is a list whose length the schema fixes, so it shares
        // decode_list outright and adds only the length check. Keeping a second
        // representation is what pinned array<T,N> to exactly [u32; 4] — it
        // silently truncated anything longer and refused negative elements.
        typ if typ.starts_with("array<") => {
            let (elem, want) = array_parts(typ)?;
            decode_list(fm, sf, elem, v)?;
            let got = v.as_array().map(|a| a.len()).unwrap_or(0);
            if got != want {
                return Err(format!("array {name}: expected {want} elements, got {got}"));
            }
        }
        // enum<a,b,c>: a bare variant name in case data, carried internally as a
        // payload-less Variant — the same representation oneof uses. An enum field
        // and a sum field thus share one shape and one derive path, so a model type
        // needs no `str`/`repr` marker to say which it is.
        typ if typ.starts_with("enum<") => {
            let tag = v.as_str().ok_or("enum must be a string")?;
            fm.set_variant(name, tag, None);
        }
        typ if typ.starts_with("oneof<") => {
            let obj = v.as_object().ok_or("oneof must be an object")?;
            if obj.len() != 1 {
                return Err(format!("oneof must name one variant, got {}", obj.len()));
            }
            let (tag, payload) = obj.iter().next().unwrap();
            let sv = sf
                .variants
                .iter()
                .find(|x| &x.name == tag)
                .ok_or_else(|| format!("unknown oneof variant {tag}"))?;
            match &sv.payload {
                None => fm.set_variant(name, tag, None),
                Some(p) => {
                    let mut tmp = FieldMap::new();
                    decode_field(&mut tmp, p, payload)?;
                    fm.set_variant(name, tag, tmp.fields.remove(&p.name));
                }
            }
        }
        typ if typ.starts_with("map<") => {
            let val_type = map_value_type(typ).to_string();
            let obj = v
                .as_object()
                .ok_or_else(|| format!("map field {} is not object", name))?;
            let mut map = HashMap::new();
            for (k, item) in obj {
                let val_sf = SchemaField {
                    name: k.clone(),
                    typ: val_type.clone(),
                    fields: sf.fields.clone(),
                    ..Default::default()
                };
                let mut tmp = FieldMap::new();
                decode_field(&mut tmp, &val_sf, item)?;
                if let Some(val) = tmp.fields.remove(k) {
                    map.insert(k.clone(), val);
                }
            }
            fm.set_map(name, map);
        }
        _ => return Err(format!("unknown type {}", sf.typ)),
    }
    Ok(())
}

/// Split "array<T,N>" into its element type and length.
#[cfg(feature = "worker")]
fn array_parts(typ: &str) -> Result<(&str, usize), String> {
    let inner = &typ[6..typ.len() - 1];
    let comma = inner
        .rfind(',')
        .ok_or_else(|| format!("array type {typ} has no length"))?;
    let n = inner[comma + 1..]
        .trim()
        .parse::<usize>()
        .map_err(|e| e.to_string())?;
    Ok((inner[..comma].trim(), n))
}

/// Decodes every element through `decode_field`, so a list supports exactly the
/// element types a bare field does. The match below only names the `Vec` each
/// element type packs into — it holds no decoding logic, which is why a scalar
/// cannot be reachable as a field but not as a list element.
#[cfg(feature = "worker")]
fn decode_list(fm: &mut FieldMap, sf: &SchemaField, elem: &str, v: &Value) -> Result<(), String> {
    let arr = v.as_array().ok_or("list must be array")?;

    // Each element is decoded as if it were a one-field record, then unwrapped.
    let elem_sf = SchemaField {
        name: "e".to_string(),
        typ: elem.to_string(),
        fields: sf.fields.clone(),
        ..Default::default()
    };
    let mut vals = Vec::with_capacity(arr.len());
    for (i, item) in arr.iter().enumerate() {
        let mut tmp = FieldMap::new();
        decode_field(&mut tmp, &elem_sf, item).map_err(|e| format!("[{i}]: {e}"))?;
        vals.push(
            tmp.fields
                .remove("e")
                .ok_or(format!("[{i}]: element did not decode"))?,
        );
    }

    // Pack the decoded elements into the typed list variant for this element type.
    macro_rules! pack {
        ($list:ident, $scalar:ident) => {{
            let mut out = Vec::with_capacity(vals.len());
            for (i, fv) in vals.into_iter().enumerate() {
                match fv {
                    FieldValue::$scalar(x) => out.push(x),
                    other => return Err(format!("[{i}]: list<{elem}> got {other:?}")),
                }
            }
            FieldValue::$list(out)
        }};
    }
    let packed = match elem {
        "uint8" => pack!(Bytes, U8),
        "uint16" => pack!(ListU16, U16),
        "uint32" => pack!(ListU32, U32),
        "uint64" => pack!(ListU64, U64),
        "int8" => pack!(ListI8, I8),
        "int16" => pack!(ListI16, I16),
        "int32" => pack!(ListI32, I32),
        "int64" => pack!(ListI64, I64),
        "uint128" => pack!(ListU128, U128),
        "int128" => pack!(ListI128, I128),
        "float32" => pack!(ListF32, F32),
        "float64" => pack!(ListF64, F64),
        "bool" => pack!(ListBool, Bool),
        "string" => pack!(ListString, Str),
        "bytes" => pack!(ListBytes, Bytes),
        "struct" => {
            let mut out = Vec::with_capacity(vals.len());
            for (i, fv) in vals.into_iter().enumerate() {
                match fv {
                    FieldValue::Struct(b) => out.push(*b),
                    other => return Err(format!("[{i}]: list<struct> got {other:?}")),
                }
            }
            FieldValue::ListStruct(out)
        }
        _ => return Err(format!("unsupported list element type {}", elem)),
    };
    fm.fields.insert(sf.name.clone(), packed);
    Ok(())
}

#[cfg(feature = "worker")]
fn decode_optional(
    fm: &mut FieldMap,
    sf: &SchemaField,
    elem: &str,
    v: &Value,
) -> Result<(), String> {
    if v.is_null() {
        match elem {
            "string" => fm.set_optional_string(&sf.name, None),
            "struct" => fm.set_optional_struct(&sf.name, None),
            _ => {
                fm.fields.insert(sf.name.clone(), FieldValue::Null);
            }
        }
        return Ok(());
    }
    match elem {
        "string" => fm.set_optional_string(
            &sf.name,
            Some(v.as_str().ok_or("optional<string>")?.to_string()),
        ),
        "struct" => {
            let nested = decode_field_map(v, &sf.fields)?;
            fm.set_optional_struct(&sf.name, Some(nested));
        }
        _ => {
            let inner = SchemaField {
                name: sf.name.clone(),
                typ: elem.to_string(),
                ..Default::default()
            };
            let mut tmp = FieldMap::new();
            decode_field(&mut tmp, &inner, v)?;
            if let Some(val) = tmp.fields.remove(&sf.name) {
                fm.fields.insert(sf.name.clone(), val);
            }
        }
    }
    Ok(())
}

#[cfg(feature = "worker")]
pub fn encode_field_map(fm: &FieldMap, schema: &[SchemaField]) -> Result<Value, String> {
    let mut map = serde_json::Map::new();
    for sf in schema {
        let fv = match fm.fields.get(&sf.name) {
            Some(v) => v,
            None => continue,
        };
        let encoded = encode_field(sf, fv)?;
        map.insert(sf.name.clone(), encoded);
    }
    Ok(Value::Object(map))
}

#[cfg(feature = "worker")]
fn encode_field(sf: &SchemaField, fv: &FieldValue) -> Result<Value, String> {
    match sf.typ.as_str() {
        "uint8" => {
            if let FieldValue::U8(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch u8".into())
            }
        }
        "uint16" => {
            if let FieldValue::U16(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch u16".into())
            }
        }
        "uint32" => {
            if let FieldValue::U32(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch u32".into())
            }
        }
        "uint64" => {
            if let FieldValue::U64(v) = fv {
                Ok(json!(v.to_string()))
            } else {
                Err("type mismatch u64".into())
            }
        }
        "uint128" => {
            if let FieldValue::U128(v) = fv {
                Ok(json!(v.to_string()))
            } else {
                Err("type mismatch u128".into())
            }
        }
        "int8" => {
            if let FieldValue::I8(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch i8".into())
            }
        }
        "int16" => {
            if let FieldValue::I16(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch i16".into())
            }
        }
        "int32" => {
            if let FieldValue::I32(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch i32".into())
            }
        }
        "int64" => {
            if let FieldValue::I64(v) = fv {
                Ok(json!(v.to_string()))
            } else {
                Err("type mismatch i64".into())
            }
        }
        "int128" => {
            if let FieldValue::I128(v) = fv {
                Ok(json!(v.to_string()))
            } else {
                Err("type mismatch i128".into())
            }
        }
        "float32" => {
            if let FieldValue::F32(v) = fv {
                Ok(json!(hex::encode(v.to_bits().to_le_bytes())))
            } else {
                Err("type mismatch f32".into())
            }
        }
        "float64" => {
            if let FieldValue::F64(v) = fv {
                Ok(json!(hex::encode(v.to_bits().to_le_bytes())))
            } else {
                Err("type mismatch f64".into())
            }
        }
        "bool" => {
            if let FieldValue::Bool(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch bool".into())
            }
        }
        "string" => {
            if let FieldValue::Str(v) = fv {
                Ok(json!(v))
            } else {
                Err("type mismatch string".into())
            }
        }
        "bytes" => {
            if let FieldValue::Bytes(v) = fv {
                Ok(json!(hex::encode(v)))
            } else {
                Err("type mismatch bytes".into())
            }
        }
        "struct" => {
            if let FieldValue::Struct(nested) = fv {
                encode_field_map(nested, &sf.fields)
            } else {
                Err("type mismatch struct".into())
            }
        }
        typ if typ.starts_with("list<") => {
            let elem = &typ[5..typ.len() - 1];
            encode_list(sf, elem, fv)
        }
        typ if typ.starts_with("optional<") => {
            let elem = &typ[9..typ.len() - 1];
            encode_optional(sf, elem, fv)
        }
        typ if typ.starts_with("array<") => {
            let (elem, _) = array_parts(typ)?;
            encode_list(sf, elem, fv)
        }
        // enum<a,b,c>: a payload-less Variant goes back out as the bare variant name.
        typ if typ.starts_with("enum<") => {
            if let FieldValue::Variant { tag, value: None } = fv {
                Ok(json!(tag))
            } else {
                Err("type mismatch enum".into())
            }
        }
        typ if typ.starts_with("oneof<") => {
            let FieldValue::Variant { tag, value } = fv else {
                return Err("type mismatch oneof".into());
            };
            let sv = sf
                .variants
                .iter()
                .find(|x| &x.name == tag)
                .ok_or_else(|| format!("unknown oneof variant {tag}"))?;
            let mut out = serde_json::Map::new();
            let payload = match (&sv.payload, value.as_deref()) {
                (None, _) => Value::Null,
                (Some(p), Some(v)) => encode_field(p, v)?,
                (Some(_), None) => return Err(format!("oneof variant {tag} needs a payload")),
            };
            out.insert(tag.clone(), payload);
            Ok(Value::Object(out))
        }
        typ if typ.starts_with("map<") => {
            if let FieldValue::Map(map) = fv {
                let val_type = map_value_type(typ).to_string();
                let mut out = serde_json::Map::new();
                for (k, v) in map {
                    let val_sf = SchemaField {
                        name: k.clone(),
                        typ: val_type.clone(),
                        fields: sf.fields.clone(),
                        ..Default::default()
                    };
                    out.insert(k.clone(), encode_field(&val_sf, v)?);
                }
                Ok(Value::Object(out))
            } else {
                Err("type mismatch map".into())
            }
        }
        _ => Err(format!("unknown type {}", sf.typ)),
    }
}

#[cfg(feature = "worker")]
/// Inverse of `decode_list` and the same shape: the match unpacks the typed list
/// variant, then every element goes back out through `encode_field`.
fn encode_list(sf: &SchemaField, elem: &str, fv: &FieldValue) -> Result<Value, String> {
    // Unpack the typed list into per-element FieldValues.
    macro_rules! unpack {
        ($list:ident, $scalar:ident) => {
            if let FieldValue::$list(v) = fv {
                v.iter()
                    .cloned()
                    .map(FieldValue::$scalar)
                    .collect::<Vec<_>>()
            } else {
                return Err(format!("type mismatch list<{elem}>"));
            }
        };
    }
    let vals = match elem {
        "uint8" => unpack!(Bytes, U8),
        "uint16" => unpack!(ListU16, U16),
        "uint32" => unpack!(ListU32, U32),
        "uint64" => unpack!(ListU64, U64),
        "int8" => unpack!(ListI8, I8),
        "int16" => unpack!(ListI16, I16),
        "int32" => unpack!(ListI32, I32),
        "int64" => unpack!(ListI64, I64),
        "uint128" => unpack!(ListU128, U128),
        "int128" => unpack!(ListI128, I128),
        "float32" => unpack!(ListF32, F32),
        "float64" => unpack!(ListF64, F64),
        "bool" => unpack!(ListBool, Bool),
        "string" => unpack!(ListString, Str),
        "bytes" => unpack!(ListBytes, Bytes),
        "struct" => {
            if let FieldValue::ListStruct(v) = fv {
                let arr: Result<Vec<_>, _> = v
                    .iter()
                    .map(|nested| encode_field_map(nested, &sf.fields))
                    .collect();
                return Ok(Value::Array(arr?));
            }
            return Err("type mismatch list<struct>".into());
        }
        _ => return Err(format!("unsupported list element type {}", elem)),
    };

    let elem_sf = SchemaField {
        name: "e".to_string(),
        typ: elem.to_string(),
        fields: sf.fields.clone(),
        ..Default::default()
    };
    let out: Result<Vec<_>, _> = vals
        .iter()
        .enumerate()
        .map(|(i, x)| encode_field(&elem_sf, x).map_err(|e| format!("[{i}]: {e}")))
        .collect();
    Ok(Value::Array(out?))
}

#[cfg(feature = "worker")]
fn encode_optional(sf: &SchemaField, elem: &str, fv: &FieldValue) -> Result<Value, String> {
    // Dispatch on what the model actually stored, not on the element type.
    // decode_optional always produces the dedicated Optional* variant, but a
    // model is free to hold a present value directly — a `#[serify(transparent)]`
    // newtype writes `Option<WireName>` as a plain String, never an
    // OptionalString. Keying on `elem` instead made those two representations
    // mutually exclusive, so such a field decoded correctly and then failed on
    // the way back out.
    match fv {
        FieldValue::OptionalString(v) => return Ok(v.as_ref().map_or(Value::Null, |s| json!(s))),
        FieldValue::OptionalStruct(v) => {
            return match v {
                None => Ok(Value::Null),
                Some(nested) => encode_field_map(nested, &sf.fields),
            };
        }
        FieldValue::Null => return Ok(Value::Null),
        _ => {}
    }
    // A present optional<T> encodes exactly as its T does.
    let inner = SchemaField {
        name: sf.name.clone(),
        typ: elem.to_string(),
        fields: sf.fields.clone(),
        ..Default::default()
    };
    encode_field(&inner, fv)
}

// Suite / Type - multi-type multi-format runner

#[cfg(feature = "worker")]
pub type SerializeFn = Box<dyn Fn(&FieldMap) -> Result<Vec<u8>, String>>;
#[cfg(feature = "worker")]
pub type DeserializeFn = Box<dyn Fn(&[u8]) -> Result<FieldMap, String>>;

/// A named serialization format with optional serializer and deserializer.
#[cfg(feature = "worker")]
pub struct Format {
    serializer: Option<SerializeFn>,
    deserializer: Option<DeserializeFn>,
}

#[cfg(feature = "worker")]
impl Format {
    pub fn new() -> Self {
        Self {
            serializer: None,
            deserializer: None,
        }
    }

    /// Both directions in one call: `Format::pair(serialize, deserialize)`.
    pub fn pair<S, D>(serialize: S, deserialize: D) -> Self
    where
        S: Fn(&FieldMap) -> Result<Vec<u8>, String> + 'static,
        D: Fn(&[u8]) -> Result<FieldMap, String> + 'static,
    {
        Self {
            serializer: Some(Box::new(serialize)),
            deserializer: Some(Box::new(deserialize)),
        }
    }

    /// Serialize direction only; the deserialize direction reports SKIPPED. For a
    /// worker whose SDK has no decoder for this type.
    pub fn serialize_only<S>(serialize: S) -> Self
    where
        S: Fn(&FieldMap) -> Result<Vec<u8>, String> + 'static,
    {
        Self {
            serializer: Some(Box::new(serialize)),
            deserializer: None,
        }
    }

    /// The canonical worker pattern: a model type `T` that maps to and from a
    /// FieldMap (`from_fm`/`to_fm`, e.g. a `#[derive(SerifyModel)]` spec's
    /// `from_field_map`/`to_field_map`) and encodes to and from bytes via its
    /// SDK codec (`encode`/`decode`). serify composes the two halves; the SDK
    /// codec stays in the worker, since serify is SDK-agnostic.
    pub fn model<T, FromFm, ToFm, Encode, Decode>(
        from_fm: FromFm,
        to_fm: ToFm,
        encode: Encode,
        decode: Decode,
    ) -> Self
    where
        FromFm: Fn(&FieldMap) -> Result<T, String> + 'static,
        ToFm: Fn(&T) -> FieldMap + 'static,
        Encode: Fn(&T) -> Vec<u8> + 'static,
        Decode: Fn(&[u8]) -> Result<T, String> + 'static,
    {
        Self::pair(
            move |fm| Ok(encode(&from_fm(fm)?)),
            move |data| Ok(to_fm(&decode(data)?)),
        )
    }

    pub fn serializer<F>(mut self, f: F) -> Self
    where
        F: Fn(&FieldMap) -> Result<Vec<u8>, String> + 'static,
    {
        self.serializer = Some(Box::new(f));
        self
    }

    pub fn deserializer<F>(mut self, f: F) -> Self
    where
        F: Fn(&[u8]) -> Result<FieldMap, String> + 'static,
    {
        self.deserializer = Some(Box::new(f));
        self
    }
}

#[cfg(feature = "worker")]
impl Default for Format {
    fn default() -> Self {
        Self::new()
    }
}

/// One data type with named formats (each holding an optional serializer and deserializer).
#[cfg(feature = "worker")]
pub struct Type {
    formats: HashMap<String, Format>,
}

#[cfg(feature = "worker")]
impl Type {
    pub fn new() -> Self {
        Self {
            formats: HashMap::new(),
        }
    }

    pub fn with_format(mut self, name: impl Into<String>, f: Format) -> Self {
        self.formats.insert(name.into(), f);
        self
    }
}

#[cfg(feature = "worker")]
impl Default for Type {
    fn default() -> Self {
        Self::new()
    }
}

/// Top-level worker configuration: a map of named types.
#[cfg(feature = "worker")]
pub struct Suite {
    types: HashMap<String, Type>,
}

#[cfg(feature = "worker")]
impl Suite {
    pub fn new() -> Self {
        Self {
            types: HashMap::new(),
        }
    }

    pub fn with_type(mut self, name: impl Into<String>, t: Type) -> Self {
        self.types.insert(name.into(), t);
        self
    }
}

#[cfg(feature = "worker")]
impl Default for Suite {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(feature = "worker")]
/// The protocol revision this library speaks. The runner requires an exact
/// match and refuses to start a worker reporting anything else.
const PROTOCOL_VERSION: u32 = 1;

// --- audit helpers ----------------------------------------------------------

/// XOR every byte in `buf` with 0xFF. Calling it twice restores the original
/// content (involution).
#[cfg(feature = "worker")]
fn xor_flip(buf: &mut [u8]) {
    for b in buf.iter_mut() {
        *b ^= 0xFF;
    }
}

/// Compare two FieldMaps and return the list of top-level keys that differ.
///
/// This is a low-level audit helper. If you use the serify conformance runner,
/// you do not need to call this directly — register your serialize/deserialize
/// functions in a Suite and Run() handles audit automatically via --audit.
/// Call this directly only if you want audit-style checks outside the runner.
#[cfg(feature = "worker")]
pub fn field_map_diffs(before: &FieldMap, after: &FieldMap) -> Vec<String> {
    let mut diffs = Vec::new();
    for k in before.fields.keys() {
        let bv = before.fields.get(k);
        let av = after.fields.get(k);
        if bv != av {
            diffs.push(k.clone());
        }
    }
    for k in after.fields.keys() {
        if !before.fields.contains_key(k) {
            diffs.push(k.clone());
        }
    }
    diffs.sort();
    diffs
}

/// Compare two JSON objects and return the list of top-level keys that differ.
///
/// This is a low-level audit helper. If you use the serify conformance runner,
/// you do not need to call this directly — register your serialize/deserialize
/// functions in a Suite and Run() handles audit automatically via --audit.
/// Call this directly only if you want audit-style checks outside the runner.
#[cfg(feature = "worker")]
pub fn json_field_diffs(before: &Value, after: &Value) -> Vec<String> {
    let mut diffs = Vec::new();
    if let (Some(b), Some(a)) = (before.as_object(), after.as_object()) {
        for (k, bv) in b {
            match a.get(k) {
                Some(av) if bv != av => {
                    diffs.push(k.clone());
                }
                None => {
                    diffs.push(k.clone());
                }
                _ => {}
            }
        }
        for k in a.keys() {
            if !b.contains_key(k) {
                diffs.push(k.clone());
            }
        }
    }
    diffs.sort();
    diffs
}

/// Active overwrite test: XOR-flip the input buffer and report which FieldMap
/// fields changed (indicating they alias the buffer). Restores original values.
///
/// This is a low-level audit helper. If you use the serify conformance runner,
/// you do not need to call this directly — register your serialize/deserialize
/// functions in a Suite and Run() handles audit automatically via --audit.
/// Call this directly only if you want audit-style checks outside the runner.
#[cfg(feature = "worker")]
pub fn detect_zero_copy(fm: &mut FieldMap, buf: &mut [u8]) -> Vec<String> {
    if buf.is_empty() {
        return vec![];
    }

    // Deep-clone the whole FieldMap; fields aliasing `buf` change when it is
    // flipped, and PartialEq compares FieldValue trees deeply, so nested
    // aliasing surfaces as a top-level diff.
    let snap = fm.clone();

    xor_flip(buf);

    let aliased = field_map_diffs(&snap, fm);

    // Restore pristine values (aliasing fields become owned copies).
    *fm = snap;

    aliased
}

/// XOR-flip the returned buffer and detect which model fields changed
/// (indicating the serializer output aliases model memory). Flips back after.
/// Returns the list of top-level field names showing output zero-copy.
///
/// This is a low-level audit helper. If you use the serify conformance runner,
/// you do not need to call this directly — register your serialize/deserialize
/// functions in a Suite and Run() handles audit automatically via --audit.
/// Call this directly only if you want audit-style checks outside the runner.
#[cfg(feature = "worker")]
pub fn detect_output_zero_copy(fm: &FieldMap, buf: &mut [u8]) -> Vec<String> {
    if buf.is_empty() {
        return vec![];
    }

    let before_snap = fm.clone();

    xor_flip(buf);

    let aliased = field_map_diffs(&before_snap, fm);

    // Flip back (involutive — restores any aliased model memory too).
    xor_flip(buf);

    aliased
}

/// Run the NDJSON worker protocol with a Suite.
/// Supports multi-type and multi-format via the `type` and `format` fields
/// in the `bind` message.
#[cfg(feature = "worker")]
pub fn run_suite(suite: Suite) {
    let stdin = std::io::stdin();
    let stdout = std::io::stdout();
    let mut out = std::io::BufWriter::new(stdout.lock());
    let mut schema: Vec<SchemaField> = Vec::new();

    // Active state set by bind.
    let mut active_type = String::new();
    let mut active_format = String::new();
    let mut bound = false;
    let mut audit_enabled = false;

    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };
        if line.is_empty() {
            continue;
        }

        let msg: Value = match serde_json::from_str(&line) {
            Ok(v) => v,
            Err(_) => continue,
        };
        let op = msg["op"].as_str().unwrap_or("");
        let id = msg["id"].as_str().unwrap_or("").to_string();

        macro_rules! emit {
            ($v:expr) => {{
                writeln!(out, "{}", $v).unwrap();
                out.flush().unwrap();
            }};
        }

        match op {
            "ping" => {
                // Health check: report liveness and the protocol revision this
                // library speaks. Binds nothing.
                emit!(json!({"op": "ping", "status": "OK",
                    "protocol_version": PROTOCOL_VERSION}));
            }

            "bind" => {
                schema = msg["schema"]
                    .as_array()
                    .map(|a| parse_schema_fields(a))
                    .unwrap_or_default();

                let type_name = msg["type"].as_str().unwrap_or("");
                let format_name = msg["format"].as_str().unwrap_or("");

                // Both type and format are required; the runner always sends them.
                if type_name.is_empty() {
                    emit!(json!({"op": "bind", "status": "ERROR",
                        "error": "bind requires a \"type\" field"}));
                    continue;
                }
                if !suite.types.contains_key(type_name) {
                    bound = false;
                    active_type.clear();
                    active_format.clear();
                    emit!(json!({"op": "bind", "status": "SKIPPED"}));
                    continue;
                }
                let t = suite.types.get(type_name).unwrap();

                if format_name.is_empty() {
                    emit!(json!({"op": "bind", "status": "ERROR",
                        "error": "bind requires a \"format\" field"}));
                    continue;
                }
                if !t.formats.contains_key(format_name) {
                    bound = false;
                    active_type.clear();
                    active_format.clear();
                    emit!(json!({"op": "bind", "status": "SKIPPED"}));
                    continue;
                }

                active_type = type_name.to_string();
                active_format = format_name.to_string();
                bound = true;
                audit_enabled = msg["audit"].as_bool().unwrap_or(false);

                emit!(json!({"op": "bind"}));
            }

            "serialize" => {
                if !bound {
                    emit!(json!({"id": id, "op": "serialize", "status": "ERROR",
                        "error": "no format configured (call bind first)"}));
                    continue;
                }
                let result = (|| -> Result<(String, Option<Value>), String> {
                    let t = suite
                        .types
                        .get(&active_type)
                        .ok_or_else(|| format!("unknown type {:?}", active_type))?;
                    let fmt = t
                        .formats
                        .get(&active_format)
                        .ok_or_else(|| format!("unknown format {:?}", active_format))?;
                    let ser = fmt
                        .serializer
                        .as_ref()
                        .ok_or_else(|| "format has no serializer".to_string())?;
                    let fm = decode_field_map(&msg["data"], &schema)?;

                    let mut audit_map = serde_json::Map::new();

                    // Mutation: snapshot FieldMap before, compare after.
                    let before_snap = if audit_enabled {
                        Some(fm.clone())
                    } else {
                        None
                    };

                    let mut b = ser(&fm)?;

                    if audit_enabled {
                        // Mutation check
                        if let Some(ref before) = before_snap {
                            let diffs = field_map_diffs(before, &fm);
                            if !diffs.is_empty() {
                                audit_map.insert("mutations".into(), json!(diffs));
                            }

                            // Output zero-copy: does returned buffer alias model fields?
                            let ozc = detect_output_zero_copy(&fm, &mut b);
                            if !ozc.is_empty() {
                                audit_map.insert("output_zero_copy_fields".into(), json!(ozc));
                            }
                        }

                        // Stability: serialize again, compare outputs.
                        if !matches!(ser(&fm), Ok(b2) if b2 == b) {
                            audit_map.insert("stable".into(), json!(false));
                        }
                    }

                    let hex = hex::encode(&b);

                    let audit = if audit_map.is_empty() {
                        None
                    } else {
                        Some(Value::Object(audit_map))
                    };
                    Ok((hex, audit))
                })();
                let resp = match result {
                    Ok((h, audit)) => {
                        let mut m = json!({"id": id, "op": "serialize", "status": "OK", "hex": h});
                        if let Some(a) = audit {
                            m.as_object_mut().unwrap().insert("audit".into(), a);
                        }
                        m
                    }
                    Err(e) => json!({"id": id, "op": "serialize", "status": "ERROR", "error": e}),
                };
                emit!(resp);
            }

            "deserialize" => {
                if !bound {
                    emit!(json!({"id": id, "op": "deserialize", "status": "ERROR",
                        "error": "no format configured (call bind first)"}));
                    continue;
                }
                let result = (|| -> Result<(Value, Option<Value>), String> {
                    let t = suite
                        .types
                        .get(&active_type)
                        .ok_or_else(|| format!("unknown type {:?}", active_type))?;
                    let fmt = t
                        .formats
                        .get(&active_format)
                        .ok_or_else(|| format!("unknown format {:?}", active_format))?;
                    let des = fmt
                        .deserializer
                        .as_ref()
                        .ok_or_else(|| "format has no deserializer".to_string())?;
                    let h = msg["hex"].as_str().ok_or("missing hex")?;
                    let mut bytes = hex::decode(h).map_err(|e| e.to_string())?;

                    let buf_snapshot = if audit_enabled {
                        Some(bytes.clone())
                    } else {
                        None
                    };

                    let mut fm = des(&bytes)?;

                    let mut audit: Option<Value> = None;
                    if audit_enabled {
                        let mut a = serde_json::Map::new();

                        // Input-buffer mutation: did the deserializer modify bytes?
                        if let Some(ref snap) = buf_snapshot {
                            if snap != &bytes {
                                a.insert("input_mutated".into(), json!(true));
                            }
                        }

                        // Deserialize stability: re-deserialize from the pristine
                        // buffer snapshot (before zero-copy corrupts it).
                        if let Some(ref snap) = buf_snapshot {
                            if !matches!(des(snap), Ok(fm2) if fm2 == fm) {
                                a.insert("deser_stable".into(), json!(false));
                            }
                        }

                        // Zero-copy: overwrite buffer, check FieldMap, restore.
                        // (must run LAST — it corrupts the buffer)
                        let zc_fields = detect_zero_copy(&mut fm, &mut bytes);
                        if !zc_fields.is_empty() {
                            a.insert("zero_copy_fields".into(), json!(zc_fields));
                        }

                        if !a.is_empty() {
                            audit = Some(Value::Object(a));
                        }
                    }

                    let data = encode_field_map(&fm, &schema)?;
                    Ok((data, audit))
                })();
                let resp = match result {
                    Ok((data, audit)) => {
                        let mut m =
                            json!({"id": id, "op": "deserialize", "status": "OK", "data": data});
                        if let Some(a) = audit {
                            m.as_object_mut().unwrap().insert("audit".into(), a);
                        }
                        m
                    }
                    Err(e) => json!({"id": id, "op": "deserialize", "status": "ERROR", "error": e}),
                };
                emit!(resp);
            }

            "exit" => std::process::exit(0),
            _ => {}
        }
    }
}

#[cfg(feature = "worker")]
#[cfg(test)]
mod tests {
    use super::*;

    // sf builds a scalar/parameterized SchemaField (no nested fields).
    fn sf(name: &str, typ: &str) -> SchemaField {
        SchemaField {
            name: name.into(),
            typ: typ.into(),
            ..Default::default()
        }
    }
    // structf builds a struct-typed SchemaField with nested fields.
    fn structf(name: &str, typ: &str, fields: Vec<SchemaField>) -> SchemaField {
        SchemaField {
            name: name.into(),
            typ: typ.into(),
            fields,
            ..Default::default()
        }
    }

    // identifier-shaped oneof: a unit variant plus two single-payload variants.
    fn oneof_schema() -> Vec<SchemaField> {
        vec![SchemaField {
            name: "id".into(),
            typ: "oneof<balanced, numeric: uint32, name: string>".into(),
            variants: vec![
                SchemaVariant {
                    name: "balanced".into(),
                    payload: None,
                },
                SchemaVariant {
                    name: "numeric".into(),
                    payload: Some(Box::new(sf("numeric", "uint32"))),
                },
                SchemaVariant {
                    name: "name".into(),
                    payload: Some(Box::new(sf("name", "string"))),
                },
            ],
            ..Default::default()
        }]
    }

    #[test]
    fn oneof_round_trip() {
        for wire in [
            json!({"id": {"numeric": 5}}),
            json!({"id": {"name": "iggy"}}),
            json!({"id": {"balanced": null}}),
        ] {
            let fm = decode_field_map(&wire, &oneof_schema()).unwrap();
            let out = encode_field_map(&fm, &oneof_schema()).unwrap();
            assert_eq!(out, wire, "round-trip mismatch");
        }
    }

    #[test]
    fn oneof_get_set_variant() {
        let wire = json!({"id": {"numeric": 42}});
        let fm = decode_field_map(&wire, &oneof_schema()).unwrap();
        let (tag, val) = fm.get_variant("id").unwrap();
        assert_eq!(tag, "numeric");
        assert_eq!(val, Some(&FieldValue::U32(42)));

        // Build one from scratch and encode it.
        let mut built = FieldMap::new();
        built.set_variant("id", "name", Some(FieldValue::Str("x".into())));
        let out = encode_field_map(&built, &oneof_schema()).unwrap();
        assert_eq!(out, json!({"id": {"name": "x"}}));
    }

    #[test]
    fn field_map_scalar_roundtrip() {
        let mut fm = FieldMap::new();
        fm.set_u8("a", 7);
        fm.set_u32("b", 1234);
        fm.set_u64("c", u64::MAX);
        fm.set_i64("d", -5);
        fm.set_f32("e", 1.5);
        fm.set_bool("f", true);
        fm.set_string("g", "hi".into());
        fm.set_bytes("h", vec![0xde, 0xad]);
        assert_eq!(fm.get_u8("a"), Some(7));
        assert_eq!(fm.get_u32("b"), Some(1234));
        assert_eq!(fm.get_u64("c"), Some(u64::MAX));
        assert_eq!(fm.get_i64("d"), Some(-5));
        assert_eq!(fm.get_f32("e"), Some(1.5));
        assert_eq!(fm.get_bool("f"), Some(true));
        assert_eq!(fm.get_string("g"), Some("hi"));
        assert_eq!(fm.get_bytes("h"), Some(&[0xde, 0xad][..]));
        // wrong-type access returns None, not a panic.
        assert_eq!(fm.get_u32("g"), None);
    }

    // Schema covering scalars, list, optional, array, map (scalar + struct) and
    // nested struct.
    fn full_schema() -> Vec<SchemaField> {
        vec![
            sf("a", "uint32"),
            sf("b", "uint64"),
            sf("c", "float32"),
            sf("d", "bool"),
            sf("e", "string"),
            sf("f", "bytes"),
            sf("g", "list<string>"),
            sf("h", "list<uint32>"),
            sf("i", "optional<string>"),
            sf("j", "array<uint32,4>"),
            sf("k", "map<string,uint32>"),
            structf("s", "struct", vec![sf("x", "uint32"), sf("y", "string")]),
            structf("m", "map<string,struct>", vec![sf("v", "uint32")]),
        ]
    }

    fn full_field_map() -> FieldMap {
        let mut fm = FieldMap::new();
        fm.set_u32("a", 42);
        fm.set_u64("b", u64::MAX);
        fm.set_f32("c", 1.5);
        fm.set_bool("d", true);
        fm.set_string("e", "héllo🎉".into());
        fm.set_bytes("f", vec![0xca, 0xfe]);
        fm.set_list_string("g", vec!["x".into(), "y".into()]);
        fm.set_list_u32("h", vec![1, 2, 3]);
        fm.set_optional_string("i", None);
        fm.set_list_u32("j", vec![4, 3, 2, 1]);
        let mut k = HashMap::new();
        k.insert("one".to_string(), FieldValue::U32(1));
        fm.set_map("k", k);
        let mut s = FieldMap::new();
        s.set_u32("x", 9);
        s.set_string("y", "z".into());
        fm.set_struct("s", s);
        let mut inner = FieldMap::new();
        inner.set_u32("v", 7);
        let mut m = HashMap::new();
        m.insert("e1".to_string(), FieldValue::Struct(Box::new(inner)));
        fm.set_map("m", m);
        fm
    }

    #[test]
    fn encode_decode_roundtrip() {
        let schema = full_schema();
        let fm = full_field_map();

        let wire = encode_field_map(&fm, &schema).expect("encode");
        // Pin a few wire-format details.
        assert_eq!(wire["b"], json!("18446744073709551615")); // u64 as decimal string
        assert!(wire["c"].as_str().unwrap().len() == 8); // f32 as 4-byte LE hex
        assert_eq!(wire["f"], json!("cafe")); // bytes as hex
        assert_eq!(wire["i"], Value::Null); // optional None

        let fm2 = decode_field_map(&wire, &schema).expect("decode");
        let wire2 = encode_field_map(&fm2, &schema).expect("re-encode");
        assert_eq!(wire, wire2, "wire form must survive decode→encode");
    }

    #[test]
    fn optional_present_roundtrips() {
        let schema = vec![sf("p", "optional<string>")];
        let mut fm = FieldMap::new();
        fm.set_optional_string("p", Some("here".into()));
        let wire = encode_field_map(&fm, &schema).unwrap();
        assert_eq!(wire["p"], json!("here"));
        let fm2 = decode_field_map(&wire, &schema).unwrap();
        assert_eq!(fm2.get_optional_string("p"), Some(Some("here")));
    }

    #[test]
    fn unknown_type_errors() {
        let schema = vec![sf("a", "u8")]; // short form no longer exists
        let wire = json!({"a": 1});
        assert!(decode_field_map(&wire, &schema).is_err());
    }

    #[test]
    fn parse_schema_fields_nested() {
        let arr = vec![
            json!({"name": "id", "type": "uint64"}),
            json!({"name": "addr", "type": "struct", "fields": [
                {"name": "street", "type": "string"},
                {"name": "zip", "type": "uint32"},
            ]}),
        ];
        let parsed = parse_schema_fields(&arr);
        assert_eq!(parsed.len(), 2);
        assert_eq!(parsed[0].name, "id");
        assert_eq!(parsed[0].typ, "uint64");
        assert_eq!(parsed[1].fields.len(), 2);
        assert_eq!(parsed[1].fields[1].name, "zip");
    }

    #[derive(SerifyModel, Debug, PartialEq)]
    struct Rec {
        user_id: u64,
        name: String,
        score: f32,
    }

    #[test]
    fn derive_field_map_roundtrip() {
        let mut fm = FieldMap::new();
        fm.set_u64("user_id", 42);
        fm.set_string("name", "Alice".into());
        fm.set_f32("score", 1.5);

        let r = Rec::from_field_map(&fm).expect("from_field_map");
        assert_eq!(
            r,
            Rec {
                user_id: 42,
                name: "Alice".into(),
                score: 1.5
            }
        );

        let fm2 = r.to_field_map();
        assert_eq!(fm2.get_u64("user_id"), Some(42));
        assert_eq!(fm2.get_string("name"), Some("Alice"));
        assert_eq!(fm2.get_f32("score"), Some(1.5));
    }

    #[derive(SerifyModel, Debug, PartialEq)]
    struct Addr {
        street: String,
        zip: u32,
    }

    #[derive(SerifyModel, Debug, PartialEq)]
    struct Rich {
        tags: Vec<String>,
        profile: Option<String>,
        scores: HashMap<String, u32>,
        addr: Addr,
    }

    // Exercises the derive's container/nested branches (Vec, Option, HashMap,
    // nested SerifyModel) through a full FieldMap round-trip.
    #[test]
    fn derive_containers_roundtrip() {
        let mut fm = FieldMap::new();
        fm.set_list_string("tags", vec!["a".into(), "b".into()]);
        fm.set_optional_string("profile", None);
        let mut scores = HashMap::new();
        scores.insert("math".to_string(), FieldValue::U32(95));
        fm.set_map("scores", scores);
        let mut addr = FieldMap::new();
        addr.set_string("street", "1 Main".into());
        addr.set_u32("zip", 10001);
        fm.set_struct("addr", addr);

        let want = Rich {
            tags: vec!["a".into(), "b".into()],
            profile: None,
            scores: HashMap::from([("math".to_string(), 95u32)]),
            addr: Addr {
                street: "1 Main".into(),
                zip: 10001,
            },
        };
        let r = Rich::from_field_map(&fm).expect("from_field_map");
        assert_eq!(r, want);

        // Round-trip back out and decode again — must be stable.
        let fm2 = r.to_field_map();
        let r2 = Rich::from_field_map(&fm2).expect("from_field_map 2");
        assert_eq!(r2, want);
    }
    // A oneof payload is aliasing-capable, so detect_zero_copy must see through
    // the variant. Rust gets this for free — FieldMap derives Clone and
    // PartialEq, so cloning a Variant deep-copies its boxed payload and the diff
    // compares it by value — but nothing pinned that, and the equivalent code in
    // Go, Node, Python, C# and Java all had to be fixed by hand.
    #[test]
    fn oneof_detect_zero_copy_on_payload() {
        let mut buf = vec![1u8, 2, 3, 4];
        let mut fm = FieldMap::new();
        fm.set_variant("id", "key", Some(FieldValue::Bytes(buf.clone())));

        // Rust cannot alias a Vec<u8> into the buffer without unsafe, so emulate
        // what an aliasing deserializer would look like: flipping the buffer must
        // be observable through the variant.
        let snap = fm.clone();
        for b in buf.iter_mut() {
            *b ^= 0xFF;
        }
        fm.set_variant("id", "key", Some(FieldValue::Bytes(buf.clone())));

        assert_eq!(field_map_diffs(&snap, &fm), vec!["id".to_string()]);
    }

    /// Pins the invariant that a list supports every element type a bare field
    /// does. Before `decode_list` routed through `decode_field` it carried its
    /// own match, and uint8/uint16/int8/int16/int32/int64/float32/float64/bool/
    /// bytes were all missing from it — declarable in a case file, accepted by
    /// `serify validate`, and only failing once a worker actually ran.
    #[test]
    fn list_supports_every_scalar_elem() {
        // (element type, the JSON array the runner sends)
        let cases: Vec<(&str, serde_json::Value)> = vec![
            ("uint8", json!([0, 255])),
            ("uint16", json!([0, 65535])),
            ("uint32", json!([0, 4294967295u32])),
            ("uint64", json!(["0", "18446744073709551615"])),
            ("int8", json!([-128, 127])),
            ("int16", json!([-32768, 32767])),
            ("int32", json!([-2147483648i32, 2147483647i32])),
            ("int64", json!(["-9223372036854775808", "0"])),
            (
                "uint128",
                json!(["340282366920938463463374607431768211455", "0"]),
            ),
            (
                "int128",
                json!(["-170141183460469231731687303715884105728", "0"]),
            ),
            (
                "float32",
                json!([hex::encode(1.5f32.to_bits().to_le_bytes())]),
            ),
            (
                "float64",
                json!([hex::encode((-2.0f64).to_bits().to_le_bytes())]),
            ),
            ("bool", json!([true, false])),
            ("string", json!(["a", ""])),
            ("bytes", json!(["dead", ""])),
        ];

        for (elem, sent) in cases {
            let schema = vec![sf("v", &format!("list<{elem}>"))];
            let raw = json!({ "v": sent });

            let fm = decode_field_map(&raw, &schema)
                .unwrap_or_else(|e| panic!("list<{elem}> decode: {e}"));

            // Re-encoding must reproduce the wire form, so the two directions
            // cannot drift apart for any element type.
            let back = encode_field_map(&fm, &schema)
                .unwrap_or_else(|e| panic!("list<{elem}> encode: {e}"));
            assert_eq!(back, raw, "list<{elem}> did not round-trip");
        }
    }

    /// `optional<T>` for a scalar and for a nested model. The derive used to
    /// recognise only `Option<String>`; anything else fell through to the
    /// "nested model" classification and failed to compile with
    /// "the trait bound `Option<f32>: SerifyField` is not satisfied", which is
    /// why no Rust model could carry an `optional<scalar>` — and why the
    /// `telemetry` case type, whose `humidity_pct` is one, had never been
    /// implemented in Rust.
    #[test]
    fn optional_scalar_and_nested_round_trip() {
        #[derive(serify_derive::SerifyModel, Debug, PartialEq)]
        struct Inner {
            x: u32,
        }

        #[derive(serify_derive::SerifyModel, Debug, PartialEq)]
        struct Probe {
            humidity_pct: Option<f32>,
            count: Option<u64>,
            nested: Option<Inner>,
        }

        for p in [
            Probe {
                humidity_pct: Some(1.5),
                count: Some(7),
                nested: Some(Inner { x: 3 }),
            },
            Probe {
                humidity_pct: None,
                count: None,
                nested: None,
            },
        ] {
            let fm = p.to_field_map();
            let back = Probe::from_field_map(&fm).expect("from_field_map");
            assert_eq!(p, back);
        }
    }

}
