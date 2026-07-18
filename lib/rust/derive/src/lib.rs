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

use proc_macro::TokenStream;
use proc_macro2::TokenStream as TokenStream2;
use quote::quote;
use syn::{
    parse_macro_input, Data, DeriveInput, Expr, Fields, GenericArgument, Lit, PathArguments, Type,
};

// Public entry point

/// Derive `SerifyModel` for a struct, an enum, or a newtype.
///
/// - **Struct with named fields** — a record. Each field is mapped to a schema
///   key using `#[serify(rename = "key")]` or, if the attribute is absent, the
///   field name as-is (Rust fields are already snake_case, so no conversion is
///   needed).
/// - **Enum** — a `oneof`. A sum type is exactly what `oneof<...>` describes, so
///   variant names become schema tags in snake_case and no converter is needed.
///   A payload-less enum is carried as a unit-variant `oneof`, which is also what
///   an `enum<...>` schema field maps to, so no marker distinguishes the two.
/// - **Single-field tuple struct** — a newtype, with `#[serify(transparent)]`:
///   the schema sees straight through the wrapper to the type it wraps. Add
///   `try_from = "Type::new"` when construction is fallible, so a validating
///   newtype keeps validating what arrives off the wire.
///
/// Supported field types:
/// - Scalar integers: `u8` `u16` `u32` `u64` `u128` `i8` `i16` `i32` `i64` `i128`
/// - Floats: `f32` `f64`
/// - `bool`
/// - `String`
/// - `Vec<u8>` (bytes)
/// - `Vec<String>` (list of strings)
/// - `Option<String>` (optional string)
/// - `[u32; 4]` (fixed-size array)
/// - Any other named type `T` - treated as a nested struct that itself
///   implements `SerifyModel`.
#[proc_macro_derive(SerifyModel, attributes(serify))]
pub fn derive_serify_model(input: TokenStream) -> TokenStream {
    let input = parse_macro_input!(input as DeriveInput);
    expand(input)
        .unwrap_or_else(|e| e.to_compile_error())
        .into()
}

