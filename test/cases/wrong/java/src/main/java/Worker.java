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

/*
 * Java half of the `wrong` meta-test. Mirrors the Go worker's byte/JSON layout.
 */

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.serify.WorkerLib;
import io.serify.WorkerLib.FieldMap;
import io.serify.WorkerLib.FormatPair;
import io.serify.WorkerLib.TypeEntry;

import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public final class Worker {

    private static final String SELF_LANG = "java";
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static List<String> toUpperSelf(List<String> langs) {
        var out = new ArrayList<String>(langs.size());
        for (var s : langs) out.add(s.equals(SELF_LANG) ? "JAVA" : s);
        return out;
    }

    // --- binary ------------------------------------------------------------

    private static byte[] binarySerialize(FieldMap fm) {
        var langs = fm.getListString("langs");
        if (!fm.getBool("binary_serialize")) langs = toUpperSelf(langs);

        var buf = ByteBuffer.allocate(1024).order(ByteOrder.LITTLE_ENDIAN);
        buf.put((byte) (fm.getBool("binary_serialize") ? 1 : 0));
        buf.put((byte) (fm.getBool("binary_deserialize") ? 1 : 0));
        buf.put((byte) (fm.getBool("json_serialize") ? 1 : 0));
        buf.put((byte) (fm.getBool("json_deserialize") ? 1 : 0));
        buf.putInt(langs.size());
        for (var s : langs) {
            var b = s.getBytes(StandardCharsets.UTF_8);
            buf.putInt(b.length);
            buf.put(b);
        }
        buf.flip();
        var out = new byte[buf.remaining()];
        buf.get(out);
        return out;
    }

    private static FieldMap binaryDeserialize(byte[] data) {
        var buf = ByteBuffer.wrap(data).order(ByteOrder.LITTLE_ENDIAN);
        var fm = new FieldMap();
        var bs = buf.get() != 0; var bd = buf.get() != 0;
        var js = buf.get() != 0; var jd = buf.get() != 0;
        fm.setBool("binary_serialize", bs);
        fm.setBool("binary_deserialize", bd);
        fm.setBool("json_serialize", js);
        fm.setBool("json_deserialize", jd);

        var n = buf.getInt();
        var langs = new ArrayList<String>(n);
        for (var i = 0; i < n; i++) {
            var slen = buf.getInt();
            var b = new byte[slen];
            buf.get(b);
            langs.add(new String(b, StandardCharsets.UTF_8));
        }
        fm.setListString("langs", bd ? langs : toUpperSelf(langs));
        return fm;
    }

    // --- json --------------------------------------------------------------

    private static byte[] jsonSerialize(FieldMap fm) throws Exception {
        var node = MAPPER.createObjectNode();
        node.put("binary_serialize", fm.getBool("binary_serialize"));
        node.put("binary_deserialize", fm.getBool("binary_deserialize"));
        node.put("json_serialize", fm.getBool("json_serialize"));
        node.put("json_deserialize", fm.getBool("json_deserialize"));
        var langs = fm.getListString("langs");
        if (!fm.getBool("json_serialize")) langs = toUpperSelf(langs);
        var arr = node.putArray("langs");
        for (var s : langs) arr.add(s);
        return MAPPER.writeValueAsBytes(node);
    }

    private static FieldMap jsonDeserialize(byte[] data) throws Exception {
        var v = MAPPER.readTree(data);
        var fm = new FieldMap();
        fm.setBool("binary_serialize", v.get("binary_serialize").asBoolean());
        fm.setBool("binary_deserialize", v.get("binary_deserialize").asBoolean());
        fm.setBool("json_serialize", v.get("json_serialize").asBoolean());
        fm.setBool("json_deserialize", v.get("json_deserialize").asBoolean());
        var raw = new ArrayList<String>();
        v.get("langs").forEach(x -> raw.add(x.asText()));
        List<String> langs = v.get("json_deserialize").asBoolean() ? raw : toUpperSelf(raw);
        fm.setListString("langs", langs);
        return fm;
    }

    // --- fault formats -----------------------------------------------------

    private static byte[] errSer(FieldMap fm) { throw new RuntimeException("injected serialize error"); }
    private static FieldMap errDeser(byte[] data) { throw new RuntimeException("injected deserialize error"); }
    private static byte[] hangSer(FieldMap fm) throws Exception { Thread.sleep(3000); return binarySerialize(fm); }
    private static byte[] crashSer(FieldMap fm) { System.exit(3); return null; }

    public static void main(String[] args) {
        WorkerLib.runSuite(Map.of("wrong", TypeEntry.formats(Map.of(
            "binary", new FormatPair(Worker::binarySerialize, Worker::binaryDeserialize),
            "json", new FormatPair(fm -> { try { return jsonSerialize(fm); } catch (Exception e) { throw new RuntimeException(e); } },
                                    data -> { try { return jsonDeserialize(data); } catch (Exception e) { throw new RuntimeException(e); } }),
            "err_ser", new FormatPair(Worker::errSer, Worker::binaryDeserialize),
            "err_deser", new FormatPair(Worker::binarySerialize, Worker::errDeser),
            "hang", new FormatPair(fm -> { try { return hangSer(fm); } catch (Exception e) { throw new RuntimeException(e); } }, Worker::binaryDeserialize),
            "crash", new FormatPair(Worker::crashSer, Worker::binaryDeserialize)
        ))));
    }

    private Worker() {}
}
