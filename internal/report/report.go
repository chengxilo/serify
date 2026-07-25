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

// Package report formats and prints test results to the terminal.
package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/chengxilo/serify/internal/compare"
)

type Status string

const (
	StatusPass  Status = "PASS"
	StatusFail  Status = "FAIL"
	StatusXFail Status = "XFAIL"
	StatusXPass Status = "XPASS"
	StatusSkip  Status = "SKIP"
	StatusError Status = "ERROR"
	StatusWarn  Status = "WARN"
)

// sectionRuleWidth is the width of the "-----" rule under the FAILURES and
// WARNINGS headings.
const sectionRuleWidth = 60

// Operation names for a Result (ser/deser match the worker op names).
const (
	OpSerialize           = "serialize"
	OpDeserialize         = "deserialize"
	OpMatrix              = "matrix"
	OpAuditMutation       = "audit-mutation"
	OpAuditZeroCopy       = "audit-zero-copy"
	OpAuditStability      = "audit-stability"
	OpAuditInputMut       = "audit-input-mutation"
	OpAuditOutputZeroCopy = "audit-output-zero-copy"
	OpAuditDeserStability = "audit-deser-stability"
)

// Result holds the outcome for one (test, language, Operation) combination.
type Result struct {
	TestID    string
	Language  string
	Operation string
	Status    Status
	Detail    string
}

// Report aggregates all results for printing.
type Report struct {
	mu        sync.Mutex
	Languages []string
	TestIDs   []string
	Results   map[string]map[string]map[string]Result // testID → lang → Operation → Result
	Failures  []Result
	Warnings  []Result
	OutputFmt string
}

func New(langs, testIDs []string, outputFmt string) *Report {
	return &Report{
		Languages: langs,
		TestIDs:   testIDs,
		Results:   make(map[string]map[string]map[string]Result),
		OutputFmt: outputFmt,
	}
}

func (r *Report) Add(res Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Results[res.TestID] == nil {
		r.Results[res.TestID] = make(map[string]map[string]Result)
	}
	if r.Results[res.TestID][res.Language] == nil {
		r.Results[res.TestID][res.Language] = make(map[string]Result)
	}
	r.Results[res.TestID][res.Language][res.Operation] = res
	sameKey := func(o Result) bool {
		return o.TestID == res.TestID && o.Language == res.Language && o.Operation == res.Operation
	}
	switch res.Status {
	case StatusPass, StatusXFail, StatusSkip:
	case StatusFail, StatusError:
		if !slices.ContainsFunc(r.Failures, sameKey) {
			r.Failures = append(r.Failures, res)
		}
	case StatusWarn, StatusXPass:
		if !slices.ContainsFunc(r.Warnings, sameKey) {
			r.Warnings = append(r.Warnings, res)
		}
	}
}

func (r *Report) Print() {
	switch r.OutputFmt {
	case "junit":
		r.printJUnit()
	case "json":
		r.printJSON()
	default:
		r.printTable()
	}
}

var (
	passColor  = color.New(color.FgGreen, color.Bold)
	failColor  = color.New(color.FgRed, color.Bold)
	xfailColor = color.New(color.FgYellow)
	xpassColor = color.New(color.FgYellow, color.Bold)
	skipColor  = color.New(color.FgCyan)
	errColor   = color.New(color.FgMagenta, color.Bold)
	warnColor  = color.New(color.FgYellow)
)

func statusColored(s Status) string {
	switch s {
	case StatusPass:
		return passColor.Sprint("PASS")
	case StatusFail:
		return failColor.Sprint("FAIL")
	case StatusXFail:
		return xfailColor.Sprint("XFAIL")
	case StatusXPass:
		return xpassColor.Sprint("XPASS")
	case StatusSkip:
		return skipColor.Sprint("SKIP")
	case StatusWarn:
		return warnColor.Sprint("WARN")
	case StatusError:
		return errColor.Sprint("ERROR")
	}
	return errColor.Sprint("UNKNOWN")
}

