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

package worker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/protocol"
)

// writeStubScript writes a minimal serify stub-worker shell script to a temp
// file and returns its path. The script answers ping, bind, serialize, and
// deserialize requests with canned OK responses in a loop (supports bind).
func writeStubScript() string {
	f, err := os.CreateTemp("", "serify-stub-*.sh")
	if err != nil {
		panic(err)
	}
	script := "#!/bin/sh\n" +
		"while read line; do\n" +
		"  op=$(echo \"$line\" | sed 's/.*\"op\":\"\\([^\"]*\\)\".*/\\1/')\n" +
		"  id=$(echo \"$line\" | sed 's/.*\"id\":\"\\([^\"]*\\)\".*/\\1/')\n" +
		"  case \"$op\" in\n" +
		"    ping)\n" +
		"      echo '{\"op\":\"ping\",\"protocol_version\":1,\"status\":\"OK\"}' ;;\n" +
		"    bind)\n" +
		"      type=$(echo \"$line\" | sed 's/.*\"type\":\"\\([^\"]*\\)\".*/\\1/')\n" +
		"      format=$(echo \"$line\" | sed 's/.*\"format\":\"\\([^\"]*\\)\".*/\\1/')\n" +
		"      if [ -z \"$type\" ] || [ -z \"$format\" ]; then\n" +
		"        echo '{\"op\":\"bind\",\"status\":\"ERROR\",\"error\":\"type and format required\"}'\n" +
		"      else\n" +
		"        echo '{\"op\":\"bind\",\"status\":\"OK\"}'\n" +
		"      fi ;;\n" +
		"    serialize) echo \"{\\\"id\\\":\\\"$id\\\",\\\"op\\\":\\\"serialize\\\",\\\"status\\\":\\\"OK\\\",\\\"hex\\\":\\\"deadbeef\\\"}\" ;;\n" +
		"    deserialize) echo \"{\\\"id\\\":\\\"$id\\\",\\\"op\\\":\\\"deserialize\\\",\\\"status\\\":\\\"OK\\\",\\\"data\\\":{\\\"x\\\":1}}\" ;;\n" +
		"    exit) exit 0 ;;\n" +
		"  esac\n" +
		"done\n"
	if _, err := f.WriteString(script); err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		panic(err)
	}
	return f.Name()
}

// writeScript writes an executable shell script to the test's temp dir and
// returns its path.
func writeScript(t *testing.T, pattern, script string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(script); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func newStubInfo() StartInfo {
	return StartInfo{
		Dir:      os.TempDir(),
		RunCmd:   writeStubScript(),
		Language: "stub",
	}
}

// --- Tests ---

func TestStart_HandshakeSuccess(t *testing.T) {
	info := newStubInfo()

	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	if w.Language != "stub" {
		t.Errorf("Language = %q, want %q", w.Language, "stub")
	}
}

// Startup must not bind a type: ping names none, so a worker that would skip
// every type in the suite still starts.
func TestStart_BindsNoTypeOrFormat(t *testing.T) {
	script := "#!/bin/sh\n" +
		"while read line; do\n" +
		"  op=$(echo \"$line\" | sed 's/.*\"op\":\"\\([^\"]*\\)\".*/\\1/')\n" +
		"  case \"$op\" in\n" +
		"    ping) echo '{\"op\":\"ping\",\"protocol_version\":1,\"status\":\"OK\"}' ;;\n" +
		"    bind) echo '{\"op\":\"bind\",\"status\":\"SKIPPED\"}' ;;\n" +
		"    exit) exit 0 ;;\n" +
		"  esac\n" +
		"done\n"
	info := StartInfo{Dir: t.TempDir(), RunCmd: writeScript(t, "skips-all-*.sh", script), Language: "skipper"}

	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed even though the worker only skips types: %v", err)
	}
	defer func() { _ = w.Stop() }()

	// The skip surfaces at Bind, where it is a recorded skip rather than a
	// startup failure.
	err = w.Bind(context.Background(), nil, "user", "binary", 3, false)
	if !errors.Is(err, ErrTypeNotSupported) {
		t.Errorf("Bind error = %v, want ErrTypeNotSupported", err)
	}
}

func TestSend_SerializeOK(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	resp, err := w.Send(context.Background(), protocol.SerializeRequest{ID: "t1", Op: "serialize", Data: map[string]any{"x": 1}}, 3)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp.Status != protocol.StatusOK {
		t.Errorf("Status = %q, want OK", resp.Status)
	}
	if resp.Hex == "" {
		t.Error("expected non-empty hex")
	}
}

