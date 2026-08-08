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

package test

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/testutil"
)

type Case struct {
	path  string
	langs []string
}

// NewCase create a Case instance.
func NewCase(name string, langs ...string) Case {
	return Case{
		path:  filepath.Join(testutil.RepoRoot(), "test", "cases", name),
		langs: langs,
	}
}

// With narrows a case to a subset of its languages. Use it for tests that
// exercise the runner rather than the libraries: driving all nine workers there
// buys no extra coverage and makes the test hostage to runtime start-up times.
func (c Case) With(langs ...string) Case {
	return Case{path: c.path, langs: langs}
}

// CasePath return the path of case.
func (c Case) CasePath() string {
	return filepath.Join(c.path, "cases")
}

// CasePathNamed returns a subdirectory path within the case directory.
func (c Case) CasePathNamed(name string) string {
	return filepath.Join(c.path, name)
}

// Workers return all the workers.
func (c Case) Workers() []testutil.Worker {
	var workers []testutil.Worker
	for _, lang := range c.langs {
		workers = append(workers, testutil.Worker{
			Language: lang,
			Path:     filepath.Join(c.path, lang),
		})
	}
	return workers
}

// WorkerPaths return the absolute paths for all workers.
func (c Case) WorkerPaths() []string {
	var paths []string
	for _, w := range c.Workers() {
		paths = append(paths, w.Path)
	}
	return paths
}

// runArgs returns the standard `serify run` argument list for this case.
func (c Case) runArgs(ref, casesDir string, extra ...string) []string {
	args := []string{"run", "--ref", ref, "--cases", casesDir, "--no-build"}
	args = append(args, extra...)
	args = append(args, c.WorkerPaths()...)
	return args
}

var (
	happy         = NewCase("happy", language.All...)
	wrong         = NewCase("wrong", language.All...)
	audit         = NewCase("audit", language.All...)
	invalidSchema = NewCase("invalid_schema", language.Go, language.Rust)
)

// missingLang records languages whose toolchain is unavailable (lang → reason).
var missingLang = map[string]string{}

// requireWorkers fails the test if any of the given languages is unavailable.
func requireWorkers(t *testing.T, langs ...string) {
	t.Helper()
	for _, lang := range langs {
		if reason, ok := missingLang[lang]; ok {
			require.Fail(t, fmt.Sprintf("required toolchain %s unavailable: %s", lang, reason))
		}
	}
}

func TestMain(m *testing.M) {
	var workers []testutil.Worker
	workers = append(workers, happy.Workers()...)
	workers = append(workers, wrong.Workers()...)
	workers = append(workers, invalidSchema.Workers()...)
	workers = append(workers, audit.Workers()...)
	for _, w := range workers {
		if err := w.Build(); err != nil {
			if errors.Is(err, testutil.ErrToolchainMissing) {
				missingLang[w.Language] = err.Error()
				continue
			}
			panic(err)
		}
	}
	m.Run()
}
