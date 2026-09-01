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

import io.serify.WorkerLib; import io.serify.WorkerLib.*; import java.nio.*; import java.nio.charset.*; import java.util.*;
public final class AuditWorker {
    // audit_model keeps its own counter: sharing one would leave this type
    // starting wherever `audit` left off, desyncing it across languages.
    static int unstableCtr, deserUnstableCtr, modelUnstableCtr;
    static byte[] marshal(FieldMap fm) {
        var t=fm.getString("tag").getBytes(StandardCharsets.UTF_8); var p=fm.getBytes("payload");
        var tags=fm.getListString("tags"); int size=4+1+t.length+4+p.length+1;
        for(var s:tags) size+=1+s.getBytes(StandardCharsets.UTF_8).length;
        var buf=ByteBuffer.allocate(size).order(ByteOrder.LITTLE_ENDIAN);
        buf.putInt(fm.getU32("value")); buf.put((byte)t.length); buf.put(t);
        buf.putInt(p.length); buf.put(p); buf.put((byte)tags.size());
        for(var s:tags){var b=s.getBytes(StandardCharsets.UTF_8);buf.put((byte)b.length);buf.put(b);}
        return buf.array();
    }
    static FieldMap unmarshal(byte[] d,boolean cp){var fm=new FieldMap();var buf=ByteBuffer.wrap(d).order(ByteOrder.LITTLE_ENDIAN);
        fm.setU32("value",buf.getInt()); int tlen=buf.get()&0xff; byte[] tb=new byte[tlen]; buf.get(tb);
        fm.setString("tag",new String(tb,StandardCharsets.UTF_8)); int plen=buf.getInt();
        byte[] pb=new byte[plen]; buf.get(pb); fm.setBytes("payload",cp?pb.clone():pb);
        int tc=buf.get()&0xff; var tags=new ArrayList<String>();
        for(int i=0;i<tc;i++){int tl=buf.get()&0xff;byte[] sb=new byte[tl];buf.get(sb);tags.add(new String(sb,StandardCharsets.UTF_8));}
        fm.setListString("tags",tags); return fm;
    }
    static byte[] cleanSer(FieldMap fm){return marshal(fm);}
    static FieldMap cleanDeser(byte[] d){return unmarshal(d,true);}
    static byte[] mutSer(FieldMap fm){var b=marshal(fm);fm.setU32("value",0);return b;}
    static byte[] unstableSer(FieldMap fm){var b=marshal(fm);var out=Arrays.copyOf(b,b.length+1);out[b.length]=(byte)unstableCtr++;return out;}
    static FieldMap duDeser(byte[] d){var fm=unmarshal(d,true);if(deserUnstableCtr++>0)fm.setU32("value",fm.getU32("value")+1);return fm;}
    static FieldMap imDeser(byte[] d){var fm=unmarshal(d,true);if(d.length>0)d[0]^=0xFF;return fm;}

    // -- audit_model: the same faults through the model path -----------------
    @WorkerLib.SerifyModel
    public static final class AuditModel {
        @WorkerLib.SerifyField public byte[] payload = new byte[0];
        @WorkerLib.SerifyField public String tag = "";
        @WorkerLib.SerifyField public int value;
        @WorkerLib.SerifyField public List<String> tags = new ArrayList<>();
    }

    static byte[] marshalModel(AuditModel m) {
        var t=m.tag.getBytes(StandardCharsets.UTF_8); int size=4+1+t.length+4+m.payload.length+1;
        for(var s:m.tags) size+=1+s.getBytes(StandardCharsets.UTF_8).length;
        var buf=ByteBuffer.allocate(size).order(ByteOrder.LITTLE_ENDIAN);
        buf.putInt(m.value); buf.put((byte)t.length); buf.put(t);
        buf.putInt(m.payload.length); buf.put(m.payload); buf.put((byte)m.tags.size());
        for(var s:m.tags){var b=s.getBytes(StandardCharsets.UTF_8);buf.put((byte)b.length);buf.put(b);}
        return buf.array();
    }
    static AuditModel unmarshalModel(byte[] d){var m=new AuditModel();var buf=ByteBuffer.wrap(d).order(ByteOrder.LITTLE_ENDIAN);
        m.value=buf.getInt(); int tlen=buf.get()&0xff; byte[] tb=new byte[tlen]; buf.get(tb);
        m.tag=new String(tb,StandardCharsets.UTF_8); int plen=buf.getInt();
        byte[] pb=new byte[plen]; buf.get(pb); m.payload=pb;
        int tc=buf.get()&0xff; m.tags=new ArrayList<>();
        for(int i=0;i<tc;i++){int tl=buf.get()&0xff;byte[] sb=new byte[tl];buf.get(sb);m.tags.add(new String(sb,StandardCharsets.UTF_8));}
        return m;
    }
    static byte[] modelMutSer(AuditModel m){var b=marshalModel(m);m.value=0;return b;}
    /** The positive control: this fault shows in the returned bytes, so it
     *  reports whether or not the model survives the call. */
    static byte[] modelUnstableSer(AuditModel m){var b=marshalModel(m);var out=Arrays.copyOf(b,b.length+1);out[b.length]=(byte)modelUnstableCtr++;return out;}

    public static void main(String[]a){WorkerLib.runSuite(Map.of(
        "audit",TypeEntry.formats(Map.of("clean",new FormatPair(AuditWorker::cleanSer,AuditWorker::cleanDeser),"mutating",new FormatPair(AuditWorker::mutSer,AuditWorker::cleanDeser),"unstable",new FormatPair(AuditWorker::unstableSer,AuditWorker::cleanDeser),"deser-unstable",new FormatPair(AuditWorker::cleanSer,AuditWorker::duDeser),"input-mutating",new FormatPair(AuditWorker::cleanSer,AuditWorker::imDeser))),
        "audit_model",TypeEntry.model(AuditModel.class,Map.of(
            "clean",new ModelFormatPair<>(AuditWorker::marshalModel,AuditWorker::unmarshalModel),
            "mutating",new ModelFormatPair<>(AuditWorker::modelMutSer,AuditWorker::unmarshalModel),
            "unstable",new ModelFormatPair<>(AuditWorker::modelUnstableSer,AuditWorker::unmarshalModel)))));}
}
