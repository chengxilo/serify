/*
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package io.serify;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.*;

import java.io.*;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.lang.reflect.RecordComponent;
import java.util.*;
import java.util.function.Function;
import java.util.Objects;

/**
 * WorkerLib — Java implementation of the cross-language serialization test framework.
 */
public final class WorkerLib {

    // ─────────────────────────────────────────────────────────────────────────
    // FieldMap
    // ─────────────────────────────────────────────────────────────────────────

    public static final class FieldMap {
        private final Map<String, Object> fields = new LinkedHashMap<>();

        public byte     getU8(String k)  { return (Byte) fields.get(k); }
        public short    getU16(String k) { return (Short) fields.get(k); }
        public int      getU32(String k) { return (Integer) fields.get(k); }
        public long     getU64(String k) { return (Long) fields.get(k); }
        /** uint128/int128: Java has no 128-bit primitive type, so both are carried as
         *  BigInteger. Parsing them into a long (as this library used to) silently truncates
         *  every value above 2^64. */
        public BigInteger getBig(String k) { return (BigInteger) fields.get(k); }
        public byte     getI8(String k)  { return (Byte) fields.get(k); }
        public short    getI16(String k) { return (Short) fields.get(k); }
        public int      getI32(String k) { return (Integer) fields.get(k); }
        public long     getI64(String k) { return (Long) fields.get(k); }
        public float    getF32(String k) { return (Float) fields.get(k); }
        public double   getF64(String k) { return (Double) fields.get(k); }
        public boolean  getBool(String k) { return (Boolean) fields.get(k); }
        public String   getString(String k) { return (String) fields.get(k); }
        public byte[]   getBytes(String k)  { return (byte[]) fields.get(k); }
        @SuppressWarnings("unchecked")
        public List<String>  getListString(String k) { return (List<String>) fields.get(k); }
        public Optional<String> getOptionalString(String k) {
            return Optional.ofNullable((String) fields.get(k));
        }
        public FieldMap getStruct(String k) { return (FieldMap) fields.get(k); }
        public FieldMap getOptionalStruct(String k) { return (FieldMap) fields.get(k); }
        @SuppressWarnings("unchecked")
        public List<FieldMap> getListStruct(String k) { return (List<FieldMap>) fields.get(k); }
        @SuppressWarnings("unchecked")
        public List<Integer> getListU32(String k) { return (List<Integer>) fields.get(k); }
        @SuppressWarnings("unchecked")
        public List<Long> getListU64(String k) { return (List<Long>) fields.get(k); }
        @SuppressWarnings("unchecked")
        public List<Float> getListF32(String k) { return (List<Float>) fields.get(k); }
        @SuppressWarnings("unchecked")
        public Map<String, Object> getMap(String k) {
            return (Map<String, Object>) fields.getOrDefault(k, Map.of());
        }

        public void setU8(String k, byte v)     { fields.put(k, v); }
        public void setU16(String k, short v)   { fields.put(k, v); }
        public void setU32(String k, int v)     { fields.put(k, v); }
        public void setU64(String k, long v)    { fields.put(k, v); }
        public void setBig(String k, BigInteger v) { fields.put(k, v); }
        public void setListBig(String k, List<BigInteger> v) { fields.put(k, v); }
        public void setI8(String k, byte v)     { fields.put(k, v); }
        public void setI16(String k, short v)   { fields.put(k, v); }
        public void setI32(String k, int v)     { fields.put(k, v); }
        public void setI64(String k, long v)    { fields.put(k, v); }
        public void setF32(String k, float v)   { fields.put(k, v); }
        public void setF64(String k, double v)  { fields.put(k, v); }
        public void setBool(String k, boolean v) { fields.put(k, v); }
        public void setString(String k, String v) { fields.put(k, v); }
        public void setBytes(String k, byte[] v)  { fields.put(k, v); }
        public void setListString(String k, List<String> v) { fields.put(k, v); }
        public void setOptionalString(String k, String v)   { fields.put(k, v); }
        public void setStruct(String k, FieldMap v)        { fields.put(k, v); }
        public void setOptionalStruct(String k, FieldMap v) { fields.put(k, v); }
        public void setListStruct(String k, List<FieldMap> v) { fields.put(k, v); }
        public void setListU32(String k, List<Integer> v) { fields.put(k, v); }
        public void setListU64(String k, List<Long> v)    { fields.put(k, v); }
        public void setListF32(String k, List<Float> v)   { fields.put(k, v); }
        public void setMap(String k, Map<String, Object> v) { fields.put(k, v); }

        /** Stores a sum value: the active variant's tag and payload ({@code null} for a unit variant). */
        public void setVariant(String k, String tag, Object v) { fields.put(k, new Variant(tag, v)); }

        /** Returns the sum value stored at {@code k}. */
        public Variant getVariant(String k) {
            var v = fields.get(k);
            if (!(v instanceof Variant variant)) {
                throw new IllegalArgumentException("field \"" + k + "\" is not a variant");
            }
            return variant;
        }

        public Map<String, Object> raw() { return fields; }
    }

    /**
     * One arm of a sum: a tag and its decoded payload ({@code null} for a unit
     * variant). A sum field stores a Variant.
     */
    public static final class Variant {
        public final String tag;
        public final Object value;

        public Variant(String tag, Object value) {
            this.tag   = tag;
            this.value = value;
        }
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Schema
    // ─────────────────────────────────────────────────────────────────────────

    public static final class SchemaField {
        public final String name;
        public final String type;
        public final List<SchemaField> fields;
        public final Map<String, String> tags;
        /** Arms of a sum&lt;...&gt; type; empty for every other type. */
        public final List<SchemaVariant> variants;

        public SchemaField(String name, String type, List<SchemaField> fields, Map<String, String> tags) {
            this(name, type, fields, tags, List.of());
        }