fn expand(input: DeriveInput) -> syn::Result<TokenStream2> {
    let name = &input.ident;

    let fields = match &input.data {
        Data::Struct(s) => match &s.fields {
            Fields::Named(f) => &f.named,
            // A single-field tuple struct is a newtype: with
            // `#[serify(transparent)]` the schema sees straight through it.
            Fields::Unnamed(f) if f.unnamed.len() == 1 => {
                let t = get_transparent(&input.attrs)?;
                if !t.transparent {
                    return Err(syn::Error::new_spanned(
                        name,
                        "a newtype needs #[serify(transparent)] so the schema knows \
                         it sees the wrapped type; add #[serify(transparent, try_from = \"Type::new\")] \
                         if construction is fallible",
                    ));
                }
                return expand_transparent(name, &f.unnamed.first().unwrap().ty, t.try_from);
            }
            _ => {
                return Err(syn::Error::new_spanned(
                    name,
                    "SerifyModel only supports structs with named fields",
                ))
            }
        },
        // An enum is a sum type, which is exactly what `oneof<...>` describes,
        // so it derives too — no hand-written converter.
        Data::Enum(e) => return expand_enum(name, e),
        _ => {
            return Err(syn::Error::new_spanned(
                name,
                "SerifyModel supports structs and enums, not unions",
            ))
        }
    };

    // `#[serify(remote = "SdkType")]` on the struct: the derive maps a FieldMap
    // to and from that foreign SDK type directly, using this struct only as the
    // compile-time-checked field spec. No SerifyModel impl on the foreign type
    // (orphan rule); instead inherent `from_field_map`/`to_field_map` on the
    // spec. `self.` field access becomes `v.` (the borrowed remote value).
    let remote = get_remote(&input.attrs)?;
    let receiver = match &remote {
        Some(_) => quote! { v },
        None => quote! { self },
    };

    let mut from_entries: Vec<TokenStream2> = Vec::new();
    let mut to_stmts: Vec<TokenStream2> = Vec::new();

    for field in fields {
        let field_ident = field.ident.as_ref().unwrap();
        let attrs = field_attrs(field)?;
        let key = attrs.rename.unwrap_or_else(|| field_ident.to_string());

        let (from_expr, to_stmt) = match attrs.with {
            // `#[serify(with = "path")]`: the field's type is bridged by a module
            // exposing `from(&FieldMap, key) -> Result<T, String>` and
            // `to(&mut FieldMap, key, &T)`. The converter owns the whole FieldMap
            // access, so it picks its own carrier (a nested struct, a bare scalar,
            // …) — the compile-time equivalent of a Go converter. An SDK type
            // (even an enum) is then used as a field directly, no mirror.
            Some(path) => {
                let with: syn::Path = syn::parse_str(&path)?;
                (
                    quote! { #with::from(fm, #key)? },
                    quote! { #with::to(&mut fm, #key, &#receiver.#field_ident); },
                )
            }
            None => type_ops(&field.ty, &key, field_ident, &receiver)?,
        };
        from_entries.push(quote! { #field_ident: #from_expr });
        to_stmts.push(to_stmt);
    }

    if let Some(remote_path) = remote {
        return Ok(quote! {
            impl #name {
                pub fn from_field_map(fm: &serify::FieldMap) -> ::std::result::Result<#remote_path, ::std::string::String> {
                    ::std::result::Result::Ok(#remote_path {
                        #(#from_entries,)*
                    })
                }

                pub fn to_field_map(v: &#remote_path) -> serify::FieldMap {
                    let mut fm = serify::FieldMap::new();
                    #(#to_stmts)*
                    fm
                }
            }
        });
    }

    Ok(quote! {
        impl serify::SerifyModel for #name {
            fn from_field_map(fm: &serify::FieldMap) -> ::std::result::Result<Self, ::std::string::String> {
                ::std::result::Result::Ok(Self {
                    #(#from_entries,)*
                })
            }

            fn to_field_map(&self) -> serify::FieldMap {
                let mut fm = serify::FieldMap::new();
                #(#to_stmts)*
                fm
            }
        }

        // As a oneof payload, a product type is a struct value.
        impl serify::SerifyPayload for #name {
            fn from_payload(v: &serify::FieldValue)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                match v {
                    serify::FieldValue::Struct(inner) =>
                        <Self as serify::SerifyModel>::from_field_map(inner),
                    other => ::std::result::Result::Err(
                        format!("expected a struct payload, got {other:?}")),
                }
            }
            fn to_payload(&self) -> serify::FieldValue {
                serify::FieldValue::Struct(::std::boxed::Box::new(
                    <Self as serify::SerifyModel>::to_field_map(self)))
            }
        }

        // As a field of an enclosing model, a product type nests.
        impl serify::SerifyField for #name {
            fn serify_read(fm: &serify::FieldMap, key: &str)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                <Self as serify::SerifyModel>::from_field_map(
                    fm.get_struct(key).ok_or_else(|| format!("missing {key}"))?)
            }
            fn serify_write(&self, fm: &mut serify::FieldMap, key: &str) {
                fm.set_struct(key, <Self as serify::SerifyModel>::to_field_map(self));
            }
        }
    })
}

// Enum -> oneof

/// `Numeric` -> `numeric`, `PartitionId` -> `partition_id`, `MessagesKey` ->
/// `messages_key`. Matches how the schema spells its variant tags.
fn snake_case(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 4);
    for (i, ch) in s.char_indices() {
        if ch.is_uppercase() {
            if i != 0 {
                out.push('_');
            }
            out.extend(ch.to_lowercase());
        } else {
            out.push(ch);
        }
    }
    out
}

