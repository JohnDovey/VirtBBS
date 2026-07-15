package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// Key kinds.
const (
	KeyNone = iota
	KeyRune
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDn
	KeyEnter
	KeyBackspace
	KeyDelete
	KeyTab
	KeyShiftTab
	KeyEsc
	KeyF1
	KeyF2
	KeyF3
	KeyCtrlL
	KeyCtrlQ
	KeyCtrlS
	KeyCtrlC
)

// Event is one keyboard event.
type Event struct {
	Kind int
	Rune rune
}

// Terminal wraps stdin/stdout in raw mode.
type Terminal struct {
	in       io.Reader
	out      io.Writer
	br       *bufio.Reader
	oldState *term.State
	raw      bool
	fd       int
}

func NewTerminal(in io.Reader, out io.Writer) *Terminal {
	t := &Terminal{in: in, out: out, br: bufio.NewReader(in), fd: -1}
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		t.fd = int(f.Fd())
		if st, err := term.MakeRaw(t.fd); err == nil {
			t.oldState = st
			t.raw = true
		}
	}
	return t
}

func (t *Terminal) Close() {
	t.Print("\x1b[0m\x1b[?25h")
	if t.raw && t.oldState != nil && t.fd >= 0 {
		_ = term.Restore(t.fd, t.oldState)
	}
}

func (t *Terminal) Write(p []byte) (int, error) { return t.out.Write(p) }
func (t *Terminal) Printf(format string, args ...any) {
	fmt.Fprintf(t.out, format, args...)
}
func (t *Terminal) Print(s string) { fmt.Fprint(t.out, s) }

func (t *Terminal) Clear() {
	t.Print("\x1b[2J\x1b[H\x1b[0m")
}

func (t *Terminal) MoveTo(row, col int) {
	t.Printf("\x1b[%d;%dH", row, col)
}

func (t *Terminal) HideCursor() { t.Print("\x1b[?25l") }
func (t *Terminal) ShowCursor() { t.Print("\x1b[?25h") }

func (t *Terminal) Size() (cols, rows int) {
	if t.fd >= 0 {
		if w, h, err := term.GetSize(t.fd); err == nil && w > 0 && h > 0 {
			return w, h
		}
	}
	return 80, 24
}

// ReadEvent reads one key event.
func (t *Terminal) ReadEvent() (Event, error) {
	b, err := t.br.ReadByte()
	if err != nil {
		return Event{}, err
	}
	switch b {
	case 3:
		return Event{Kind: KeyCtrlC}, nil
	case 12:
		return Event{Kind: KeyCtrlL}, nil
	case 17:
		return Event{Kind: KeyCtrlQ}, nil
	case 19:
		return Event{Kind: KeyCtrlS}, nil
	case 127, 8:
		return Event{Kind: KeyBackspace}, nil
	case '\r', '\n':
		return Event{Kind: KeyEnter}, nil
	case '\t':
		return Event{Kind: KeyTab}, nil
	case 0x1b:
		return t.readEscape()
	}
	if b < 0x20 {
		return Event{Kind: KeyNone}, nil
	}
	if b < 0x80 {
		return Event{Kind: KeyRune, Rune: rune(b)}, nil
	}
	// UTF-8 multi-byte
	buf := []byte{b}
	for i := 0; i < 3; i++ {
		if utf8.FullRune(buf) {
			break
		}
		nb, err := t.br.ReadByte()
		if err != nil {
			break
		}
		buf = append(buf, nb)
	}
	r, _ := utf8.DecodeRune(buf)
	if r == utf8.RuneError {
		return Event{Kind: KeyNone}, nil
	}
	return Event{Kind: KeyRune, Rune: r}, nil
}

