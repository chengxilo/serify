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


// The conformance worker: serify's entire footprint in the C# follower.
//
// It registers the two types that cross the socket and hands each the very
// functions Client.cs calls — Request.Marshal is not a test double.

using System.Collections.Generic;
using Serify;

// Named Program, not Worker, so `Serify.Worker.RunSuite` below stays unambiguous.
internal static class Program
{
    private static void Main()
    {
        Serify.Worker.RunSuite(new Dictionary<string, TypeEntry>
        {
            ["request"] = TypeEntry.Model<Request>(new()
            {
                ["binary"] = (r => r.Marshal(), Request.Unmarshal),
            }),
            ["response"] = TypeEntry.Model<Response>(new()
            {
                ["binary"] = (r => r.Marshal(), Response.Unmarshal),
            }),
        });
    }
}