/// Derive for an enum: each variant becomes one arm of a `oneof`.
///
/// - A unit variant (`Balanced`) carries no payload.
/// - A one-field tuple variant (`PartitionId(u32)`) carries that value.
///
/// Two impls are emitted. `SerifyField` — as **inherent** functions, so they win
/// over the blanket impl when an enclosing struct writes `T::serify_read(fm,
/// key)` — puts the value under a key of the enclosing map. `SerifyModel` puts
/// it under `"value"`, which is the one-field-record shape a standalone sum type
/// under test uses.
fn expand_enum(name: &syn::Ident, data: &syn::DataEnum) -> syn::Result<TokenStream2> {
    let mut read_arms: Vec<TokenStream2> = Vec::new();
    let mut write_arms: Vec<TokenStream2> = Vec::new();

    for variant in &data.variants {
        let vid = &variant.ident;
        let attrs = variant_attrs(variant)?;
        let tag = attrs.rename.unwrap_or_else(|| snake_case(&vid.to_string()));

        match &variant.fields {
            Fields::Unit => {
                read_arms.push(quote! { (#tag, _) => Self::#vid });
                write_arms.push(quote! { Self::#vid => (#tag, ::std::option::Option::None) });
            }
            Fields::Unnamed(f) if f.unnamed.len() == 1 => {
                let ty = &f.unnamed.first().unwrap().ty;
                // `#[serify(with = "path")]` on a variant: the payload is
                // bridged by a module exposing `from(&FieldValue) -> Result<T,
                // String>` and `to(&T) -> FieldValue`. Needed when the payload
                // type is not one the classifier can map — a validating newtype,
                // say — and it keeps the conversion next to the type it guards.
                let (unwrap, wrap) = match &attrs.with {
                    Some(path) => {
                        let w: syn::Path = syn::parse_str(path)?;
                        (
                            quote! { #w::from(payload).map_err(|e| format!("variant {}: {}", #tag, e))? },
                            quote! { #w::to(v) },
                        )
                    }
                    None => payload_ops(ty, tag.as_str())?,
                };
                read_arms.push(quote! {
                    (#tag, ::std::option::Option::Some(payload)) => Self::#vid(#unwrap)
                });
                write_arms.push(quote! {
                    Self::#vid(v) => (#tag, ::std::option::Option::Some(#wrap))
                });
            }
            _ => {
                return Err(syn::Error::new_spanned(
                    vid,
                    "SerifyModel enum variants must be unit or single-field tuple variants \
                     (a multi-field variant should carry one struct instead)",
                ))
            }
        }
    }

    Ok(quote! {
        // As a field of an enclosing model, a sum type is a variant.
        impl serify::SerifyField for #name {
            fn serify_read(fm: &serify::FieldMap, key: &str)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                let (tag, payload) = fm.get_variant(key)
                    .ok_or_else(|| format!("field {key} is not a oneof"))?;
                ::std::result::Result::Ok(match (tag, payload) {
                    #(#read_arms,)*
                    (t, _) => return ::std::result::Result::Err(
                        format!("unknown variant {t:?} for {}", stringify!(#name))),
                })
            }

            fn serify_write(&self, fm: &mut serify::FieldMap, key: &str) {
                // Exactly one variant is live, so there are no inactive fields
                // needing a default.
                let (tag, payload) = match self { #(#write_arms,)* };
                fm.set_variant(key, tag, payload);
            }
        }

        impl serify::SerifyModel for #name {
            fn from_field_map(fm: &serify::FieldMap)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                <Self as serify::SerifyField>::serify_read(fm, "value")
            }

            fn to_field_map(&self) -> serify::FieldMap {
                let mut fm = serify::FieldMap::new();
                <Self as serify::SerifyField>::serify_write(self, &mut fm, "value");
                fm
            }
        }
    })
}

/// Unwrap a `FieldValue` payload into the variant's type, and wrap it back.
///
/// A payload is a single value, so this covers the same shapes a struct field
/// does: scalars, `String`, `Vec<u8>`, and a nested model carried as a struct.
fn payload_ops(ty: &Type, tag: &str) -> syn::Result<(TokenStream2, TokenStream2)> {
    Ok(match classify(ty) {
        TypeKind::Scalar(ref s) => {
            let fv = syn::parse_str::<syn::Path>(&format!("serify::FieldValue::{}", capitalize(s)))
                .unwrap();
            let want = s.clone();
            (
                quote! {
                    match payload {
                        #fv(v) => *v,
                        other => return ::std::result::Result::Err(
                            format!("variant {}: expected {}, got {:?}", #tag, #want, other)),
                    }
                },
                quote! { #fv(*v) },
            )
        }
        TypeKind::Str => (
            quote! {
                match payload {
                    serify::FieldValue::Str(s) => s.clone(),
                    other => return ::std::result::Result::Err(
                        format!("variant {}: expected string, got {:?}", #tag, other)),
                }
            },
            quote! { serify::FieldValue::Str(v.clone()) },
        ),
        TypeKind::Bytes => (
            quote! {
                match payload {
                    serify::FieldValue::Bytes(b) => b.clone(),
                    other => return ::std::result::Result::Err(
                        format!("variant {}: expected bytes, got {:?}", #tag, other)),
                }
            },
            quote! { serify::FieldValue::Bytes(v.clone()) },
        ),
        TypeKind::Nested(ref nested_ty) => (
            quote! {
                <#nested_ty as serify::SerifyPayload>::from_payload(payload)
                    .map_err(|e| format!("variant {}: {}", #tag, e))?
            },
            quote! { <#nested_ty as serify::SerifyPayload>::to_payload(v) },
        ),
        _ => {
            return Err(syn::Error::new_spanned(
                ty,
                "unsupported oneof payload type; use a scalar, String, Vec<u8>, \
                 or a type deriving SerifyModel",
            ))
        }
    })
}

#[derive(Default)]
struct VariantAttrs {
    rename: Option<String>,
    with: Option<String>,
}

fn variant_attrs(variant: &syn::Variant) -> syn::Result<VariantAttrs> {
    let mut out = VariantAttrs::default();
    for attr in &variant.attrs {
        if !attr.path().is_ident("serify") {
            continue;
        }
        attr.parse_nested_meta(|meta| {
            if meta.path.is_ident("rename") {
                out.rename = Some(meta.value()?.parse::<syn::LitStr>()?.value());
                Ok(())
            } else if meta.path.is_ident("with") {
                out.with = Some(meta.value()?.parse::<syn::LitStr>()?.value());
                Ok(())
            } else {
                Err(meta.error("unknown serify attribute on variant; expected `rename` or `with`"))
            }
        })?;
    }
    Ok(out)
}

// Attribute helpers

/// `#[serify(transparent)]` / `#[serify(transparent, try_from = "Type::new")]`
/// on a single-field tuple struct.
struct TransparentAttrs {
    transparent: bool,
    try_from: Option<syn::Path>,
}

fn get_transparent(attrs: &[syn::Attribute]) -> syn::Result<TransparentAttrs> {
    let mut out = TransparentAttrs {
        transparent: false,
        try_from: None,
    };
    for attr in attrs {
        if !attr.path().is_ident("serify") {
            continue;
        }
        let _ = attr.parse_nested_meta(|meta| {
            if meta.path.is_ident("transparent") {
                out.transparent = true;
                Ok(())
            } else if meta.path.is_ident("try_from") {
                let s: syn::LitStr = meta.value()?.parse()?;
                out.try_from = Some(s.parse()?);
                Ok(())
            } else {
                // `remote` is parsed separately; ignore anything else here.
                let _ = meta.value();
                Ok(())
            }
        });
    }
    Ok(out)
}

/// Derive for a newtype: the wrapper is invisible to the schema, which sees only
/// the type it wraps.
///
/// `try_from` names a fallible constructor (`WireName::new`), so a newtype that
/// exists to *validate* its contents keeps validating them — the derive never
/// writes the private field directly.
fn expand_transparent(
    name: &syn::Ident,
    inner: &Type,
    try_from: Option<syn::Path>,
) -> syn::Result<TokenStream2> {
    // Everything below delegates to the inner type's own `SerifyField` /
    // `SerifyPayload` impl, so the derive never has to recognise `#inner`
    // syntactically — whatever the schema supports, a newtype over it supports.
    let build = match &try_from {
        Some(ctor) => quote! { #ctor(__raw).map_err(|e| ::std::string::ToString::to_string(&e))? },
        None => quote! { Self(__raw) },
    };

    Ok(quote! {
        impl #name {
            fn __serify_wrap(__raw: #inner)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                ::std::result::Result::Ok(#build)
            }
        }

        // Transparent: the wrapper occupies the field exactly as its inner type
        // would.
        impl serify::SerifyField for #name {
            fn serify_read(fm: &serify::FieldMap, key: &str)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                let __raw = <#inner as serify::SerifyField>::serify_read(fm, key)?;
                Self::__serify_wrap(__raw)
            }
            fn serify_write(&self, fm: &mut serify::FieldMap, key: &str) {
                serify::SerifyField::serify_write(&self.0, fm, key);
            }
        }

        impl serify::SerifyPayload for #name {
            fn from_payload(v: &serify::FieldValue)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                let __raw = <#inner as serify::SerifyPayload>::from_payload(v)?;
                Self::__serify_wrap(__raw)
            }
            fn to_payload(&self) -> serify::FieldValue {
                serify::SerifyPayload::to_payload(&self.0)
            }
        }

        // Standalone, a transparent newtype is the one-field record its schema
        // declares, carrying the value under "value".
        impl serify::SerifyModel for #name {
            fn from_field_map(fm: &serify::FieldMap)
                -> ::std::result::Result<Self, ::std::string::String>
            {
                <Self as serify::SerifyField>::serify_read(fm, "value")
            }
            fn to_field_map(&self) -> serify::FieldMap {
                let mut fm = serify::FieldMap::new();
                <Self as serify::SerifyField>::serify_write(self, &mut fm, "value");
                fm
            }
        }
    })
}

fn get_remote(attrs: &[syn::Attribute]) -> syn::Result<Option<syn::Path>> {
    for attr in attrs {
        if !attr.path().is_ident("serify") {
            continue;
        }
        let mut remote: Option<syn::Path> = None;
        attr.parse_nested_meta(|meta| {
            if meta.path.is_ident("remote") {
                let s: syn::LitStr = meta.value()?.parse()?;
                remote = Some(s.parse()?);
                Ok(())
            } else {
                Err(meta.error("unknown serify attribute on struct; expected `remote`"))
            }
        })?;
        if remote.is_some() {
            return Ok(remote);
        }
    }
    Ok(None)
}

#[derive(Default)]
struct FieldAttrs {
    rename: Option<String>,
    with: Option<String>,
}

fn field_attrs(field: &syn::Field) -> syn::Result<FieldAttrs> {
    let mut out = FieldAttrs::default();
    for attr in &field.attrs {
        if !attr.path().is_ident("serify") {
            continue;
        }
        attr.parse_nested_meta(|meta| {
            if meta.path.is_ident("rename") {
                out.rename = Some(meta.value()?.parse::<syn::LitStr>()?.value());
                Ok(())
            } else if meta.path.is_ident("with") {
                out.with = Some(meta.value()?.parse::<syn::LitStr>()?.value());
                Ok(())
            } else {
                Err(meta.error("unknown serify attribute; expected `rename` or `with`"))
            }
        })?;
    }
    Ok(out)
}

// Type classification

enum TypeKind {
    // u8 / u16 / u32 / u64 / i8 / i16 / i32 / i64 / f32 / f64 / bool
    Scalar(String),
    // String
    Str,
    // Vec<u8>
    Bytes,
    // Vec<String>
    ListString,
    // Vec<T> for any scalar T other than u8 (u8 is Bytes) — u16/u32/.../bool
    ListScalar(String),
    // Vec<Vec<u8>>
    ListBytes,
    // Vec<T> where T: SerifyModel
    ListNested(syn::Ident),
    // Option<String>
    OptionString,
    // Option<T> for any scalar T — the FieldMap carries it as the scalar or Null
    OptionScalar(String),
    // Option<T> where T: SerifyModel
    OptionNested(syn::Ident),
    // [T; N] for any scalar T and any N. The library carries an array<T,N> as
    // the same Vec<T> a list<T> uses, so this holds the element type and length.
    ArrayN(String, usize),
    // HashMap<String, scalar>
    MapScalar(String),
    // HashMap<String, String>
    MapString,
    // HashMap<String, T> where T: SerifyModel
    MapNested(syn::Ident),
    // Any other named type - treated as nested SerifyModel
    Nested(syn::Ident),
}

fn classify(ty: &Type) -> TypeKind {
    match ty {
        Type::Path(tp) => {
            // Ignore leading `::` and crate qualifiers; look at the last segment only.
            let seg = match tp.path.segments.last() {
                Some(s) => s,
                None => {
                    return TypeKind::Nested(syn::Ident::new(
                        "Unknown",
                        proc_macro2::Span::call_site(),
                    ))
                }
            };
            let ident = seg.ident.to_string();
            match ident.as_str() {
                "u8" => TypeKind::Scalar("u8".into()),
                "u16" => TypeKind::Scalar("u16".into()),
                "u32" => TypeKind::Scalar("u32".into()),
                "u64" => TypeKind::Scalar("u64".into()),
                "u128" => TypeKind::Scalar("u128".into()),
                "i8" => TypeKind::Scalar("i8".into()),
                "i16" => TypeKind::Scalar("i16".into()),
                "i32" => TypeKind::Scalar("i32".into()),
                "i64" => TypeKind::Scalar("i64".into()),
                "i128" => TypeKind::Scalar("i128".into()),
                "f32" => TypeKind::Scalar("f32".into()),
                "f64" => TypeKind::Scalar("f64".into()),
                "bool" => TypeKind::Scalar("bool".into()),
                "String" => TypeKind::Str,
                "Vec" => {
                    if let PathArguments::AngleBracketed(ab) = &seg.arguments {
                        if let Some(GenericArgument::Type(inner)) = ab.args.first() {
                            // Vec<u8> is bytes; a list<uint8> is the same Vec<u8>
                            // and shares the FieldMap's Bytes value, so both land here.
                            if path_is_ident(inner, "u8") {
                                return TypeKind::Bytes;
                            }
                            if path_is_ident(inner, "String") {
                                return TypeKind::ListString;
                            }
                            // Vec<Vec<u8>> — a list whose elements are byte strings.
                            if let Type::Path(ip) = inner {
                                if let Some(is) = ip.path.segments.last() {
                                    if is.ident == "Vec" {
                                        if let PathArguments::AngleBracketed(iab) = &is.arguments {
                                            if let Some(GenericArgument::Type(inner2)) =
                                                iab.args.first()
                                            {
                                                if path_is_ident(inner2, "u8") {
                                                    return TypeKind::ListBytes;
                                                }
                                            }
                                        }
                                    }
                                    match is.ident.to_string().as_str() {
                                        "u16" | "u32" | "u64" | "u128" | "i8" | "i16" | "i32"
                                        | "i64" | "i128" | "f32" | "f64" | "bool" => {
                                            return TypeKind::ListScalar(is.ident.to_string());
                                        }
                                        _ => {}
                                    }
                                }
                            }
                            // Any other named element type is a nested model.
                            if let Type::Path(ip) = inner {
                                if let Some(is) = ip.path.segments.last() {
                                    return TypeKind::ListNested(is.ident.clone());
                                }
                            }
                        }
                    }
                    TypeKind::Nested(seg.ident.clone())
                }
                "HashMap" => {
                    if let PathArguments::AngleBracketed(ab) = &seg.arguments {
                        let args: Vec<_> = ab.args.iter().collect();
                        if args.len() == 2 {
                            if let GenericArgument::Type(key_ty) = args[0] {
                                if path_is_ident(key_ty, "String") {
                                    if let GenericArgument::Type(Type::Path(vp)) = args[1] {
                                        if let Some(vs) = vp.path.segments.last() {
                                            let s = vs.ident.to_string();
                                            match s.as_str() {
                                                "u8" | "u16" | "u32" | "u64" | "i8" | "i16"
                                                | "i32" | "i64" | "f32" | "f64" | "bool" => {
                                                    return TypeKind::MapScalar(s);
                                                }
                                                "String" => return TypeKind::MapString,
                                                _ => return TypeKind::MapNested(vs.ident.clone()),
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                    TypeKind::Nested(seg.ident.clone())
                }
                "Option" => {
                    if let PathArguments::AngleBracketed(ab) = &seg.arguments {
                        if let Some(GenericArgument::Type(inner)) = ab.args.first() {
                            if path_is_ident(inner, "String") {
                                return TypeKind::OptionString;
                            }
                            // Only Option<String> used to be recognised, so an
                            // Option<f32> fell through to Nested and failed to
                            // compile with "the trait bound `Option<f32>:
                            // SerifyField` is not satisfied" — which is why no
                            // Rust model could carry an optional<scalar>.
                            if let Type::Path(ip) = inner {
                                if let Some(is) = ip.path.segments.last() {
                                    match is.ident.to_string().as_str() {
                                        "u8" | "u16" | "u32" | "u64" | "u128" | "i8" | "i16"
                                        | "i32" | "i64" | "i128" | "f32" | "f64" | "bool" => {
                                            return TypeKind::OptionScalar(is.ident.to_string());
                                        }
                                        _ => return TypeKind::OptionNested(is.ident.clone()),
                                    }
                                }
                            }
                        }
                    }
                    TypeKind::Nested(seg.ident.clone())
                }
                _ => TypeKind::Nested(seg.ident.clone()),
            }
        }
        Type::Array(arr) => {
            if let Expr::Lit(el) = &arr.len {
                if let Lit::Int(n) = &el.lit {
                    if let Ok(len) = n.base10_parse::<usize>() {
                        if let Type::Path(ep) = &*arr.elem {
                            if let Some(es) = ep.path.segments.last() {
                                match es.ident.to_string().as_str() {
                                    "u8" | "u16" | "u32" | "u64" | "u128" | "i8" | "i16"
                                    | "i32" | "i64" | "i128" | "f32" | "f64" | "bool" => {
                                        return TypeKind::ArrayN(es.ident.to_string(), len);
                                    }
                                    _ => {}
                                }
                            }
                        }
                    }
                }
            }
            // Fall through to unknown
            TypeKind::Nested(syn::Ident::new(
                "_UnknownArray",
                proc_macro2::Span::call_site(),
            ))
        }
        _ => TypeKind::Nested(syn::Ident::new("_Unknown", proc_macro2::Span::call_site())),
    }
}

fn capitalize(s: &str) -> String {
    let mut c = s.chars();
    match c.next() {
        None => String::new(),
        Some(f) => f.to_uppercase().collect::<String>() + c.as_str(),
    }
}

fn path_is_ident(ty: &Type, ident: &str) -> bool {
    if let Type::Path(tp) = ty {
        tp.path.is_ident(ident)
    } else {
        false
    }
}

// Code generation

/// Returns `(from_expr, to_stmt)` for one field. `receiver` is the value the
/// to_field_map side reads from — `self` normally, `v` in remote mode.
fn type_ops(
    ty: &Type,
    key: &str,
    field_ident: &syn::Ident,
    receiver: &TokenStream2,
) -> syn::Result<(TokenStream2, TokenStream2)> {
    let missing = format!("missing {key}");

    Ok(match classify(ty) {
        TypeKind::Scalar(ref s) => {
            let getter = syn::Ident::new(&format!("get_{s}"), proc_macro2::Span::call_site());
            let setter = syn::Ident::new(&format!("set_{s}"), proc_macro2::Span::call_site());
            (
                quote! { fm.#getter(#key).ok_or(#missing)? },
                quote! { fm.#setter(#key, #receiver.#field_ident); },
            )
        }

        TypeKind::Str => (
            quote! { fm.get_string(#key).ok_or(#missing)?.to_string() },
            quote! { fm.set_string(#key, #receiver.#field_ident.clone()); },
        ),

        TypeKind::Bytes => (
            quote! { fm.get_bytes(#key).ok_or(#missing)?.to_vec() },
            quote! { fm.set_bytes(#key, #receiver.#field_ident.clone()); },
        ),

        TypeKind::ListString => (
            quote! { fm.get_list_string(#key).ok_or(#missing)?.clone() },
            quote! { fm.set_list_string(#key, #receiver.#field_ident.clone()); },
        ),

        TypeKind::ListScalar(ref s) => {
            let getter = syn::Ident::new(&format!("get_list_{s}"), proc_macro2::Span::call_site());
            let setter = syn::Ident::new(&format!("set_list_{s}"), proc_macro2::Span::call_site());
            (
                quote! { fm.#getter(#key).ok_or(#missing)?.clone() },
                quote! { fm.#setter(#key, #receiver.#field_ident.clone()); },
            )
        }

        TypeKind::ListBytes => (
            quote! { fm.get_list_bytes(#key).ok_or(#missing)?.clone() },
            quote! { fm.set_list_bytes(#key, #receiver.#field_ident.clone()); },
        ),

        // optional<scalar>: get_<scalar> already returns None for both an absent
        // field and a stored Null, which is exactly Option<T>'s semantics.
        TypeKind::OptionScalar(ref sc) => {
            let getter = syn::Ident::new(&format!("get_{sc}"), proc_macro2::Span::call_site());
            let setter = syn::Ident::new(&format!("set_{sc}"), proc_macro2::Span::call_site());
            (
                quote! { fm.#getter(#key) },
                quote! {
                    match #receiver.#field_ident {
                        ::std::option::Option::Some(v) => fm.#setter(#key, v),
                        ::std::option::Option::None    => fm.set_null(#key),
                    }
                },
            )
        }

        // Delegated to `SerifyField for Option<T>` rather than open-coded: the
        // derive cannot see whether `#nested_ty` is a product type or a
        // `#[serify(transparent)]` newtype, and only that type's own
        // `SerifyField` knows how it occupies a field.
        TypeKind::OptionNested(ref nested_ty) => (
            quote! {
                <::std::option::Option<#nested_ty> as ::serify::SerifyField>::serify_read(fm, #key)?
            },
            quote! {
                ::serify::SerifyField::serify_write(&#receiver.#field_ident, &mut fm, #key);
            },
        ),

        TypeKind::OptionString => (
            // get_optional_string returns Option<Option<&str>>:
            //   None        → field not in map (treat as None)
            //   Some(None)  → null
            //   Some(Some(s)) → value
            quote! {
                fm.get_optional_string(#key)
                    .unwrap_or(None)
                    .map(|s| s.to_string())
            },
            quote! { fm.set_optional_string(#key, #receiver.#field_ident.clone()); },
        ),

        TypeKind::ListNested(ref nested_ty) => (
            quote! {
                fm.get_list_struct(#key).ok_or(#missing)?
                    .iter()
                    .map(#nested_ty::from_field_map)
                    .collect::<::std::result::Result<::std::vec::Vec<_>, _>>()?
            },
            quote! {
                fm.set_list_struct(#key, #receiver.#field_ident.iter()
                    .map(serify::SerifyModel::to_field_map)
                    .collect());
            },
        ),

        // An array<T,N> shares the list<T> representation, so it reads and writes
        // through the same accessors and only checks the length.
        TypeKind::ArrayN(ref elem, len) => {
            let getter =
                syn::Ident::new(&format!("get_list_{elem}"), proc_macro2::Span::call_site());
            let setter =
                syn::Ident::new(&format!("set_list_{elem}"), proc_macro2::Span::call_site());
            let elem_ty = syn::parse_str::<syn::Type>(elem).unwrap();
            (
                quote! {{
                    let src = fm.#getter(#key).ok_or(#missing)?;
                    if src.len() != #len {
                        return Err(format!("field {}: expected {} elements, got {}", #key, #len, src.len()));
                    }
                    let mut out = [<#elem_ty as ::std::default::Default>::default(); #len];
                    out.copy_from_slice(&src[..#len]);
                    out
                }},
                quote! { fm.#setter(#key, #receiver.#field_ident.to_vec()); },
            )
        }

        TypeKind::MapScalar(ref s) => {
            let fv_path =
                syn::parse_str::<syn::Path>(&format!("serify::FieldValue::{}", capitalize(s)))
                    .unwrap();
            (
                quote! {
                    fm.get_map(#key).ok_or(#missing)?
                        .iter()
                        .map(|(k, v)| if let #fv_path(n) = v { Ok((k.clone(), *n)) }
                             else { Err(format!("field {}: expected {}, got {:?}", #key, #s, v)) })
                        .collect::<::std::result::Result<::std::collections::HashMap<_, _>, _>>()?
                },
                quote! {
                    fm.set_map(#key, #receiver.#field_ident.iter()
                        .map(|(k, v)| (k.clone(), #fv_path(*v)))
                        .collect());
                },
            )
        }

        TypeKind::MapString => (
            quote! {
                fm.get_map(#key).ok_or(#missing)?
                    .iter()
                    .map(|(k, v)| if let serify::FieldValue::Str(s) = v { Ok((k.clone(), s.clone())) }
                         else { Err(format!("field {}: expected string, got {:?}", #key, v)) })
                    .collect::<::std::result::Result<::std::collections::HashMap<_, _>, _>>()?
            },
            quote! {
                fm.set_map(#key, #receiver.#field_ident.iter()
                    .map(|(k, v)| (k.clone(), serify::FieldValue::Str(v.clone())))
                    .collect());
            },
        ),

        TypeKind::MapNested(ref nested_ty) => (
            quote! {
                fm.get_map(#key).ok_or(#missing)?
                    .iter()
                    .map(|(k, v)| {
                        if let serify::FieldValue::Struct(inner_fm) = v {
                            Ok((k.clone(), #nested_ty::from_field_map(inner_fm)?))
                        } else { Err(format!("field {}: expected struct", #key)) }
                    })
                    .collect::<::std::result::Result<::std::collections::HashMap<_, _>, _>>()?
            },
            quote! {
                fm.set_map(#key, #receiver.#field_ident.iter()
                    .map(|(k, v)| (k.clone(), serify::FieldValue::Struct(
                        ::std::boxed::Box::new(<#nested_ty as serify::SerifyModel>::to_field_map(v))
                    )))
                    .collect());
            },
        ),

        // A named type decides for itself how it occupies a field: a struct
        // nests, an enum becomes a variant. `serify_read`/`serify_write` resolve
        // to the blanket `SerifyField` impl for products, and to the enum
        // derive's inherent functions for sums — inherent wins over trait.
        TypeKind::Nested(ref nested_ty) => (
            quote! { <#nested_ty as serify::SerifyField>::serify_read(fm, #key)? },
            quote! { serify::SerifyField::serify_write(&#receiver.#field_ident, &mut fm, #key); },
        ),
    })
}
