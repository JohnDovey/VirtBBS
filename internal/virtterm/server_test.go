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
//   v0.8.0  2026-06-26  Phase 2 (VirtTerm): smoke test — confirms a TLS client
//                        can complete the handshake against a freshly
//                        generated self-signed cert and that bytes pass
//                        through Handler unmodified in both directions.
//                        (The Handler signature is identical to
//                        internal/telnet's, and main.go wires the same
//                        telnetHandler closure into both listeners, so this
//                        establishes the transport works; session.go itself
//                        has no net.Conn-specific assumptions, verified by
//                        code inspection during planning.)
// ============================================================================

package virtterm

import (
	"bufio"
	"crypto/tls"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestVirtTermTLSHandshakeAndEcho(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	echoHandler := func(rw io.ReadWriteCloser, remoteAddr string) {
		buf := make([]byte, 4096)
		n, err := rw.Read(buf)
		if err != nil {
			return
		}
		_, _ = rw.Write(buf[:n])
	}

	srv := &Server{Addr: "127.0.0.1:0", CertFile: certFile, KeyFile: keyFile, Handler: echoHandler}

	cert, err := loadOrGenerateCert(certFile, keyFile)
	if err != nil {
		t.Fatalf("loadOrGenerateCert: %v", err)
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp", srv.Addr, tlsCfg)
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				srv.Handler(conn, conn.RemoteAddr().String())
			}(c)
		}
	}()

	addr := ln.Addr().String()
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("tls.Dial (handshake failed): %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	const msg = "Command: hello virtterm\r\n"
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := bufio.NewReader(conn)
	echoed := make([]byte, len(msg))
	if _, err := io.ReadFull(r, echoed); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(echoed) != msg {
		t.Fatalf("echoed = %q, want %q", echoed, msg)
	}
}

func TestLoadOrGenerateCertPersists(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	cert1, err := loadOrGenerateCert(certFile, keyFile)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	cert2, err := loadOrGenerateCert(certFile, keyFile)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if string(cert1.Certificate[0]) != string(cert2.Certificate[0]) {
		t.Fatalf("second call regenerated the cert instead of loading the persisted one")
	}
}
