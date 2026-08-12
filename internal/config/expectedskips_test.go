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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LoadExpectedSkips decides how much coverage a worker is allowed to be
// missing, and every way it can go wrong fails *open*: an absent file, an empty
// file and a file whose keys do not parse all yield a zero ExpectedSkips, which
// Covers answers false for, which turns every skip into a failure. The one
// outcome that must never be silent is a malformed file — that has to reach the
// caller as an error rather than as "declares nothing", because the two lead to
// opposite verdicts on the same run.

func writeSkipFile(t *testing.T, dir, lang, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, lang+".yaml"), []byte(body), 0o600))
}

func TestLoadExpectedSkips_MissingFileDeclaresNothing(t *testing.T) {
	es, err := LoadExpectedSkips(t.TempDir(), "go")
	require.NoError(t, err, "a worker with no file is not an error, it covers everything")
	assert.Empty(t, es.Types, "missing file must declare no types")
	assert.Empty(t, es.Operations, "missing file must declare no operations")
	assert.False(t, es.Covers("customer", "serialize"), "nothing is covered when nothing is declared")
}

func TestLoadExpectedSkips_TypesAndOperations(t *testing.T) {
	dir := t.TempDir()
	writeSkipFile(t, dir, "python", `
types:
  - telemetry
operations:
  deserialize:
    - order
`)

	es, err := LoadExpectedSkips(dir, "python")
	require.NoError(t, err)

	// A bare type covers both directions.
	assert.True(t, es.Covers("telemetry", "serialize"), "declared type must cover serialize")
	assert.True(t, es.Covers("telemetry", "deserialize"), "declared type must cover deserialize")

	// An operations entry covers that direction only. This is the whole point
	// of the split: a worker that can decode but not encode declares one, and a
	// regression in the other direction still fails.
	assert.True(t, es.Covers("order", "deserialize"), "declared operation must cover its own direction")
	assert.False(t, es.Covers("order", "serialize"), "an operations entry must not leak into the other direction")

	assert.False(t, es.Covers("customer", "serialize"), "an undeclared type must stay uncovered")
}

func TestLoadExpectedSkips_EmptyFileDeclaresNothing(t *testing.T) {
	dir := t.TempDir()
	writeSkipFile(t, dir, "go", "")

	es, err := LoadExpectedSkips(dir, "go")
	require.NoError(t, err, "an empty file is a declaration of nothing, not a parse failure")
	assert.False(t, es.Covers("anything", "serialize"), "an empty file must cover nothing")
}

// The failure that must not be silent. A file the loader cannot parse has to
// surface as an error: swallowing it would leave the caller holding an empty
// ExpectedSkips, which reads as "this worker declares no gaps" and flips every
// skip in the run to FAIL for a reason no one can see.
func TestLoadExpectedSkips_MalformedFileErrors(t *testing.T) {
	dir := t.TempDir()
	writeSkipFile(t, dir, "rust", "types: [unterminated\n")

	_, err := LoadExpectedSkips(dir, "rust")
	require.Error(t, err, "a malformed file must not degrade to an empty declaration")
	assert.Contains(t, err.Error(), "rust.yaml", "the error must name the file so it can be fixed")
}

// Each language reads its own file and no other, so one worker's declaration
// can never widen another's.
func TestLoadExpectedSkips_IsPerLanguage(t *testing.T) {
	dir := t.TempDir()
	writeSkipFile(t, dir, "cpp", "types:\n  - telemetry\n")

	cpp, err := LoadExpectedSkips(dir, "cpp")
	require.NoError(t, err)
	assert.True(t, cpp.Covers("telemetry", "serialize"))

	java, err := LoadExpectedSkips(dir, "java")
	require.NoError(t, err)
	assert.False(t, java.Covers("telemetry", "serialize"), "cpp's declaration must not cover java")
}
