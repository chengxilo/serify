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

/**
 * Happy-path Java worker: {@code all_types} in binary and json.
 *
 * <p>Go is the --ref language and owns both byte layouts; see
 * test/cases/happy/go/type.go. The json format must match Go's encoding/json
 * byte-for-byte (with SetEscapeHTML(false)): schema field order, map keys in
 * UTF-8 byte order, []byte as base64, floats in shortest form without a
 * trailing .0, and U+2028/U+2029 escaped (Go escapes those unconditionally).
 *
 * <p>Java has no unsigned types: u32 lives in an int and u64 in a long as bit
 * patterns, printed with Integer/Long.toUnsignedString.
 */

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.serify.WorkerLib;
import io.serify.WorkerLib.FieldMap;
import io.serify.WorkerLib.FormatPair;
import io.serify.WorkerLib.TypeEntry;

import java.io.ByteArrayOutputStream;
import java.math.BigDecimal;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Base64;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

public final class Worker {

    private static final List<String> STATUS_VARIANTS =
            List.of("pending", "paid", "shipped", "delivered", "cancelled");

    private static byte statusOrdinal(String s) {
        int i = STATUS_VARIANTS.indexOf(s);
        if (i < 0) throw new IllegalArgumentException("unknown status \"" + s + "\"");
        return (byte) i;
    }

    /** UTF-8 byte order (== code-point order), not UTF-16 code-unit order. */
    private static int byteCompare(String a, String b) {
        byte[] ab = a.getBytes(StandardCharsets.UTF_8);
        byte[] bb = b.getBytes(StandardCharsets.UTF_8);
        int n = Math.min(ab.length, bb.length);
        for (int i = 0; i < n; i++) {
            int d = (ab[i] & 0xff) - (bb[i] & 0xff);
            if (d != 0) return d;
        }
        return ab.length - bb.length;
    }

    // --- binary format -------------------------------------------------------

