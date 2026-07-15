package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Key codes returned by ReadKey.
const (
	KeyNone = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyQuit
	KeyEnter
	Key1
	Key2
	Key3
	Key4
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

// NewTerminal creates a terminal. Prefer raw mode when stdin is a TTY.
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

// Close restores the terminal state.
func (t *Terminal) Close() {
	if t.raw && t.oldState != nil {
		if f, ok := t.in.(*os.File); ok {
			_ = term.Restore(int(f.Fd()), t.oldState)
		}
	}
}

func (t *Terminal) Write(p []byte) (int, error) { return t.out.Write(p) }

func (t *Terminal) Printf(format string, args ...any) {
	fmt.Fprintf(t.out, format, args...)
}

func (t *Terminal) Print(s string) {
	fmt.Fprint(t.out, s)
}

func (t *Terminal) Clear() {
	t.Print("\x1b[2J\x1b[H")
}

func (t *Terminal) MoveTo(row, col int) {
	t.Printf("\x1b[%d;%dH", row, col)
}

// ReadKey reads one logical key (arrow / quit / enter).
func (t *Terminal) ReadKey() (int, error) {
	b, err := t.br.ReadByte()
	if err != nil {
		return KeyNone, err
	}
	switch b {
	case 3: // Ctrl-C
		return KeyQuit, nil
	case 'q', 'Q':
		return KeyQuit, nil
	case '\r', '\n':
		return KeyEnter, nil
	case '1':
		return Key1, nil
	case '2':
		return Key2, nil
	case '3':
		return Key3, nil
	case '4':
		return Key4, nil
	case 0x1b:
		return t.readEscape()
	case 0xe0, 0x00: // some DOS-style prefixes
		nb, err := t.br.ReadByte()
		if err != nil {
			return KeyOther, nil
		}
		switch nb {
		case 'H':
			return KeyUp, nil
		case 'P':
			return KeyDown, nil
		case 'K':
			return KeyLeft, nil
		case 'M':
			return KeyRight, nil
		}
		return KeyOther, nil
	}
	return KeyOther, nil
}

// DigitFromKey returns 1–4 for number keys, or 0 if not a digit choice.
func DigitFromKey(key int) int {
	switch key {
	case Key1:
		return 1
	case Key2:
		return 2
	case Key3:
		return 3
	case Key4:
		return 4
	}
	return 0
}

func (t *Terminal) readEscape() (int, error) {
	b, err := t.br.ReadByte()
	if err != nil {
		return KeyQuit, nil // lone ESC = quit
	}
	if b != '[' && b != 'O' {
		return KeyOther, nil
	}
	b, err = t.br.ReadByte()
	if err != nil {
		return KeyOther, nil
	}
	switch b {
	case 'A':
		return KeyUp, nil
	case 'B':
		return KeyDown, nil
	case 'C':
		return KeyRight, nil
	case 'D':
		return KeyLeft, nil
	}
	return KeyOther, nil
}

// PromptLine reads a cooked line (for -local name). Temporarily leaves raw mode.
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

// ANSI color helpers (local; no VirtBBS dependency).
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
	cYellow       = 33
	cCyan         = 36
	cGreen        = 32
	cMagenta      = 35
)