func TestSend_ResponseIDMismatch(t *testing.T) {
	// Use a subprocess that responds with a wrong ID.
	script := "#!/bin/sh\n" +
		"while read line; do\n" +
		"  echo '{\"id\":\"wrong\",\"op\":\"serialize\",\"status\":\"OK\",\"hex\":\"aa\"}'\n" +
		"done\n"
	f, err := os.CreateTemp(t.TempDir(), "mismatch-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(script); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.Name(), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(f.Name())
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()
	defer func() { _ = cmd.Process.Kill() }()

	w := &Worker{
		Language: "test",
		cmd:      cmd,
		stdin:    stdin,
		writer:   protocol.NewWriter(stdin),
		reader:   protocol.NewReader(stdout),
	}

	_, err = w.Send(context.Background(), protocol.SerializeRequest{ID: "correct", Op: "serialize", Data: map[string]any{}}, 2)
	if err == nil {
		t.Error("expected error for response ID mismatch")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "id mismatch") {
		t.Errorf("error should mention id mismatch: %v", err)
	}
}

// Serialize and deserialize for one case share an id, so a stale serialize
// response left in the stream would satisfy the id check when the runner is
// waiting for a deserialize. Only the op tells them apart. Without that check
// the response is accepted, carries hex instead of data, and is reported as a
// data mismatch — a stream fault disguised as a conformance failure.
func TestSend_WrongOpSameID(t *testing.T) {
	script := "#!/bin/sh\n" +
		"read -r _\n" +
		"echo '{\"id\":\"c1\",\"op\":\"serialize\",\"status\":\"OK\",\"hex\":\"aa\"}'\n" +
		"sleep 30\n"
	cmd := exec.Command(writeScript(t, "wrong-op-*.sh", script))
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()
	defer func() { _ = cmd.Process.Kill() }()

	w := &Worker{
		Language: "test",
		cmd:      cmd,
		stdin:    stdin,
		writer:   protocol.NewWriter(stdin),
		reader:   protocol.NewReader(stdout),
	}

	_, err := w.Send(context.Background(),
		protocol.DeserializeRequest{ID: "c1", Op: "deserialize", Hex: "aa"}, 2)
	if err == nil {
		t.Fatal("expected an error for a serialize response to a deserialize request")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "op mismatch") {
		t.Errorf("error should mention op mismatch: %v", err)
	}
	if !w.dead {
		t.Error("worker should be marked dead: the stream is desynced")
	}
}

func TestSend_Timeout(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 10")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()
	defer func() { _ = cmd.Process.Kill() }()

	w := &Worker{
		Language: "test",
		cmd:      cmd,
		stdin:    stdin,
		writer:   protocol.NewWriter(stdin),
		reader:   protocol.NewReader(stdout),
	}

	_, err := w.Send(context.Background(), protocol.SerializeRequest{ID: "t1", Op: "serialize", Data: map[string]any{}}, 1)
	if err == nil {
		t.Error("expected timeout error")
	}
	if !w.dead {
		t.Error("worker should be marked dead after timeout")
	}
}

func TestSend_CrashMidRequest(t *testing.T) {
	cmd := exec.Command("sh", "-c", "read line; exit 1")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()

	w := &Worker{
		Language: "test",
		cmd:      cmd,
		stdin:    stdin,
		writer:   protocol.NewWriter(stdin),
		reader:   protocol.NewReader(stdout),
	}

	_, err := w.Send(context.Background(), protocol.SerializeRequest{ID: "t1", Op: "serialize", Data: map[string]any{}}, 2)
	if err == nil {
		t.Error("expected error from crashed worker")
	}
}

func TestBind_TimeoutMarksWorkerDead(t *testing.T) {
	// A worker that answers the startup ping but hangs on bind. If the timed-out
	// bind did not kill the worker, its abandoned reader goroutine would race a
	// later request on the shared scanner.
	script := "#!/bin/sh\n" +
		"read line\n" +
		"echo '{\"op\":\"ping\",\"protocol_version\":1,\"status\":\"OK\"}'\n" +
		"sleep 30\n"
	info := StartInfo{
		Dir:      t.TempDir(),
		RunCmd:   writeScript(t, "hang-bind-*.sh", script),
		Language: "hang",
	}
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	err = w.Bind(context.Background(), []config.Field{{Name: "y", Type: config.FieldType{Base: "string"}}}, "order", "binary", 1, false)
	if err == nil {
		t.Fatal("expected timeout error from hung bind")
	}
	if !w.dead {
		t.Error("worker should be marked dead after bind timeout")
	}
	// A subsequent Bind must refuse immediately instead of racing the
	// abandoned reader.
	err = w.Bind(context.Background(), []config.Field{{Name: "y", Type: config.FieldType{Base: "string"}}}, "order", "binary", 1, false)
	if err == nil || !strings.Contains(err.Error(), "dead") {
		t.Errorf("second bind should fail fast with a dead-worker error, got: %v", err)
	}
}

func TestSend_ContextCancelInterruptsWait(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 30")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()
	defer func() { _ = cmd.Process.Kill() }()

	w := &Worker{
		Language: "test",
		cmd:      cmd,
		stdin:    stdin,
		writer:   protocol.NewWriter(stdin),
		reader:   protocol.NewReader(stdout),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := w.Send(ctx, protocol.SerializeRequest{ID: "t1", Op: "serialize", Data: map[string]any{}}, 30)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Send took %v; cancellation should interrupt the wait well before the 30s timeout", elapsed)
	}
	if !w.dead {
		t.Error("worker should be marked dead after a cancelled in-flight request")
	}
}

func TestBind_Success(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	err = w.Bind(context.Background(), []config.Field{{Name: "y", Type: config.FieldType{Base: "string"}}}, "order", "binary", 3, false)
	if err != nil {
		t.Fatalf("Bind failed: %v", err)
	}
}

func TestBind_WorkerDead(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	err = w.Bind(context.Background(), []config.Field{}, "order", "binary", 3, false)
	if err == nil {
		t.Error("expected error from dead worker")
	}
}

func TestStderr_ReturnsSlice(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	lines := w.Stderr()
	if lines == nil {
		t.Error("Stderr() should return a slice, not nil")
	}
}

// Binding an init still requires both halves; only ping may name neither.
func TestBind_RequiresNonEmptyType(t *testing.T) {
	w, err := Start(context.Background(), newStubInfo(), 3)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	err = w.Bind(context.Background(), nil, "", "binary", 3, false)
	if err == nil {
		t.Fatal("expected error for empty type in bind")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Errorf("error should reference 'type': %v", err)
	}
}

func TestBind_RequiresNonEmptyFormat(t *testing.T) {
	w, err := Start(context.Background(), newStubInfo(), 3)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	err = w.Bind(context.Background(), nil, "user", "", 3, false)
	if err == nil {
		t.Fatal("expected error for empty format in bind")
	}
	if !strings.Contains(err.Error(), "format") {
		t.Errorf("error should reference 'format': %v", err)
	}
}

func TestStart_BadCommand(t *testing.T) {
	info := StartInfo{
		Dir:      t.TempDir(),
		RunCmd:   "/nonexistent/worker/binary",
		Language: "bad",
	}
	_, err := Start(context.Background(), info, 2)
	if err == nil {
		t.Error("expected error for non-existent worker binary")
	}
}

func TestLanguage(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	if w.Language != "stub" {
		t.Errorf("Language = %q, want stub", w.Language)
	}
}

func TestSend_DeserializeOK(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = w.Stop() }()

	resp, err := w.Send(context.Background(), protocol.DeserializeRequest{ID: "t2", Op: "deserialize", Hex: "aabb"}, 3)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if resp.Status != protocol.StatusOK {
		t.Errorf("Status = %q, want OK", resp.Status)
	}
	if resp.Data == nil {
		t.Error("expected non-nil data")
	}
}

