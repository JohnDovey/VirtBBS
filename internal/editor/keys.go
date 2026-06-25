// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
// (see editor.go for full text)
//
// Change History:
//   v0.0.7  2026-06-24  Key reader with escape-sequence timeout
// ============================================================================

package editor

import (
	"io"
	"time"
)

// key identifies a logical key press independent of encoding.
type key int

const (
	keyNone key = iota
	keyChar     // printable character (see KeyPress.R)
	keyEnter
	keyBackspace
	keyDelete  // Del key (forward delete)
	keyUp
	keyDown
	keyLeft
	keyRight
	keyHome
	keyEnd
	keyPageUp
	keyPageDown
	keyIns       // Insert key — toggle insert/overwrite
	keyCtrlA     // abort
	keyCtrlB     // insert blank line
	keyCtrlD     // delete char at cursor (like GNU nano / Emacs)
	keyCtrlE     // jump to end of line (Emacs)
	keyCtrlG     // delete word right
	keyCtrlK     // cut line to cut-buffer
	keyCtrlL     // refresh / redraw screen
	keyCtrlQ     // quit (same as abort, some keyboards)
	keyCtrlS     // save
	keyCtrlU     // paste (uncut)
	keyCtrlW     // delete word backward
	keyCtrlX     // cut line (alias for Ctrl+K)
	keyCtrlY     // delete entire line
	keyF1        // help
	keyF2        // toggle insert/overwrite
	keyEsc       // bare escape (= abort in some contexts)
	keyUnknown
)

// KeyPress represents one logical key press from the user.
type KeyPress struct {
	K key
	R rune // valid when K == keyChar
}

// keyReader wraps an io.Reader and delivers KeyPress values with
// escape-sequence assembly and optional timeout for bare ESC detection.
type keyReader struct {
	src  io.Reader
	buf  chan byte
	done chan struct{}
}

func newKeyReader(r io.Reader) *keyReader {
	kr := &keyReader{
		src:  r,
		buf:  make(chan byte, 128),
		done: make(chan struct{}),
	}
	go kr.feed()
	return kr
}

// stop signals the feed goroutine to exit.
func (kr *keyReader) stop() {
	select {
	case <-kr.done:
	default:
		close(kr.done)
	}
}

// feed reads bytes from the underlying reader and pushes them to buf.
func (kr *keyReader) feed() {
	b := make([]byte, 1)
	for {
		_, err := kr.src.Read(b)
		if err != nil {
			return
		}
		select {
		case kr.buf <- b[0]:
		case <-kr.done:
			return
		}
	}
}

// readByte returns the next byte, waiting up to timeout.
// ok is false on timeout or channel close.
func (kr *keyReader) readByte(timeout time.Duration) (byte, bool) {
	select {
	case b, ok := <-kr.buf:
		return b, ok
	case <-time.After(timeout):
		return 0, false
	case <-kr.done:
		return 0, false
	}
}

// next reads and returns the next logical key press.
// It blocks indefinitely for the first byte, then uses a 50 ms timeout
// to assemble escape sequences.
func (kr *keyReader) next() KeyPress {
	b, ok := <-kr.buf
	if !ok {
		return KeyPress{K: keyEsc}
	}

	switch {
	case b == 0x1B: // ESC — start of escape sequence?
		b2, ok := kr.readByte(50 * time.Millisecond)
		if !ok {
			// Bare ESC
			return KeyPress{K: keyEsc}
		}
		switch b2 {
		case '[': // CSI sequence
			return kr.readCSI()
		case 'O': // SS3 sequence (some terminals: F1-F4, Home, End)
			b3, ok := kr.readByte(50 * time.Millisecond)
			if !ok {
				return KeyPress{K: keyEsc}
			}
			switch b3 {
			case 'H':
				return KeyPress{K: keyHome}
			case 'F':
				return KeyPress{K: keyEnd}
			case 'P':
				return KeyPress{K: keyF1}
			case 'Q':
				return KeyPress{K: keyF2}
			}
			return KeyPress{K: keyUnknown}
		}
		return KeyPress{K: keyEsc}

	case b == 0x0D || b == 0x0A: // Enter / CR / LF
		return KeyPress{K: keyEnter}

	case b == 0x08 || b == 0x7F: // Backspace
		return KeyPress{K: keyBackspace}

	case b == 0x01: // Ctrl+A
		return KeyPress{K: keyCtrlA}

	case b == 0x02: // Ctrl+B
		return KeyPress{K: keyCtrlB}

	case b == 0x04: // Ctrl+D
		return KeyPress{K: keyCtrlD}

	case b == 0x05: // Ctrl+E
		return KeyPress{K: keyCtrlE}

	case b == 0x07: // Ctrl+G
		return KeyPress{K: keyCtrlG}

	case b == 0x0B: // Ctrl+K
		return KeyPress{K: keyCtrlK}

	case b == 0x0C: // Ctrl+L
		return KeyPress{K: keyCtrlL}

	case b == 0x11: // Ctrl+Q
		return KeyPress{K: keyCtrlQ}

	case b == 0x13: // Ctrl+S
		return KeyPress{K: keyCtrlS}

	case b == 0x15: // Ctrl+U
		return KeyPress{K: keyCtrlU}

	case b == 0x17: // Ctrl+W
		return KeyPress{K: keyCtrlW}

	case b == 0x18: // Ctrl+X
		return KeyPress{K: keyCtrlX}

	case b == 0x19: // Ctrl+Y
		return KeyPress{K: keyCtrlY}

	case b >= 0x20: // printable ASCII (and extended via UTF-8 in future)
		return KeyPress{K: keyChar, R: rune(b)}
	}

	return KeyPress{K: keyUnknown}
}

// readCSI assembles a CSI (ESC [) escape sequence.
func (kr *keyReader) readCSI() KeyPress {
	// Collect parameter digits and intermediate bytes.
	var params []byte
	for {
		b, ok := kr.readByte(50 * time.Millisecond)
		if !ok {
			return KeyPress{K: keyUnknown}
		}
		if b >= 0x40 && b <= 0x7E {
			// Final byte of CSI sequence.
			return kr.dispatchCSI(params, b)
		}
		params = append(params, b)
	}
}

// dispatchCSI maps a parsed CSI sequence to a KeyPress.
func (kr *keyReader) dispatchCSI(params []byte, final byte) KeyPress {
	p := string(params)
	switch final {
	case 'A':
		return KeyPress{K: keyUp}
	case 'B':
		return KeyPress{K: keyDown}
	case 'C':
		return KeyPress{K: keyRight}
	case 'D':
		return KeyPress{K: keyLeft}
	case 'H':
		return KeyPress{K: keyHome}
	case 'F':
		return KeyPress{K: keyEnd}
	case '~':
		switch p {
		case "1", "7":
			return KeyPress{K: keyHome}
		case "2":
			return KeyPress{K: keyIns}
		case "3":
			return KeyPress{K: keyDelete}
		case "4", "8":
			return KeyPress{K: keyEnd}
		case "5":
			return KeyPress{K: keyPageUp}
		case "6":
			return KeyPress{K: keyPageDown}
		case "11":
			return KeyPress{K: keyF1}
		case "12":
			return KeyPress{K: keyF2}
		}
	}
	return KeyPress{K: keyUnknown}
}
