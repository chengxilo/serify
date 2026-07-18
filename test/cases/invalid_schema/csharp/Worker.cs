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
internal static class ISWorker {
    static byte[] Ser(FieldMap fm)=>Encoding.UTF8.GetBytes(fm.GetString("id"));
    static FieldMap Deser(byte[] d){var fm=new FieldMap();fm.SetString("id",Encoding.UTF8.GetString(d));return fm;}
    static void Main()=>Worker.RunSuite(new(){["invalid_schema"]=new(){["byte"]=(Ser,Deser)}});
}