        public SchemaField(String name, String type, List<SchemaField> fields, Map<String, String> tags,
                           List<SchemaVariant> variants) {
            this.name     = name;
            this.type     = type;
            this.fields   = fields   != null ? fields   : List.of();
            this.tags     = tags     != null ? tags     : Map.of();
            this.variants = variants != null ? variants : List.of();
        }

        /** Returns the sum arm named {@code tag}. */
        public SchemaVariant findVariant(String tag) {
            for (var sv : variants) if (sv.name.equals(tag)) return sv;
            throw new IllegalArgumentException("unknown variant \"" + tag + "\"");
        }
    }

    /** One arm of a sum: a tag and its payload schema ({@code null} for a unit variant). */
    public static final class SchemaVariant {
        public final String name;
        public final SchemaField payload;

        public SchemaVariant(String name, SchemaField payload) {
            this.name    = name;
            this.payload = payload;
        }
    }

    private static List<SchemaField> parseSchemaFields(JsonNode arr) {
        var result = new ArrayList<SchemaField>();
        for (var f : arr) {
            var name = f.path("name").asText();
            var type = f.path("type").asText();
            var nested = f.has("fields") ? parseSchemaFields(f.get("fields")) : List.<SchemaField>of();
            var tags = new LinkedHashMap<String, String>();
            if (f.has("tags")) f.get("tags").fields().forEachRemaining(e -> tags.put(e.getKey(), e.getValue().asText()));
            var variants = f.has("variants") ? parseSchemaVariants(f.get("variants")) : List.<SchemaVariant>of();
            result.add(new SchemaField(name, type, nested, tags, variants));
        }
        return result;
    }

    private static List<SchemaVariant> parseSchemaVariants(JsonNode arr) {
        var result = new ArrayList<SchemaVariant>();
        for (var v : arr) {
            var payloadNode = v.get("payload");
            SchemaField payload = null;
            if (payloadNode != null && payloadNode.isObject()) {
                // Reuse the field parser by wrapping the payload in a one-element array.
                var wrapper = new ObjectMapper().createArrayNode().add(payloadNode);
                payload = parseSchemaFields(wrapper).get(0);
            }
            result.add(new SchemaVariant(v.path("name").asText(), payload));
        }
        return result;
    }

    // ─────────────────────────────────────────────────────────────────────────
    // run()
    // ─────────────────────────────────────────────────────────────────────────

    /**
     * The protocol revision this library speaks. The runner requires an exact
     * match and refuses to start a worker reporting anything else.
     */
    private static final int PROTOCOL_VERSION = 2;

    // --- audit helpers --------------------------------------------------------

    /**
     * A snapshot of a byte[] field in a FieldMap, used for zero-copy detection.
     *
     * <p>This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this
     * directly — register your serialize/deserialize functions in a Suite and run() handles audit automatically.
     * Call this directly only if you want audit-style checks outside the runner.
     *
     * @param fm   the FieldMap containing the field
     * @param key  the field key
     * @param orig a clone of the original byte[] value
     */
    /** {@code orig} is a {@code byte[]}, or a {@link Variant} whose payload is one. */
    public record ByteSnap(FieldMap fm, String key, Object orig) {}

    /**
     * Recursively walks a FieldMap and collects snapshots of every byte[] value.
     *
     * <p>This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this
     * directly — register your serialize/deserialize functions in a Suite and run() handles audit automatically.
     * Call this directly only if you want audit-style checks outside the runner.
     *
     * @param fm    the FieldMap to walk
     * @param snaps the list to which ByteSnap entries are appended
     */
    public static void collectByteSnaps(FieldMap fm, List<ByteSnap> snaps) {
        var keys = new ArrayList<>(fm.raw().keySet());
        Collections.sort(keys);
        for (var key : keys) {
            var v = fm.raw().get(key);
            if (v instanceof byte[] bytes) {
                snaps.add(new ByteSnap(fm, key, bytes.clone()));
            } else if (v instanceof FieldMap nested) {
                collectByteSnaps(nested, snaps);
            } else if (v instanceof List<?> list) {
                for (var item : list) {
                    if (item instanceof FieldMap nested) collectByteSnaps(nested, snaps);
                }
            } else if (v instanceof Map<?, ?> map) {
                for (var item : map.values()) {
                    if (item instanceof FieldMap nested) collectByteSnaps(nested, snaps);
                }
            } else if (v instanceof Variant variant) {
                // The variant itself cannot alias, but its payload can: snapshot the
                // whole field so a zero-copy payload shows up as a change.
                if (variant.value instanceof byte[] payload) {
                    snaps.add(new ByteSnap(fm, key, new Variant(variant.tag, payload.clone())));
                } else if (variant.value instanceof FieldMap payloadFm) {
                    collectByteSnaps(payloadFm, snaps);
                }
            }
        }
    }

    /**
     * Compares two ObjectNodes and returns the field keys whose values differ.
     *
     * <p>This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this
     * directly — register your serialize/deserialize functions in a Suite and run() handles audit automatically.
     * Call this directly only if you want audit-style checks outside the runner.
     *
     * @param before the state before an operation
     * @param after  the state after an operation
     * @return the list of keys with differing values
     */
    public static List<String> dictDiffs(ObjectNode before, ObjectNode after) {
        var diffs = new ArrayList<String>();
        var allKeys = new TreeSet<String>();
        before.fieldNames().forEachRemaining(allKeys::add);
        after.fieldNames().forEachRemaining(allKeys::add);
        for (var k : allKeys) {
            var bv = before.has(k) ? before.get(k) : null;
            var av = after.has(k) ? after.get(k) : null;
            if (!Objects.equals(bv, av)) diffs.add(k);
        }
        return diffs;
    }