    private static void writeLenStr(ByteArrayOutputStream out, String s) {
        byte[] b = s.getBytes(StandardCharsets.UTF_8);
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(b.length).array());
        out.writeBytes(b);
    }

    private static String readLenStr(ByteBuffer buf) {
        byte[] b = new byte[buf.getInt()];
        buf.get(b);
        return new String(b, StandardCharsets.UTF_8);
    }

    private static byte[] binarySerialize(FieldMap fm) {
        ByteArrayOutputStream out = new ByteArrayOutputStream();

        ByteBuffer head = ByteBuffer.allocate(43).order(ByteOrder.LITTLE_ENDIAN);
        head.put(fm.getU8("uint8"));
        head.putShort(fm.getU16("uint16"));
        head.putInt(fm.getU32("uint32"));
        head.putLong(fm.getU64("uint64"));
        head.put(fm.getI8("int8"));
        head.putShort(fm.getI16("int16"));
        head.putInt(fm.getI32("int32"));
        head.putLong(fm.getI64("int64"));
        head.putFloat(fm.getF32("float32"));
        head.putDouble(fm.getF64("float64"));
        head.put((byte) (fm.getBool("bool") ? 1 : 0));
        out.writeBytes(head.array());

        writeLenStr(out, fm.getString("string"));

        byte[] raw = fm.getBytes("bytes");
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(raw.length).array());
        out.writeBytes(raw);

        List<String> list = fm.getListString("list");
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(list.size()).array());
        for (String s : list) writeLenStr(out, s);

        var opt = fm.getOptionalString("optional");
        if (opt.isEmpty()) {
            out.write(0);
        } else {
            out.write(1);
            writeLenStr(out, opt.get());
        }

        ByteBuffer arr = ByteBuffer.allocate(16).order(ByteOrder.LITTLE_ENDIAN);
        for (int n : fm.getListU32("array")) arr.putInt(n);
        out.writeBytes(arr.array());

        FieldMap p = fm.getStruct("struct");
        ByteBuffer pt = ByteBuffer.allocate(12).order(ByteOrder.LITTLE_ENDIAN);
        pt.putInt(p.getI32("x"));
        pt.putInt(p.getI32("y"));
        pt.putInt(p.getI32("z"));
        out.writeBytes(pt.array());
        writeLenStr(out, p.getString("name"));

        Map<String, Object> m = fm.getMap("map");
        List<String> keys = new ArrayList<>(m.keySet());
        keys.sort(Worker::byteCompare);
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(keys.size()).array());
        for (String k : keys) {
            writeLenStr(out, k);
            int v = ((Number) m.get(k)).intValue();
            out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(v).array());
        }

        Map<String, Object> ms = fm.getMap("map_struct");
        List<String> mkeys = new ArrayList<>(ms.keySet());
        mkeys.sort(Worker::byteCompare);
        out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(mkeys.size()).array());
        for (String k : mkeys) {
            FieldMap t = (FieldMap) ms.get(k);
            writeLenStr(out, k);
            writeLenStr(out, t.getString("name"));
            out.writeBytes(ByteBuffer.allocate(4).order(ByteOrder.LITTLE_ENDIAN).putInt(t.getU32("weight")).array());
        }

        out.write(statusOrdinal(fm.getString("status")));
        return out.toByteArray();
    }

    private static FieldMap binaryDeserialize(byte[] data) {
        ByteBuffer buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        FieldMap fm = new FieldMap();
        fm.setU8("uint8", buf.get());
        fm.setU16("uint16", buf.getShort());
        fm.setU32("uint32", buf.getInt());
        fm.setU64("uint64", buf.getLong());
        fm.setI8("int8", buf.get());
        fm.setI16("int16", buf.getShort());
        fm.setI32("int32", buf.getInt());
        fm.setI64("int64", buf.getLong());
        fm.setF32("float32", buf.getFloat());
        fm.setF64("float64", buf.getDouble());
        fm.setBool("bool", buf.get() != 0);
        fm.setString("string", readLenStr(buf));

        byte[] raw = new byte[buf.getInt()];
        buf.get(raw);
        fm.setBytes("bytes", raw);

        int nlist = buf.getInt();
        List<String> list = new ArrayList<>(nlist);
        for (int i = 0; i < nlist; i++) list.add(readLenStr(buf));
        fm.setListString("list", list);

        fm.setOptionalString("optional", buf.get() != 0 ? readLenStr(buf) : null);

        var arr = new ArrayList<Integer>();
        for (int i = 0; i < 4; i++) arr.add(buf.getInt());
        fm.setListU32("array", arr);

        FieldMap p = new FieldMap();
        p.setI32("x", buf.getInt());
        p.setI32("y", buf.getInt());
        p.setI32("z", buf.getInt());
        p.setString("name", readLenStr(buf));
        fm.setStruct("struct", p);

        Map<String, Object> m = new HashMap<>();
        int nmap = buf.getInt();
        for (int i = 0; i < nmap; i++) {
            String k = readLenStr(buf);
            m.put(k, buf.getInt());
        }
        fm.setMap("map", m);

        Map<String, Object> ms = new HashMap<>();
        int nms = buf.getInt();
        for (int i = 0; i < nms; i++) {
            String k = readLenStr(buf);
            FieldMap t = new FieldMap();
            t.setString("name", readLenStr(buf));
            t.setU32("weight", buf.getInt());
            ms.put(k, t);
        }
        fm.setMap("map_struct", ms);

        int ord = buf.get() & 0xff;
        if (ord >= STATUS_VARIANTS.size()) {
            throw new IllegalArgumentException("status ordinal " + ord + " out of range");
        }
        fm.setString("status", STATUS_VARIANTS.get(ord));
        return fm;
    }

    // --- json format -----------------------------------------------------------

    /**
     * Go's encoding/json string escaping with SetEscapeHTML(false): only \n,
     * \r, \t are named (\b and \f become u00xx escapes), and U+2028/U+2029 are
     * escaped unconditionally.
     */
    private static String goStr(String s) {
        StringBuilder sb = new StringBuilder("\"");
        for (int i = 0; i < s.length(); i++) {
            char ch = s.charAt(i);
            switch (ch) {
                case '"' -> sb.append("\\\"");
                case '\\' -> sb.append("\\\\");
                case '\n' -> sb.append("\\n");
                case '\r' -> sb.append("\\r");
                case '\t' -> sb.append("\\t");
                case '\u2028' -> sb.append("\\u2028");
                case '\u2029' -> sb.append("\\u2029");
                default -> {
                    if (ch < 0x20) {
                        sb.append(String.format("\\u%04x", (int) ch));
                    } else {
                        sb.append(ch);
                    }
                }
            }
        }
        return sb.append('"').toString();
    }

    /**
     * Go prints floats in shortest round-trip form without a trailing .0 and
     * (in this value range) without an exponent. Double.toString gives the
     * digits; BigDecimal removes Java's E-notation for magnitudes >= 1e7.
     */
    private static String goF64(double v) {
        String plain = new BigDecimal(Double.toString(v)).toPlainString();
        return plain.endsWith(".0") ? plain.substring(0, plain.length() - 2) : plain;
    }

    private static String goF32(float v) {
        String plain = new BigDecimal(Float.toString(v)).toPlainString();
        return plain.endsWith(".0") ? plain.substring(0, plain.length() - 2) : plain;
    }

    private static byte[] jsonSerialize(FieldMap fm) {
        StringBuilder sb = new StringBuilder("{");
        sb.append("\"uint8\":").append(Byte.toUnsignedInt(fm.getU8("uint8")));
        sb.append(",\"uint16\":").append(Short.toUnsignedInt(fm.getU16("uint16")));
        sb.append(",\"uint32\":").append(Integer.toUnsignedString(fm.getU32("uint32")));
        sb.append(",\"uint64\":").append(Long.toUnsignedString(fm.getU64("uint64")));
        sb.append(",\"int8\":").append(fm.getI8("int8"));
        sb.append(",\"int16\":").append(fm.getI16("int16"));
        sb.append(",\"int32\":").append(fm.getI32("int32"));
        sb.append(",\"int64\":").append(fm.getI64("int64"));
        sb.append(",\"float32\":").append(goF32(fm.getF32("float32")));
        sb.append(",\"float64\":").append(goF64(fm.getF64("float64")));
        sb.append(",\"bool\":").append(fm.getBool("bool"));
        sb.append(",\"string\":").append(goStr(fm.getString("string")));
        sb.append(",\"bytes\":\"").append(Base64.getEncoder().encodeToString(fm.getBytes("bytes"))).append('"');

        sb.append(",\"list\":[");
        List<String> list = fm.getListString("list");
        for (int i = 0; i < list.size(); i++) {
            if (i > 0) sb.append(',');
            sb.append(goStr(list.get(i)));
        }
        sb.append(']');

        var opt = fm.getOptionalString("optional");
        sb.append(",\"optional\":").append(opt.isEmpty() ? "null" : goStr(opt.get()));

        sb.append(",\"array\":[");
        var arr = fm.getListU32("array");
        for (int i = 0; i < arr.size(); i++) {
            if (i > 0) sb.append(',');
            sb.append(Integer.toUnsignedString(arr.get(i)));
        }
        sb.append(']');

        FieldMap p = fm.getStruct("struct");
        sb.append(",\"struct\":{\"x\":").append(p.getI32("x"))
          .append(",\"y\":").append(p.getI32("y"))
          .append(",\"z\":").append(p.getI32("z"))
          .append(",\"name\":").append(goStr(p.getString("name"))).append('}');

        Map<String, Object> m = fm.getMap("map");
        List<String> keys = new ArrayList<>(m.keySet());
        keys.sort(Worker::byteCompare);
        sb.append(",\"map\":{");
        for (int i = 0; i < keys.size(); i++) {
            if (i > 0) sb.append(',');
            String k = keys.get(i);
            sb.append(goStr(k)).append(':')
              .append(Integer.toUnsignedString(((Number) m.get(k)).intValue()));
        }
        sb.append('}');

        Map<String, Object> ms = fm.getMap("map_struct");
        List<String> mkeys = new ArrayList<>(ms.keySet());
        mkeys.sort(Worker::byteCompare);
        sb.append(",\"map_struct\":{");
        for (int i = 0; i < mkeys.size(); i++) {
            if (i > 0) sb.append(',');
            String k = mkeys.get(i);
            FieldMap t = (FieldMap) ms.get(k);
            sb.append(goStr(k)).append(":{\"name\":").append(goStr(t.getString("name")))
              .append(",\"weight\":").append(Integer.toUnsignedString(t.getU32("weight"))).append('}');
        }
        sb.append('}');

        sb.append(",\"status\":").append(goStr(fm.getString("status")));
        sb.append('}');
        return sb.toString().getBytes(StandardCharsets.UTF_8);
    }

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static FieldMap jsonDeserialize(byte[] data) throws Exception {
        JsonNode v = MAPPER.readTree(data);

        FieldMap fm = new FieldMap();
        fm.setU8("uint8", (byte) v.get("uint8").asInt());
        fm.setU16("uint16", (short) v.get("uint16").asInt());
        fm.setU32("uint32", (int) v.get("uint32").asLong());
        fm.setU64("uint64", v.get("uint64").bigIntegerValue().longValue());
        fm.setI8("int8", (byte) v.get("int8").asInt());
        fm.setI16("int16", (short) v.get("int16").asInt());
        fm.setI32("int32", v.get("int32").asInt());
        fm.setI64("int64", v.get("int64").asLong());
        fm.setF32("float32", (float) v.get("float32").asDouble());
        fm.setF64("float64", v.get("float64").asDouble());
        fm.setBool("bool", v.get("bool").asBoolean());
        fm.setString("string", v.get("string").asText());
        fm.setBytes("bytes", Base64.getDecoder().decode(v.get("bytes").asText()));

        List<String> list = new ArrayList<>();
        v.get("list").forEach(x -> list.add(x.asText()));
        fm.setListString("list", list);

        JsonNode opt = v.get("optional");
        fm.setOptionalString("optional", opt.isNull() ? null : opt.asText());

        var arr = new ArrayList<Integer>();
        JsonNode ja = v.get("array");
        for (int i = 0; i < 4; i++) arr.add((int) ja.get(i).asLong());
        fm.setListU32("array", arr);

        JsonNode st = v.get("struct");
        FieldMap p = new FieldMap();
        p.setI32("x", st.get("x").asInt());
        p.setI32("y", st.get("y").asInt());
        p.setI32("z", st.get("z").asInt());
        p.setString("name", st.get("name").asText());
        fm.setStruct("struct", p);

        Map<String, Object> m = new HashMap<>();
        v.get("map").fields().forEachRemaining(e -> m.put(e.getKey(), (int) e.getValue().asLong()));
        fm.setMap("map", m);

        Map<String, Object> ms = new HashMap<>();
        v.get("map_struct").fields().forEachRemaining(e -> {
            FieldMap t = new FieldMap();
            t.setString("name", e.getValue().get("name").asText());
            t.setU32("weight", (int) e.getValue().get("weight").asLong());
            ms.put(e.getKey(), t);
        });
        fm.setMap("map_struct", ms);

        fm.setString("status", v.get("status").asText());
        return fm;
    }

    public static void main(String[] args) {
        WorkerLib.runSuite(Map.of(
                "all_types", TypeEntry.formats(Map.of(
                        "binary", new FormatPair(Worker::binarySerialize, Worker::binaryDeserialize),
                        "json", new FormatPair(fm -> {
                            try {
                                return jsonSerialize(fm);
                            } catch (Exception e) {
                                throw new RuntimeException(e);
                            }
                        }, data -> {
                            try {
                                return jsonDeserialize(data);
                            } catch (Exception e) {
                                throw new RuntimeException(e);
                            }
                        })))));
    }

    private Worker() {}
}
