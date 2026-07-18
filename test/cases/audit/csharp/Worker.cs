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

using System.Text; using Serify;
internal static class AuditWorker {
    static int unstableCtr, deserUnstableCtr;
    static byte[] Marshal(FieldMap fm) {
        var t=Encoding.UTF8.GetBytes(fm.GetString("tag")); var p=fm.GetBytes("payload");
        var tags=fm.GetListString("tags"); int size=4+1+t.Length+4+p.Length+1;
        foreach(var s in tags) size+=1+Encoding.UTF8.GetByteCount(s);
        var buf=new byte[size]; int off=0;
        BitConverter.GetBytes(fm.GetU32("value")).CopyTo(buf,off); off+=4;
        buf[off++]=(byte)t.Length; t.CopyTo(buf,off); off+=t.Length;
        BitConverter.GetBytes((uint)p.Length).CopyTo(buf,off); off+=4;
        p.CopyTo(buf,off); off+=p.Length; buf[off++]=(byte)tags.Length;
        foreach(var s in tags){var b=Encoding.UTF8.GetBytes(s);buf[off++]=(byte)b.Length;b.CopyTo(buf,off);off+=b.Length;}
        return buf;
    }
    static FieldMap Unmarshal(byte[] d,bool cp){var fm=new FieldMap();int off=0;
        fm.SetU32("value",BitConverter.ToUInt32(d,off));off+=4;
        int tlen=d[off++];fm.SetString("tag",Encoding.UTF8.GetString(d,off,tlen));off+=tlen;
        int plen=BitConverter.ToInt32(d,off);off+=4;
        fm.SetBytes("payload",cp?d[off..(off+plen)].ToArray():d[off..(off+plen)]);off+=plen;
        int tc=d[off++];var tags=new string[tc];
        for(int i=0;i<tc;i++){int tl=d[off++];tags[i]=Encoding.UTF8.GetString(d,off,tl);off+=tl;}
        fm.SetListString("tags",tags);return fm;
    }
    static byte[] CleanSer(FieldMap fm)=>Marshal(fm);
    static FieldMap CleanDeser(byte[] d)=>Unmarshal(d,true);
    static byte[] MutSer(FieldMap fm){var b=Marshal(fm);fm.SetU32("value",0);return b;}
    static byte[] UnstableSer(FieldMap fm){var b=Marshal(fm);Array.Resize(ref b,b.Length+1);b[^1]=(byte)unstableCtr++;return b;}
    static FieldMap DuDeser(byte[] d){var fm=Unmarshal(d,true);if(deserUnstableCtr++>0)fm.SetU32("value",fm.GetU32("value")+1);return fm;}
    static FieldMap ImDeser(byte[] d){var fm=Unmarshal(d,true);if(d.Length>0)d[0]^=0xFF;return fm;}
    static void Main()=>Worker.RunSuite(new(){["audit"]=new(){["clean"]=(CleanSer,CleanDeser),["mutating"]=(MutSer,CleanDeser),["unstable"]=(UnstableSer,CleanDeser),["deser-unstable"]=(CleanSer,DuDeser),["input-mutating"]=(CleanSer,ImDeser)}});
}
