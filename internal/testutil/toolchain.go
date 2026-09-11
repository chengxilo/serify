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
	"os/exec"

	"github.com/chengxilo/serify/internal/lang"
)

// ToolchainProbes maps each language to the binaries that must be on PATH.
var ToolchainProbes = map[string][]string{
	lang.Go:   {"go"},
	lang.Rust: {"cargo"},
	// python3 only. Every python worker.yaml runs `python3 worker.py`, so `python`
	// is never invoked — demanding it dropped python from SERIFY_REQUIRE on any
	// machine that ships only the versioned name, which is most of them. Same
	// mistake the PHP probe made with composer.
	lang.Python: {"python3"},
	lang.Node:   {"node", "npm"},
	lang.Java:   {"mvn", "java"},
	lang.Cpp:    {"g++"},
	lang.CSharp: {"dotnet"},
	lang.Elixir: {"mix"},
	// php only. Every php worker.yaml in the repo sets `build: ""`, and the
	// library is loaded with require_once rather than an autoloader, so
	// composer never runs — demanding it here made SERIFY_REQUIRE drop php on
	// machines that could have run it perfectly well.
	lang.PHP: {"php"},
}

// MissingToolchain returns the first missing binary for the given language,
// or "" if all required binaries are available.
func MissingToolchain(lang string) string {
	bins, ok := ToolchainProbes[lang]
	if !ok {
		return "unknown language"
	}
	for _, bin := range bins {
		if _, err := exec.LookPath(bin); err != nil {
			return bin + " not found"
		}
	}
	return ""
}
