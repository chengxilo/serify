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

// Package builder auto-detects worker languages, resolves build/run commands
// (worker.yaml optionally overrides the defaults), and runs them.
package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chengxilo/serify/internal/config"
)

// WorkerInfo describes a resolved worker after detection and (optional) build.
type WorkerInfo struct {
	Dir      string
	Language string
	Manifest config.WorkerManifest
}

// Detect auto-detects the language of a worker directory and resolves its
// build/run commands. worker.yaml is optional: if present, its non-empty
// build/run fields override the language defaults.
func Detect(dir string) (*WorkerInfo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	lang, err := DetectLanguage(abs)
	if err != nil {
		return nil, err
	}

	defaults, ok := Defaults[lang]
	if !ok {
		return nil, fmt.Errorf("no default build/run commands for language %q", lang)
	}
	defaultBuild := defaults.Build
	manifest := config.WorkerManifest{
		Build: &defaultBuild,
		Run:   defaults.Run,
	}

	yamlManifest, err := config.LoadWorkerManifest(abs)
	if err != nil {
		return nil, fmt.Errorf("read worker.yaml in %s: %w", abs, err)
	}
	if yamlManifest != nil {
		// An explicit `build: ""` means "no build step"; an absent key means
		// "use the language default".
		if yamlManifest.Build != nil {
			manifest.Build = yamlManifest.Build
		}
		if yamlManifest.Run != "" {
			manifest.Run = yamlManifest.Run
		}
	}

	if manifest.Run == "" {
		return nil, fmt.Errorf("worker in %s (%s): no run command (set it in worker.yaml)", abs, lang)
	}

	return &WorkerInfo{Dir: abs, Language: lang, Manifest: manifest}, nil
}

// Build runs the worker's build command.
func Build(info *WorkerInfo) error {
	if info.Manifest.Build == nil || *info.Manifest.Build == "" {
		return nil // no build needed (e.g. Python, or an explicit `build: ""`)
	}

	cmd := exec.CommandContext(context.Background(), "sh", "-c", *info.Manifest.Build)
	cmd.Dir = info.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed for %s (%s): %w", info.Language, info.Dir, err)
	}
	return nil
}
