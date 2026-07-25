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

package orchestrate

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/worker"
)

// stubScript builds a stub worker that serializes to a fixed hex and
// deserializes to fixed data — both parameterized so a test can make two workers
// disagree on bytes while agreeing on the decoded value. deserData must already
// carry shell-escaped quotes (e.g. `{\"x\":1}`).
func stubScript(serHex, deserData string) string {
	return fmt.Sprintf(`#!/bin/sh
while read line; do
  op=$(echo "$line" | sed 's/.*"op":"\([^"]*\)".*/\1/')
  id=$(echo "$line" | sed 's/.*"id":"\([^"]*\)".*/\1/')
  case "$op" in
    ping) echo '{"op":"ping","protocol_version":2,"status":"OK"}' ;;
    bind) echo '{"op":"bind","status":"OK"}' ;;
    serialize) echo "{\"id\":\"$id\",\"op\":\"serialize\",\"status\":\"OK\",\"hex\":\"%s\"}" ;;
    deserialize) echo "{\"id\":\"$id\",\"op\":\"deserialize\",\"status\":\"OK\",\"data\":%s}" ;;
    exit) exit 0 ;;
  esac
done
`, serHex, deserData)
}

// stubWorkerScript is a minimal shell worker: it answers ping and bind with OK,
// serialize with fixed hex, and deserialize with the fixed data {"x":1}.
const stubWorkerScript = `#!/bin/sh
while read line; do
  op=$(echo "$line" | sed 's/.*"op":"\([^"]*\)".*/\1/')
  id=$(echo "$line" | sed 's/.*"id":"\([^"]*\)".*/\1/')
  case "$op" in
    ping) echo '{"op":"ping","protocol_version":2,"status":"OK"}' ;;
    bind) echo '{"op":"bind","status":"OK"}' ;;
    serialize) echo "{\"id\":\"$id\",\"op\":\"serialize\",\"status\":\"OK\",\"hex\":\"deadbeef\"}" ;;
    deserialize) echo "{\"id\":\"$id\",\"op\":\"deserialize\",\"status\":\"OK\",\"data\":{\"x\":1}}" ;;
    exit) exit 0 ;;
  esac
done
`

// hangingWorkerScript answers ping and bind, then hangs on every other request.
const hangingWorkerScript = `#!/bin/sh
while read line; do
  op=$(echo "$line" | sed 's/.*"op":"\([^"]*\)".*/\1/')
  case "$op" in
    ping) echo '{"op":"ping","protocol_version":2,"status":"OK"}' ;;
    bind) echo '{"op":"bind","status":"OK"}' ;;
    exit) exit 0 ;;
    *) sleep 60 ;;
  esac
done
`

// skippingWorkerScript is healthy but declares it does not implement the type.
// It still answers ping: startup names no type, so there is nothing to skip.
const skippingWorkerScript = `#!/bin/sh
while read line; do
  op=$(echo "$line" | sed 's/.*"op":"\([^"]*\)".*/\1/')
  case "$op" in
    ping) echo '{"op":"ping","protocol_version":2,"status":"OK"}' ;;
    bind) echo '{"op":"bind","status":"SKIPPED","reason":"type not registered"}' ;;
    exit) exit 0 ;;
  esac
done
`

// dyingWorkerScript answers the startup ping from worker.Start, then dies on the
// per-type Bind — the shape of a worker that crashes partway through a run.
const dyingWorkerScript = `#!/bin/sh
while read line; do
  op=$(echo "$line" | sed 's/.*"op":"\([^"]*\)".*/\1/')
  case "$op" in
    ping) echo '{"op":"ping","protocol_version":2,"status":"OK"}' ;;
    bind) exit 1 ;;
    exit) exit 0 ;;
  esac
done
`

func startStubWorker(t *testing.T, lang, script string) *worker.Worker {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "orchestrate-stub-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(script); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		t.Fatal(err)
	}

	w, err := worker.Start(context.Background(), worker.StartInfo{
		Dir:      t.TempDir(),
		RunCmd:   f.Name(),
		Language: lang,
	}, 5)
	if err != nil {
		t.Fatalf("Start %s: %v", lang, err)
	}
	t.Cleanup(func() { _ = w.Stop() })
	return w
}

