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

// C# example worker.
//
// This file is the whole worker: it names the types it can handle, one
// serializer/deserializer pair per format. Everything else lives in the files
// beside it — those stand in for the types an application already owns, each
// carrying its own schema binding and byte layout.
//
// Types this worker does not register (customer, order, telemetry) are reported
// to the runner as SKIPPED.

using System;
using System.Collections.Generic;
using Serify;

// Named Program, not Worker, so `Serify.Worker.RunSuite` below stays unambiguous.
internal static class Program
{
    private static void Main()
    {
        Serify.Worker.RunSuite(new Dictionary<string, Dictionary<string, (Func<FieldMap, byte[]>?, Func<byte[], FieldMap>?)>>
        {
            ["ledger"] = new()
            {
                ["binary"] = (
                    fm => SerifyModel.FromFieldMap<LedgerEntry>(fm).Marshal(),
                    data => SerifyModel.ToFieldMap(LedgerEntry.Unmarshal(data))),
            },
            ["signals"] = new()
            {
                ["binary"] = (
                    fm => SerifyModel.FromFieldMap<SignalCapture>(fm).Marshal(),
                    data => SerifyModel.ToFieldMap(SignalCapture.Unmarshal(data))),
            },
            ["notification"] = new()
            {
                ["binary"] = (
                    fm => SerifyModel.FromFieldMap<NotificationRecord>(fm).Marshal(),
                    data => SerifyModel.ToFieldMap(NotificationRecord.Unmarshal(data))),
            },
        });
    }
}