    /**
     * Detects whether any byte[] fields in the FieldMap alias the input buffer by XOR-flipping the buffer
     * and checking for changes, then restoring the original values.
     *
     * <p>This is a low-level audit helper. If you use the serify conformance runner, you do not need to call this
     * directly — register your serialize/deserialize functions in a Suite and run() handles audit automatically.
     * Call this directly only if you want audit-style checks outside the runner.
     *
     * @param fm  the FieldMap to inspect
     * @param buf the input buffer (will be temporarily XOR-flipped and restored)
     * @return the list of field keys that alias the input buffer
     */
    public static List<String> detectZeroCopy(FieldMap fm, byte[] buf) {
        if (buf.length == 0) return List.of();

        var snaps = new ArrayList<ByteSnap>();
        collectByteSnaps(fm, snaps);

        // XOR-flip
        for (int i = 0; i < buf.length; i++) buf[i] ^= 0xFF;

        var aliased = new ArrayList<String>();
        for (var snap : snaps) {
            var cur = snap.fm.raw().get(snap.key);
            if (snap.orig instanceof byte[] orig) {
                if (cur instanceof byte[] curBytes && !Arrays.equals(curBytes, orig)) aliased.add(snap.key);
            } else if (snap.orig instanceof Variant orig
                    && cur instanceof Variant curVariant
                    && orig.value instanceof byte[] origPayload
                    && curVariant.value instanceof byte[] curPayload
                    && !Arrays.equals(curPayload, origPayload)) {
                aliased.add(snap.key);
            }
        }

        // Restore
        for (var snap : snaps) snap.fm.raw().put(snap.key, snap.orig);

        return aliased;
    }

    /** One (serialize, deserialize) pair for a single format. */
    public record FormatPair(Function<FieldMap, byte[]> serialize,
                             Function<byte[], FieldMap> deserialize) {
        public FormatPair(Function<FieldMap, byte[]> serialize) {
            this(serialize, null);
        }
    }

    /** Single-type worker: handles whatever type/format the runner asks for. */
    public static void run(
            Function<FieldMap, byte[]> serialize,
            Function<byte[], FieldMap> deserialize) {
        runSuite(Map.of(), (t, f) -> new FormatPair(serialize, deserialize));
    }

    /**
     * Multi-type worker. `suite` maps type name -> format name -> FormatPair. A
     * (type, format) that is not registered is reported to the runner as SKIPPED
     * rather than failing the run.
     */
    public static void runSuite(Map<String, Map<String, FormatPair>> suite) {
        runSuite(suite, (t, f) -> {
            var formats = suite.get(t);
            return formats == null ? null : formats.get(f);
        });
    }

