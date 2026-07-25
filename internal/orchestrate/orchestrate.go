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

// Package orchestrate drives the test rounds across all workers concurrently.
package orchestrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/chengxilo/serify/internal/compare"
	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/protocol"
	"github.com/chengxilo/serify/internal/report"
	"github.com/chengxilo/serify/internal/typekind"
	"github.com/chengxilo/serify/internal/worker"
)

// Options controls orchestration behaviour.
type Options struct {
	FullMatrix bool
	TimeoutSec int
	KnownFails map[string]map[string]string // lang → testID → reason
	Audit      bool
	// Oracle is the comparison strategy for the format currently under test
	// (config.OracleBytes or config.OracleSemantic). RunSuite sets it per format
	// before calling Run; empty is treated as OracleBytes.
	Oracle string
}

// RunSuite tests every type in the set against the workers, across each of the
// type's serialization formats. For each (type, format) it binds the workers to
// that schema and format, then runs that type's cases (ids namespaced as
// "type/format/case") via Run. Workers that cannot bind a (type, format)
// combination are marked SKIP for it. Every tested type declares its formats
// explicitly (enforced at load time).
func RunSuite(
	ctx context.Context,
	set *config.CasesSet,
	workers map[string]*worker.Worker,
	rep *report.Report,
	opts Options,
) error {
	var firstErr error
	for _, ty := range set.Types {
		for _, format := range ty.Formats {
			// Resolve the comparison oracle for this format (defaults to bytes).
			fmtOpts := opts
			fmtOpts.Oracle = set.OracleFor(format)
			if err := runTypeFormat(ctx, set, ty, format, workers, rep, fmtOpts); err != nil {
				if firstErr == nil {
					firstErr = err
				}
				// Continue with remaining types/formats — the error has
				// already been recorded as ERROR rows for this type/format.
			}
		}
	}
	return firstErr
}

// runTypeFormat binds all workers to one (type, format) and runs its cases.
func runTypeFormat(
	ctx context.Context,
	set *config.CasesSet,
	ty *config.CasesFile,
	format string,
	workers map[string]*worker.Worker,
	rep *report.Report,
	opts Options,
) error {
	sub := make(map[string]*worker.Worker, len(workers))
	for lang, w := range workers {
		err := w.Bind(ctx, ty.Schema, ty.Name, format, opts.TimeoutSec, opts.Audit)
		switch {
		case err == nil:
			sub[lang] = w
		case errors.Is(err, worker.ErrTypeNotSupported):
			// The worker is healthy and told us it does not implement this
			// (type, format). That is a declaration, not a failure.
			markTypeSkipped(rep, lang, ty, format, err.Error())
		default:
			// Anything else — a crash, a timeout, a malformed bind response —
			// is a real failure. Reporting it as SKIP would hide a dead worker
			// behind a zero exit code.
			markTypeErrored(rep, lang, ty, format, err.Error())
		}
	}
	if sub[set.ReferenceLanguage] == nil {
		// Reference worker can't run this type/format; nothing to compare against.
		for lang := range workers {
			if _, ok := sub[lang]; ok {
				markTypeSkipped(rep, lang, ty, format, SkipRefUnsupported)
			}
		}
		return nil
	}

	cf := &config.CasesFile{
		ReferenceLanguage: set.ReferenceLanguage,
		Schema:            ty.Schema,
		Cases:             namespacedCases(ty, format),
	}
	if err := Run(ctx, cf, sub, rep, opts); err != nil {
		return fmt.Errorf("type %s (%s): %w", ty.Name, format, err)
	}
	return nil
}

// namespacedCases prefixes each case id with the type and format for global
// uniqueness.
func namespacedCases(ty *config.CasesFile, format string) []config.TestCase {
	out := make([]config.TestCase, len(ty.Cases))
	for i, tc := range ty.Cases {
		tc.Name = config.TestIDFmt(ty.Name, format, tc.Name)
		out[i] = tc
	}
	return out
}

// markTypeSkipped records SKIP for every case+round of a (type, format) for one language.
func markTypeSkipped(rep *report.Report, lang string, ty *config.CasesFile, format, reason string) {
	markType(rep, lang, ty, format, report.StatusSkip, reason)
}

// markTypeErrored records ERROR for every case+round of a (type, format) for one
// language. Unlike SKIP this counts as a failure and drives a non-zero exit.
func markTypeErrored(rep *report.Report, lang string, ty *config.CasesFile, format, reason string) {
	markType(rep, lang, ty, format, report.StatusError, reason)
}

func markType(
	rep *report.Report, lang string, ty *config.CasesFile, format string, status report.Status, reason string,
) {
	for _, tc := range ty.Cases {
		id := config.TestIDFmt(ty.Name, format, tc.Name)
		for _, round := range []string{report.OpSerialize, report.OpDeserialize} {
			rep.Add(
				report.Result{TestID: id, Language: lang, Operation: round, Status: status, Detail: reason},
			)
		}
	}
}