//nolint:gocognit // prints the failure and warning sections, then tallies every status
func (r *Report) printTable() {
	records, err := r.Records()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get records from report: %v\n", err)
		os.Exit(1)
	}
	// The grid is rendered from the canonical records (same data as the CSV).
	_ = RenderTable(os.Stdout, records)

	if len(r.Failures) > 0 {
		fmt.Println()
		fmt.Println(failColor.Sprint("FAILURES:"))
		fmt.Println(strings.Repeat("-", sectionRuleWidth))
		for _, f := range r.Failures {
			fmt.Printf("[%s / %s / %s]\n", f.TestID, f.Language, f.Operation)
			if f.Detail != "" {
				fmt.Print(compare.ColorizeDiff("  "+f.Detail,
					failColor.Sprint("-"), failColor.Sprint("+")))
			}
			fmt.Println()
		}
	}

	if len(r.Warnings) > 0 {
		fmt.Println()
		fmt.Println(warnColor.Sprint("WARNINGS:"))
		fmt.Println(strings.Repeat("-", sectionRuleWidth))
		for _, w := range r.Warnings {
			fmt.Printf("[%s / %s / %s]\n", w.TestID, w.Language, w.Operation)
			if w.Detail != "" {
				fmt.Print(compare.ColorizeDiff("  "+w.Detail,
					warnColor.Sprint("-"), warnColor.Sprint("+")))
			}
			fmt.Println()
		}
	}

	if unimplemented := r.unimplementedTypes(); len(unimplemented) > 0 {
		fmt.Println()
		fmt.Println(warnColor.Sprintf(
			"No worker implements: %s", strings.Join(unimplemented, ", ")))
		fmt.Println("  Every result for these is SKIP, which in the grid above reads the same as a")
		fmt.Println("  healthy run — nothing about them was actually compared.")
	}

	var pass, fail, xfail, xpass, skip, warn int
	for _, byLang := range r.Results {
		for _, byOperation := range byLang {
			for _, res := range byOperation {
				switch res.Status {
				case StatusPass:
					pass++
				case StatusFail, StatusError:
					fail++
				case StatusXFail:
					xfail++
				case StatusXPass:
					xpass++
				case StatusSkip:
					skip++
				case StatusWarn:
					warn++
				}
			}
		}
	}
	exitStr := "(exit code 0)"
	if fail > 0 {
		exitStr = "(exit code 1)"
	}

	fmt.Printf("%v  %v  %v  %v  %v  %v  %s\n",
		failColor.Sprintf("FAIL: %d", fail),
		xfailColor.Sprintf("XFAIL: %d", xfail),
		xpassColor.Sprintf("XPASS: %d", xpass),
		skipColor.Sprintf("SKIPPED: %d", skip),
		warnColor.Sprintf("WARN: %d", warn),
		passColor.Sprintf("PASSED: %d", pass),
		exitStr,
	)
}

// unimplementedTypes returns the types for which every worker skipped every
// case, in name order. A type nobody implements produces a column of SKIP, which
// looks exactly like a healthy run in the grid: the run reports exit 0 having
// compared nothing at all about it.
func (r *Report) unimplementedTypes() []string {
	seen := make(map[string]bool)
	live := make(map[string]bool)
	for testID, byLang := range r.Results {
		typ, _, _ := strings.Cut(testID, "/")
		seen[typ] = true
		for _, byOperation := range byLang {
			for _, res := range byOperation {
				if res.Status != StatusSkip {
					live[typ] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for typ := range seen {
		if !live[typ] {
			out = append(out, typ)
		}
	}
	sort.Strings(out)
	return out
}

func (r *Report) Success() bool {
	return len(r.Failures) == 0
}

type junitSuites struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Time     float64     `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string     `xml:"name,attr"`
	Classname string     `xml:"classname,attr"`
	Time      float64    `xml:"time,attr"`
	Failure   *junitFail `xml:"failure,omitempty"`
	Skipped   *junitSkip `xml:"skipped,omitempty"`
}

type junitFail struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

type junitSkip struct{}

func (r *Report) printJSON() {
	records, err := r.Records()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get records from report: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(records); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
		os.Exit(1)
	}
}

func (r *Report) printJUnit() {
	suites := junitSuites{}
	for _, lang := range r.Languages {
		suite := junitSuite{Name: lang}
		for _, tid := range r.TestIDs {
			for _, Operation := range []string{OpSerialize, OpDeserialize} {
				suite.Tests++
				tc := junitCase{
					Name:      fmt.Sprintf("%s/%s", tid, Operation),
					Classname: lang,
					Time:      float64(time.Millisecond) / float64(time.Second),
				}
				res, ok := r.Results[tid][lang][Operation]
				if ok {
					switch res.Status {
					case StatusFail, StatusError:
						suite.Failures++
						tc.Failure = &junitFail{Message: string(res.Status), Text: res.Detail}
					case StatusSkip, StatusXFail:
						tc.Skipped = &junitSkip{}
					case StatusPass, StatusXPass, StatusWarn:
					}
				}
				suite.Cases = append(suite.Cases, tc)
			}
		}
		suites.Suites = append(suites.Suites, suite)
	}
	out, _ := xml.MarshalIndent(suites, "", "  ")
	fmt.Println(xml.Header + string(out))
}