func TestSend_DeadWorker(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	_, err = w.Send(context.Background(), protocol.SerializeRequest{ID: "t1", Op: "serialize", Data: map[string]any{}}, 2)
	if err == nil {
		t.Error("expected error from dead worker")
	}
}

func TestStop_Idempotent(t *testing.T) {
	info := newStubInfo()
	w, err := Start(context.Background(), info, 5)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := w.Stop(); err != nil {
		t.Fatalf("First Stop failed: %v", err)
	}
	// Second Stop should succeed (idempotent).
	if err := w.Stop(); err != nil {
		t.Fatalf("Second Stop failed: %v", err)
	}
}

func TestPing_VersionMismatch(t *testing.T) {
	script := "#!/bin/sh\nread -r _\necho '{\"op\":\"ping\",\"status\":\"OK\",\"protocol_version\":99}'\n"
	info := StartInfo{
		Dir:      t.TempDir(),
		RunCmd:   writeScript(t, "serify-stub-oldver-*.sh", script),
		Language: "oldver",
	}
	_, err := Start(context.Background(), info, 3)
	if err == nil {
		t.Fatal("expected an error for a worker speaking another protocol version")
	}
	if !strings.Contains(err.Error(), "protocol version mismatch") {
		t.Errorf("error should name the mismatch: %v", err)
	}
	// Both numbers belong in the message: neither alone tells you what to rebuild.
	if !strings.Contains(err.Error(), "99") ||
		!strings.Contains(err.Error(), strconv.Itoa(protocol.ProtocolVersion)) {
		t.Errorf("error should report both versions: %v", err)
	}
}

func TestPing_MissingVersion(t *testing.T) {
	script := "#!/bin/sh\nread -r _\necho '{\"op\":\"ping\",\"status\":\"OK\"}'\n"
	info := StartInfo{
		Dir:      t.TempDir(),
		RunCmd:   writeScript(t, "serify-stub-nover-*.sh", script),
		Language: "nover",
	}
	_, err := Start(context.Background(), info, 3)
	if err == nil {
		t.Fatal("expected an error when ping omits protocol_version")
	}
	if !strings.Contains(err.Error(), "protocol_version") {
		t.Errorf("error should name the missing field: %v", err)
	}
}