    private static void runSuite(
            Map<String, Map<String, FormatPair>> suite,
            java.util.function.BiFunction<String, String, FormatPair> resolve) {

        Function<FieldMap, byte[]> serialize = null;
        Function<byte[], FieldMap> deserialize = null;

        var mapper = new ObjectMapper();
        var schema = new ArrayList<SchemaField>();
        var out    = new PrintStream(new BufferedOutputStream(System.out), false, java.nio.charset.StandardCharsets.UTF_8);
        boolean auditEnabled = false;

        try (var br = new BufferedReader(new InputStreamReader(System.in, java.nio.charset.StandardCharsets.UTF_8))) {
            String line;
            while ((line = br.readLine()) != null) {
                line = line.strip();
                if (line.isEmpty()) continue;

                JsonNode msg = mapper.readTree(line);
                String op    = msg.path("op").asText("");
                String id    = msg.path("id").asText("");

                switch (op) {
                    case "ping" -> {
                        // Health check: report liveness and the protocol revision
                        // this library speaks. Binds nothing.
                        var resp = mapper.createObjectNode();
                        resp.put("op", "ping");
                        resp.put("status", "OK");
                        resp.put("protocol_version", PROTOCOL_VERSION);
                        out.println(mapper.writeValueAsString(resp));
                        out.flush();
                    }
                    case "bind" -> {
                        schema.clear();
                        if (msg.has("schema")) schema.addAll(parseSchemaFields(msg.get("schema")));
                        auditEnabled = msg.path("audit").asBoolean(false);
                        var pair = resolve.apply(msg.path("type").asText(""), msg.path("format").asText(""));
                        var resp = mapper.createObjectNode();
                        resp.put("op", "bind");
                        if (pair == null) {
                            serialize = null;
                            deserialize = null;
                            resp.put("status", "SKIPPED");
                        } else {
                            serialize = pair.serialize();
                            deserialize = pair.deserialize();
                        }
                        out.println(mapper.writeValueAsString(resp));
                        out.flush();
                    }
                    case "serialize" -> {
                        try {
                            if (serialize == null) {
                                var r = mapper.createObjectNode();
                                r.put("id", id);
                                r.put("op", "serialize");
                                if (deserialize != null) {
                                    r.put("status", "SKIPPED");
                                    r.put("reason", "direction not registered");
                                } else {
                                    r.put("status", "ERROR");
                                    r.put("error", "no serializer configured (call bind first)");
                                }
                                out.println(mapper.writeValueAsString(r));
                                out.flush();
                                break;
                            }
                            var fm  = decodeFieldMap(msg.path("data"), schema);

                            ObjectNode before = auditEnabled ? encodeFieldMap(fm, schema, mapper) : null;

                            byte[] outputBytes = serialize.apply(fm);
                            var hex = bytesToHex(outputBytes);

                            var r = mapper.createObjectNode();
                            r.put("id", id); r.put("op", "serialize");
                            r.put("status", "OK"); r.put("hex", hex);

                            if (auditEnabled && before != null) {
                                var audit = mapper.createObjectNode();

                                // Mutation
                                var after = encodeFieldMap(fm, schema, mapper);
                                var diffs = dictDiffs(before, after);
                                if (!diffs.isEmpty()) audit.set("mutations", mapper.valueToTree(diffs));

                                // Output zero-copy: does returned buffer alias model fields?
                                if (outputBytes.length > 0) {
                                    var beforeClone = encodeFieldMap(fm, schema, mapper);
                                    for (int i = 0; i < outputBytes.length; i++) outputBytes[i] ^= 0xFF;
                                    var afterFlip = encodeFieldMap(fm, schema, mapper);
                                    for (int i = 0; i < outputBytes.length; i++) outputBytes[i] ^= 0xFF; // restore
                                    var ozc = dictDiffs(beforeClone, afterFlip);
                                    if (!ozc.isEmpty()) audit.set("output_zero_copy_fields", mapper.valueToTree(ozc));
                                }

                                // Stability: serialize again, compare output.
                                try {
                                    byte[] b2 = serialize.apply(fm);
                                    if (!hex.equals(bytesToHex(b2)))
                                        audit.put("stable", false);
                                } catch (Exception e) {
                                    audit.put("stable", false);
                                }

                                if (!audit.isEmpty()) r.set("audit", audit);
                            }

                            out.println(mapper.writeValueAsString(r));
                        } catch (Exception e) {
                            emitError(out, mapper, id, "serialize", "ERROR", e.getMessage());
                        }
                        out.flush();
                    }
                    case "deserialize" -> {
                        try {
                            if (deserialize == null) {
                                var r = mapper.createObjectNode();
                                r.put("id", id);
                                r.put("op", "deserialize");
                                if (serialize != null) {
                                    r.put("status", "SKIPPED");
                                    r.put("reason", "direction not registered");
                                } else {
                                    r.put("status", "ERROR");
                                    r.put("error", "no deserializer configured (call bind first)");
                                }
                                out.println(mapper.writeValueAsString(r));
                                out.flush();
                                break;
                            }
                            var bytes = hexToBytes(msg.path("hex").asText());
                            byte[] bufSnapshot = auditEnabled ? bytes.clone() : null;

                            var fm = deserialize.apply(bytes);

                            var r = mapper.createObjectNode();
                            r.put("id", id); r.put("op", "deserialize");
                            r.put("status", "OK"); r.set("data", encodeFieldMap(fm, schema, mapper));

                            if (auditEnabled && bufSnapshot != null) {
                                var audit = mapper.createObjectNode();

                                // Deserialize stability: re-deserialize from a fresh clone.
                                try {
                                    byte[] buf2 = bufSnapshot.clone();
                                    FieldMap fm2 = deserialize.apply(buf2);
                                    var beforeData = encodeFieldMap(fm, schema, mapper);
                                    var afterData = encodeFieldMap(fm2, schema, mapper);
                                    var dserDiffs = dictDiffs(beforeData, afterData);
                                    if (!dserDiffs.isEmpty()) audit.put("deser_stable", false);
                                } catch (Exception e) {
                                    audit.put("deser_stable", false);
                                }

                                // Input-buffer mutation
                                if (!Arrays.equals(bufSnapshot, bytes))
                                    audit.put("input_mutated", true);

                                // Zero-copy: XOR-flip buffer, check FieldMap, restore.
                                var zc = detectZeroCopy(fm, bytes);
                                if (!zc.isEmpty()) audit.set("zero_copy_fields", mapper.valueToTree(zc));

                                if (!audit.isEmpty()) r.set("audit", audit);
                            }

                            out.println(mapper.writeValueAsString(r));
                        } catch (Exception e) {
                            emitError(out, mapper, id, "deserialize", "ERROR", e.getMessage());
                        }
                        out.flush();
                    }
                    case "exit" -> System.exit(0);
                }
            }
        } catch (IOException e) {
            System.exit(1);
        }
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Decode
    // ─────────────────────────────────────────────────────────────────────────

    public static FieldMap decodeFieldMap(JsonNode data, List<SchemaField> schema) {
        var fm = new FieldMap();
        for (var sf : schema) {
            var el = data.get(sf.name);
            if (el == null || el.isMissingNode()) continue;
            decodeField(fm, sf, el);
        }
        return fm;
    }

    private static void decodeField(FieldMap fm, SchemaField sf, JsonNode el) {
        var name = sf.name;
        var type = sf.type;
        switch (type) {
            case "uint8"  -> fm.setU8(name, (byte) el.intValue());
            case "uint16" -> fm.setU16(name, (short) el.intValue());
            case "uint32" -> fm.setU32(name, el.intValue());
            case "uint64" -> fm.setU64(name, Long.parseUnsignedLong(el.asText()));
            case "uint128", "int128" -> fm.setBig(name, new BigInteger(el.asText()));
            case "int8"  -> fm.setI8(name, (byte) el.intValue());
            case "int16" -> fm.setI16(name, (short) el.intValue());
            case "int32" -> fm.setI32(name, el.intValue());
            case "int64" -> fm.setI64(name, Long.parseLong(el.asText()));
            case "float32" -> {
                byte[] b = hexToBytes(el.asText());
                fm.setF32(name, ByteBuffer.wrap(b).order(ByteOrder.LITTLE_ENDIAN).getFloat());
            }
            case "float64" -> {
                byte[] b = hexToBytes(el.asText());
                fm.setF64(name, ByteBuffer.wrap(b).order(ByteOrder.LITTLE_ENDIAN).getDouble());
            }
            case "bool"   -> fm.setBool(name, el.booleanValue());
            case "string" -> fm.setString(name, el.asText());
            case "bytes"  -> fm.setBytes(name, hexToBytes(el.asText()));
            case "struct" -> fm.setStruct(name, decodeFieldMap(el, sf.fields));
            default -> {
                if (type.startsWith("list<"))    { decodeList(fm, sf, type.substring(5, type.length()-1), el); }
                else if (type.startsWith("optional<")) { decodeOptional(fm, sf, type.substring(9, type.length()-1), el); }
                else if (type.startsWith("array<")) {
                    // An array<T,N> is a list whose length the schema fixes, so it
                    // shares decodeList outright and adds only the length check. A
                    // separate representation is what pinned array<T,N> to int[4]
                    // — it silently truncated anything longer.
                    var parts = splitArrayType(type);
                    decodeList(fm, sf, parts.elem(), el);
                    int got = ((List<?>) fm.raw().get(name)).size();
                    if (got != parts.len())
                        throw new IllegalArgumentException(
                            "array " + name + ": expected " + parts.len() + " elements, got " + got);
                }
                // enum<a,b,c>: the variant name travels as a string.
                else if (type.startsWith("enum<")) { fm.setString(name, el.asText()); }
                else if (type.startsWith("sum<")) { decodeSum(fm, sf, el); }
                else if (type.startsWith("map<")) {
                    var parts = splitMapTypes(type);
                    fm.setMap(name, decodeMap(parts[1], sf.fields, el));
                }
                // Falling off the chain left the field absent from the FieldMap,
                // which surfaces far downstream as a missing value rather than as
                // "this library does not know that type".
                else throw new IllegalArgumentException("unknown type \"" + type + "\"");
            }
        }
    }

    /**
     * Decodes a sum wire value {tag: payload} (payload null for a unit variant)
     * into a Variant, decoding the payload per the variant's own schema.
     */
    private static void decodeSum(FieldMap fm, SchemaField sf, JsonNode el) {
        if (el.size() != 1) {
            throw new IllegalArgumentException("sum must name exactly one variant, got " + el.size());
        }
        var entry = el.fields().next();
        var sv = sf.findVariant(entry.getKey());
        if (sv.payload == null) { fm.setVariant(sf.name, entry.getKey(), null); return; }
        var tmp = new FieldMap();
        decodeField(tmp, sv.payload, entry.getValue());
        fm.setVariant(sf.name, entry.getKey(), tmp.raw().get(sv.payload.name));
    }

    /** The element type and length of an array&lt;T,N&gt;. */
    private record ArrayType(String elem, int len) {}

    /** Split "array&lt;T,N&gt;" into its element type and length. */
    private static ArrayType splitArrayType(String type) {
        var inner = type.substring(6, type.length() - 1);
        int comma = inner.lastIndexOf(',');
        return new ArrayType(inner.substring(0, comma).trim(),
                             Integer.parseInt(inner.substring(comma + 1).trim()));
    }

    /** Element types a list may carry: every scalar, plus a nested struct. */
    private static final Set<String> LIST_ELEMS = Set.of(
        "uint8", "uint16", "uint32", "uint64", "uint128",
        "int8", "int16", "int32", "int64", "int128",
        "float32", "float64", "bool", "string", "bytes",
        "struct");

    /**
     * Decodes every element through {@link #decodeField}, so a list supports
     * exactly the element types a bare field does. This used to carry its own
     * switch with no default arm at all, so a list of any element type it did not
     * name — uint8, uint16, int8, int16, int32, int64, float64, bool, bytes —
     * left the field silently absent from the FieldMap rather than raising.
     */
    private static void decodeList(FieldMap fm, SchemaField sf, String elem, JsonNode el) {
        if (!LIST_ELEMS.contains(elem))
            throw new IllegalArgumentException("unsupported list element type \"" + elem + "\"");
        if (elem.equals("struct")) {
            var list = new ArrayList<FieldMap>();
            el.forEach(x -> list.add(decodeFieldMap(x, sf.fields)));
            fm.setListStruct(sf.name, list);
            return;
        }
        var elemSf = new SchemaField("e", elem, sf.fields, sf.tags);
        var out = new ArrayList<Object>();
        el.forEach(item -> {
            var tmp = new FieldMap();
            decodeField(tmp, elemSf, item);
            out.add(tmp.raw().get("e"));
        });
        fm.raw().put(sf.name, out);
    }

    private static void decodeOptional(FieldMap fm, SchemaField sf, String elem, JsonNode el) {
        if (el.isNull()) {
            switch (elem) {
                case "string" -> fm.setOptionalString(sf.name, null);
                case "struct" -> fm.setOptionalStruct(sf.name, null);
                default -> fm.raw().put(sf.name, null);
            }
            return;
        }
        switch (elem) {
            case "string" -> fm.setOptionalString(sf.name, el.asText());
            case "struct" -> fm.setOptionalStruct(sf.name, decodeFieldMap(el, sf.fields));
            default -> {
                var inner = new SchemaField(sf.name, elem, sf.fields, sf.tags);
                var tmp = new FieldMap();
                decodeField(tmp, inner, el);
                var v = tmp.raw().get(sf.name);
                if (v != null) fm.raw().put(sf.name, v);
            }
        }
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Map helpers
    // ─────────────────────────────────────────────────────────────────────────

    private static String[] splitMapTypes(String type) {
        var inner = type.substring(4, type.length() - 1); // strip "map<" and ">"
        int depth = 0;
        for (int i = 0; i < inner.length(); i++) {
            char ch = inner.charAt(i);
            if (ch == '<' || ch == '[') depth++;
            else if (ch == '>' || ch == ']') depth--;
            else if (ch == ',' && depth == 0)
                return new String[]{inner.substring(0, i).trim(), inner.substring(i + 1).trim()};
        }
        return new String[]{"", inner.trim()};
    }

    @SuppressWarnings("unchecked")
    private static Map<String, Object> decodeMap(String valType,
            List<SchemaField> nestedSchema, JsonNode el) {
        var m = new LinkedHashMap<String, Object>();
        var it = el.fields();
        while (it.hasNext()) {
            var kv = it.next();
            m.put(kv.getKey(), switch (valType) {
                case "struct" -> decodeFieldMap(kv.getValue(), nestedSchema);
                case "uint8", "int8" -> (byte) kv.getValue().asInt();
                case "uint16", "int16" -> (short) kv.getValue().asInt();
                case "uint32" -> (int) kv.getValue().asLong();
                case "int32" -> kv.getValue().asInt();
                case "bool" -> kv.getValue().asBoolean();
                case "string" -> kv.getValue().asText();
                case "uint64" -> Long.parseUnsignedLong(kv.getValue().asText());
                case "int64" -> Long.parseLong(kv.getValue().asText());
                case "uint128", "int128" -> new BigInteger(kv.getValue().asText());
                case "float32" -> {
                    byte[] b = hexToBytes(kv.getValue().asText());
                    yield ByteBuffer.wrap(b).order(ByteOrder.LITTLE_ENDIAN).getFloat();
                }
                case "float64" -> {
                    byte[] b = hexToBytes(kv.getValue().asText());
                    yield ByteBuffer.wrap(b).order(ByteOrder.LITTLE_ENDIAN).getDouble();
                }
                case "bytes" -> hexToBytes(kv.getValue().asText());
                default -> (Object) kv.getValue();
            });
        }
        return m;
    }

    @SuppressWarnings("unchecked")
    private static JsonNode encodeMap(SchemaField sf, String valType, Object v, ObjectMapper mapper) {
        var src = (Map<String, Object>) v;
        var an = mapper.createObjectNode();
        var keys = new ArrayList<>(src.keySet());
        Collections.sort(keys);
        for (var k : keys) {
            var item = src.get(k);
            an.set(k, switch (valType) {
                case "struct" -> encodeFieldMap((FieldMap) item, sf.fields, mapper);
                case "uint8" -> new IntNode(Byte.toUnsignedInt((Byte) item));
                case "uint16" -> new IntNode(Short.toUnsignedInt((Short) item));
                case "uint32" -> new LongNode(Integer.toUnsignedLong(((Number) item).intValue()));
                case "uint64" -> new TextNode(Long.toUnsignedString((Long) item));
                case "int64" -> new TextNode(Long.toString((Long) item));
                case "uint128", "int128" -> new TextNode(((BigInteger) item).toString());
                case "float32" -> {
                    var bb = ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN);
                    bb.putFloat((Float) item); yield new TextNode(bytesToHex(bb.array()));
                }
                case "float64" -> {
                    var bb = ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN);
                    bb.putDouble((Double) item); yield new TextNode(bytesToHex(bb.array()));
                }
                case "bytes" -> new TextNode(bytesToHex((byte[]) item));
                default -> mapper.valueToTree(item);
            });
        }
        return an;
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Encode
    // ─────────────────────────────────────────────────────────────────────────

    public static ObjectNode encodeFieldMap(FieldMap fm, List<SchemaField> schema, ObjectMapper mapper) {
        var node = mapper.createObjectNode();
        for (var sf : schema) {
            var v = fm.raw().get(sf.name);
            if (v == null && !fm.raw().containsKey(sf.name)) continue;
            node.set(sf.name, encodeField(sf, v, mapper));
        }
        return node;
    }

    /** Inverse of decodeSum: a Variant becomes {tag: payload}. */
    private static JsonNode encodeSum(SchemaField sf, Object v, ObjectMapper mapper) {
        if (!(v instanceof Variant variant)) {
            throw new IllegalArgumentException("expected a Variant for sum field \"" + sf.name + "\"");
        }
        var sv   = sf.findVariant(variant.tag);
        var node = mapper.createObjectNode();
        if (sv.payload == null) node.putNull(variant.tag);
        else node.set(variant.tag, encodeField(sv.payload, variant.value, mapper));
        return node;
    }

    @SuppressWarnings("unchecked")
    private static JsonNode encodeField(SchemaField sf, Object v, ObjectMapper mapper) {
        var type = sf.type;
        return switch (type) {
            // Java has no unsigned primitives: a u8/u16 at its max is stored
            // as -1, so encode through the unsigned view.
            case "uint8" -> new IntNode(Byte.toUnsignedInt((Byte) v));
            case "uint16" -> new IntNode(Short.toUnsignedInt((Short) v));
            case "int8","int16","int32" -> mapper.valueToTree(v);
            case "uint32" -> new LongNode(Integer.toUnsignedLong((Integer) v));
            case "uint64" -> new TextNode(Long.toUnsignedString((Long) v));
            case "int64" -> new TextNode(Long.toString((Long) v));
            case "uint128", "int128" -> new TextNode(((BigInteger) v).toString());
            case "float32" -> {
                var bb = ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN);
                bb.putFloat((Float) v); yield new TextNode(bytesToHex(bb.array()));
            }
            case "float64" -> {
                var bb = ByteBuffer.allocate(8).order(ByteOrder.LITTLE_ENDIAN);
                bb.putDouble((Double) v); yield new TextNode(bytesToHex(bb.array()));
            }
            case "bool"   -> BooleanNode.valueOf((Boolean) v);
            case "string" -> new TextNode((String) v);
            case "bytes"  -> new TextNode(bytesToHex((byte[]) v));
            case "struct" -> encodeFieldMap((FieldMap) v, sf.fields, mapper);
            default -> {
                if (type.startsWith("list<")) yield encodeList(sf, type.substring(5, type.length()-1), v, mapper);
                if (type.startsWith("optional<")) yield encodeOptional(sf, type.substring(9, type.length()-1), v, mapper);
                if (type.startsWith("array<")) yield encodeList(sf, splitArrayType(type).elem(), v, mapper);
                if (type.startsWith("enum<")) yield new TextNode((String) v);
                if (type.startsWith("sum<")) yield encodeSum(sf, v, mapper);
                if (type.startsWith("map<")) {
                    var parts = splitMapTypes(type);
                    yield encodeMap(sf, parts[1], v, mapper);
                }
                throw new IllegalArgumentException("unknown type \"" + type + "\"");
            }
        };
    }

