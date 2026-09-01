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

// Package taskstore tests the taskstore example: the byte contract between the
// Go leader and its eight followers, and the server those bytes actually belong
// to.
//
// The two halves are the point of the example. A conformance run proves the
// codecs agree; only starting the server and pointing every client at it proves
// the codecs are the ones the server uses.
package taskstore

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/testutil"
)

// caseIDs is every test id the suite defines, spelled out rather than derived.
// A generated list would follow the case files wherever they went, including
// into being empty — which is the one outcome this test exists to notice.
var caseIDs = []string{
	"request/binary/list_all",
	"request/binary/create",
	"request/binary/read",
	"request/binary/update",
	"request/binary/delete_max_id",
	"response/binary/accepted",
	"response/binary/found",
	"response/binary/listing",
	"response/binary/empty_listing",
	"response/binary/failed",
	"response/binary/boundary",
}

// allLangs is every language with a worker directory here. Go leads; the rest
// follow.
var allLangs = []string{
	language.Go, language.Python, language.Rust, language.Node, language.Cpp,
	language.CSharp, language.Java, language.PHP, language.Elixir,
}

func exampleDir() string {
	return filepath.Join(testutil.RepoRoot(), "examples", "taskstore")
}

// availableLangs returns the languages whose toolchain is on this machine.
//
// SERIFY_REQUIRE names the ones that must be there, as a comma-separated list.
// Without it a toolchain that failed to install is merely absent: every
// assertion then passes having compared whatever happened to be present, and
// the run is green for a parity check that never ran. CI sets it.
func availableLangs(t *testing.T) []string {
	t.Helper()

	required := os.Getenv("SERIFY_REQUIRE")
	var present, absent []string
	for _, lang := range allLangs {
		reason := testutil.MissingToolchain(lang)
		if reason == "" {
			present = append(present, lang)
			continue
		}
		if strings.Contains(required, lang) {
			absent = append(absent, fmt.Sprintf("%s (%s)", lang, reason))
		}
	}
	if len(absent) > 0 {
		require.FailNow(t, "SERIFY_REQUIRE names toolchains that are unavailable: "+strings.Join(absent, ", "))
	}

	// Go is the leader: it owns the layout every follower is compared against,
	// so a run without it compares nothing to nothing.
	require.Contains(t, present, language.Go, "the go toolchain is required")
	return present
}

// sharedRun runs the suite once across every available worker and caches the
// grid.
//
// It is also what builds the workers: `serify run` builds each one before
// driving it, so the client test below can rely on the artifacts existing
// rather than repeating nine build commands of its own.
var (
	sharedOnce  sync.Once
	sharedGrid  testutil.ResultGrid
	sharedErr   string
	sharedLangs []string
)

func run(t *testing.T) (testutil.ResultGrid, []string) {
	t.Helper()
	langs := availableLangs(t)

	sharedOnce.Do(func() {
		dir := exampleDir()
		csv := filepath.Join(os.TempDir(), "serify-taskstore.csv")

		// No --ref: the suite names go as its leader in cases/_config.yaml,
		// which is where a reference belongs — it is a property of the cases,
		// not of whoever types the command.
		args := []string{"run", "--cases", filepath.Join(dir, "cases"), "--csv", csv}
		for _, lang := range langs {
			args = append(args, filepath.Join(dir, lang))
		}

		out, code := testutil.RunSerify(t, args...)
		if code != 0 {
			sharedErr = fmt.Sprintf("expected a clean run, got exit %d\n%s", code, out)
			return
		}
		sharedGrid = testutil.ReadResultGrid(t, csv)
		sharedLangs = langs
	})

	if sharedErr != "" {
		require.FailNow(t, sharedErr)
	}
	return sharedGrid, sharedLangs
}

// TestTaskstore_Conformance requires every worker to agree with the leader on
// every case, byte for byte — every format in this suite declares
// `oracle: bytes`, so a PASS here means identical bytes and not merely
// equivalent values.
func TestTaskstore_Conformance(t *testing.T) {
	grid, langs := run(t)

	for _, id := range caseIDs {
		for _, lang := range langs {
			for _, op := range []string{"serialize", "deserialize"} {
				testutil.AssertCell(t, grid, id, lang, op, report.StatusPass, nil)
			}
		}
	}
}

// clientCmd is how one language's client is built and invoked.
//
// Every client takes the same arguments, including `--addr` — Go's flag package
// accepts one dash or two, so the leader needs no special case.
type clientCmd struct {
	// build runs in the language's directory, if the worker build did not
	// already produce the client. Most of them come free: cargo, tsc, maven and
	// mix all build every source in the tree, so only the three that name a
	// single entry point need a line here.
	build []string
	// argv, to which --addr and the command's own arguments are appended.
	argv []string
}

