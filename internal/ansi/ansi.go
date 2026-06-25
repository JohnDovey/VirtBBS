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
//   v0.0.1  2026-06-24  Initial implementation
// ============================================================================

// Package ansi provides ANSI escape sequence helpers for terminal output.
package ansi

import "fmt"

// Color codes (foreground)
const (
	Black   = 30
	Red     = 31
	Green   = 32
	Yellow  = 33
	Blue    = 34
	Magenta = 35
	Cyan    = 36
	White   = 37

	BrightBlack   = 90
	BrightRed     = 91
	BrightGreen   = 92
	BrightYellow  = 93
	BrightBlue    = 94
	BrightMagenta = 95
	BrightCyan    = 96
	BrightWhite   = 97
)

// Reset returns the ANSI reset sequence.
func Reset() string { return "\x1b[0m" }

// Color returns an ANSI foreground color sequence.
func Color(code int) string { return fmt.Sprintf("\x1b[%dm", code) }

// Bold returns an ANSI bold sequence.
func Bold() string { return "\x1b[1m" }

// ClearScreen returns the sequence to clear the screen and home the cursor.
func ClearScreen() string { return "\x1b[2J\x1b[H" }

// MoveTo returns a cursor positioning sequence (1-based row/col).
func MoveTo(row, col int) string { return fmt.Sprintf("\x1b[%d;%dH", row, col) }

// Colorize wraps text in a color code followed by a reset.
func Colorize(code int, text string) string {
	return Color(code) + text + Reset()
}

// Header renders a simple BBS-style header bar.
func Header(title string) string {
	return Bold() + Color(BrightCyan) + "=[ " + Color(BrightWhite) + title + Color(BrightCyan) + " ]=" + Reset()
}

// Prompt renders a colored prompt string.
func Prompt(text string) string {
	return Color(BrightGreen) + text + Reset()
}
