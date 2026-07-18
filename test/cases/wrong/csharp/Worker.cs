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

using System.Text; using System.Text.Json; using Serify;
internal static class WrongWorker {
    private const string SelfLang = "csharp";
    private static string[] ToUpperSelf(string[] a) { for (int i=0;i<a.Length;i++) if (a[i]==SelfLang) a[i]="CSHARP"; return a; }
    private static List<string> ToUpperSelf(List<string> a) => a.ConvertAll(s => s==SelfLang?"CSHARP":s);

    static byte[] BinarySerialize(FieldMap fm) {
        var langs = fm.GetListString("langs");
        if (!fm.GetBool("binary_serialize")) langs = ToUpperSelf(langs);
        using var ms = new MemoryStream();
        ms.WriteByte((byte)(fm.GetBool("binary_serialize")?1:0));
        ms.WriteByte((byte)(fm.GetBool("binary_deserialize")?1:0));
        ms.WriteByte((byte)(fm.GetBool("json_serialize")?1:0));
        ms.WriteByte((byte)(fm.GetBool("json_deserialize")?1:0));
        using var bw = new BinaryWriter(ms);
        bw.Write(langs.Length);
        foreach (var s in langs) { var b = Encoding.UTF8.GetBytes(s); bw.Write(b.Length); bw.Write(b); }
        bw.Flush(); return ms.ToArray();
    }
    static FieldMap BinaryDeserialize(byte[] data) {
        var fm = new FieldMap();
        var bs=data[0]!=0; var bd=data[1]!=0; var js=data[2]!=0; var jd=data[3]!=0;
        fm.SetBool("binary_serialize",bs); fm.SetBool("binary_deserialize",bd);
        fm.SetBool("json_serialize",js); fm.SetBool("json_deserialize",jd);
        using var r = new BinaryReader(new MemoryStream(data,4,data.Length-4));
        var n = r.ReadInt32(); var langs=new string[n];
        for (int i=0;i<n;i++){var slen=r.ReadInt32();langs[i]=Encoding.UTF8.GetString(r.ReadBytes(slen));}
        fm.SetListString("langs",bd?langs:ToUpperSelf(langs)); return fm;
    }
    static byte[] JsonSerialize(FieldMap fm) {
        var langs = fm.GetListString("langs").ToList();
        if (!fm.GetBool("json_serialize")) langs = ToUpperSelf(langs);
        var d = new Dictionary<string,object?>{
            ["binary_serialize"]=fm.GetBool("binary_serialize"),["binary_deserialize"]=fm.GetBool("binary_deserialize"),
            ["json_serialize"]=fm.GetBool("json_serialize"),["json_deserialize"]=fm.GetBool("json_deserialize"),
            ["langs"]=langs,
        };
        return JsonSerializer.SerializeToUtf8Bytes(d);
    }
    static FieldMap JsonDeserialize(byte[] data) {
        using var doc=JsonDocument.Parse(data); var v=doc.RootElement; var fm=new FieldMap();
        fm.SetBool("binary_serialize",v.GetProperty("binary_serialize").GetBoolean());
        fm.SetBool("binary_deserialize",v.GetProperty("binary_deserialize").GetBoolean());
        fm.SetBool("json_serialize",v.GetProperty("json_serialize").GetBoolean());
        fm.SetBool("json_deserialize",v.GetProperty("json_deserialize").GetBoolean());
        var langs=v.GetProperty("langs").EnumerateArray().Select(x=>x.GetString()!).ToList();
        if (!v.GetProperty("json_deserialize").GetBoolean()) langs=ToUpperSelf(langs);
        fm.SetListString("langs",langs.ToArray()); return fm;
    }
    static byte[] ErrSer(FieldMap fm)=>throw new InvalidOperationException("injected serialize error");
    static FieldMap ErrDeser(byte[] d)=>throw new InvalidOperationException("injected deserialize error");
    static byte[] HangSer(FieldMap fm){Thread.Sleep(3000);return BinarySerialize(fm);}
    static byte[] CrashSer(FieldMap fm){Environment.Exit(3);return null!;}
    static void Main()=>Worker.RunSuite(new(){["wrong"]=new(){
        ["binary"]=(BinarySerialize,BinaryDeserialize),["json"]=(JsonSerialize,JsonDeserialize),
        ["err_ser"]=(ErrSer,BinaryDeserialize),["err_deser"]=(BinarySerialize,ErrDeser),
        ["hang"]=(HangSer,BinaryDeserialize),["crash"]=(CrashSer,BinaryDeserialize),
    }});
}