var clients = map[string]clientCmd{
	language.Go:     {build: []string{"go", "build", "-o", "client", "./cmd/client"}, argv: []string{"./client"}},
	language.Python: {argv: []string{"python3", "client.py"}},
	language.Rust:   {argv: []string{"target/release/client"}},
	language.Node:   {argv: []string{"node", "dist/client.js"}},
	language.Cpp: {
		build: []string{"g++", "-O2", "-std=c++17", "-I../../../lib/cpp", "-o", "client", "client.cpp"},
		argv:  []string{"./client"},
	},
	language.CSharp: {
		build: []string{"dotnet", "build", "-c", "Release", "Client/Client.csproj"},
		argv:  []string{"dotnet", "run", "-c", "Release", "--project", "Client/Client.csproj", "--"},
	},
	language.Java:   {argv: []string{"java", "-cp", "target/taskstore-0.1.0.jar", "Client"}},
	language.PHP:    {argv: []string{"php", "client.php"}},
	language.Elixir: {argv: []string{"mix", "run", "-e", "Client.main(System.argv())", "--"}},
}

// TestTaskstore_Clients starts the Go server and drives it with every client.
//
// This is the assertion the conformance run cannot make. serify compares
// codecs; it never opens a socket. If the server were rewired to some second,
// private encoder, every conformance cell would still pass and this test would
// fail.
func TestTaskstore_Clients(t *testing.T) {
	_, langs := run(t) // also guarantees every worker, and most clients, are built

	dir := exampleDir()
	bin := t.TempDir()
	serverBin := filepath.Join(bin, "server")
	build := exec.Command("go", "build", "-o", serverBin, "./cmd/server")
	build.Dir = filepath.Join(dir, language.Go)
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building the server:\n%s", out)

	addr := startServer(t, serverBin)

	// A non-ASCII title per language: a length prefix counted in characters
	// instead of bytes survives every ASCII case and dies here.
	titles := map[string]string{}
	for _, lang := range langs {
		titles[lang] = "任务 from " + lang
	}

	for _, lang := range langs {
		lang := lang
		spec := clients[lang]
		require.NotEmpty(t, spec.argv, "no client invocation declared for %s", lang)
		langDir := filepath.Join(dir, lang)

		t.Run(lang, func(t *testing.T) {
			if len(spec.build) > 0 {
				b := exec.Command(spec.build[0], spec.build[1:]...)
				b.Dir = langDir
				out, err := b.CombinedOutput()
				require.NoError(t, err, "building the %s client:\n%s", lang, out)
			}

			client := func(args ...string) string {
				t.Helper()
				full := append(append([]string{}, spec.argv[1:]...), "--addr", addr)
				full = append(full, args...)
				cmd := exec.Command(spec.argv[0], full...)
				cmd.Dir = langDir
				out, err := cmd.CombinedOutput()
				// A client exits 1 on an api_error, which some assertions
				// expect, so only a crash is worth failing on here.
				if err != nil && !strings.Contains(fmt.Sprint(err), "exit status 1") {
					require.NoError(t, err, "%s client %v:\n%s", lang, args, out)
				}
				return string(out)
			}

			// Write: the server has to understand what this client sends.
			created := client("create", titles[lang], "high", "a,b", "1755820800")
			require.Contains(t, created, titles[lang])
			require.Contains(t, created, "due=1755820800")

			// Read: this client has to understand what the server sends back,
			// including the tasks every other client created before it.
			listed := client("list")
			require.Contains(t, listed, titles[lang])
			require.Contains(t, listed, "#a #b")

			// An error is part of the protocol, not an exception to it.
			require.Contains(t, client("read", "999999"), "error: not_found")
		})
	}

	// Finally, the leader reads back everything the followers wrote.
	goSpec := clients[language.Go]
	all := exec.Command(goSpec.argv[0], "--addr", addr, "list")
	all.Dir = filepath.Join(dir, language.Go)
	out, err = all.CombinedOutput()
	require.NoError(t, err, "listing from the go client:\n%s", out)
	for _, lang := range langs {
		require.Contains(t, string(out), titles[lang],
			"the go client cannot see the task the %s client created", lang)
	}
}

// startServer launches the server on an ephemeral port and returns the address
// it reports. Passing :0 and reading the address back beats hardcoding a port
// that another test, or another developer, might already hold.
func startServer(t *testing.T, serverBin string) string {
	t.Helper()

	cmd := exec.Command(serverBin, "-addr", "127.0.0.1:0")
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	found := make(chan string, 1)
	go func() {
		listening := regexp.MustCompile(`listening on (\S+)`)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if m := listening.FindStringSubmatch(scanner.Text()); m != nil {
				found <- m[1]
				return
			}
		}
		close(found)
	}()

	select {
	case addr, ok := <-found:
		require.True(t, ok, "server exited without reporting an address")
		return addr
	case <-time.After(10 * time.Second):
		require.FailNow(t, "server did not report an address within 10s")
		return ""
	}
}