// Run executes all test rounds and populates rep.
// skip/xfail/audit handling. A real decomposition of this is worth doing separately.
//
//nolint:gocognit,gocyclo,cyclop,funlen // matrix driver: cases x languages x ops, each with its own
func Run(
	ctx context.Context,
	cases *config.CasesFile,
	workers map[string]*worker.Worker,
	rep *report.Report,
	opts Options,
) error {
	// Build ordered language list (reference first)
	refLang := cases.ReferenceLanguage
	langs := OrderedLangs(workers, refLang)

	fieldNames := make([]string, len(cases.Schema))
	// floatFields carry their value as IEEE-754 hex on the wire; DataDiff treats
	// two NaN encodings as equal for them (see compare.DataDiff).
	floatFields := make(map[string]bool)
	for i, f := range cases.Schema {
		fieldNames[i] = f.Name
		if f.Type.Base == typekind.Float32 || f.Type.Base == typekind.Float64 {
			floatFields[f.Name] = true
		}
	}

	var firstErr error

	for _, tc := range cases.Cases {
		// Encode data once for all workers
		encoded, err := encodeCase(tc, cases.Schema)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("test case %s: %w", tc.Name, err)
			}
			for _, lang := range langs {
				for _, op := range []string{report.OpSerialize, report.OpDeserialize} {
					rep.Add(report.Result{
						TestID: tc.Name, Language: lang,
						Operation: op,
						Status:    report.StatusError,
						Detail:    err.Error(),
					})
				}
			}
			continue
		}

		// -- Round 1: Serialize ------------------------------------------
		hexResults := make(map[string]string, len(langs))
		serializedOK := make(map[string]bool, len(langs))
		var hexMu sync.Mutex
		g, gctx := errgroup.WithContext(ctx)

		for _, lang := range langs {
			w := workers[lang]
			g.Go(func() error {
				resp, err := w.Send(gctx, protocol.SerializeRequest{
					ID:   tc.Name,
					Op:   "serialize",
					Data: encoded,
				}, opts.TimeoutSec)

				// Check context cancellation during the send.
				select {
				case <-gctx.Done():
					return nil
				default:
				}

				status, detail := resolveResult(lang, tc.Name, resp, err, opts.KnownFails)

				hexMu.Lock()
				okNow := resp != nil && resp.Status == protocol.StatusOK
				if okNow {
					hexResults[lang] = resp.Hex
					serializedOK[lang] = true
				}
				hexMu.Unlock()

				// Audit: serialize mutation + stability + output zero-copy.
				// Recorded before the verdict, not after: a finding is a property
				// of what the worker did, independent of whether its bytes match,
				// and the deferred-verdict return below would otherwise drop the
				// warnings of every non-reference worker.
				if opts.Audit && resp != nil && resp.Audit != nil {
					a := resp.Audit
					if len(a.Mutations) > 0 {
						warn(rep, tc.Name, lang, report.OpAuditMutation,
							"mutated fields: "+strings.Join(a.Mutations, ", "))
					}
					if a.Stable != nil && !*a.Stable {
						warn(rep, tc.Name, lang, report.OpAuditStability,
							"serializer produced different output on repeat call")
					}
					if len(a.OutputZeroCopyFields) > 0 {
						warn(rep, tc.Name, lang, report.OpAuditOutputZeroCopy,
							"output aliases model fields: "+strings.Join(a.OutputZeroCopyFields, ", "))
					}
				}

				// A non-reference worker that serialized OK has not been judged
				// yet: only the byte comparison below can say whether it passed,
				// and therefore whether a known failure actually held. Recording
				// a status here would report a byte mismatch as OK — and, worse,
				// turn a known failure into an XPASS before the bytes were even
				// looked at.
				if okNow && lang != refLang {
					return nil
				}

				rep.Add(report.Result{
					TestID:    tc.Name,
					Language:  lang,
					Operation: report.OpSerialize,
					Status:    status,
					Detail:    detail,
				})

				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}

		refHex := hexResults[refLang]

		// Compare all non-reference serialization results against reference.
		// Only compare when both the reference and the candidate serialized OK;
		// a worker that returned SKIP/ERROR already has its status recorded.
		for _, lang := range langs {
			if lang == refLang {
				continue
			}
			if !serializedOK[lang] {
				continue // already recorded above (SKIP/ERROR)
			}
			reason, known := opts.KnownFails[lang][tc.Name]

			if !serializedOK[refLang] {
				// Nothing to compare against; the worker itself succeeded.
				rep.Add(report.Result{
					TestID: tc.Name, Language: lang, Operation: report.OpSerialize,
					Status: report.StatusPass,
				})
				continue
			}

			var diff string
			switch opts.Oracle {
			case config.OracleSemantic:
				// Semantic oracle: the reference deserializes the candidate's
				// bytes and we compare the decoded value, not the bytes — so map
				// entry order and other non-canonical wire freedom do not fail.
				// This checks the candidate's serializer; Round 2 checks its
				// deserializer, giving bidirectional semantic conformance.
				diff = semanticSerializeDiff(ctx, workers[refLang], tc.Name, lang,
					hexResults[lang], encoded, fieldNames, floatFields, opts.TimeoutSec)
			default: // OracleBytes (also the empty default)
				diff = compare.HexDiff(refHex, hexResults[lang])
			}
			status, detail := verdict(diff, reason, known)
			rep.Add(report.Result{
				TestID: tc.Name, Language: lang, Operation: report.OpSerialize,
				Status: status, Detail: detail,
			})
		}

		// -- Round 2: Deserialize ----------------------------------------
		if !serializedOK[refLang] {
			// Reference failed to serialize; skip deser for this test case
			for _, lang := range langs {
				rep.Add(report.Result{
					TestID:    tc.Name,
					Language:  lang,
					Operation: report.OpDeserialize,
					Status:    report.StatusSkip,
					Detail:    SkipRefSerializeFailed,
				})
			}
			continue
		}

		g2, gctx2 := errgroup.WithContext(ctx)
		for _, lang := range langs {
			w := workers[lang]
			g2.Go(func() error {
				resp, err := w.Send(gctx2, protocol.DeserializeRequest{
					ID:  tc.Name,
					Op:  "deserialize",
					Hex: refHex,
				}, opts.TimeoutSec)

				select {
				case <-gctx2.Done():
					return nil
				default:
				}

				status, detail := resolveResult(lang, tc.Name, resp, err, opts.KnownFails)

				// Only the data comparison can judge a worker that responded OK.
				// Letting resolveResult decide first meant a known failure
				// returned XPASS, which failed the `status == Pass` guard and so
				// skipped the comparison entirely — reporting "expected to fail
				// but passed" for data that had never been compared.
				if resp != nil && resp.Status == protocol.StatusOK {
					reason, known := opts.KnownFails[lang][tc.Name]
					status, detail = verdict(
						compare.DataDiff(encoded, resp.Data, fieldNames, floatFields), reason, known)
				}

				rep.Add(report.Result{
					TestID:    tc.Name,
					Language:  lang,
					Operation: report.OpDeserialize,
					Status:    status,
					Detail:    detail,
				})

				// Audit: zero-copy + input-buffer mutation + deser-stability
				if opts.Audit && resp != nil && resp.Audit != nil {
					a := resp.Audit
					if len(a.ZeroCopyFields) > 0 {
						warn(rep, tc.Name, lang, report.OpAuditZeroCopy,
							"zero-copy fields: "+strings.Join(a.ZeroCopyFields, ", "))
					}
					if a.InputMutated {
						warn(rep, tc.Name, lang, report.OpAuditInputMut,
							"deserializer modified input buffer")
					}
					if a.DeserStable != nil && !*a.DeserStable {
						warn(rep, tc.Name, lang, report.OpAuditDeserStability,
							"deserializer produced different result on repeat call")
					}
				}

				return nil
			})
		}
		if err := g2.Wait(); err != nil {
			return err
		}

		// -- Round 3: Full Matrix (optional) ----------------------------
		if opts.FullMatrix {
			if err := runMatrix(ctx, tc, encoded, fieldNames, floatFields, langs, workers, hexResults, rep, opts); err != nil {
				return err
			}
		}
	}
	return firstErr
}