func testSchema() []config.Field {
	return []config.Field{{Name: "x", Type: config.FieldType{Base: "uint32"}}}
}

func testCasesSet() *config.CasesSet {
	return &config.CasesSet{
		ReferenceLanguage: "ref",
		Types: []*config.CasesFile{{
			Name:    "thing",
			Formats: []string{"binary"},
			Schema:  testSchema(),
			Cases:   []config.TestCase{{Name: "basic", Data: map[string]any{"x": 1}}},
		}},
	}
}

func newTestReport(set *config.CasesSet, langs []string) *report.Report {
	return report.New(langs, set.TestIDs(), "table")
}

func TestRunSuite_HappyPath(t *testing.T) {
	workers := map[string]*worker.Worker{
		"ref":   startStubWorker(t, "ref", stubWorkerScript),
		"other": startStubWorker(t, "other", stubWorkerScript),
	}
	set := testCasesSet()
	rep := newTestReport(set, []string{"ref", "other"})

	err := RunSuite(context.Background(), set, workers, rep, Options{TimeoutSec: 5})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}

	id := config.TestIDFmt("thing", "binary", "basic")
	for _, lang := range []string{"ref", "other"} {
		for _, op := range []string{report.OpSerialize, report.OpDeserialize} {
			res, ok := rep.Results[id][lang][op]
			if !ok {
				t.Fatalf("missing result %s/%s/%s", id, lang, op)
			}
			if res.Status != report.StatusPass {
				t.Errorf("%s %s: status %q (detail %q), want PASS", lang, op, res.Status, res.Detail)
			}
		}
	}
}

func TestRunSuite_HungWorkerIsErrorOthersPass(t *testing.T) {
	workers := map[string]*worker.Worker{
		"ref":  startStubWorker(t, "ref", stubWorkerScript),
		"hang": startStubWorker(t, "hang", hangingWorkerScript),
	}
	set := testCasesSet()
	rep := newTestReport(set, []string{"ref", "hang"})

	err := RunSuite(context.Background(), set, workers, rep, Options{TimeoutSec: 1})
	if err != nil {
		t.Fatalf("RunSuite: %v", err)
	}

	id := config.TestIDFmt("thing", "binary", "basic")
	if res := rep.Results[id]["ref"][report.OpSerialize]; res.Status != report.StatusPass {
		t.Errorf("ref serialize: status %q (detail %q), want PASS", res.Status, res.Detail)
	}
	if res := rep.Results[id]["hang"][report.OpSerialize]; res.Status != report.StatusError {
		t.Errorf("hang serialize: status %q, want ERROR", res.Status)
	}
	// The hung worker was killed after the timeout; its deserialize round must
	// fail fast as a dead worker, not hang again.
	if res := rep.Results[id]["hang"][report.OpDeserialize]; res.Status != report.StatusError {
		t.Errorf("hang deserialize: status %q, want ERROR", res.Status)
	}
}

// A worker that declares it does not implement a type is not a failure: the run
// stays green. This is the channel an honest worker uses to say "my SDK has no
// code for these bytes" instead of hand-rolling the wire format.
func TestRunSuite_DeclaredSkipIsNotAFailure(t *testing.T) {
	workers := map[string]*worker.Worker{
		"ref":  startStubWorker(t, "ref", stubWorkerScript),
		"skip": startStubWorker(t, "skip", skippingWorkerScript),
	}
	set := testCasesSet()
	rep := newTestReport(set, []string{"ref", "skip"})

	if err := RunSuite(context.Background(), set, workers, rep, Options{TimeoutSec: 5}); err != nil {
		t.Fatalf("RunSuite: %v", err)
	}

	id := config.TestIDFmt("thing", "binary", "basic")
	for _, op := range []string{report.OpSerialize, report.OpDeserialize} {
		if res := rep.Results[id]["skip"][op]; res.Status != report.StatusSkip {
			t.Errorf("skip %s: status %q (detail %q), want SKIP", op, res.Status, res.Detail)
		}
	}
	if got := rep.ExitCode(); got != 0 {
		t.Errorf("ExitCode = %d, want 0: a declared skip must not fail the run", got)
	}
}

