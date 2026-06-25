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
//   v0.0.3  2026-06-24  Phase 9: FidoNet address type
// ============================================================================

// Package fido implements FidoNet packet handling for VirtBBS.
// Supports FTS-0001 .PKT files (NetMail + EchoMail), tossing incoming
// packets into the message store, and scanning outbound messages into packets.
package fido

import (
	"fmt"
	"strconv"
	"strings"
)

// Addr is a FidoNet 4D address: Zone:Net/Node.Point
// Point is 0 for non-point systems.
type Addr struct {
	Zone  int
	Net   int
	Node  int
	Point int
}

// String returns the canonical 4D address representation.
// If Point is 0 it returns 3D form (Zone:Net/Node).
func (a Addr) String() string {
	if a.Point != 0 {
		return fmt.Sprintf("%d:%d/%d.%d", a.Zone, a.Net, a.Node, a.Point)
	}
	return fmt.Sprintf("%d:%d/%d", a.Zone, a.Net, a.Node)
}

// ParseAddr parses a FidoNet address string in any of:
//
//	Zone:Net/Node
//	Zone:Net/Node.Point
func ParseAddr(s string) (Addr, error) {
	s = strings.TrimSpace(s)
	var a Addr

	// Split on '.'  for point
	if dot := strings.Index(s, "."); dot >= 0 {
		p, err := strconv.Atoi(s[dot+1:])
		if err != nil {
			return a, fmt.Errorf("invalid point in %q", s)
		}
		a.Point = p
		s = s[:dot]
	}

	// Split Zone:Rest
	colon := strings.Index(s, ":")
	if colon < 0 {
		return a, fmt.Errorf("missing zone in %q", s)
	}
	zone, err := strconv.Atoi(s[:colon])
	if err != nil {
		return a, fmt.Errorf("invalid zone in %q", s)
	}
	a.Zone = zone
	s = s[colon+1:]

	// Split Net/Node
	slash := strings.Index(s, "/")
	if slash < 0 {
		return a, fmt.Errorf("missing net/node separator in %q", s)
	}
	net, err := strconv.Atoi(s[:slash])
	if err != nil {
		return a, fmt.Errorf("invalid net in %q", s)
	}
	node, err := strconv.Atoi(s[slash+1:])
	if err != nil {
		return a, fmt.Errorf("invalid node in %q", s)
	}
	a.Net = net
	a.Node = node
	return a, nil
}

// Equal reports whether two addresses are the same (ignoring point = 0 vs absent).
func (a Addr) Equal(b Addr) bool {
	return a.Zone == b.Zone && a.Net == b.Net && a.Node == b.Node && a.Point == b.Point
}
