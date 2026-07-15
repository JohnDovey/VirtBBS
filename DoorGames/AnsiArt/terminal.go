package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	KeyNone = iota
	KeyQuit
	KeyEnter
	KeyOther
)

// Terminal wraps stdin/stdout for ANSI door I/O.
type Terminal struct {
	in       io.Reader
	out      io.Writer
	br       *bufio.Reader
	oldState *term.State
	raw      bool
}

func NewTerminal(in io.Reader, out io.Writer) *Terminal {
	t := &Terminal{in: in, out: out, br: bufio.NewReader(in)}
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if st, err := term.MakeRaw(int(f.Fd())); err == nil {
			t.oldState = st
			t.raw = true
		}
	}
	return t
}

func (t *Terminal) Close() {
	if t.raw && t.oldState != nil {
		if f, ok := t.in.(*os.File); ok {
			_ = term.Restore(int(f.Fd()), t.oldState)
		}
	}
}

func (t *Terminal) Print(s string)                 { fmt.Fprint(t.out, s) }
func (t *Terminal) Printf(f string, a ...any)       { fmt.Fprintf(t.out, f, a...) }
func (t *Terminal) Clear()                          { t.Print("\x1b[2J\x1b[H") }
func (t *Terminal) Write(p []byte) (int, error)     { return t.out.Write(p) }
func (t *Terminal) ReadWriter() io.ReadWriter       { return struct {
	io.Reader
	io.Writer
}{t.in, t.out} }

func (t *Terminal) ReadKey() (int, error) {
	b, err := t.br.ReadByte()
	if err != nil {
		return KeyNone, err
	}
	switch b {
	case 3, 'q', 'Q':
		return KeyQuit, nil
	case '\r', '\n':
		return KeyEnter, nil
	case 0x1b:
		// consume CSI if any
		nb, err := t.br.ReadByte()
		if err != nil {
			return KeyQuit, nil
		}
		if nb == '[' || nb == 'O' {
			_, _ = t.br.ReadByte()
		}
		return KeyOther, nil
	}
	return int(b), nil // return raw as KeyOther-ish via positive for digits/letters
}

// ReadChoice returns a trimmed uppercased letter/digit choice or "" on quit.
func (t *Terminal) ReadChoice() (string, error) {
	for {
		k, err := t.ReadKey()
		if err != nil {
			return "", err
		}
		if k == KeyQuit {
			return "Q", nil
		}
		if k == KeyEnter {
			continue
		}
		if k >= 32 && k < 127 {
			return strings.ToUpper(string(rune(k))), nil
		}
	}
}

func (t *Terminal) PromptLine(prompt string) string {
	t.Print(prompt)
	if t.raw && t.oldState != nil {
		if f, ok := t.in.(*os.File); ok {
			_ = term.Restore(int(f.Fd()), t.oldState)
			defer func() {
				if st, err := term.MakeRaw(int(f.Fd())); err == nil {
					t.oldState = st
				}
			}()
		}
	}
	line, _ := bufio.NewReader(t.in).ReadString('\n')
	t.br = bufio.NewReader(t.in)
	return strings.TrimSpace(line)
}

func color(code int, s string) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", code, s)
}

const (
	cBrightCyan   = 96
	cBrightWhite  = 97
	cBrightYellow = 93
	cBrightGreen  = 92
	cBrightRed    = 91
	cBrightBlack  = 90
	cCyan         = 36
)