func runMatrix(
	ctx context.Context,
	tc config.TestCase,
	encoded map[string]any,
	fieldNames []string,
	floatFields map[string]bool,
	langs []string,
	workers map[string]*worker.Worker,
	hexResults map[string]string,
	rep *report.Report,
	opts Options,
) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, srcLang := range langs {
		for _, dstLang := range langs {
			srcHex := hexResults[srcLang]
			if srcHex == "" {
				// Record SKIP for this matrix cell — source language's serialize failed.
				rep.Add(report.Result{
					TestID:    tc.Name,
					Language:  fmt.Sprintf("%s→%s", srcLang, dstLang),
					Operation: report.OpMatrix,
					Status:    report.StatusSkip,
					Detail:    "source serialize failed",
				})
				continue
			}
			w := workers[dstLang]
			g.Go(func() error {
				resp, err := w.Send(gctx, protocol.DeserializeRequest{
					ID:  fmt.Sprintf("%s/matrix-%s→%s", tc.Name, srcLang, dstLang),
					Op:  "deserialize",
					Hex: srcHex,
				}, opts.TimeoutSec)
				status, detail := resolveResult(dstLang, tc.Name, resp, err, opts.KnownFails)
				if status == report.StatusPass && resp != nil {
					diff := compare.DataDiff(encoded, resp.Data, fieldNames, floatFields)
					if diff != "" {
						status = report.StatusFail
						detail = diff
					}
				}
				rep.Add(report.Result{
					TestID:    tc.Name,
					Language:  fmt.Sprintf("%s→%s", srcLang, dstLang),
					Operation: report.OpMatrix,
					Status:    status,
					Detail:    detail,
				})
				return nil
			})
		}
	}
	return g.Wait()
}

