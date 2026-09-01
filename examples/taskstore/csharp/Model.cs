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


// The records the server stores and sends. Each mirrors the case file of the
// same name and owns its own byte layout.
//
// [SerifyModel] plus one [SerifyField] per property is the entire schema
// binding. An enum needs nothing from it — it travels as its variant *name*, so
// Priority is a plain string and Priorities fixes the ordinal this codec writes.

using System;
using System.IO;
using Serify;

internal static class Enums
{
    /// <summary>Declaration order of the enum in cases/task.yaml.</summary>
    internal static readonly string[] Priorities = { "low", "normal", "high" };

    /// <summary>Declaration order of the enum in cases/api_error.yaml.</summary>
    internal static readonly string[] ErrorCodes = { "not_found", "invalid", "conflict" };
}

/// <summary>
/// A stored task: cases/task.yaml.
///
/// Named TaskItem rather than Task because ImplicitUsings brings in
/// System.Threading.Tasks. The class name never reaches the wire — the schema
/// binds field names, and the sum tags come from the arm names in Message.cs —
/// so renaming it costs nothing.
/// </summary>
[SerifyModel]
internal sealed class TaskItem
{
    [SerifyField] public ulong Id { get; set; }
    [SerifyField] public string Title { get; set; } = "";
    [SerifyField] public bool Done { get; set; }
    [SerifyField] public string Priority { get; set; } = "normal";
    [SerifyField] public string[] Tags { get; set; } = Array.Empty<string>();
    [SerifyField("due_at")] public long? DueAt { get; set; }

    public void Put(MemoryStream ms)
    {
        Wire.PutU64(ms, Id);
        Wire.PutStr(ms, Title);
        ms.WriteByte(Done ? (byte)1 : (byte)0);
        Wire.PutEnum(ms, Enums.Priorities, Priority);
        Wire.PutTags(ms, Tags);
        Wire.PutOptionalI64(ms, DueAt);
    }

    public static TaskItem Take(ref Reader r) => new()
    {
        Id = r.U64(),
        Title = r.Str(),
        Done = r.Bool(),
        Priority = r.Enum(Enums.Priorities),
        Tags = r.Tags(),
        DueAt = r.OptionalI64(),
    };
}

/// <summary>A task the server has not assigned an id to yet: cases/draft.yaml.</summary>
[SerifyModel]
internal sealed class Draft
{
    [SerifyField] public string Title { get; set; } = "";
    [SerifyField] public string Priority { get; set; } = "normal";
    [SerifyField] public string[] Tags { get; set; } = Array.Empty<string>();
    [SerifyField("due_at")] public long? DueAt { get; set; }

    public void Put(MemoryStream ms)
    {
        Wire.PutStr(ms, Title);
        Wire.PutEnum(ms, Enums.Priorities, Priority);
        Wire.PutTags(ms, Tags);
        Wire.PutOptionalI64(ms, DueAt);
    }

    public static Draft Take(ref Reader r) => new()
    {
        Title = r.Str(),
        Priority = r.Enum(Enums.Priorities),
        Tags = r.Tags(),
        DueAt = r.OptionalI64(),
    };
}

/// <summary>The body of a list response: cases/task_page.yaml.</summary>
[SerifyModel]
internal sealed class TaskPage
{
    [SerifyField] public TaskItem[] Items { get; set; } = Array.Empty<TaskItem>();
    [SerifyField] public uint Total { get; set; }

    public void Put(MemoryStream ms)
    {
        Wire.PutU32(ms, (uint)Items.Length);
        foreach (var t in Items) t.Put(ms);
        Wire.PutU32(ms, Total);
    }

    public static TaskPage Take(ref Reader r)
    {
        uint n = r.U32();
        var items = new TaskItem[n];
        for (uint i = 0; i < n; i++) items[i] = TaskItem.Take(ref r);
        return new TaskPage { Items = items, Total = r.U32() };
    }
}

/// <summary>A failed request: cases/api_error.yaml.</summary>
[SerifyModel]
internal sealed class ApiError
{
    [SerifyField] public string Code { get; set; } = "not_found";
    [SerifyField] public string Message { get; set; } = "";

    public void Put(MemoryStream ms)
    {
        Wire.PutEnum(ms, Enums.ErrorCodes, Code);
        Wire.PutStr(ms, Message);
    }

    public static ApiError Take(ref Reader r) => new()
    {
        Code = r.Enum(Enums.ErrorCodes),
        Message = r.Str(),
    };
}
