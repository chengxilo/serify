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
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chengxilo/serify/internal/builder"
	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/orchestrate"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/worker"
)

func newRunCmd() *cobra.Command {
	var opts runOpts

	cmd := &cobra.Command{
		Use:   "run <worker-dir> [<worker-dir>...]",
		Short: "Run serialization conformance tests",
		Long: `Run cross-language serialization/deserialization conformance tests.

Each worker-dir is a worker directory. The language is auto-detected from marker
files (go.mod, Cargo.toml, etc.). An optional worker.yaml can override the
default build and run commands.

Examples:
  serify run --ref rust --cases examples/cases go rust
  serify run --ref rust --full-matrix --output json go rust
  serify run --ref rust --audit --csv out.csv go rust`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTests(cmd.Context(), args, opts)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opts.casesFile, "cases", "cases", "Path to the directory of per-type test case files")
	f.StringVar(&opts.refLang, "ref", "", "Reference language to compare against (overrides the suite's reference_language)")
	f.BoolVar(&opts.fullMatrix, "full-matrix", false, "Run N×N cross-language deserialization matrix")
	f.BoolVar(&opts.buildOnly, "build-only", false, "Build workers but do not run tests")
	f.BoolVar(&opts.noBuild, "no-build", false, "Skip build step")
	f.IntVar(&opts.timeoutSec, "timeout", 10, "Per-request worker timeout in seconds")
	f.StringVar(&opts.outputFmt, "output", "table", "Output format: table | json | junit")
	f.StringVar(&opts.csvPath, "csv", "", "Also write the full results to this CSV file")
	f.StringVar(
		&opts.knowFDir,
		"known-failures",
		"known_failures",
		"Directory containing <lang>.yaml known failure files",
	)
	f.StringVar(
		&opts.expectSkipDir,
		"expect-skips",
		"expected_skips",
		"Directory containing <lang>.yaml files declaring the coverage a worker may skip; "+
			"any other skip fails the run",
	)
	f.BoolVar(
		&opts.audit,
		"audit",
		false,
		"Enable unsafe-behavior auditing (mutation, stability, output-ZC, zero-copy, input-mutation, deser-stability)",
	)

	return cmd
}

type runOpts struct {
	casesFile     string
	refLang       string
	fullMatrix    bool
	buildOnly     bool
	noBuild       bool
	timeoutSec    int
	outputFmt     string
	csvPath       string
	knowFDir      string
	expectSkipDir string
	audit         bool
}

func (o runOpts) validate() error {
	switch o.outputFmt {
	case "table", "json", "junit":
	default:
		return fmt.Errorf("--output must be table, json, or junit (got %q)", o.outputFmt)
	}
	if o.timeoutSec <= 0 {
		return fmt.Errorf("--timeout must be > 0 (got %d)", o.timeoutSec)
	}
	if o.buildOnly && o.noBuild {
		return errors.New("--build-only and --no-build are mutually exclusive")
	}
	return nil
}

func runTests(ctx context.Context, workerDirs []string, opts runOpts) error {
	if err := opts.validate(); err != nil {
		return err
	}

	set, err := config.LoadSuite(opts.casesFile)
	if err != nil {
		return fmt.Errorf("load cases: %w", err)
	}
	// --ref overrides whatever the suite declared; a suite that declares one
	// makes the flag optional rather than obsolete, since a run may legitimately
	// want to compare against a different worker.
	if opts.refLang != "" {
		set.ReferenceLanguage = opts.refLang
	}
	if set.ReferenceLanguage == "" && !opts.buildOnly {
		return fmt.Errorf("no reference language: pass --ref, or declare reference_language in %s/%s",
			opts.casesFile, config.SuiteConfigFile)
	}

	workerInfos, err := detectAndBuild(workerDirs, opts.noBuild)
	if err != nil {
		return err
	}
	fmt.Println()

	if opts.buildOnly {
		fmt.Println("Build complete (--build-only).")
		return nil
	}

	workers, err := startWorkers(ctx, workerInfos, opts.timeoutSec)
	if err != nil {
		return err
	}
	defer func() {
		for _, w := range workers {
			_ = w.Stop()
		}
	}()

	if _, ok := workers[set.ReferenceLanguage]; !ok {
		return fmt.Errorf("reference language %q not among provided workers (%s)",
			set.ReferenceLanguage, joinLangs(workers))
	}

	knownFails := loadKnownFailures(opts.knowFDir, workers)

	langs := orchestrate.OrderedLangs(workers, set.ReferenceLanguage)
	testIDs := set.TestIDs()
	rep := report.New(langs, testIDs, opts.outputFmt)

	fmt.Println("Suite:")
	for _, ty := range set.Types {
		fmt.Printf("  %-10s formats: %-22s cases: %d\n",
			ty.Name, strings.Join(ty.Formats, ", "), len(ty.Cases))
	}
	fmt.Printf("\nRunning %d checks (type × format × case) across %d languages: %s\n\n",
		len(testIDs), len(workers), strings.Join(langs, ", "))

	if err := orchestrate.RunSuite(ctx, set, workers, rep, orchestrate.Options{
		FullMatrix: opts.fullMatrix,
		TimeoutSec: opts.timeoutSec,
		KnownFails: knownFails,
		Audit:      opts.audit,
	}); err != nil {
		return err
	}

	// A skip is exit-code-neutral, so undeclared coverage loss would pass
	// silently. Opt in by creating the directory: enforcement only runs when it
	// exists, so a suite that has not declared its gaps keeps its old behaviour
	// instead of every existing skip suddenly failing.
	if expected, ok := loadExpectedSkips(opts.expectSkipDir, workers); ok {
		orchestrate.CheckExpectedSkips(rep, expected)
	}

	rep.Print()

	if opts.csvPath != "" {
		records, err := rep.Records()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get csv record: %v\n", err)
		} else {
			if err := writeCSVFile(opts.csvPath, records); err != nil {
				return fmt.Errorf("write csv: %w", err)
			}
			fmt.Printf("Wrote results to %s\n", opts.csvPath)
		}
	}

	if !rep.Success() {
		return errors.New("test failed")
	}
	return nil
}

