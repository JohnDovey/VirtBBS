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
//   v0.0.2  2026-06-24  Phase 10: Handler uses io.ReadWriteCloser for node kick support
// ============================================================================

// Package telnet implements an RFC 854 Telnet server.
package telnet

import (
	"fmt"
	"io"
	"net"
)

// IAC commands
const (
	IAC  = 0xFF
	DONT = 0xFE
	DO   = 0xFD
	WONT = 0xFC
	WILL = 0xFB
	SB   = 0xFA
	SE   = 0xF0

	// Options
	OptEcho     = 0x01
	OptSGA      = 0x03 // suppress go-ahead
	OptLineMode = 0x22 // linemode (RFC 1116)
	OptNAWS     = 0x1F // window size
	OptTermType = 0x18

	// Linemode sub-option
	LinemodeMode = 0x01
)

// Conn wraps a net.Conn, stripping Telnet IAC sequences from reads and
// presenting a clean io.ReadWriter to the session layer.
type Conn struct {
	raw      net.Conn
	termType string
	width    int
	height   int
	buf      []byte // unprocessed raw bytes
}

func NewConn(c net.Conn) *Conn {
	return &Conn{raw: c, width: 80, height: 24}
}

func (c *Conn) RemoteAddr() net.Addr { return c.raw.RemoteAddr() }
func (c *Conn) TermType() string     { return c.termType }
func (c *Conn) Width() int           { return c.width }
func (c *Conn) Height() int          { return c.height }

// Negotiate sends initial Telnet option negotiations to put the client into
// character-at-a-time mode (no local line buffering).
//
// The key sequence:
//   - WILL ECHO  — server handles echoing, suppresses client local echo
//   - WILL SGA   — server side: suppress go-ahead (required for char mode)
//   - DO SGA     — ask client to also suppress go-ahead (both sides = char mode)
//   - DO LINEMODE with MODE 0 subnegotiation — explicitly disable line mode
//   - DO TTYPE   — request terminal type
//   - DO NAWS    — request window size
func (c *Conn) Negotiate() error {
	seq := []byte{
		// Character mode negotiation
		IAC, WILL, OptEcho,
		IAC, WILL, OptSGA,
		IAC, DO, OptSGA,
		// Disable linemode explicitly: DO LINEMODE, then SB LINEMODE MODE 0 SE
		IAC, DO, OptLineMode,
		IAC, SB, OptLineMode, LinemodeMode, 0x00, IAC, SE,
		// Terminal info
		IAC, DO, OptTermType,
		IAC, DO, OptNAWS,
	}
	_, err := c.raw.Write(seq)
	return err
}

// Write sends raw bytes to the client.
func (c *Conn) Write(p []byte) (int, error) {
	// Escape any 0xFF bytes in the data stream
	escaped := make([]byte, 0, len(p)+4)
	for _, b := range p {
		escaped = append(escaped, b)
		if b == IAC {
			escaped = append(escaped, IAC)
		}
	}
	return c.raw.Write(escaped)
}

// Read reads application data, transparently processing IAC sequences.
// IAC sequences (and SB...SE subnegotiations) may be split across separate
// TCP segments; any incomplete trailing sequence is held in c.buf and
// prepended to the next underlying read so its bytes are never leaked into
// the application data stream.
func (c *Conn) Read(p []byte) (int, error) {
	raw := make([]byte, len(p)*2)
	n, err := c.raw.Read(raw)
	if n == 0 {
		return 0, err
	}

	// Prepend any leftover bytes from an incomplete sequence last time.
	if len(c.buf) > 0 {
		raw = append(c.buf, raw[:n]...)
		c.buf = nil
		n = len(raw)
	}

	out := p[:0]
	i := 0
	for i < n {
		b := raw[i]
		if b != IAC {
			out = append(out, b)
			i++
			continue
		}
		// Need at least 2 more bytes for a command — if not available yet,
		// stash the incomplete tail and wait for more data.
		if i+1 >= n {
			c.buf = append(c.buf, raw[i:n]...)
			break
		}
		cmd := raw[i+1]
		switch cmd {
		case WILL, WONT, DO, DONT:
			if i+2 < n {
				c.handleOption(cmd, raw[i+2])
				i += 3
			} else {
				c.buf = append(c.buf, raw[i:n]...)
				i = n
			}
		case SB:
			// Find SE
			end := -1
			for j := i + 2; j < n-1; j++ {
				if raw[j] == IAC && raw[j+1] == SE {
					end = j
					break
				}
			}
			if end > 0 {
				c.handleSubneg(raw[i+2 : end])
				i = end + 2
			} else {
				// Subnegotiation not yet complete — hold it for next read.
				c.buf = append(c.buf, raw[i:n]...)
				i = n
			}
		case IAC:
			out = append(out, IAC)
			i += 2
		default:
			i += 2
		}
	}
	return len(out), err
}

func (c *Conn) handleOption(cmd, opt byte) {
	switch {
	case cmd == WILL && opt == OptTermType:
		// Ask client to send its terminal type
		_, _ = c.raw.Write([]byte{IAC, SB, OptTermType, 1, IAC, SE})
	case cmd == DO && opt == OptEcho:
		// Client agrees server echoes — fine
	case cmd == WILL && opt == OptSGA:
		// Client will suppress go-ahead — good, char mode confirmed
	case cmd == WONT && opt == OptLineMode:
		// Client disabled line mode — exactly what we want
	case cmd == WILL && opt == OptLineMode:
		// Client wants to keep linemode on; refuse and re-send MODE 0
		_, _ = c.raw.Write([]byte{
			IAC, SB, OptLineMode, LinemodeMode, 0x00, IAC, SE,
		})
	}
}

func (c *Conn) handleSubneg(data []byte) {
	if len(data) < 2 {
		return
	}
	switch data[0] {
	case OptTermType:
		if data[1] == 0 && len(data) > 2 {
			c.termType = string(data[2:])
		}
	case OptNAWS:
		if len(data) >= 5 {
			c.width = int(data[1])<<8 | int(data[2])
			c.height = int(data[3])<<8 | int(data[4])
		}
	}
}

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.raw.Close() }

// Server listens for Telnet connections on addr and calls handler for each.
type Server struct {
	Addr    string
	Handler func(io.ReadWriteCloser, string) // conn, remoteAddr
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("telnet listen %s: %w", s.Addr, err)
	}
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(c net.Conn) {
			tc := NewConn(c)
			_ = tc.Negotiate()
			s.Handler(tc, c.RemoteAddr().String())
		}(c)
	}
}
