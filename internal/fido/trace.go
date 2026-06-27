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
//   v0.4.0  2026-06-25  Initial implementation — TRACE netmail test utility,
//                        mirroring ping.go
// ============================================================================

package fido

// Package fido — trace.go
//
// TRACE mirrors the PING convention (ping.go) exactly — a netmail message
// with Subject "TRACE" sent to a node triggers an automatic reply — but
// the reply reports this system's own routing details (address, uplink,
// BBS software/version) rather than just confirming receipt. Like PING,
// this is a single-hop test: VirtBBS is a leaf node and cannot orchestrate
// true multi-system traceroute-style forwarding, which would require every
// intermediate system to cooperatively relay the TRACE onward.

import (
	"fmt"
	"strings"
	"time"
)

// TraceSubject is the conventional netmail subject used to request routing
// diagnostics from a node. Matched case-insensitively on receipt.
const TraceSubject = "TRACE"

// TraceReplySubject is the subject used for the automatic reply to a TRACE.
const TraceReplySubject = "TRACE REPLY"

// IsTrace reports whether subject is a TRACE test message.
func IsTrace(subject string) bool {
	return strings.EqualFold(strings.TrimSpace(subject), TraceSubject)
}

// IsTraceReply reports whether subject is a TRACE reply. Checked so the
// auto-responder never replies to its own kind of reply, which would
// otherwise create an infinite loop between two auto-responding systems —
// the same loop-guard PING uses for PONG.
func IsTraceReply(subject string) bool {
	return strings.EqualFold(strings.TrimSpace(subject), TraceReplySubject)
}

// BuildTraceReply constructs the automatic reply to an inbound TRACE
// netmail pm, addressed back to its sender. our is this node's own address
// on the network the TRACE arrived on; botName is the sender identity used
// for the automatic reply (e.g. "VirtBBS TraceBot").
func BuildTraceReply(nd *NetworkDef, our Addr, botName string, pm *Message) *NetmailMsg {
	uplink := nd.UplinkAddr()
	var body strings.Builder
	fmt.Fprintf(&body, "Automatic TRACE reply from %s.\r\n", our.String())
	fmt.Fprintf(&body, "Your TRACE was received at %s.\r\n", time.Now().Format("02 Jan 06  15:04:05"))
	fmt.Fprintf(&body, "Original TRACE timestamp: %s\r\n", pm.DateTime)
	fmt.Fprintf(&body, "\r\nRoute information for %s:\r\n", our.String())
	fmt.Fprintf(&body, "  This node:  %s\r\n", our.String())
	if uplink != (Addr{}) {
		fmt.Fprintf(&body, "  Uplink:     %s\r\n", uplink.String())
	} else {
		body.WriteString("  Uplink:     (none configured)\r\n")
	}
	fmt.Fprintf(&body, "  Software:   VirtBBS\r\n")

	return &NetmailMsg{
		FromName: botName,
		FromAddr: our.String(),
		ToName:   pm.FromName,
		ToAddr:   pm.OrigAddr.String(),
		Subject:  TraceReplySubject,
		Body:     body.String(),
	}
}

// AutoRespondTrace builds and writes a TRACE reply PKT for an inbound TRACE
// netmail pm, routed via the network's configured uplink. Called by the
// toss pipeline whenever an inbound netmail's Subject is "TRACE".
func AutoRespondTrace(nd *NetworkDef, pm *Message) error {
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return fmt.Errorf("cannot auto-reply to TRACE: invalid local address %q", nd.Address)
	}
	uplink := nd.UplinkAddr()
	if uplink == (Addr{}) {
		return fmt.Errorf("cannot auto-reply to TRACE: no uplink configured")
	}

	reply := BuildTraceReply(nd, our, "VirtBBS TraceBot", pm)
	outDir := OutboundDir(nd.OutboundDir, uplink, uplink, false)
	_, err := WritePKT(our, uplink, nd.Password, outDir, []*NetmailMsg{reply})
	return err
}

// SendTrace composes and writes a TRACE netmail PKT to toAddr on network
// nd, for sysop-initiated routing diagnostics. fromName is the local
// sender identity (typically the sysop's name); toName is looked up by the
// caller (e.g. from the nodelist) or defaults to "Sysop" when unknown.
func SendTrace(nd *NetworkDef, fromName, toName, toAddr string) (pktPath string, err error) {
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return "", fmt.Errorf("invalid local address %q", nd.Address)
	}
	dest, err := ParseAddr(toAddr)
	if err != nil {
		return "", fmt.Errorf("invalid destination address %q: %w", toAddr, err)
	}

	msg := &NetmailMsg{
		FromName: fromName,
		FromAddr: our.String(),
		ToName:   toName,
		ToAddr:   dest.String(),
		Subject:  TraceSubject,
		Body:     fmt.Sprintf("TRACE from %s at %s.\r\n", our.String(), time.Now().Format("02 Jan 06  15:04:05")),
	}

	uplink := nd.UplinkAddr()
	if uplink == (Addr{}) {
		return "", fmt.Errorf("no uplink configured")
	}
	outDir := OutboundDir(nd.OutboundDir, uplink, uplink, false)
	return WritePKT(our, uplink, nd.Password, outDir, []*NetmailMsg{msg})
}
