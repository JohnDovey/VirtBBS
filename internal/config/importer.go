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

package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ImportPCBoardDAT reads a PCBoard PCBOARD.DAT file (line-oriented text) and
// returns a VirtBBS Config populated with the values it recognises.
// PCBOARD.DAT fields are positional (line number determines meaning).
// Reference: pcboard/Develop 04-15-97/PCBOARD.DOC
func ImportPCBoardDAT(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r\n"))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	cfg := defaults()

	// Line numbers below are 1-based as documented in PCBOARD.DOC.
	// We map only the fields VirtBBS uses; all others are ignored.
	get := func(n int) string {
		if n < 1 || n > len(lines) {
			return ""
		}
		return strings.TrimSpace(lines[n-1])
	}

	// Line 1: version string (ignored)
	// Line 4: sysop name
	if v := get(4); v != "" {
		cfg.Sysop.Name = v
	}
	// Line 5: sysop password (plain — we store as-is; sysop should reset)
	// We do NOT import the plain password into the hash field.

	// Line 52: BBS name
	if v := get(52); v != "" {
		cfg.BBS.Name = v
	}

	// Line 57: max nodes
	// (numeric string)
	if v := get(57); v != "" {
		var n int
		if _, err := parseIntLine(v, &n); err == nil && n > 0 {
			cfg.BBS.MaxNodes = n
		}
	}

	// Modem/port settings are intentionally not imported — VirtBBS uses Telnet/SSH.

	return cfg, nil
}

func parseIntLine(s string, out *int) (string, error) {
	_, err := fmt.Sscanf(s, "%d", out)
	return s, err
}