    /**
     * Inverse of {@link #decodeList}: every element goes back out through
     * {@link #encodeField}, so the two directions cannot cover different element
     * types. The old version fell through to {@code mapper.valueToTree(v)} for
     * anything it did not name, which silently emitted the wrong wire form.
     */
    @SuppressWarnings("unchecked")
    private static JsonNode encodeList(SchemaField sf, String elem, Object v, ObjectMapper mapper) {
        if (!LIST_ELEMS.contains(elem))
            throw new IllegalArgumentException("unsupported list element type \"" + elem + "\"");
        var an = mapper.createArrayNode();
        if (elem.equals("struct")) {
            ((List<FieldMap>) v).forEach(fm -> an.add(encodeFieldMap(fm, sf.fields, mapper)));
            return an;
        }
        var elemSf = new SchemaField("e", elem, sf.fields, sf.tags);
        ((List<?>) v).forEach(x -> an.add(encodeField(elemSf, x, mapper)));
        return an;
    }

    private static JsonNode encodeOptional(SchemaField sf, String elem, Object v, ObjectMapper mapper) {
        if (v == null) return NullNode.instance;
        return switch (elem) {
            case "string" -> new TextNode((String) v);
            case "struct" -> encodeFieldMap((FieldMap) v, sf.fields, mapper);
            default -> encodeField(new SchemaField(sf.name, elem, sf.fields, sf.tags), v, mapper);
        };
    }

