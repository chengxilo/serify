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


// The C# client for the taskstore server.
//
//     dotnet run -c Release --project Client/Client.csproj -- list
//     dotnet run -c Release --project Client/Client.csproj -- create "Buy milk" normal errand,home 1755820800
//     dotnet run -c Release --project Client/Client.csproj -- read 1001
//     dotnet run -c Release --project Client/Client.csproj -- update 1001 "Buy oat milk" true high errand
//     dotnet run -c Release --project Client/Client.csproj -- delete 1001
//
// Same arguments and same bytes as the other clients, so it talks to the Go
// server without either side knowing which language the other is.

using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.Sockets;

internal static class Client
{
    private const string Usage = """
usage: client [--addr host:port] <command> [args]

  list
  create <title> [priority] [tag,tag] [due-unix]
  read   <id>
  update <id> <title> <done> [priority] [tag,tag] [due-unix]
  delete <id>

priority is low, normal or high (default normal).
""";

    private static int Main(string[] argv)
    {
        var args = new List<string>(argv);

        string addr = "127.0.0.1:9977";
        if (args.Count >= 2 && args[0] == "--addr")
        {
            addr = args[1];
            args.RemoveRange(0, 2);
        }

        Op op;
        try
        {
            op = ParseOp(args);
        }
        catch (ArgumentException e)
        {
            Console.Error.WriteLine($"{e.Message}\n\n{Usage}");
            return 2;
        }

        Response resp;
        try
        {
            resp = Send(addr, new Request { RequestId = 1, Op = op });
        }
        catch (Exception e) when (e is SocketException or System.IO.IOException)
        {
            Console.Error.WriteLine($"{addr}: {e.Message}");
            return 1;
        }

        Console.WriteLine(Render(resp.Result));
        return resp.Result is ApiResult.Failed ? 1 : 0;
    }

    /// <summary>Opens a connection, writes one request frame and reads one response frame.</summary>
    private static Response Send(string addr, Request req)
    {
        byte[] payload = req.Marshal();

        int colon = addr.LastIndexOf(':');
        string host = addr[..colon];
        int port = int.Parse(addr[(colon + 1)..]);

        using var client = new TcpClient(host, port);
        using var stream = client.GetStream();
        Frame.Write(stream, payload);
        return Response.Unmarshal(Frame.Read(stream));
    }

    private static Op ParseOp(List<string> args)
    {
        string Arg(int i, string fallback) =>
            i < args.Count && args[i].Length > 0 ? args[i] : fallback;

        static ulong Id(string raw) =>
            ulong.TryParse(raw, out var id) ? id : throw new ArgumentException($"bad id \"{raw}\"");

        if (args.Count == 0) throw new ArgumentException("no command given");

        switch (args[0])
        {
            case "list":
                return new Op.ListAll();

            case "create":
                if (args.Count < 2) throw new ArgumentException("create needs a title");
                return new Op.Create(new Draft
                {
                    Title = args[1],
                    Priority = ParsePriority(Arg(2, "normal")),
                    Tags = ParseTags(Arg(3, "")),
                    DueAt = ParseDue(Arg(4, "")),
                });

            case "read":
                if (args.Count < 2) throw new ArgumentException("read needs an id");
                return new Op.Read(Id(args[1]));

            case "delete":
                if (args.Count < 2) throw new ArgumentException("delete needs an id");
                return new Op.Delete(Id(args[1]));

            case "update":
                if (args.Count < 4) throw new ArgumentException("update needs an id, a title and done");
                if (args[3] is not ("true" or "false"))
                    throw new ArgumentException($"bad done \"{args[3]}\", want true or false");
                return new Op.Update(new TaskItem
                {
                    Id = Id(args[1]),
                    Title = args[2],
                    Done = args[3] == "true",
                    Priority = ParsePriority(Arg(4, "normal")),
                    Tags = ParseTags(Arg(5, "")),
                    DueAt = ParseDue(Arg(6, "")),
                });

            default:
                throw new ArgumentException($"unknown command \"{args[0]}\"");
        }
    }

    /// <summary>
    /// Rejects a bad priority here, before anything is encoded. This is the only
    /// place in the project where one can exist: an enum has no wire
    /// representation outside its declared variants, so by the time a request is
    /// bytes the value is already known to be good — which is why the server does
    /// not check it again.
    /// </summary>
    private static string ParsePriority(string s) =>
        Array.IndexOf(Enums.Priorities, s) >= 0
            ? s
            : throw new ArgumentException(
                $"\"{s}\" is not a priority ({string.Join(", ", Enums.Priorities)})");

    private static string[] ParseTags(string s) =>
        s.Length == 0 ? Array.Empty<string>() : s.Split(',');

    private static long? ParseDue(string s) => long.TryParse(s, out var due) ? due : null;

    private static string Render(ApiResult result) => result switch
    {
        ApiResult.Accepted => "accepted",
        ApiResult.Found(var task) => RenderTask(task),
        ApiResult.Listing(var page) => string.Join("\n",
            page.Items.Select(RenderTask).Append($"({page.Items.Length} shown, {page.Total} total)")),
        ApiResult.Failed(var err) => $"error: {err.Code}: {err.Message}",
        _ => $"unknown result {result.GetType().Name}",
    };

    private static string RenderTask(TaskItem t)
    {
        var line = $"[{(t.Done ? "x" : " ")}] {t.Id}  {t.Title,-30}  {t.Priority}";
        if (t.Tags.Length > 0) line += "  #" + string.Join(" #", t.Tags);
        if (t.DueAt is { } due) line += $"  due={due}";
        return line;
    }
}
