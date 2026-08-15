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

// Package worker manages a single language worker subprocess.
package worker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chengxilo/serify/internal/config"
	"github.com/chengxilo/serify/internal/protocol"
)

// shutdownGrace is how long Close waits for a worker to exit after the "exit"
// request before killing the process.
const shutdownGrace = 5 * time.Second

// Fatal transport errors: after either of these the worker process is out of
// sync with the runner and is killed (see Send).
var (
	errTimeout    = errors.New("timeout")
	errIDMismatch = errors.New("response id mismatch")
	errOpMismatch = errors.New("response op mismatch")
)

// ErrTypeNotSupported means the worker answered bind with SKIPPED: it is
// running fine but does not implement that (type, format). Only bind operation
// can produce it.
var ErrTypeNotSupported = errors.New("worker does not support this type/format")

// Worker represents a running language worker process.
type Worker struct {
	Language string
	Dir      string

	cmd   *exec.Cmd
	stdin io.WriteCloser

	writer *protocol.Writer
	reader *protocol.Reader

	mu          sync.Mutex
	stderrLines []string
	dead        bool
	deadReason  string
}

// StartInfo describes the worker process to launch.
type StartInfo struct {
	Dir      string
	RunCmd   string
	Language string
}

// Start launches the worker subprocess and performs the ping handshake.
func Start(ctx context.Context, info StartInfo, timeoutSec int) (*Worker, error) {
	dir := info.Dir
	lang := info.Language

	parts := strings.Fields(info.RunCmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty run command for %s", lang)
	}

	// On Windows, native binaries need .exe; if the bare path doesn't exist, try appending it.
	if runtime.GOOS == "windows" {
		bin := parts[0]
		absbin := bin
		if !filepath.IsAbs(bin) {
			absbin = filepath.Join(dir, bin)
		}
		if _, err := os.Stat(absbin); os.IsNotExist(err) {
			if _, err2 := os.Stat(absbin + ".exe"); err2 == nil {
				parts[0] = bin + ".exe"
			}
		}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = dir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s worker: %w", lang, err)
	}

	w := &Worker{
		Language: lang,
		Dir:      dir,
		cmd:      cmd,
		stdin:    stdin,
		writer:   protocol.NewWriter(stdin),
		reader:   protocol.NewReader(stdout),
	}

	// Collect stderr in background
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			w.mu.Lock()
			w.stderrLines = append(w.stderrLines, s.Text())
			w.mu.Unlock()
		}
	}()

	if err := w.ping(ctx, timeoutSec); err != nil {
		_ = w.Stop()
		return nil, fmt.Errorf("failed to ping %s worker: %w", lang, err)
	}

	return w, nil
}

// ping is the startup handshake: it confirms the process came up and speaks the
// same protocol revision as this runner.
func (w *Worker) ping(ctx context.Context, timeoutSec int) error {
	if err := w.writer.Write(protocol.NewPingRequest()); err != nil {
		return err
	}

	resp, err := w.readWithTimeout(ctx, protocol.OpPing, "", timeoutSec)
	if err != nil {
		w.markFatal(err)
		return err
	}
	if resp.Status == protocol.StatusError {
		return fmt.Errorf("%s", resp.Error)
	}
	if resp.Status != protocol.StatusOK {
		return fmt.Errorf("ping answered with status %q; only %q is valid",
			resp.Status, protocol.StatusOK)
	}
	if resp.ProtocolVersion == nil {
		return errors.New("worker reported no protocol_version in its ping response")
	}
	if *resp.ProtocolVersion != protocol.ProtocolVersion {
		return fmt.Errorf("protocol version mismatch: worker library speaks %d, "+
			"this serify needs %d — rebuild the worker against the appropriate library",
			*resp.ProtocolVersion, protocol.ProtocolVersion)
	}
	return nil
}

// Bind points the worker at one (type, format) and its schema: once it returns,
// the worker's serialize and deserialize operate on that pair. The runner sends
// one Bind per (type, format) group in the suite, so a single process handles
// every type in turn. When audit is true, the worker enables unsafe-behaviour
// detection checks. Cancelling ctx interrupts the wait for the worker's response.
//
// A worker that does not implement the pair answers SKIPPED, which comes back as
// ErrTypeNotSupported — a declaration, not a failure.
func (w *Worker) Bind(
	ctx context.Context,
	schema []config.Field,
	typ string,
	format string,
	timeoutSec int,
	audit bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dead {
		return fmt.Errorf("worker is dead: %s", w.deadReason)
	}
	if typ == "" {
		return errors.New("bind requires a type")
	}
	if format == "" {
		return errors.New("bind requires a format")
	}

	req := protocol.NewBindRequest(protocol.SchemaFields(schema), typ, format, audit)
	if err := w.writer.Write(req); err != nil {
		return err
	}

	resp, err := w.readWithTimeout(ctx, protocol.OpBind, "", timeoutSec)
	if err != nil {
		w.markFatal(err)
		return err
	}
	if resp.Status == protocol.StatusError {
		return fmt.Errorf("%s", resp.Error)
	}
	if resp.Status == protocol.StatusSkipped {
		// The worker is healthy, it just doesn't implement this (type, format).
		if resp.Reason == "" {
			return ErrTypeNotSupported
		}
		return fmt.Errorf("%w: %s", ErrTypeNotSupported, resp.Reason)
	}
	return nil
}