// A worker that dies during Bind must not be reported as SKIP. Skip and death
// used to be indistinguishable, so a crashed worker produced an all-green run.
func TestRunSuite_DeadWorkerIsErrorNotSkip(t *testing.T) {
	workers := map[string]*worker.Worker{
		"ref":  startStubWorker(t, "ref", stubWorkerScript),
		"dead": startStubWorker(t, "dead", dyingWorkerScript),
	}
	set := testCasesSet()
	rep := newTestReport(set, []string{"ref", "dead"})

	if err := RunSuite(context.Background(), set, workers, rep, Options{TimeoutSec: 5}); err != nil {
		t.Fatalf("RunSuite: %v", err)
	}

	id := config.TestIDFmt("thing", "binary", "basic")
	for _, op := range []string{report.OpSerialize, report.OpDeserialize} {
		if res := rep.Results[id]["dead"][op]; res.Status != report.StatusError {
			t.Errorf("dead %s: status %q (detail %q), want ERROR", op, res.Status, res.Detail)
		}
	}
	if got := rep.ExitCode(); got == 0 {
		t.Error("ExitCode = 0, want non-zero: a worker that died must fail the run")
	}
}

func TestRunSuite_CancelledContextReturnsPromptly(t *testing.T) {
	workers := map[string]*worker.Worker{
		"ref": startStubWorker(t, "ref", hangingWorkerScript),
	}
	set := testCasesSet()
	rep := newTestReport(set, []string{"ref"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		done <- RunSuite(ctx, set, workers, rep, Options{TimeoutSec: 30})
	}()

	select {
	case <-done:
		// Bind/Send observe the cancelled context instead of waiting out the
		// 30s timeout.
	case <-time.After(10 * time.Second):
		t.Fatal("RunSuite did not return promptly with a cancelled context")
	}
}

// TestRunSuite_SemanticOracle_ByteDivergence pins the core of the oracle switch:
// two workers that emit different bytes but decode to the same value must FAIL
// under the bytes oracle and PASS under the semantic oracle.
func TestRunSuite_SemanticOracle_ByteDivergence(t *testing.T) {
	id := config.TestIDFmt("thing", "binary", "basic")

	run := func(t *testing.T, oracle string) report.Result {
		workers := map[string]*worker.Worker{
			"ref":   startStubWorker(t, "ref", stubScript("aaaa", `{\"x\":1}`)),
			"other": startStubWorker(t, "other", stubScript("bbbb", `{\"x\":1}`)),
		}
		set := testCasesSet()
		set.Oracles = map[string]string{"binary": oracle}
		rep := newTestReport(set, []string{"ref", "other"})
		if err := RunSuite(context.Background(), set, workers, rep, Options{TimeoutSec: 5}); err != nil {
			t.Fatalf("RunSuite: %v", err)
		}
		return rep.Results[id]["other"][report.OpSerialize]
	}

	if got := run(t, config.OracleBytes); got.Status != report.StatusFail {
		t.Errorf("bytes oracle: other serialize = %q (%s), want FAIL on byte divergence",
			got.Status, got.Detail)
	}
	if got := run(t, config.OracleSemantic); got.Status != report.StatusPass {
		t.Errorf("semantic oracle: other serialize = %q (%s), want PASS (decodes to same value)",
			got.Status, got.Detail)
	}
}

// TestRunSuite_SemanticOracle_ValueMismatchFails: when the reference decodes a
// candidate's bytes to a different value, the semantic oracle must FAIL.
func TestRunSuite_SemanticOracle_ValueMismatchFails(t *testing.T) {
	workers := map[string]*worker.Worker{
		"ref":   startStubWorker(t, "ref", stubScript("aaaa", `{\"x\":2}`)),
		"other": startStubWorker(t, "other", stubScript("bbbb", `{\"x\":1}`)),
	}
	set := testCasesSet()
	set.Oracles = map[string]string{"binary": config.OracleSemantic}
	rep := newTestReport(set, []string{"ref", "other"})
	if err := RunSuite(context.Background(), set, workers, rep, Options{TimeoutSec: 5}); err != nil {
		t.Fatalf("RunSuite: %v", err)
	}
	id := config.TestIDFmt("thing", "binary", "basic")
	if got := rep.Results[id]["other"][report.OpSerialize]; got.Status != report.StatusFail {
		t.Errorf("semantic oracle: other serialize = %q (%s), want FAIL on value mismatch",
			got.Status, got.Detail)
	}
}