    // ─────────────────────────────────────────────────────────────────────────
    // Hex utilities
    // ─────────────────────────────────────────────────────────────────────────

    public static String bytesToHex(byte[] b) {
        var sb = new StringBuilder(b.length * 2);
        for (byte x : b) sb.append(String.format("%02x", x & 0xff));
        return sb.toString();
    }

    public static byte[] hexToBytes(String hex) {
        int len = hex.length();
        byte[] out = new byte[len / 2];
        for (int i = 0; i < len; i += 2)
            out[i / 2] = (byte) Integer.parseInt(hex, i, i + 2, 16);
        return out;
    }

    private static void emitError(PrintStream out, ObjectMapper mapper, String id, String op, String status, String error) {
        try {
            var r = mapper.createObjectNode();
            r.put("id", id); r.put("op", op); r.put("status", status); r.put("error", error);
            out.println(mapper.writeValueAsString(r));
        } catch (Exception ignored) {}
    }

    // --- Model annotations ---------------------------------------------------

    /** Marks a class as a Serify model. */
    @java.lang.annotation.Retention(java.lang.annotation.RetentionPolicy.RUNTIME)
    @java.lang.annotation.Target(java.lang.annotation.ElementType.TYPE)
    public @interface SerifyModel {}

    /** Maps a field to a serify schema key.  Defaults to the field name. */
    @java.lang.annotation.Retention(java.lang.annotation.RetentionPolicy.RUNTIME)
    @java.lang.annotation.Target(java.lang.annotation.ElementType.FIELD)
    public @interface SerifyField {
        /** Override the schema key (default: field name). */
        String value() default "";
    }

