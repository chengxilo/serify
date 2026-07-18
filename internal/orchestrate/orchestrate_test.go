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
	"errors"
	"testing"

	"github.com/chengxilo/serify/internal/protocol"
	"github.com/chengxilo/serify/internal/report"
)

func TestResolveResult_OK(t *testing.T) {
	resp := &protocol.Response{Status: protocol.StatusOK}
	status, detail := resolveResult("go", "test/binary/basic", resp, nil, nil)
	if status != report.StatusPass {
		t.Errorf("status = %q, want %q", status, report.StatusPass)
	}
	if detail != "" {
		t.Errorf("detail = %q, want empty", detail)
	}
}

func TestResolveResult_Error(t *testing.T) {
	status, detail := resolveResult("go", "test/binary/basic", nil, errors.New("timeout"), nil)
	if status != report.StatusError {
		t.Errorf("status = %q, want %q", status, report.StatusError)
	}
	if detail != "timeout" {
		t.Errorf("detail = %q", detail)
	}
}

func TestResolveResult_NilResponse(t *testing.T) {
	status, _ := resolveResult("go", "test/binary/basic", nil, nil, nil)
	if status != report.StatusError {
		t.Errorf("status = %q, want %q", status, report.StatusError)
	}
}

func TestResolveResult_Skipped(t *testing.T) {
	resp := &protocol.Response{Status: protocol.StatusSkipped, Reason: "not supported"}
	status, detail := resolveResult("go", "test/binary/basic", resp, nil, nil)
	if status != report.StatusSkip {
		t.Errorf("status = %q, want %q", status, report.StatusSkip)
	}
	if detail != "not supported" {
		t.Errorf("detail = %q", detail)
	}
}

func TestResolveResult_WorkerError(t *testing.T) {
	resp := &protocol.Response{Status: protocol.StatusError, Error: "bad data"}
	status, _ := resolveResult("go", "test/binary/basic", resp, nil, nil)
	if status != report.StatusFail {
		t.Errorf("status = %q, want %q", status, report.StatusFail)
	}
}

func TestResolveResult_KnownFailure(t *testing.T) {
	resp := &protocol.Response{Status: protocol.StatusError, Error: "bad data"}
	kf := map[string]map[string]string{
		"go": {"test/binary/basic": "known issue #42"},
	}
	status, detail := resolveResult("go", "test/binary/basic", resp, nil, kf)
	if status != report.StatusXFail {
		t.Errorf("status = %q, want %q", status, report.StatusXFail)
	}
	if detail != "known issue #42" {
		t.Errorf("detail = %q", detail)
	}
}

func TestResolveResult_UnknownStatus(t *testing.T) {
	resp := &protocol.Response{Status: "WEIRD"}
	status, _ := resolveResult("go", "test/binary/basic", resp, nil, nil)
	if status != report.StatusError {
		t.Errorf("status = %q, want %q", status, report.StatusError)
	}
}
