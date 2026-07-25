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

// Package compare provides hex-level and field-level diff utilities.
// All diffs are plain text (no ANSI escape codes); colorization is the
// caller's responsibility at render time.
package compare

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/google/go-cmp/cmp"
)

// HexDiff compares two hex strings byte-by-byte and returns an
// offset-anchored diff with a context window, or "" if equal.
func HexDiff(expected, got string) string {
	expBytes, err := hex.DecodeString(expected)
	if err != nil {
		return fmt.Sprintf("Error decoding expected hex: %v", err)
	}
	gotBytes, err := hex.DecodeString(got)
	if err != nil {
		return fmt.Sprintf("Error decoding got hex: %v", err)
	}

	if expected == got {
		return ""
	}

	return hexOffsetDiff(expBytes, gotBytes)
}

// DataDiff compares two decoded data maps field by field.
// Returns a description of differences, or "" if equal.
//
// Fields named in floatFields hold a float32/float64 value in its wire form —
// IEEE-754 little-endian hex. For those, two values that both decode to NaN are
// treated as equal: all NaNs are the same value, so the bit pattern is not
// significant under a value (semantic) comparison. Pass nil to compare every
// field verbatim (byte-exact), which is what the bytes oracle's other rounds do.
func DataDiff(expected, got map[string]any, fieldOrder []string, floatFields map[string]bool) string {
	if len(fieldOrder) == 0 {
		// fallback: compare all keys in expected (sorted for stable output)
		fieldOrder = slices.Sorted(maps.Keys(expected))
	}

	var diffs []string
	for _, k := range fieldOrder {
		ev := expected[k]
		gv := got[k]

		if floatFields[k] && bothNaNHex(ev, gv) {
			continue // both NaN → equal by value, whatever the bit pattern
		}

		// cmp.Diff output is plain text (no ANSI codes); callers that render
		// to a terminal can colorize lines by their - / + prefix.
		if diff := cmp.Diff(ev, gv); diff != "" {
			diffs = append(diffs, fmt.Sprintf("  field %q:\n%s", k, diff))
		}
	}
	// Check for unexpected fields in got.
	for k := range got {
		if _, ok := expected[k]; !ok {
			diffs = append(diffs, fmt.Sprintf("  field %q: unexpected (not in expected)", k))
		}
	}
	return strings.Join(diffs, "\n")
}

// bothNaNHex reports whether a and b are both float32/float64 wire hex strings
// (4 or 8 bytes, little-endian) that decode to NaN.
func bothNaNHex(a, b any) bool {
	return isNaNHex(a) && isNaNHex(b)
}

func isNaNHex(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return false
	}
	switch len(raw) {
	case 4:
		return math.IsNaN(float64(math.Float32frombits(binary.LittleEndian.Uint32(raw))))
	case 8:
		return math.IsNaN(math.Float64frombits(binary.LittleEndian.Uint64(raw)))
	default:
		return false
	}
}

// contextWindow is the number of bytes on each side of the first divergence
// to include in the diff output.
const contextWindow = 16

// hexRowSize is the number of bytes displayed per hex-dump row.
const hexRowSize = 16

// hexColWidth is the number of characters in the hex column per byte
// (2 hex digits + 1 space).
const hexColPad = 3

// hexOffsetDiff produces a purpose-built binary diff with:
//   - length comparison
//   - first divergent byte offset
//   - context window of hex + ASCII for both buffers
//   - marker on the divergent byte column
func hexOffsetDiff(a, b []byte) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "length: expected %d, got %d\n", len(a), len(b))

	// Find the first divergent byte.
	firstDiv := -1
	minLen := min(len(a), len(b))
	for i := range minLen {
		if a[i] != b[i] {
			firstDiv = i
			break
		}
	}
	if firstDiv < 0 && len(a) != len(b) {
		// Lengths differ but common prefix matches: divergence is after the shorter buffer.
		firstDiv = minLen
	}
	if firstDiv < 0 {
		return "" // shouldn't happen if HexDiff found a diff
	}

	fmt.Fprintf(&sb, "first divergence at offset %d (0x%x)\n", firstDiv, firstDiv)

	// Compute context window boundaries.
	start, end := contextBounds(firstDiv, len(a), len(b))

	sb.WriteString("\n--- expected\n")
	sb.WriteString(hexRows(a, start, end, firstDiv))
	sb.WriteString("+++ got\n")
	sb.WriteString(hexRows(b, start, end, firstDiv))

	return sb.String()
}

// contextBounds returns the [start, end) byte range for the context window
// around the divergent byte at div.
func contextBounds(div, lenA, lenB int) (int, int) {
	start := max(div-contextWindow, 0)
	end := min(div+1+contextWindow, max(lenA, lenB))
	return start, end
}

// hexRows formats the bytes in [start, end) as hex + ASCII rows.
// The divergent byte at div gets a visual marker.
func hexRows(data []byte, start, end, div int) string {
	var sb strings.Builder
	for rowStart := start; rowStart < end; rowStart += hexRowSize {
		// rowWidth is the row's width in the context window, before clamping to
		// this buffer. The blank row below is padded to it — clamping first would
		// make it negative whenever this buffer ends before the row starts, which
		// is exactly the case for the shorter of two buffers.
		rowWidth := min(rowStart+hexRowSize, end) - rowStart
		if rowStart >= len(data) {
			// Completely past this buffer's end.
			fmt.Fprintf(&sb, "%08x  %s\n", rowStart, strings.Repeat("   ", rowWidth))
			break
		}
		rowEnd := min(rowStart+rowWidth, len(data))

		// Offset column.
		fmt.Fprintf(&sb, "%08x  ", rowStart)

		// Hex column.
		hexParts := make([]string, 0, rowEnd-rowStart)
		for i := rowStart; i < rowEnd; i++ {
			h := fmt.Sprintf("%02x", data[i])
			if i == div {
				h = "[" + h + "]" // mark the divergent byte
			}
			hexParts = append(hexParts, h)
		}
		hexLine := strings.Join(hexParts, " ")
		sb.WriteString(hexLine)

		// Pad hex column if this row is short.
		if rowEnd-rowStart < hexRowSize {
			pad := (hexRowSize - (rowEnd - rowStart)) * hexColPad
			sb.WriteString(strings.Repeat(" ", pad))
		}

		// ASCII gutter.
		sb.WriteString("  |")
		for i := rowStart; i < rowEnd; i++ {
			b := data[i]
			if b >= 32 && b < 127 {
				sb.WriteByte(b)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}
	return sb.String()
}

// ColorizeDiff applies terminal color to - / + prefixed lines in a plain-text
// diff string. Use this at render time to get colored output without baking
// ANSI codes into the data layer.
func ColorizeDiff(plain, minusColor, plusColor string) string {
	var sb strings.Builder
	for line := range strings.SplitSeq(plain, "\n") {
		switch {
		case strings.HasPrefix(line, "- "):
			sb.WriteString(minusColor)
			sb.WriteString(line)
			sb.WriteString("\033[0m")
		case strings.HasPrefix(line, "+ "):
			sb.WriteString(plusColor)
			sb.WriteString(line)
			sb.WriteString("\033[0m")
		default:
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