// Send sends a request and waits for a response. Cancelling ctx interrupts the
// wait for the worker's response.
func (w *Worker) Send(ctx context.Context, req any, timeoutSec int) (*protocol.Response, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dead {
		return nil, fmt.Errorf("worker is dead: %s", w.deadReason)
	}

	if err := w.writer.Write(req); err != nil {
		return nil, fmt.Errorf("%s worker write: %w", w.Language, err)
	}

	// Match the response against the request. The id alone is not enough: the
	// serialize and deserialize rounds for one case share an id, so only the op
	// distinguishes them.
	resp, err := w.readWithTimeout(ctx, extractOp(req), extractID(req), timeoutSec)
	if err != nil {
		w.markFatal(err)
	}
	return resp, err
}

// markFatal kills the worker and marks it dead when err means the response
// stream can no longer be trusted: a timeout or cancellation abandoned the
// reader goroutine mid-read, and an id or op mismatch means runner and worker
// are desynced. Killing the process unblocks the abandoned reader and prevents a
// later request from racing it on the shared reader, or from consuming the
// stale response.
func (w *Worker) markFatal(err error) {
	if errors.Is(err, errTimeout) || errors.Is(err, errIDMismatch) ||
		errors.Is(err, errOpMismatch) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		w.killAndMark(err.Error())
	}
}

// extractOp returns the op of a request value, or "" if it is not one the
// runner correlates by op.
func extractOp(req any) protocol.Op {
	switch r := req.(type) {
	case protocol.SerializeRequest:
		return r.Op
	case protocol.DeserializeRequest:
		return r.Op
	default:
		return ""
	}
}

// extractID returns the ID field from a request value, or "" if none.
func extractID(req any) string {
	switch r := req.(type) {
	case protocol.SerializeRequest:
		return r.ID
	case protocol.DeserializeRequest:
		return r.ID
	default:
		return ""
	}
}

func (w *Worker) readWithTimeout(
	ctx context.Context,
	expectOp protocol.Op, expectID string,
	timeoutSec int,
) (*protocol.Response, error) {
	type result struct {
		resp *protocol.Response
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		resp, err := w.reader.Read()
		ch <- result{resp, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		// Validate the response ID against the expected request ID.
		if expectID != "" && r.resp.ID != expectID {
			return nil, fmt.Errorf("%w (got %q, want %q)", errIDMismatch, r.resp.ID, expectID)
		}
		if expectOp != "" && r.resp.Op != expectOp {
			return nil, fmt.Errorf("%w (got %q, want %q)", errOpMismatch, r.resp.Op, expectOp)
		}
		return r.resp, nil
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		return nil, fmt.Errorf("%w after %ds", errTimeout, timeoutSec)
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	}
}

// killAndMark kills the worker process and marks it dead with reason.
func (w *Worker) killAndMark(reason string) {
	if w.dead {
		return
	}
	w.dead = true
	w.deadReason = reason
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
		// Reap asynchronously: Wait also closes the stdout pipe, which unblocks
		// any reader goroutine still parked on it.
		go func() { _ = w.cmd.Wait() }()
	}
}

// Stop sends exit and waits for the process to finish. If the worker doesn't
// exit within ~5 seconds, it is killed.
func (w *Worker) Stop() error {
	w.mu.Lock()
	if w.dead {
		w.mu.Unlock()
		return nil // already stopped
	}
	w.dead = true
	w.deadReason = "stopped"
	w.mu.Unlock()

	// Send exit and close stdin.
	_ = w.writer.Write(protocol.NewExitRequest())
	_ = w.stdin.Close()

	if w.cmd == nil || w.cmd.Process == nil {
		return nil
	}

	// Wait for a graceful exit in a goroutine; kill on timeout.
	done := make(chan error, 1)
	go func() {
		done <- w.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(shutdownGrace):
		_ = w.cmd.Process.Kill()
		<-done // reap
		return fmt.Errorf("worker %s did not exit within %s; killed", w.Language, shutdownGrace)
	}
}

// Stderr returns collected stderr output.
func (w *Worker) Stderr() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, len(w.stderrLines))
	copy(out, w.stderrLines)
	return out
}