// semanticSerializeDiff implements the semantic serialize oracle: it asks the
// reference worker to deserialize a candidate's serialized bytes and diffs the
// decoded value against the expected case data (order-insensitive for maps). A
// non-empty return means the candidate's output did not decode to the right
// value — or the reference could not decode it at all; "" means it conformed.
func semanticSerializeDiff(
	ctx context.Context,
	ref *worker.Worker,
	testID, srcLang, srcHex string,
	encoded map[string]any,
	fieldNames []string,
	floatFields map[string]bool,
	timeoutSec int,
) string {
	resp, err := ref.Send(ctx, protocol.DeserializeRequest{
		ID:  fmt.Sprintf("%s/semantic-%s", testID, srcLang),
		Op:  "deserialize",
		Hex: srcHex,
	}, timeoutSec)
	if err != nil {
		return fmt.Sprintf("reference could not deserialize %s output: %v", srcLang, err)
	}
	if resp == nil || resp.Status != protocol.StatusOK {
		detail := "reference could not deserialize " + srcLang + " output"
		if resp != nil && resp.Error != "" {
			detail += ": " + resp.Error
		}
		return detail
	}
	return compare.DataDiff(encoded, resp.Data, fieldNames, floatFields)
}

// verdict turns one comparison outcome into a status, honouring known failures.
// diff is empty when the bytes (or decoded data) matched.
//
// A known failure can only be settled here, after the comparison: judging it
// from the worker's response status alone reports a byte mismatch as OK, and
// turns a known failure into an XPASS before the bytes have been looked at.
func verdict(diff, reason string, known bool) (report.Status, string) {
	switch {
	case diff != "" && known:
		return report.StatusXFail, reason
	case diff != "":
		return report.StatusFail, diff
	case known:
		return report.StatusXPass, xpassDetail(reason)
	default:
		return report.StatusPass, ""
	}
}

func xpassDetail(reason string) string {
	return fmt.Sprintf("expected to fail (%s) but passed", reason)
}

func resolveResult(
	lang, testID string,
	resp *protocol.Response,
	err error,
	knownFails map[string]map[string]string,
) (report.Status, string) {
	if err != nil {
		return report.StatusError, err.Error()
	}
	if resp == nil {
		return report.StatusError, "no response"
	}

	switch resp.Status {
	case protocol.StatusOK:
		// Check known failures — if expected to fail but passed, return XPASS.
		// The reference worker has nothing to compare against, so its verdict is
		// settled here; every other worker is judged by verdict() after the
		// comparison.
		if reason, ok := knownFails[lang][testID]; ok {
			return report.StatusXPass, xpassDetail(reason)
		}
		return report.StatusPass, ""
	case protocol.StatusSkipped:
		return report.StatusSkip, resp.Reason
	case protocol.StatusError:
		// Check known failures
		if reason, ok := knownFails[lang][testID]; ok {
			return report.StatusXFail, reason
		}
		return report.StatusFail, fmt.Sprintf("Status: %s\nError: %q", resp.Status, resp.Error)
	default:
		return report.StatusError, fmt.Sprintf("unexpected status %q", resp.Status)
	}
}

// warn records one WARN row (used for audit findings).
func warn(rep *report.Report, testID, lang, op, detail string) {
	rep.Add(report.Result{
		TestID: testID, Language: lang,
		Operation: op,
		Status:    report.StatusWarn,
		Detail:    detail,
	})
}

// encodeCase encodes a case's data to wire form, then normalizes it through a
// JSON round-trip so comparisons against worker responses (which arrive via
// json.Unmarshal → map[string]any) use identical Go types on both sides.
func encodeCase(tc config.TestCase, schema []config.Field) (map[string]any, error) {
	encoded, err := protocol.EncodeData(tc.Data, schema)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	normalized, err := jsonRoundTrip(encoded)
	if err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}
	return normalized, nil
}

func jsonRoundTrip(m map[string]any) (map[string]any, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OrderedLangs returns the worker languages with refLang first and the rest
// sorted alphabetically, so report ordering is deterministic across runs.
func OrderedLangs(workers map[string]*worker.Worker, refLang string) []string {
	langs := []string{refLang}
	for _, lang := range slices.Sorted(maps.Keys(workers)) {
		if lang != refLang {
			langs = append(langs, lang)
		}
	}
	return langs
}
