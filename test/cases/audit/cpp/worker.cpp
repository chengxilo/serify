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

#include "serify.hpp"
#include <cstdint>
#include <string>
#include <vector>
using namespace serify;

static int unstable_ctr, deser_unstable_ctr;

template<typename U> static void put_le(std::vector<uint8_t>& o, U u, size_t n) { for(size_t i=0;i<n;i++) o.push_back((uint8_t)(u>>(8*i))); }
template<typename U> static U take_le(const std::vector<uint8_t>& d, size_t& off, size_t n) { U u=0; for(size_t i=0;i<n;i++) u|=(U)d[off+i]<<(8*i); off+=n; return u; }

static std::vector<uint8_t> marshal(const FieldMap& fm) {
    auto tags=fm.get_list_string("tags");
    std::vector<uint8_t> o;
    put_le<uint32_t>(o,fm.get_u32("value"),4);
    auto t=fm.get_string("tag"); o.push_back((uint8_t)t.size()); o.insert(o.end(),t.begin(),t.end());
    auto p=fm.get_bytes("payload"); put_le<uint32_t>(o,(uint32_t)p.size(),4); o.insert(o.end(),p.begin(),p.end());
    o.push_back((uint8_t)tags.size());
    for(auto& s:tags){o.push_back((uint8_t)s.size()); o.insert(o.end(),s.begin(),s.end());}
    return o;
}
static FieldMap unmarshal(const std::vector<uint8_t>& d, bool copy_payload) {
    FieldMap fm; size_t off=0;
    fm.set_u32("value",take_le<uint32_t>(d,off,4));
    auto tlen=take_le<uint8_t>(d,off,1); fm.set_string("tag",std::string(d.begin()+off,d.begin()+off+tlen)); off+=tlen;
    auto plen=take_le<uint32_t>(d,off,4);
    fm.set_bytes("payload",copy_payload?Bytes(d.begin()+off,d.begin()+off+plen):Bytes(d.begin()+off,d.begin()+off+plen));
    off+=plen;
    auto tc=take_le<uint8_t>(d,off,1); std::vector<std::string> tags;
    for(int i=0;i<tc;i++){auto tl=take_le<uint8_t>(d,off,1); tags.emplace_back(d.begin()+off,d.begin()+off+tl); off+=tl;}
    fm.set_list_string("tags",std::move(tags));
    return fm;
}
static std::vector<uint8_t> clean_ser(const FieldMap& fm){return marshal(fm);}
static FieldMap clean_deser(const std::vector<uint8_t>& d){return unmarshal(d,true);}
static std::vector<uint8_t> mut_ser(const FieldMap& fm){auto b=marshal(fm);const_cast<FieldMap&>(fm).set_u32("value",0);return b;}
static std::vector<uint8_t> unstable_ser(const FieldMap& fm){auto b=marshal(fm);b.push_back((uint8_t)unstable_ctr++);return b;}
static FieldMap du_deser(const std::vector<uint8_t>& d){auto fm=unmarshal(d,true);if(deser_unstable_ctr++>0)fm.set_u32("value",fm.get_u32("value")+1);return fm;}
static FieldMap im_deser(const std::vector<uint8_t>& d){auto fm=unmarshal(d,true);if(!d.empty())const_cast<uint8_t&>(d[0])^=0xFF;return fm;}
int main(){SuiteMap s;s["audit"]["clean"]={clean_ser,clean_deser};s["audit"]["mutating"]={mut_ser,clean_deser};s["audit"]["unstable"]={unstable_ser,clean_deser};s["audit"]["deser-unstable"]={clean_ser,du_deser};s["audit"]["input-mutating"]={clean_ser,im_deser};run_suite(s);return 0;}