func (t *Terminal) readEscape() (Event, error) {
	if t.br.Buffered() == 0 {
		deadline := time.Now().Add(50 * time.Millisecond)
		for time.Now().Before(deadline) && t.br.Buffered() == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		if t.br.Buffered() == 0 {
			return Event{Kind: KeyEsc}, nil
		}
	}
	b, err := t.br.ReadByte()
	if err != nil {
		return Event{Kind: KeyEsc}, nil
	}
	if b == 'O' {
		nb, err := t.br.ReadByte()
		if err != nil {
			return Event{Kind: KeyEsc}, nil
		}
		switch nb {
		case 'P':
			return Event{Kind: KeyF1}, nil
		case 'Q':
			return Event{Kind: KeyF2}, nil
		case 'R':
			return Event{Kind: KeyF3}, nil
		case 'H':
			return Event{Kind: KeyHome}, nil
		case 'F':
			return Event{Kind: KeyEnd}, nil
		}
		return Event{Kind: KeyNone}, nil
	}
	if b != '[' {
		return Event{Kind: KeyEsc}, nil
	}
	var params []byte
	for {
		nb, err := t.br.ReadByte()
		if err != nil {
			return Event{Kind: KeyNone}, nil
		}
		if (nb >= 'A' && nb <= 'Z') || (nb >= 'a' && nb <= 'z') || nb == '~' {
			return parseCSI(params, nb), nil
		}
		params = append(params, nb)
		if len(params) > 16 {
			return Event{Kind: KeyNone}, nil
		}
	}
}

func parseCSI(params []byte, final byte) Event {
	s := string(params)
	switch final {
	case 'A':
		return Event{Kind: KeyUp}
	case 'B':
		return Event{Kind: KeyDown}
	case 'C':
		return Event{Kind: KeyRight}
	case 'D':
		return Event{Kind: KeyLeft}
	case 'H':
		return Event{Kind: KeyHome}
	case 'F':
		return Event{Kind: KeyEnd}
	case 'Z':
		return Event{Kind: KeyShiftTab}
	case '~':
		switch s {
		case "1", "7":
			return Event{Kind: KeyHome}
		case "4", "8":
			return Event{Kind: KeyEnd}
		case "3":
			return Event{Kind: KeyDelete}
		case "5":
			return Event{Kind: KeyPgUp}
		case "6":
			return Event{Kind: KeyPgDn}
		case "11":
			return Event{Kind: KeyF1}
		case "12":
			return Event{Kind: KeyF2}
		case "13":
			return Event{Kind: KeyF3}
		}
	}
	return Event{Kind: KeyNone}
}

// PromptLine temporarily leaves raw mode to read a line.
func (t *Terminal) PromptLine(prompt string) string {
	t.ShowCursor()
	t.Print(prompt)
	if t.raw && t.oldState != nil && t.fd >= 0 {
		_ = term.Restore(t.fd, t.oldState)
		defer func() {
			if st, err := term.MakeRaw(t.fd); err == nil {
				t.oldState = st
				t.raw = true
			}
			t.br = bufio.NewReader(t.in)
		}()
	}
	line, _ := bufio.NewReader(t.in).ReadString('\n')
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	return line
}

// PromptLineEdit reads a line in raw mode, starting with initial text already in
// the field (editable). Returns ok=false if the user presses Esc.
func (t *Terminal) PromptLineEdit(row, col int, prompt, initial string) (string, bool) {
	buf := []rune(initial)
	pos := len(buf)
	t.ShowCursor()
	redraw := func() {
		t.MoveTo(row, col)
		t.Print("\x1b[K")
		t.Print(prompt)
		t.Print(string(buf))
		// Place cursor at pos within the field
		t.MoveTo(row, col+len([]rune(prompt))+pos)
	}
	redraw()
	for {
		ev, err := t.ReadEvent()
		if err != nil {
			t.HideCursor()
			return string(buf), false
		}
		switch ev.Kind {
		case KeyEsc:
			t.HideCursor()
			return string(buf), false
		case KeyEnter:
			t.HideCursor()
			return string(buf), true
		case KeyBackspace:
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
				redraw()
			}
		case KeyDelete:
			if pos < len(buf) {
				buf = append(buf[:pos], buf[pos+1:]...)
				redraw()
			}
		case KeyLeft:
			if pos > 0 {
				pos--
				redraw()
			}
		case KeyRight:
			if pos < len(buf) {
				pos++
				redraw()
			}
		case KeyHome:
			pos = 0
			redraw()
		case KeyEnd:
			pos = len(buf)
			redraw()
		case KeyRune:
			if ev.Rune >= 32 {
				buf = append(buf[:pos], append([]rune{ev.Rune}, buf[pos:]...)...)
				pos++
				redraw()
			}
		}
	}
}
