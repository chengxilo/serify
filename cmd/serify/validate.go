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

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chengxilo/serify/internal/builder"
	"github.com/chengxilo/serify/internal/config"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [--cases dir] [worker-dirs...]",
		Short: "Validate case files and worker detection without running tests",
		Long: `Load and validate case files, and optionally detect worker directories.

This is a dry-run: no workers are started, no tests are executed.
Exit code is non-zero on any validation error.

Positional arguments are worker directories, the same as "serify run"; the case
directory is named with --cases.

Examples:
  serify validate                          # validate ./cases only
  serify validate --cases my-cases         # validate my-cases only
  serify validate go rust                  # ./cases, plus detect two workers
  serify validate --cases my-cases go rust`,
		RunE: func(cmd *cobra.Command, args []string) error {
			casesDir, _ := cmd.Flags().GetString("cases")
			return runValidate(casesDir, args)
		},
	}

	cmd.Flags().String("cases", "cases", "Path to the directory of per-type test case files")
	return cmd
}

func runValidate(casesDir string, workerDirs []string) error {
	// Check the worker directories before loading the cases. Doing it the other
	// way round meant a bad positional argument still printed a full, successful
	// suite report for whatever --cases defaulted to — a wall of green output for
	// a directory the user never named, followed by an unrelated-looking error.
	infos := make([]*builder.WorkerInfo, 0, len(workerDirs))
	for _, dir := range workerDirs {
		info, err := builder.Detect(dir)
		if err != nil {
			if looksLikeCasesDir(dir) {
				return fmt.Errorf("%s holds case files, not a worker: "+
					"pass it as `--cases %s` (positional arguments are worker directories, as in `serify run`)", dir, dir)
			}
			return fmt.Errorf("detect worker in %s: %w", dir, err)
		}
		infos = append(infos, info)
	}

	// Validate case files.
	set, err := config.LoadSuite(casesDir)
	if err != nil {
		return fmt.Errorf("load cases: %w", err)
	}

	fmt.Printf("Suite: %s\n", casesDir)
	for _, ty := range set.Types {
		fmt.Printf("  %-12s formats: %-22s cases: %d\n",
			ty.Name, strings.Join(ty.Formats, ", "), len(ty.Cases))
	}
	fmt.Printf("\n  Total: %d types, %d test ids\n\n", len(set.Types), len(set.TestIDs()))

	for _, info := range infos {
		fmt.Printf("Worker %-12s at %s\n", info.Language, info.Dir)
		build := ""
		if info.Manifest.Build != nil {
			build = *info.Manifest.Build
		}
		fmt.Printf("  build: %s\n", build)
		fmt.Printf("  run:   %s\n", info.Manifest.Run)
	}

	return nil
}

// looksLikeCasesDir reports whether dir holds case files. Worker detection has
// already failed on it, so this only has to separate "you meant --cases" from
// "this is not a worker directory at all".
func looksLikeCasesDir(dir string) bool {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return false
	}
	for _, m := range matches {
		if !strings.HasPrefix(filepath.Base(m), "_") {
			return true
		}
	}
	return false
}