    /** Reflection helpers for @SerifyModel / @SerifyField. */
    public static final class SerifyModelHelper {
        /**
         * Default schema key for a field: {@code entryId} becomes {@code entry_id}.
         * Java fields are camelCase and schema keys are snake_case, so bridging
         * the two is the library's job — give @SerifyField a value to override.
         */
        private static String defaultKey(String fieldName) {
            var sb = new StringBuilder(fieldName.length() + 4);
            for (int i = 0; i < fieldName.length(); i++) {
                char c = fieldName.charAt(i);
                if (Character.isUpperCase(c)) {
                    if (i != 0) sb.append('_');
                    sb.append(Character.toLowerCase(c));
                } else {
                    sb.append(c);
                }
            }
            return sb.toString();
        }

        /** Convert a @SerifyModel instance to a FieldMap. */
        public static FieldMap toFieldMap(Object obj) {
            var fm = new FieldMap();
            for (var field : obj.getClass().getDeclaredFields()) {
                if (!field.isAnnotationPresent(SerifyField.class)) continue;
                var ann   = field.getAnnotation(SerifyField.class);
                var key   = !ann.value().isEmpty() ? ann.value() : defaultKey(field.getName());
                try {
                    field.setAccessible(true);
                    var val = field.get(obj);
                    // A sealed interface is Java's sum type, so a field declared
                    // with one is a sum — see toVariant.
                    if (field.getType().isSealed()) fm.raw().put(key, toVariant(field.getType(), val));
                    else setFieldMapValue(fm, key, val);
                } catch (IllegalAccessException ignored) {}
            }
            return fm;
        }

        /** Populate a @SerifyModel from a FieldMap. */
        public static <T> T fromFieldMap(FieldMap fm, Class<T> cls) {
            try {
                var obj = cls.getDeclaredConstructor().newInstance();
                for (var field : cls.getDeclaredFields()) {
                    if (!field.isAnnotationPresent(SerifyField.class)) continue;
                    var ann = field.getAnnotation(SerifyField.class);
                    var key = !ann.value().isEmpty() ? ann.value() : defaultKey(field.getName());
                    if (!fm.raw().containsKey(key)) continue;
                    var val = fm.raw().get(key);
                    field.setAccessible(true);
                    if (field.getType().isSealed() && val instanceof Variant v) {
                        field.set(obj, fromVariant(field.getType(), v));
                    } else {
                        field.set(obj, val);
                    }
                }
                return obj;
            } catch (Exception e) {
                throw new RuntimeException("Failed to construct " + cls.getName(), e);
            }
        }

