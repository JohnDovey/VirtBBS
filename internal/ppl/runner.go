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

package ppl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Run compiles and executes a PPL source file (.PPS) given an Environment.
// Returns any execution error.
func Run(ppsPath string, env *Environment) error {
	src, err := os.ReadFile(ppsPath)
	if err != nil {
		return fmt.Errorf("ppl: read %s: %w", ppsPath, err)
	}
	env.PPEPath = filepath.Dir(ppsPath)
	return RunSource(string(src), env)
}

// RunSource compiles and executes a PPL source string.
func RunSource(src string, env *Environment) error {
	lexer := NewLexer(src)
	tokens := lexer.Tokenize()
	parser := NewParser(tokens)
	prog, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("ppl parse: %w", err)
	}
	interp := NewInterpreter(prog, env)
	return interp.Run()
}

// EnvFromSession creates a PPL Environment connected to a BBS session's I/O.
// rw is the session ReadWriter, user fields are passed in separately.
func EnvFromSession(rw io.ReadWriter, userName, userCity string, userSec, timesOn, nodeNum int, bbsName, sysopName string) *Environment {
	rd := newBufReader(rw)
	return &Environment{
		Print: func(s string) {
			_, _ = io.WriteString(rw, s)
		},
		Input: func(prompt string) string {
			if prompt != "" {
				_, _ = io.WriteString(rw, prompt)
			}
			line, _ := rd.ReadString('\n')
			return strings.TrimRight(line, "\r\n")
		},
		ReadKey: func() byte {
			buf := make([]byte, 1)
			_, _ = rw.Read(buf)
			return buf[0]
		},
		DisplayFile: func(path string) {
			data, err := os.ReadFile(path)
			if err != nil {
				return
			}
			_, _ = rw.Write(data)
		},
		Hangup: func() {
			// The session layer handles the actual disconnect;
			// we just signal quit via the interpreter's signal mechanism.
		},
		UserName:    userName,
		UserCity:    userCity,
		UserSec:     userSec,
		UserTimesOn: timesOn,
		NodeNum:     nodeNum,
		BBSName:     bbsName,
		SysopName:   sysopName,
	}
}

// newBufReader wraps an io.ReadWriter with a simple line reader.
func newBufReader(r io.Reader) interface{ ReadString(byte) (string, error) } {
	return &simpleBufReader{r: r}
}

type simpleBufReader struct {
	r   io.Reader
	buf []byte
}

func (b *simpleBufReader) ReadString(delim byte) (string, error) {
	for {
		p := make([]byte, 1)
		n, err := b.r.Read(p)
		if n > 0 {
			b.buf = append(b.buf, p[0])
			if p[0] == delim {
				line := string(b.buf)
				b.buf = b.buf[:0]
				return line, nil
			}
		}
		if err != nil {
			return string(b.buf), err
		}
	}
}