func detectAndBuild(workerDirs []string, noBuild bool) ([]*builder.WorkerInfo, error) {
	infos := make([]*builder.WorkerInfo, 0, len(workerDirs))
	// Results are keyed by language everywhere downstream — the report, the CSV
	// and the table all have one column per language — so two workers of the
	// same language are ambiguous: the second would overwrite the first and the
	// run would report a single column and a single worker's pass count, with
	// nothing to say a worker's results had been discarded. Reject it instead.
	seenLang := make(map[string]string, len(workerDirs))
	fmt.Println("Detecting workers...")
	for _, dir := range workerDirs {
		info, err := builder.Detect(dir)
		if err != nil {
			return nil, fmt.Errorf("detect worker in %s: %w", dir, err)
		}
		if prev, dup := seenLang[info.Language]; dup {
			return nil, fmt.Errorf(
				"two %s workers given (%s and %s): results are reported per language, "+
					"so only one worker per language can run at a time", info.Language, prev, dir)
		}
		seenLang[info.Language] = dir
		fmt.Printf("  %-30s → %-8s", dir, info.Language)
		infos = append(infos, info)

		if !noBuild {
			if err := builder.Build(info); err != nil {
				fmt.Println("  build: FAILED")
				return nil, err
			}
			fmt.Println("  build: ok")
		} else {
			fmt.Println("  (no-build)")
		}
	}
	return infos, nil
}

func startWorkers(
	ctx context.Context,
	infos []*builder.WorkerInfo,
	timeoutSec int,
) (map[string]*worker.Worker, error) {
	workers := make(map[string]*worker.Worker, len(infos))
	for _, info := range infos {
		w, err := worker.Start(ctx, worker.StartInfo{
			Dir:      info.Dir,
			RunCmd:   info.Manifest.Run,
			Language: info.Language,
		}, timeoutSec)
		if err != nil {
			for _, w := range workers {
				_ = w.Stop()
			}
			return nil, fmt.Errorf("start %s worker: %w", info.Language, err)
		}
		workers[info.Language] = w
	}
	return workers, nil
}

func loadKnownFailures(dir string, workers map[string]*worker.Worker) map[string]map[string]string {
	knownFails := make(map[string]map[string]string)
	for lang := range workers {
		kf, err := config.LoadKnownFailures(dir, lang)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load known failures for %s: %v\n", lang, err)
			continue
		}
		if len(kf) == 0 {
			continue
		}
		knownFails[lang] = make(map[string]string, len(kf))
		for _, f := range kf {
			knownFails[lang][f.TestID] = f.Reason
		}
	}
	return knownFails
}

// loadExpectedSkips reads the per-language declarations of allowed coverage
// gaps. It reports ok=false when the directory does not exist, which means the
// suite has not opted in and enforcement must stay off. Once opted in, a
// language with no file is expected to cover everything.
func loadExpectedSkips(dir string, workers map[string]*worker.Worker) (map[string]config.ExpectedSkips, bool) {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, false
	}
	out := make(map[string]config.ExpectedSkips, len(workers))
	for lang := range workers {
		es, err := config.LoadExpectedSkips(dir, lang)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load expected skips for %s: %v\n", lang, err)
			continue
		}
		out[lang] = es
	}
	return out, true
}

func writeCSVFile(path string, recs []report.Record) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := report.WriteCSV(f, recs); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func joinLangs(workers map[string]*worker.Worker) string {
	return strings.Join(slices.Sorted(maps.Keys(workers)), ", ")
}
