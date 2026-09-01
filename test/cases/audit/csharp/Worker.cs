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
    // audit_model keeps its own counter: sharing one would leave this type
    // starting wherever `audit` left off, desyncing it across languages.
    static int unstableCtr, deserUnstableCtr, modelUnstableCtr;
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

    // ── audit_model: the same faults through the model path ──────────────────
    static byte[] MarshalModel(AuditModel m) {
        var t=Encoding.UTF8.GetBytes(m.Tag); int size=4+1+t.Length+4+m.Payload.Length+1;
        foreach(var s in m.Tags) size+=1+Encoding.UTF8.GetByteCount(s);
        var buf=new byte[size]; int off=0;
        BitConverter.GetBytes(m.Value).CopyTo(buf,off); off+=4;
        buf[off++]=(byte)t.Length; t.CopyTo(buf,off); off+=t.Length;
        BitConverter.GetBytes((uint)m.Payload.Length).CopyTo(buf,off); off+=4;
        m.Payload.CopyTo(buf,off); off+=m.Payload.Length; buf[off++]=(byte)m.Tags.Length;
        foreach(var s in m.Tags){var b=Encoding.UTF8.GetBytes(s);buf[off++]=(byte)b.Length;b.CopyTo(buf,off);off+=b.Length;}
        return buf;
    }
    static AuditModel UnmarshalModel(byte[] d){var m=new AuditModel();int off=0;
        m.Value=BitConverter.ToUInt32(d,off);off+=4;
        int tlen=d[off++];m.Tag=Encoding.UTF8.GetString(d,off,tlen);off+=tlen;
        int plen=BitConverter.ToInt32(d,off);off+=4;
        m.Payload=d[off..(off+plen)].ToArray();off+=plen;
        int tc=d[off++];var tags=new string[tc];
        for(int i=0;i<tc;i++){int tl=d[off++];tags[i]=Encoding.UTF8.GetString(d,off,tl);off+=tl;}
        m.Tags=tags;return m;
    }
    static byte[] ModelMutSer(AuditModel m){var b=MarshalModel(m);m.Value=0;return b;}
    /// The positive control: this fault shows in the returned bytes, so it
    /// reports whether or not the model survives the call.
    static byte[] ModelUnstableSer(AuditModel m){var b=MarshalModel(m);Array.Resize(ref b,b.Length+1);b[^1]=(byte)modelUnstableCtr++;return b;}

    static void Main()=>Worker.RunSuite(new(){
        ["audit"]=TypeEntry.Formats(new(){["clean"]=(CleanSer,CleanDeser),["mutating"]=(MutSer,CleanDeser),["unstable"]=(UnstableSer,CleanDeser),["deser-unstable"]=(CleanSer,DuDeser),["input-mutating"]=(CleanSer,ImDeser)}),
        ["audit_model"]=TypeEntry.Model<AuditModel>(new(){
            ["clean"]=(MarshalModel,UnmarshalModel),
            ["mutating"]=(ModelMutSer,UnmarshalModel),
            ["unstable"]=(ModelUnstableSer,UnmarshalModel),
        }),
    });
}

[SerifyModel]
internal sealed class AuditModel {
    [SerifyField] public byte[] Payload { get; set; } = Array.Empty<byte>();
    [SerifyField] public string Tag { get; set; } = "";
    [SerifyField] public uint Value { get; set; }
    [SerifyField] public string[] Tags { get; set; } = Array.Empty<string>();
}
