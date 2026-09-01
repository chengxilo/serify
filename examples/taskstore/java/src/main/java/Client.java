/**
 * Copyright 2026 Chengxi Luo
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */


import java.io.IOException;
import java.net.Socket;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;

/**
 * The Java client for the taskstore server.
 *
 * <pre>
 *   java -cp target/taskstore-0.1.0.jar Client list
 *   java -cp target/taskstore-0.1.0.jar Client create "Buy milk" normal errand,home 1755820800
 *   java -cp target/taskstore-0.1.0.jar Client read 1001
 *   java -cp target/taskstore-0.1.0.jar Client update 1001 "Buy oat milk" true high errand
 *   java -cp target/taskstore-0.1.0.jar Client delete 1001
 * </pre>
 *
 * <p>The jar's manifest names Worker as its entry point, so the client is
 * reached with -cp and its class name — one jar, two mains, one codec.
 *
 * <p>Same arguments and same bytes as the other clients, so it talks to the Go
 * server without either side knowing which language the other is.
 */
public final class Client {

    private static final String USAGE = """
            usage: Client [--addr host:port] <command> [args]

              list
              create <title> [priority] [tag,tag] [due-unix]
              read   <id>
              update <id> <title> <done> [priority] [tag,tag] [due-unix]
              delete <id>

            priority is low, normal or high (default normal).""";

    public static void main(String[] argv) {
        var args = new ArrayList<>(Arrays.asList(argv));

        String addr = "127.0.0.1:9977";
        if (args.size() >= 2 && args.get(0).equals("--addr")) {
            addr = args.get(1);
            args.subList(0, 2).clear();
        }

        Op op;
        try {
            op = parseOp(args);
        } catch (IllegalArgumentException e) {
            System.err.println(e.getMessage() + "\n\n" + USAGE);
            System.exit(2);
            return;
        }

        var req = new Request();
        req.requestId = 1;
        req.op = op;

        try {
            var resp = send(addr, req);
            System.out.println(render(resp.result));
            System.exit(resp.result instanceof ApiResult.Failed ? 1 : 0);
        } catch (IOException e) {
            System.err.println(addr + ": " + e.getMessage());
            System.exit(1);
        }
    }

    /** Opens a connection, writes one request frame and reads one response frame. */
    private static Response send(String addr, Request req) throws IOException {
        byte[] payload = req.marshal();

        int colon = addr.lastIndexOf(':');
        String host = addr.substring(0, colon);
        int port = Integer.parseInt(addr.substring(colon + 1));

        try (var sock = new Socket(host, port)) {
            Frame.write(sock.getOutputStream(), payload);
            return Response.unmarshal(Frame.read(sock.getInputStream()));
        }
    }

    private static Op parseOp(List<String> args) {
        if (args.isEmpty()) throw new IllegalArgumentException("no command given");

        switch (args.get(0)) {
            case "list":
                return new Op.ListAll();

            case "create": {
                if (args.size() < 2) throw new IllegalArgumentException("create needs a title");
                var d = new Draft();
                d.title = args.get(1);
                d.priority = parsePriority(arg(args, 2, "normal"));
                d.tags = parseTags(arg(args, 3, ""));
                d.dueAt = parseDue(arg(args, 4, ""));
                return new Op.Create(d);
            }

            case "read":
                if (args.size() < 2) throw new IllegalArgumentException("read needs an id");
                return new Op.Read(taskId(args.get(1)));

            case "delete":
                if (args.size() < 2) throw new IllegalArgumentException("delete needs an id");
                return new Op.Delete(taskId(args.get(1)));

            case "update": {
                if (args.size() < 4) {
                    throw new IllegalArgumentException("update needs an id, a title and done");
                }
                String done = args.get(3);
                if (!done.equals("true") && !done.equals("false")) {
                    throw new IllegalArgumentException(
                            "bad done \"" + done + "\", want true or false");
                }
                var t = new Task();
                t.id = taskId(args.get(1));
                t.title = args.get(2);
                t.done = done.equals("true");
                t.priority = parsePriority(arg(args, 4, "normal"));
                t.tags = parseTags(arg(args, 5, ""));
                t.dueAt = parseDue(arg(args, 6, ""));
                return new Op.Update(t);
            }

            default:
                throw new IllegalArgumentException("unknown command \"" + args.get(0) + "\"");
        }
    }

    private static String arg(List<String> args, int i, String fallback) {
        return i < args.size() && !args.get(i).isEmpty() ? args.get(i) : fallback;
    }

    private static long taskId(String raw) {
        try {
            return Long.parseUnsignedLong(raw);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("bad id \"" + raw + "\"");
        }
    }

    /**
     * Rejects a bad priority here, before anything is encoded. This is the only
     * place in the project where one can exist: an enum has no wire
     * representation outside its declared variants, so by the time a request is
     * bytes the value is already known to be good — which is why the server does
     * not check it again.
     */
    private static String parsePriority(String s) {
        if (!Wire.PRIORITIES.contains(s)) {
            throw new IllegalArgumentException(
                    "\"" + s + "\" is not a priority (" + String.join(", ", Wire.PRIORITIES) + ")");
        }
        return s;
    }

    private static List<String> parseTags(String s) {
        return s.isEmpty() ? List.of() : Arrays.asList(s.split(",", -1));
    }

    private static Long parseDue(String s) {
        try {
            return s.isEmpty() ? null : Long.valueOf(s);
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private static String render(ApiResult result) {
        if (result instanceof ApiResult.Accepted) return "accepted";
        if (result instanceof ApiResult.Found f) return renderTask(f.value());
        if (result instanceof ApiResult.Listing l) {
            var page = l.value();
            var lines = new ArrayList<String>(page.items.size() + 1);
            for (var t : page.items) lines.add(renderTask(t));
            lines.add("(" + page.items.size() + " shown, "
                    + Integer.toUnsignedString(page.total) + " total)");
            return String.join("\n", lines);
        }
        var err = ((ApiResult.Failed) result).value();
        return "error: " + err.code + ": " + err.message;
    }

    private static String renderTask(Task t) {
        // toUnsignedString, because a uint64 rides in a long as its bit pattern:
        // an id past 2^63 would otherwise print negative.
        var line = String.format("[%s] %s  %-30s  %s",
                t.done ? "x" : " ", Long.toUnsignedString(t.id), t.title, t.priority);
        if (!t.tags.isEmpty()) line += "  #" + String.join(" #", t.tags);
        if (t.dueAt != null) line += "  due=" + t.dueAt;
        return line;
    }

    private Client() {}
}
