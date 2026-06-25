// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.1.1  2026-06-25  Initial implementation — UTF-8 to CP437 output translation
// ============================================================================

package ansi

import "strings"

// cp437Map translates Unicode runes used in VirtBBS source/display files
// (box-drawing, block elements, and a handful of punctuation marks) into
// their single-byte CP437 codes. Classic BBS terminals such as SyncTerm,
// NetRunner, and mTelnet assume the CP437 codepage, not UTF-8 — sending
// the raw multi-byte UTF-8 sequence for e.g. '╔' causes each byte to be
// rendered as its own (wrong) CP437 glyph, producing visible garbage like
// "Γòö" instead of a box-drawing corner.
//
// Where CP437 has no equivalent glyph (em dash, ellipsis, multiplication
// sign) the rune is folded to a single-width ASCII approximation instead,
// to avoid disturbing fixed-width layouts.
var cp437Map = map[rune]byte{
	'§': 0x15, // section sign
	'±': 0xF1, // plus-minus sign
	'×': 'x',  // multiplication sign (no CP437 glyph) — ASCII fallback
	'—': '-',  // em dash (no CP437 glyph) — ASCII fallback
	'…': '.',  // ellipsis (no CP437 glyph) — single-width ASCII fallback
	'→': 0x1A, // rightwards arrow
	'─': 0xC4, // box drawings light horizontal
	'│': 0xB3, // box drawings light vertical
	'┌': 0xDA, // box drawings light down and right
	'┐': 0xBF, // box drawings light down and left
	'└': 0xC0, // box drawings light up and right
	'┘': 0xD9, // box drawings light up and left
	'═': 0xCD, // box drawings double horizontal
	'║': 0xBA, // box drawings double vertical
	'╔': 0xC9, // box drawings double down and right
	'╗': 0xBB, // box drawings double down and left
	'╚': 0xC8, // box drawings double up and right
	'╝': 0xBC, // box drawings double up and left
	'▀': 0xDF, // upper half block
	'▄': 0xDC, // lower half block
	'█': 0xDB, // full block
}

// ToCP437 rewrites s, converting the Unicode characters above into their
// CP437 single-byte codes and leaving plain ASCII untouched. Any other
// non-ASCII rune (none expected in current VirtBBS source/display files)
// is folded to '?' as a safe fallback rather than corrupting the stream.
//
// Call this as the last step before writing text to a terminal connection
// — i.e. inside the I/O funnel functions, not earlier — so that internal
// string-building logic (width padding, etc.) continues to operate on
// ordinary UTF-8 strings where each of these characters is exactly one
// rune wide.
func ToCP437(s string) string {
	hasNonASCII := false
	for _, r := range s {
		if r > 127 {
			hasNonASCII = true
			break
		}
	}
	if !hasNonASCII {
		return s // fast path: nothing to translate
	}

	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if r < 128 {
			sb.WriteByte(byte(r))
			continue
		}
		if b, ok := cp437Map[r]; ok {
			sb.WriteByte(b)
			continue
		}
		sb.WriteByte('?')
	}
	return sb.String()
}
