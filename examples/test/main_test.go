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

package example

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chengxilo/serify/internal/language"
	"github.com/chengxilo/serify/internal/testutil"
)

// availableLangs lists languages whose toolchain is on PATH (go is always available).
var availableLangs []string

// missingLang records languages whose toolchain is unavailable (lang → reason).
var missingLang = map[string]string{}

func TestMain(m *testing.M) {
	path := filepath.Join(testutil.RepoRoot(), "examples")

	// Probe all example languages.
	allLangs := []string{
		language.Go, language.Rust, language.Python, language.Node, language.Java,
		language.Cpp, language.CSharp, language.Elixir, language.PHP,
	}
	for _, lang := range allLangs {
		workerDir := filepath.Join(path, lang)
		if _, err := os.Stat(workerDir); os.IsNotExist(err) {
			continue // no example worker for this language
		}
		if reason := testutil.MissingToolchain(lang); reason != "" {
			missingLang[lang] = reason
			continue
		}
		availableLangs = append(availableLangs, lang)
	}

	// SERIFY_REQUIRE names the languages that must be present in this
	// environment, as a comma-separated list. Without it a toolchain that failed
	// to install is merely absent from availableLangs: every test then passes
	// having compared the languages that happened to be there, and the run is
	// green for a parity check that never ran. CI sets it per matrix leg.
	if req := os.Getenv("SERIFY_REQUIRE"); req != "" {
		var absent []string
		for _, lang := range strings.Split(req, ",") {
			lang = strings.TrimSpace(lang)
			if lang == "" {
				continue
			}
			if !slices.Contains(availableLangs, lang) {
				reason := missingLang[lang]
				if reason == "" {
					reason = "no example worker directory"
				}
				absent = append(absent, fmt.Sprintf("%s (%s)", lang, reason))
			}
		}
		if len(absent) > 0 {
			fmt.Fprintf(os.Stderr,
				"SERIFY_REQUIRE=%s but these are unavailable: %s\n", req, strings.Join(absent, ", "))
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

// skipUnless skips the test when the language's toolchain is unavailable. Use it
// for a language the test needs but the environment is not obliged to have;
// SERIFY_REQUIRE is what makes a language mandatory.
func skipUnless(t *testing.T, lang string) {
	t.Helper()
	if reason, ok := missingLang[lang]; ok {
		t.Skipf("toolchain %s unavailable: %s", lang, reason)
	}
}

// requireLang fails the test if the language's toolchain is unavailable.
func requireLang(t *testing.T, lang string) {
	t.Helper()
	if reason, ok := missingLang[lang]; ok {
		require.Fail(t, fmt.Sprintf("required toolchain %s unavailable: %s", lang, reason))
	}
}
