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
//   v0.8.0  2026-06-26  Phase 2 (VirtTerm): initial implementation — a minimal
//                        TLS terminal-transport listener (self-signed cert
//                        generated on first run, same pattern as the SSH host
//                        key in internal/sshsrv). Each accepted connection is
//                        handed unmodified to the Handler — in practice
//                        session.Run(rw, remoteAddr, deps, echoInput=true),
//                        exactly like Telnet — since session.go has no
//                        net.Conn-specific assumptions.
// ============================================================================

// Package virtterm implements the server side of VirtTerm's custom
// transport: plain TLS (no IAC negotiation, no SSH channel framing — just a
// raw byte stream once the handshake completes). It exists purely to give
// VirtTerm (a .NET WinForms client) a non-Telnet, non-SSH way to reach the
// same session.Run() text-mode BBS experience over the open internet.
package virtterm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"time"
)

// Server listens for VirtTerm TLS connections.
type Server struct {
	Addr     string
	CertFile string // path to PEM cert; generated alongside KeyFile if missing
	KeyFile  string // path to PEM EC private key; generated if missing
	Handler  func(io.ReadWriteCloser, string) // conn, remote address
}

// ListenAndServe loads (or generates) the TLS certificate and accepts
// connections until the listener errors or is closed.
func (s *Server) ListenAndServe() error {
	cert, err := loadOrGenerateCert(s.CertFile, s.KeyFile)
	if err != nil {
		return fmt.Errorf("virtterm cert: %w", err)
	}

	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	ln, err := tls.Listen("tcp", s.Addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("virtterm listen %s: %w", s.Addr, err)
	}
	defer ln.Close()

	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(conn net.Conn) {
			defer conn.Close()
			s.Handler(conn, conn.RemoteAddr().String())
		}(c)
	}
}

// loadOrGenerateCert loads a PEM cert/key pair from disk, generating a new
// self-signed EC certificate (valid 10 years) on first run if either file
// is missing — mirroring internal/sshsrv's host-key bootstrap pattern.
func loadOrGenerateCert(certFile, keyFile string) (tls.Certificate, error) {
	if certFile != "" && keyFile != "" {
		if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
			return cert, nil
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "VirtBBS VirtTerm"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
		BasicConstraintsValid: true,
	}

	derCert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	derKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derCert})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: derKey})

	if certFile != "" {
		_ = os.WriteFile(certFile, certPEM, 0600)
	}
	if keyFile != "" {
		_ = os.WriteFile(keyFile, keyPEM, 0600)
	}

	return tls.X509KeyPair(certPEM, keyPEM)
}
