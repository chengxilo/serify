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

// Command client is the Go client for the taskstore server: it builds a
// request, sends it as one frame, and prints the response.
//
//	go run ./cmd/client list
//	go run ./cmd/client create "Buy milk" normal errand,home 1755820800
//	go run ./cmd/client read 1001
//	go run ./cmd/client update 1001 "Buy oat milk" true high errand
//	go run ./cmd/client delete 1001
//
// python/client.py takes the same arguments and speaks the same bytes. Running
// one against a server started by the other is the end-to-end version of what
// the conformance suite checks case by case.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/chengxilo/serify/example-taskstore/api"
)

const usage = `usage: client [-addr host:port] <command> [args]

  list
  create <title> [priority] [tag,tag] [due-unix]
  read   <id>
  update <id> <title> <done> [priority] [tag,tag] [due-unix]
  delete <id>

priority is low, normal or high (default normal).`

func main() {
	addr := flag.String("addr", "127.0.0.1:9977", "server address")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	flag.Parse()

	op, err := parseOp(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n%s\n", err, usage)
		os.Exit(2)
	}

	resp, err := send(*addr, api.Request{RequestID: 1, Op: op})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Println(render(resp.Result))
	if _, failed := resp.Result.(api.Failed); failed {
		os.Exit(1)
	}
}

// send opens a connection, writes one request frame and reads one response
// frame. Nothing in here knows what an operation is — the codec does that.
func send(addr string, req api.Request) (api.Response, error) {
	var resp api.Response

	payload, err := req.MarshalBinary()
	if err != nil {
		return resp, fmt.Errorf("marshal request: %w", err)
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return resp, fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	if err := api.WriteFrame(conn, payload); err != nil {
		return resp, fmt.Errorf("write: %w", err)
	}
	frame, err := api.ReadFrame(conn)
	if err != nil {
		return resp, fmt.Errorf("read: %w", err)
	}
	if err := resp.UnmarshalBinary(frame); err != nil {
		return resp, fmt.Errorf("unmarshal response: %w", err)
	}
	return resp, nil
}

func parseOp(args []string) (api.Op, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command given")
	}
	switch args[0] {
	case "list":
		return api.ListAll{}, nil

	case "create":
		if len(args) < 2 {
			return nil, fmt.Errorf("create needs a title")
		}
		priority, err := parsePriority(arg(args, 2, api.PriorityNormal))
		if err != nil {
			return nil, err
		}
		return api.Create(api.Draft{
			Title:    args[1],
			Priority: priority,
			Tags:     parseTags(arg(args, 3, "")),
			DueAt:    parseDue(arg(args, 4, "")),
		}), nil

	case "read", "delete":
		if len(args) < 2 {
			return nil, fmt.Errorf("%s needs an id", args[0])
		}
		id, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad id %q", args[1])
		}
		if args[0] == "read" {
			return api.Read(id), nil
		}
		return api.Delete(id), nil

	case "update":
		if len(args) < 4 {
			return nil, fmt.Errorf("update needs an id, a title and done")
		}
		id, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad id %q", args[1])
		}
		done, err := strconv.ParseBool(args[3])
		if err != nil {
			return nil, fmt.Errorf("bad done %q, want true or false", args[3])
		}
		priority, err := parsePriority(arg(args, 4, api.PriorityNormal))
		if err != nil {
			return nil, err
		}
		return api.Update(api.Task{
			ID:       id,
			Title:    args[2],
			Done:     done,
			Priority: priority,
			Tags:     parseTags(arg(args, 5, "")),
			DueAt:    parseDue(arg(args, 6, "")),
		}), nil

	default:
		return nil, fmt.Errorf("unknown command %q", args[0])
	}
}

func arg(args []string, i int, fallback string) string {
	if i < len(args) && args[i] != "" {
		return args[i]
	}
	return fallback
}

// parsePriority rejects a bad priority here, before anything is encoded. This
// is the only place in the project where one can exist: an enum has no wire
// representation outside its declared variants, so by the time a request is
// bytes the value is already known to be good — and the server therefore does
// not check it again.
func parsePriority(s string) (string, error) {
	if !api.ValidPriority(s) {
		return "", fmt.Errorf("%q is not a priority (low, normal or high)", s)
	}
	return s, nil
}

func parseTags(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func parseDue(s string) *int64 {
	if s == "" {
		return nil
	}
	due, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &due
}

// render prints a result. The switch is exhaustive by hand — the compiler will
// not tell you when it stops being.
func render(r api.Result) string {
	switch v := r.(type) {
	case api.Accepted:
		return "accepted"
	case api.Found:
		return renderTask(api.Task(v))
	case api.Listing:
		page := api.TaskPage(v)
		lines := make([]string, 0, len(page.Items)+1)
		for _, t := range page.Items {
			lines = append(lines, renderTask(t))
		}
		lines = append(lines, fmt.Sprintf("(%d shown, %d total)", len(page.Items), page.Total))
		return strings.Join(lines, "\n")
	case api.Failed:
		return fmt.Sprintf("error: %s: %s", v.Code, v.Message)
	default:
		return fmt.Sprintf("unknown result %T", r)
	}
}

func renderTask(t api.Task) string {
	mark := " "
	if t.Done {
		mark = "x"
	}
	line := fmt.Sprintf("[%s] %d  %-30s  %s", mark, t.ID, t.Title, t.Priority)
	if len(t.Tags) > 0 {
		line += "  #" + strings.Join(t.Tags, " #")
	}
	if t.DueAt != nil {
		line += fmt.Sprintf("  due=%d", *t.DueAt)
	}
	return line
}
