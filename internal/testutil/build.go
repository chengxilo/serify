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
	"errors"
	"fmt"
	"os/exec"

	"github.com/chengxilo/serify/internal/builder"
	"github.com/chengxilo/serify/internal/language"
)

// ErrToolchainMissing is returned by Worker.Build when the required toolchain
// is not installed. Callers can use errors.Is to distinguish missing toolchains
// from real build failures.
var ErrToolchainMissing = errors.New("toolchain not available")

// buildProbes maps a language to the binary a fixture worker needs before it can
// be built and run. It is deliberately narrower than ToolchainProbes, which also
// covers what the *examples* need: a fixture whose manifest builds nothing still
// needs its interpreter to run.
var buildProbes = map[string]string{
	language.Go:     "go",
	language.Rust:   "cargo",
	language.Python: "python3",
	language.Node:   "npm",
	language.Java:   "mvn",
	language.Cpp:    "g++",
	language.CSharp: "dotnet",
	language.Elixir: "mix",
	language.PHP:    "php",
}

// Worker is the definition of worker we want to build.
type Worker struct {
	Language string
	Path     string
}

// Build compiles the fixture worker through the same detect-and-build path the
// serify CLI uses, so the worker's own manifest is authoritative.
//
// This package used to hardcode one build command per language. Those commands
// silently drifted from the manifests they were standing in for: the Java and
// Node fixtures each compile the serify worker library before compiling
// themselves, a step the hardcoded versions omitted. Both languages therefore
// kept building against a stale copy of the library, which stayed invisible for
// as long as nothing in the library changed shape.
func (w Worker) Build() error {
	probe, known := buildProbes[w.Language]
	if !known {
		return fmt.Errorf("unsupported language: %s", w.Language)
	}
	if _, err := exec.LookPath(probe); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrToolchainMissing, probe, err)
	}

	info, err := builder.Detect(w.Path)
	if err != nil {
		return fmt.Errorf("detect %s worker in %s: %w", w.Language, w.Path, err)
	}
	return builder.Build(info)
}
