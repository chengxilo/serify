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
using namespace serify;
static std::vector<uint8_t> ser(const FieldMap& fm){auto s=fm.get_string("id");return {s.begin(),s.end()};}
static FieldMap deser(const std::vector<uint8_t>& d){FieldMap fm;fm.set_string("id",std::string(d.begin(),d.end()));return fm;}
int main(){SuiteMap s;s["invalid_schema"]["byte"]={ser,deser};run_suite(s);return 0;}