        // ── sum ───────────────────────────────────────────────────────────
        //
        // A sealed interface is Java's sum type, and it is all the binding needs:
        // getPermittedSubclasses() names the arms and each arm's record
        // components give its payload. No converter, no registration.
        //
        // The arity rule is the same one every serify binding uses, because a
        // sum is a sum-of-products:
        //
        //     0 components -> a unit variant, no payload
        //     1 component  -> that component's value is the payload
        //     N components -> the payload is a struct holding the N components

        /** Schema tag for an arm: its simple name in snake_case. */
        private static String armTag(Class<?> arm) {
            return defaultKey(arm.getSimpleName());
        }

        private static RecordComponent[] armComponents(Class<?> arm) {
            var rcs = arm.getRecordComponents();
            if (rcs == null) {
                throw new IllegalArgumentException(
                        arm.getName() + " is a sum arm, so it must be a record — "
                        + "its components are the variant's payload");
            }
            return rcs;
        }

        private static Object component(Object arm, RecordComponent rc) {
            try {
                var accessor = rc.getAccessor();
                accessor.setAccessible(true);
                return accessor.invoke(arm);
            } catch (ReflectiveOperationException e) {
                throw new RuntimeException("reading sum payload " + rc.getName(), e);
            }
        }

        /** Render the active arm of a sealed hierarchy as a Variant. */
        private static Variant toVariant(Class<?> sealed, Object val) {
            if (val == null) throw new IllegalArgumentException(sealed.getSimpleName() + " is unset");
            var arm = val.getClass();
            if (!sealed.isAssignableFrom(arm)) {
                throw new IllegalArgumentException(arm.getName() + " is not a " + sealed.getSimpleName());
            }
            var rcs = armComponents(arm);
            if (rcs.length == 0) return new Variant(armTag(arm), null);
            if (rcs.length == 1) {
                var payload = component(val, rcs[0]);
                // A single payload that is itself a model travels as a struct.
                if (rcs[0].getType().isAnnotationPresent(SerifyModel.class)) {
                    payload = toFieldMap(payload);
                }
                return new Variant(armTag(arm), payload);
            }
            var payload = new FieldMap();
            for (var rc : rcs) setFieldMapValue(payload, defaultKey(rc.getName()), component(val, rc));
            return new Variant(armTag(arm), payload);
        }

        /** Rebuild the active arm of a sealed hierarchy from a Variant. */
        private static Object fromVariant(Class<?> sealed, Variant v) {
            for (var arm : sealed.getPermittedSubclasses()) {
                if (!armTag(arm).equals(v.tag)) continue;
                var rcs = armComponents(arm);
                var args = new Object[rcs.length];
                if (rcs.length == 1) {
                    args[0] = rcs[0].getType().isAnnotationPresent(SerifyModel.class)
                            ? fromFieldMap((FieldMap) v.value, rcs[0].getType())
                            : v.value;
                } else if (rcs.length > 1) {
                    if (!(v.value instanceof FieldMap payload)) {
                        throw new IllegalArgumentException(
                                "variant \"" + v.tag + "\" needs a struct payload");
                    }
                    for (int i = 0; i < rcs.length; i++) {
                        args[i] = payload.raw().get(defaultKey(rcs[i].getName()));
                    }
                }
                try {
                    var types = new Class<?>[rcs.length];
                    for (int i = 0; i < rcs.length; i++) types[i] = rcs[i].getType();
                    var ctor = arm.getDeclaredConstructor(types);
                    ctor.setAccessible(true);
                    return ctor.newInstance(args);
                } catch (ReflectiveOperationException e) {
                    throw new RuntimeException("constructing sum arm " + arm.getName(), e);
                }
            }
            var known = new StringBuilder();
            for (var arm : sealed.getPermittedSubclasses()) {
                if (known.length() > 0) known.append(", ");
                known.append(armTag(arm));
            }
            throw new IllegalArgumentException(
                    "unknown variant \"" + v.tag + "\" (declared: " + known + ")");
        }

        @SuppressWarnings("unchecked")
        private static void setFieldMapValue(FieldMap fm, String key, Object val) {
            // A null is a field, not an absent one: it is how an optional<T>
            // says "no value", and skipping it would silently omit the key.
            if (val == null) { fm.raw().put(key, null); return; }
            if (val instanceof Byte v)          fm.setU8(key, v);
            else if (val instanceof Short v)    fm.setU16(key, v);
            else if (val instanceof Integer v)  fm.setU32(key, v);
            else if (val instanceof Long v)     fm.setU64(key, v);
            else if (val instanceof Float v)    fm.setF32(key, v);
            else if (val instanceof Double v)   fm.setF64(key, v);
            else if (val instanceof Boolean v)  fm.setBool(key, v);
            else if (val instanceof String v)   fm.setString(key, v);
            else if (val instanceof byte[] v)   fm.setBytes(key, v);
            else if (val instanceof BigInteger v) fm.setBig(key, v);
            else if (val instanceof Variant v)  fm.raw().put(key, v);
            else if (val instanceof List<?> v) {
                // Store the list as-is: the schema, not the first element,
                // decides the element type on the wire. Guessing from v.get(0)
                // meant an *empty* list stored nothing at all — the field simply
                // vanished from the FieldMap — and any list of booleans,
                // BigIntegers, Shorts, Doubles or byte[] fell through the same way.
                fm.raw().put(key, v);
            }
            else if (val instanceof FieldMap v) fm.setStruct(key, v);
            else if (val instanceof Map<?,?> v) fm.setMap(key, (Map<String, Object>) v);
            else fm.setString(key, val.toString());
        }
    }
}
