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
    static int unstableCtr, deserUnstableCtr;
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
    public static void main(String[]a){WorkerLib.runSuite(Map.of("audit",Map.of("clean",new FormatPair(AuditWorker::cleanSer,AuditWorker::cleanDeser),"mutating",new FormatPair(AuditWorker::mutSer,AuditWorker::cleanDeser),"unstable",new FormatPair(AuditWorker::unstableSer,AuditWorker::cleanDeser),"deser-unstable",new FormatPair(AuditWorker::cleanSer,AuditWorker::duDeser),"input-mutating",new FormatPair(AuditWorker::cleanSer,AuditWorker::imDeser))));}
}
