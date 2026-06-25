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
//   v0.0.2  2026-06-24  Phase 10: Handler uses io.ReadWriteCloser; pass ssh.Channel directly
// ============================================================================

// Package sshsrv implements an SSH server for VirtBBS.
package sshsrv

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

// Server wraps golang.org/x/crypto/ssh to provide a PTY SSH server.
type Server struct {
	Addr       string
	HostKeyFile string // path to PEM-encoded EC private key; generated if missing
	// ValidatePassword is called to authenticate users; return true to allow.
	ValidatePassword func(user, pass string) bool
	Handler          func(io.ReadWriteCloser, string) // conn, username
}

func (s *Server) ListenAndServe() error {
	hostKey, err := loadOrGenerateHostKey(s.HostKeyFile)
	if err != nil {
		return fmt.Errorf("ssh host key: %w", err)
	}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if s.ValidatePassword != nil && s.ValidatePassword(c.User(), string(pass)) {
				return nil, nil
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}
	cfg.AddHostKey(hostKey)

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("ssh listen %s: %w", s.Addr, err)
	}
	defer ln.Close()

	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(c, cfg)
	}
}

func (s *Server) handleConn(c net.Conn, cfg *ssh.ServerConfig) {
	defer c.Close()
	sconn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session channels supported")
			continue
		}
		ch, requests, err := newChan.Accept()
		if err != nil {
			return
		}
		go handleSession(ch, requests, sconn.User(), s.Handler)
	}
}

func handleSession(ch ssh.Channel, requests <-chan *ssh.Request, user string, handler func(io.ReadWriteCloser, string)) {
	defer ch.Close()
	for req := range requests {
		switch req.Type {
		case "pty-req":
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
		case "shell", "exec":
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
			// ssh.Channel implements io.ReadWriteCloser directly.
			handler(ch, user)
			return
		default:
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
		}
	}
}

func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return parseHostKey(data)
		}
	}
	// Generate ephemeral key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	if path != "" {
		der, _ := x509.MarshalECPrivateKey(key)
		blk := &pem.Block{Type: "EC PRIVATE KEY", Bytes: der}
		_ = os.WriteFile(path, pem.EncodeToMemory(blk), 0600)
	}
	return ssh.NewSignerFromKey(key)
}

func parseHostKey(data []byte) (ssh.Signer, error) {
	for {
		var blk *pem.Block
		blk, data = pem.Decode(data)
		if blk == nil {
			break
		}
		s, err := ssh.ParsePrivateKey(pem.EncodeToMemory(blk))
		if err == nil {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no valid private key found in host key file")
}
