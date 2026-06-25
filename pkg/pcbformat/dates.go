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

package pcbformat

import (
	"fmt"
	"strings"
	"time"
)

// ParseYYMMDD parses a PCBoard YYMMDD 6-byte date string.
func ParseYYMMDD(b []byte) (time.Time, error) {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "000000" {
		return time.Time{}, nil
	}
	return time.Parse("060102", s)
}

// FormatYYMMDD formats a time.Time as a PCBoard YYMMDD 6-byte string.
func FormatYYMMDD(t time.Time) string {
	if t.IsZero() {
		return "000000"
	}
	return t.Format("060102")
}

// ParseHHMM parses a PCBoard HH:MM 5-byte time string.
func ParseHHMM(b []byte) (hour, min int, err error) {
	s := TrimFixed(b)
	if s == "" {
		return 0, 0, nil
	}
	_, err = fmt.Sscanf(s, "%d:%d", &hour, &min)
	return
}
