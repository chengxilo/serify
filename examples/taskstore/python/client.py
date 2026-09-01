# Copyright 2026 Chengxi Luo
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""The Python client for the taskstore server.

    python3 client.py list
    python3 client.py create "Buy milk" normal errand,home 1755820800
    python3 client.py read 1001
    python3 client.py update 1001 "Buy oat milk" true high errand
    python3 client.py delete 1001

It takes the same arguments as go/cmd/client and speaks the same bytes, so it
talks to the Go server without either side knowing which language the other is.
That is the end-to-end version of what the conformance suite checks case by
case — and the suite is what makes it work on the first try instead of after an
afternoon of hexdumps.

Like api.py, this imports only the standard library. serify is a development
dependency of this project, not a runtime one.
"""

import argparse
import socket
import sys

import api

PRIORITY_HELP = "priority is low, normal or high (default normal)"


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0], epilog=PRIORITY_HELP)
    parser.add_argument("--addr", default="127.0.0.1:9977", help="server address (default %(default)s)")
    parser.add_argument("command", choices=("list", "create", "read", "update", "delete"))
    parser.add_argument("args", nargs="*")
    ns = parser.parse_args(argv)

    try:
        op = parse_op(ns.command, ns.args)
    except ValueError as e:
        parser.error(str(e))

    host, _, port = ns.addr.rpartition(":")
    try:
        resp = send((host, int(port)), api.Request(request_id=1, op=op))
    except OSError as e:
        print(f"{ns.addr}: {e}", file=sys.stderr)
        return 1

    print(render(resp.result))
    return 1 if isinstance(resp.result, api.Failed) else 0


def send(addr: tuple[str, int], req: api.Request) -> api.Response:
    """Open a connection, write one request frame, read one response frame."""
    payload = req.marshal()
    with socket.create_connection(addr) as sock, sock.makefile("rwb") as stream:
        api.write_frame(stream, payload)
        return api.Response.unmarshal(api.read_frame(stream))


def parse_op(command: str, args: list[str]) -> api.Op:
    def arg(i: int, fallback: str = "") -> str:
        return args[i] if i < len(args) and args[i] else fallback

    def task_id(raw: str) -> int:
        if not raw.isdigit():
            raise ValueError(f"bad id {raw!r}")
        return int(raw)

    match command:
        case "list":
            return api.ListAll()

        case "create":
            if not args:
                raise ValueError("create needs a title")
            return api.Create(api.Draft(
                title=args[0],
                priority=parse_priority(arg(1, "normal")),
                tags=parse_tags(arg(2)),
                due_at=parse_due(arg(3)),
            ))

        case "read":
            if not args:
                raise ValueError("read needs an id")
            return api.Read(task_id(args[0]))

        case "delete":
            if not args:
                raise ValueError("delete needs an id")
            return api.Delete(task_id(args[0]))

        case "update":
            if len(args) < 3:
                raise ValueError("update needs an id, a title and done")
            if args[2] not in ("true", "false"):
                raise ValueError(f"bad done {args[2]!r}, want true or false")
            return api.Update(api.Task(
                id=task_id(args[0]),
                title=args[1],
                done=args[2] == "true",
                priority=parse_priority(arg(3, "normal")),
                tags=parse_tags(arg(4)),
                due_at=parse_due(arg(5)),
            ))

    raise ValueError(f"unknown command {command!r}")


def parse_priority(s: str) -> str:
    """Reject a bad priority here, before anything is encoded.

    This is the only place in the project where one can exist: an enum has no
    wire representation outside its declared variants, so by the time a request
    is bytes the value is already known to be good — which is why the server
    does not check it again either.
    """
    if s not in api.PRIORITIES:
        raise ValueError(f"{s!r} is not a priority ({', '.join(api.PRIORITIES)})")
    return s


def parse_tags(s: str) -> list[str]:
    return s.split(",") if s else []


def parse_due(s: str) -> int | None:
    return int(s) if s.lstrip("-").isdigit() else None


def render(result: api.Result) -> str:
    """Print a result. `match` over the union is exhaustive in a way Go's type
    switch is not — mypy flags a missing arm here."""
    match result:
        case api.Accepted():
            return "accepted"
        case api.Found(value=task):
            return render_task(task)
        case api.Listing(value=page):
            lines = [render_task(t) for t in page.items]
            lines.append(f"({len(page.items)} shown, {page.total} total)")
            return "\n".join(lines)
        case api.Failed(value=err):
            return f"error: {err.code}: {err.message}"
    return f"unknown result {type(result).__name__}"


def render_task(t: api.Task) -> str:
    line = f"[{'x' if t.done else ' '}] {t.id}  {t.title:<30}  {t.priority}"
    if t.tags:
        line += "  #" + " #".join(t.tags)
    if t.due_at is not None:
        line += f"  due={t.due_at}"
    return line


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
