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

package builder

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/chengxilo/serify/internal/config"
	"github.com/stretchr/testify/require"
)

// strPtr is for building WorkerManifest literals: Build is a *string so that an
// explicit `build: ""` (no build step) differs from an absent key (use default).
func strPtr(s string) *string { return &s }

func TestBuild_NoBuildCommand(t *testing.T) {
	// An explicit `build: ""` means this worker needs no build step.
	info := &WorkerInfo{
		Dir:      t.TempDir(),
		Language: "python",
		Manifest: config.WorkerManifest{
			Build: strPtr(""),
			Run:   "python3 worker.py",
		},
	}
	err := Build(info)
	require.NoError(t, err, "Build: %v", err)
}

func TestBuild_RunsCommand(t *testing.T) {
	dir := t.TempDir()
	info := &WorkerInfo{
		Dir:      dir,
		Language: "go",
		Manifest: config.WorkerManifest{
			Build: strPtr("echo built > out.txt"),
			Run:   "./worker",
		},
	}
	err := Build(info)
	require.NoError(t, err, "Build: %v", err)
	_, err = os.Stat(filepath.Join(dir, "out.txt"))
	require.NoError(t, err, "build command did not run: %v", err)
}

// TestBuild_AlwaysRuns is the property this package exists to guarantee. serify
// used to skip the build when a marker file was newer than everything in the
// worker directory. That cache could not see a worker's dependencies outside its
// own directory (a library it compiles against), so editing them left the marker
// valid and the run silently exercised a stale binary. Build now always shells
// out and lets the language's own build tool decide what to recompile.
func TestBuild_AlwaysRuns(t *testing.T) {
	dir := t.TempDir()
	info := &WorkerInfo{
		Dir:      dir,
		Language: "go",
		Manifest: config.WorkerManifest{
			// Append a line per invocation so the count is observable.
			Build: strPtr("echo run >> count.txt"),
			Run:   "./worker",
		},
	}
	const runs = 3
	for i := range runs {
		err := Build(info)
		require.NoError(t, err, "Build #%d: %v", i+1, err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "count.txt"))
	require.NoError(t, err, "read count: %v", err)
	lines := 0
	for _, b := range got {
		if b == '\n' {
			lines++
		}
	}
	require.Equal(t, runs, lines, "build ran %s times, want %s: nothing may cache the build away",
		strconv.Itoa(lines), strconv.Itoa(runs))
}

func TestBuild_CommandFails(t *testing.T) {
	info := &WorkerInfo{
		Dir:      t.TempDir(),
		Language: "go",
		Manifest: config.WorkerManifest{
			Build: strPtr("exit 1"),
			Run:   "./worker",
		},
	}
	require.Error(t, Build(info), "expected an error when the build command fails")
}
