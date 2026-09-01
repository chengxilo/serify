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


// The two sums and the two messages that carry them.
//
// C# has no discriminated union keyword, but an abstract record with a private
// constructor and nested sealed records is the closed hierarchy that stands in
// for one — only the arms below can be an Op — and that is all the binding
// needs. No converter, no registration; the arm's record name in snake_case is
// its schema tag.

using System;
using System.IO;
using Serify;

/// <summary>The operation a request asks for: the `sum` in cases/op.yaml.</summary>
internal abstract record Op
{
    internal sealed record ListAll : Op;                 // arity 0 — a unit variant
    internal sealed record Create(Draft Value) : Op;     // arity 1, a model payload
    internal sealed record Read(ulong Value) : Op;       // arity 1, a scalar payload
    internal sealed record Update(TaskItem Value) : Op;  // arity 1, a model payload
    internal sealed record Delete(ulong Value) : Op;

    private Op() { }   // only the nested records above can derive
}

/// <summary>What the server answers with: the `sum` in cases/result.yaml.</summary>
internal abstract record ApiResult
{
    internal sealed record Accepted : ApiResult;
    internal sealed record Found(TaskItem Value) : ApiResult;
    internal sealed record Listing(TaskPage Value) : ApiResult;
    internal sealed record Failed(ApiError Value) : ApiResult;

    private ApiResult() { }
}

/// <summary>One request frame: cases/request.yaml.</summary>
[SerifyModel]
internal sealed class Request
{
    [SerifyField("request_id")] public uint RequestId { get; set; }
    [SerifyField] public Op Op { get; set; } = new Op.ListAll();

    public byte[] Marshal()
    {
        using var ms = new MemoryStream();
        Wire.PutU32(ms, RequestId);

        // The tag ordinal is the arm's position in the case file's sum, which is
        // the declaration order above. The schema tag *names* are the binding's
        // business and never appear here.
        switch (Op)
        {
            case Op.ListAll:
                ms.WriteByte(0);    // a unit variant is nothing but its tag
                break;
            case Op.Create(var draft):
                ms.WriteByte(1);
                draft.Put(ms);
                break;
            case Op.Read(var id):
                ms.WriteByte(2);
                Wire.PutU64(ms, id);
                break;
            case Op.Update(var task):
                ms.WriteByte(3);
                task.Put(ms);
                break;
            case Op.Delete(var id):
                ms.WriteByte(4);
                Wire.PutU64(ms, id);
                break;
            default:
                throw new InvalidOperationException($"unhandled op {Op.GetType().Name}");
        }
        return ms.ToArray();
    }

    public static Request Unmarshal(byte[] data)
    {
        var r = new Reader(data);
        uint requestId = r.U32();
        Op op = r.U8() switch
        {
            0 => new Op.ListAll(),
            1 => new Op.Create(Draft.Take(ref r)),
            2 => new Op.Read(r.U64()),
            3 => new Op.Update(TaskItem.Take(ref r)),
            4 => new Op.Delete(r.U64()),
            var tag => throw new InvalidDataException($"unknown op tag {tag}"),
        };
        return new Request { RequestId = requestId, Op = op };
    }
}

/// <summary>
/// One response frame: cases/response.yaml. RequestId echoes the request's, so a
/// client with several in flight can tell them apart.
/// </summary>
[SerifyModel]
internal sealed class Response
{
    [SerifyField("request_id")] public uint RequestId { get; set; }
    [SerifyField] public ApiResult Result { get; set; } = new ApiResult.Accepted();

    public byte[] Marshal()
    {
        using var ms = new MemoryStream();
        Wire.PutU32(ms, RequestId);

        switch (Result)
        {
            case ApiResult.Accepted:
                ms.WriteByte(0);
                break;
            case ApiResult.Found(var task):
                ms.WriteByte(1);
                task.Put(ms);
                break;
            case ApiResult.Listing(var page):
                ms.WriteByte(2);
                page.Put(ms);
                break;
            case ApiResult.Failed(var err):
                ms.WriteByte(3);
                err.Put(ms);
                break;
            default:
                throw new InvalidOperationException($"unhandled result {Result.GetType().Name}");
        }
        return ms.ToArray();
    }

    public static Response Unmarshal(byte[] data)
    {
        var r = new Reader(data);
        uint requestId = r.U32();
        ApiResult result = r.U8() switch
        {
            0 => new ApiResult.Accepted(),
            1 => new ApiResult.Found(TaskItem.Take(ref r)),
            2 => new ApiResult.Listing(TaskPage.Take(ref r)),
            3 => new ApiResult.Failed(ApiError.Take(ref r)),
            var tag => throw new InvalidDataException($"unknown result tag {tag}"),
        };
        return new Response { RequestId = requestId, Result = result };
    }
}
