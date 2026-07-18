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

package orchestrate

import (
	"fmt"
	"strings"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/report"
)

// Cascade skips: recorded when the *reference* could not produce bytes, so every
// worker's round is skipped as a consequence. These are fallout from a failure
// that is already reported elsewhere, not a statement about a worker's coverage,
// so expected-skips does not govern them.
const (
	SkipRefSerializeFailed = "reference serialize failed"
	SkipRefUnsupported     = "reference worker does not support this type/format"
)

func isCascadeSkip(detail string) bool {
	return detail == SkipRefSerializeFailed || detail == SkipRefUnsupported
}

// CheckExpectedSkips enforces the coverage each worker declares.
//
// A SKIP is exit-code-neutral: it means "this worker does not implement that".
// That is the honest answer when an SDK genuinely has no such code — but it also
// means a renamed type, a dropped registration or a typo'd worker entry turns
// green instead of failing, and the lost coverage never surfaces. Declaring the
// allowed gaps makes every other skip a failure.
//
// Undeclared skip  -> FAIL (coverage regressed)
// Declared but nothing skipped -> WARN (stale entry, delete it)
//
// A language with no expected-skips file is expected to cover everything.
func CheckExpectedSkips(rep *report.Report, expected map[string]config.ExpectedSkips) {
	var undeclared []report.Result
	// lang -> declared entry -> actually used
	used := map[string]map[string]bool{}
	for lang, es := range expected {
		used[lang] = map[string]bool{}
		for _, t := range es.Types {
			used[lang][t] = false
		}
		for op, types := range es.Operations {
			for _, t := range types {
				used[lang][op+"/"+t] = false
			}
		}
	}

	for testID, byLang := range rep.Results {
		typeName, _, _ := strings.Cut(testID, "/")
		for lang, byOp := range byLang {
			for op, res := range byOp {
				if res.Status != report.StatusSkip || isCascadeSkip(res.Detail) {
					continue
				}
				es := expected[lang]
				switch {
				case containsStr(es.Types, typeName):
					used[lang][typeName] = true
				case containsStr(es.Operations[op], typeName):
					used[lang][op+"/"+typeName] = true
				default:
					undeclared = append(undeclared, report.Result{
						TestID:    testID,
						Language:  lang,
						Operation: op,
						Status:    report.StatusFail,
						Detail: fmt.Sprintf(
							"undeclared skip: %q/%s is not covered by expected-skips for %s "+
								"(a real gap must be declared; otherwise coverage regressed)",
							typeName, op, lang),
					})
				}
			}
		}
	}

	for _, r := range undeclared {
		rep.Add(r)
	}
	for lang, keys := range used {
		for key, seen := range keys {
			if seen {
				continue
			}
			rep.Add(report.Result{
				TestID:    key,
				Language:  lang,
				Operation: report.OpSerialize,
				Status:    report.StatusWarn,
				Detail:    fmt.Sprintf("stale expected-skip %q for %s: nothing skipped, remove it", key, lang),
			})
		}
	}
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
