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

package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

const (
	// serifyTimeout guards against a hung worker wedging the whole test binary.
	// The all-language conformance run builds eight worker toolchains before it
	// executes a single case; a cold cargo/npm/mix/dotnet build alone can take
	// minutes, so this has to be generous enough not to fire on a slow-but-healthy
	// run. It only exists to catch a genuinely wedged worker.
	serifyTimeout = 10 * time.Minute
	// serifyWaitDelay is the grace period for the CLI's I/O to drain after its
	// context is cancelled, before the process is killed outright.
	serifyWaitDelay = 5 * time.Second
)

var (
	serifyOnce sync.Once
	serifyPath string
	errSerify  error
)

// buildSerify builds the serify CLI once and returns the binary path.
func buildSerify(t *testing.T) string {
	t.Helper()
	serifyOnce.Do(func() {
		// Not t.TempDir(): the binary is cached in package state and reused by
		// every later test, but t.TempDir() is removed when the *first* test to
		// reach this Do() finishes, which would leave the rest pointing at a
		// deleted path. The OS reclaims this dir instead.
		dir, err := os.MkdirTemp("", "serify-cli") //nolint:usetesting // see above
		if err != nil {
			errSerify = err
			return
		}
		bin := filepath.Join(dir, "serify")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", bin, "./cmd/serify")
		cmd.Dir = RepoRoot()
		if out, err := cmd.CombinedOutput(); err != nil {
			errSerify = fmt.Errorf("build serify: %w\n%s", err, out)
			return
		}
		serifyPath = bin
	})
	if errSerify != nil {
		t.Fatalf("%v", errSerify)
	}
	return serifyPath
}

// RunSerify will run the serify tool. The test would build serify CLI once, after that
// it will automatically use the built binary to run the command. It returns the combined output and exit code.
// A 2-minute timeout guards against hung workers; on timeout the test is failed with the captured output.
func RunSerify(t *testing.T, args ...string) (string, int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), serifyTimeout)
	defer cancel()

	// #nosec G204 -- the "tainted" argv is the test's own args, and the binary is
	// the serify CLI this package just compiled from the repo under test.
	cmd := exec.CommandContext(ctx, buildSerify(t), args...)
	cmd.Dir = RepoRoot()
	cmd.WaitDelay = serifyWaitDelay
	out, err := cmd.CombinedOutput()

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("serify timed out after %s\nOutput: %s", serifyTimeout, out)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("failed to execute command: %v\nOutput: %s", err, out)
	}

	return string(out), 0
}
