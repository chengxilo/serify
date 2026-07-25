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

package compare

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestHexDiff_Equal(t *testing.T) {
	a := hex.EncodeToString([]byte("hello"))
	b := hex.EncodeToString([]byte("hello"))
	if diff := HexDiff(a, b); diff != "" {
		t.Errorf("equal buffers should produce empty diff, got:\n%s", diff)
	}
}

func TestHexDiff_OffsetAndContext(t *testing.T) {
	a := make([]byte, 100)
	b := make([]byte, 100)
	// Diverge at byte 42.
	b[42] = 0xFF

	diff := HexDiff(hex.EncodeToString(a), hex.EncodeToString(b))
	if diff == "" {
		t.Fatal("expected non-empty diff for divergent buffers")
	}
	if !strings.Contains(diff, "length: expected 100, got 100") {
		t.Errorf("diff should report lengths:\n%s", diff)
	}
	if !strings.Contains(diff, "first divergence at offset 42 (0x2a)") {
		t.Errorf("diff should report first divergence offset:\n%s", diff)
	}
	if !strings.Contains(diff, "[00]") {
		t.Errorf("diff should mark expected's byte at offset 42 with [00]:\n%s", diff)
	}
	if !strings.Contains(diff, "[ff]") {
		t.Errorf("diff should mark got's byte at offset 42 with [ff]:\n%s", diff)
	}
}

func TestHexDiff_LengthMismatch(t *testing.T) {
	a := hex.EncodeToString([]byte("abc"))
	b := hex.EncodeToString([]byte("abcdef"))
	diff := HexDiff(a, b)
	if !strings.Contains(diff, "length: expected 3, got 6") {
		t.Errorf("diff should report length mismatch:\n%s", diff)
	}
}

func TestHexDiff_InvalidHex(t *testing.T) {
	diff := HexDiff("not hex", "deadbeef")
	if !strings.Contains(diff, "Error decoding expected hex") {
		t.Errorf("diff should report decode error:\n%s", diff)
	}
}

func TestHexDiff_NoANSI(t *testing.T) {
	// All diffs must be plain text — no ANSI escapes.
	a := make([]byte, 32)
	b := make([]byte, 32)
	b[10] = 0xAB

	diff := HexDiff(hex.EncodeToString(a), hex.EncodeToString(b))
	if strings.Contains(diff, "\x1b[") {
		t.Errorf("diff must contain no ANSI escapes:\n%s", diff)
	}
}

func TestHexDiff_ContextWindow(t *testing.T) {
	// Verify the context window shows ~16 bytes around the divergence.
	a := make([]byte, 128)
	b := make([]byte, 128)
	b[64] = 0xCC // diverge at byte 64

	diff := HexDiff(hex.EncodeToString(a), hex.EncodeToString(b))

	// Should include bytes around offset 64 (context window of 16).
	// Byte 64-16 = 48 should be visible, byte 64+16 = 80 should be too.
	if !strings.Contains(diff, "00000030") { // offset 48
		t.Errorf("context window should include bytes near divergence:\n%s", diff)
	}
	if !strings.Contains(diff, "00000050") { // offset 80
		t.Errorf("context window should include bytes after divergence:\n%s", diff)
	}
}

func TestHexDiff_BeginningOfBuffer(t *testing.T) {
	// Divergence at offset 0 — context should handle bounds correctly.
	a := []byte{0x00, 0x01, 0x02}
	b := []byte{0xFF, 0x01, 0x02}

	diff := HexDiff(hex.EncodeToString(a), hex.EncodeToString(b))
	if !strings.Contains(diff, "first divergence at offset 0 (0x0)") {
		t.Errorf("should detect divergence at offset 0:\n%s", diff)
	}
}

func TestColorizeDiff(t *testing.T) {
	plain := "- removed\n+ added\n  unchanged"
	colored := ColorizeDiff(plain, "\033[31m", "\033[32m")

	if !strings.Contains(colored, "\033[31m- removed\033[0m") {
		t.Errorf("minus lines should be colored:\n%s", colored)
	}
	if !strings.Contains(colored, "\033[32m+ added\033[0m") {
		t.Errorf("plus lines should be colored:\n%s", colored)
	}
	if !strings.Contains(colored, "  unchanged") {
		t.Errorf("unchanged lines should not be colored:\n%s", colored)
	}
}

func TestDataDiff_PlainText(t *testing.T) {
	a := map[string]any{"x": 1, "y": "hello"}
	b := map[string]any{"x": 2, "y": "hello"}
	diff := DataDiff(a, b, []string{"x", "y"}, nil)
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if strings.Contains(diff, "\x1b[") {
		t.Errorf("DataDiff must contain no ANSI escapes:\n%s", diff)
	}
}

// For a float field, two different NaN bit patterns are equal by value; for a
// non-float field the same hex strings must still diff (bit-exact).
func TestDataDiff_NaNFloatEquality(t *testing.T) {
	qNaN32 := "0000c07f" // 0x7fc00000, quiet NaN
	sNaN32 := "0100807f" // 0x7f800001, a different NaN bit pattern
	qNaN64 := "000000000000f87f"

	// float field: different NaN encodings compare equal.
	if diff := DataDiff(
		map[string]any{"f": qNaN32}, map[string]any{"f": sNaN32},
		[]string{"f"}, map[string]bool{"f": true},
	); diff != "" {
		t.Errorf("float NaN field: want equal, got diff:\n%s", diff)
	}
	if diff := DataDiff(
		map[string]any{"d": qNaN64}, map[string]any{"d": "0000000000f0ff7f"},
		[]string{"d"}, map[string]bool{"d": true},
	); diff != "" {
		t.Errorf("float64 NaN field: want equal, got diff:\n%s", diff)
	}

	// Same strings, but NOT declared a float field → must still diff (a bytes
	// field must stay bit-exact).
	if diff := DataDiff(
		map[string]any{"b": qNaN32}, map[string]any{"b": sNaN32},
		[]string{"b"}, nil,
	); diff == "" {
		t.Error("non-float field: want diff on differing bytes, got equal")
	}

	// A real value difference in a float field is still caught (1.0 vs 2.0).
	if diff := DataDiff(
		map[string]any{"f": "0000803f"}, map[string]any{"f": "00000040"},
		[]string{"f"}, map[string]bool{"f": true},
	); diff == "" {
		t.Error("float field 1.0 vs 2.0: want diff, got equal")
	}
}
