package web

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// acceptWebSocket upgrades an HTTP connection to a minimal RFC 6455 WebSocket.
func acceptWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		!strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return nil, fmt.Errorf("not a websocket request")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("hijack not supported")
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	accept := computeAcceptKey(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := bufrw.WriteString(response); err != nil {
		conn.Close()
		return nil, err
	}
	if err := bufrw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	return &wsConn{conn: conn, br: bufio.NewReader(conn)}, nil
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// wsConn adapts a WebSocket to io.ReadWriteCloser for door.Run.
type wsConn struct {
	conn net.Conn
	br   *bufio.Reader
	mu   sync.Mutex

	readBuf []byte
	closed  bool
}

func (c *wsConn) Read(p []byte) (int, error) {
	for len(c.readBuf) == 0 {
		payload, err := c.readFrame()
		if err != nil {
			return 0, err
		}
		c.readBuf = payload
	}
	n := copy(p, c.readBuf)
	c.readBuf = c.readBuf[n:]
	return n, nil
}

func (c *wsConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	if err := c.writeFrame(0x1, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *wsConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	_ = c.writeFrame(0x8, nil)
	return c.conn.Close()
}

func (c *wsConn) readFrame() ([]byte, error) {
	b1, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	opcode := b1 & 0x0f
	b2, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	masked := b2&0x80 != 0
	length := uint64(b2 & 0x7f)
	switch length {
	case 126:
		var ext uint16
		if err := binary.Read(c.br, binary.BigEndian, &ext); err != nil {
			return nil, err
		}
		length = uint64(ext)
	case 127:
		if err := binary.Read(c.br, binary.BigEndian, &length); err != nil {
			return nil, err
		}
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	switch opcode {
	case 0x1, 0x2:
		return payload, nil
	case 0x8:
		return nil, io.EOF
	case 0x9:
		c.mu.Lock()
		_ = c.writeFrame(0xA, payload)
		c.mu.Unlock()
		return c.readFrame()
	default:
		return c.readFrame()
	}
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	n := len(payload)
	switch {
	case n < 126:
		header = append(header, byte(n))
	case n <= 0xffff:
		header = append(header, 126, byte(n>>8), byte(n))
	default:
		header = append(header, 127)
		var lenBuf [8]byte
		binary.BigEndian.PutUint64(lenBuf[:], uint64(n))
		header = append(header, lenBuf[:]...)
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if n > 0 {
		_, err := c.conn.Write(payload)
		return err
	}
	return nil
}
