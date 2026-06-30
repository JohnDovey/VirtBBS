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
//   v0.1.1  2026-06-25  Initial implementation — PING/PONG netmail test utility
//   v0.3.0  2026-06-25  AutoRespondPing takes a *NetworkDef instead of *Config,
//                        so it works for any configured network, not just primary
// ============================================================================

package fido

// Package fido — ping.go
//
// Implements the long-standing FidoNet netmail "ping" testing convention:
// a netmail message with Subject "PING" sent to a node triggers an
// automatic "PONG" reply confirming mail flow and reporting basic timing.
// This is not an FTS standard, but a widely supported utility convention
// (e.g. the classic FrontDoor/D'Bridge "Ping" tools).

import (
	"fmt"
	"strings"
	"time"
)

// PingSubject is the conventional netmail subject used to test mail flow
// between two systems. Matched case-insensitively on receipt.
const PingSubject = "PING"

// PongSubject is the subject used for the automatic reply to a PING.
const PongSubject = "PONG"

// IsPing reports whether subject is a PING test message.
func IsPing(subject string) bool {
	return strings.EqualFold(strings.TrimSpace(subject), PingSubject)
}

// IsPong reports whether subject is a PONG reply. Checked so the auto-
// responder never replies to its own kind of reply, which would otherwise
// create an infinite ping/pong loop between two auto-responding systems.
func IsPong(subject string) bool {
	return strings.EqualFold(strings.TrimSpace(subject), PongSubject)
}

// IsPingMessage reports a PING test by subject or the FTSC nodelist robot
// convention (ToName "PING").
func IsPingMessage(pm *Message) bool {
	return IsPing(pm.Subject) || strings.EqualFold(strings.TrimSpace(pm.ToName), PingSubject)
}

// IsNetmailUtilityTest reports PING/PONG/TRACE diagnostic netmail. Such
// messages must not be processed as AreaFix/FileFix: htick routes TRACE
// through its areafix module (ToName AreaFix, Subject TRACE) and classic
// AreaFix treats the subject line as a password.
func IsNetmailUtilityTest(pm *Message) bool {
	if pm == nil {
		return false
	}
	return IsPingMessage(pm) ||
		IsPong(pm.Subject) ||
		IsTrace(pm.Subject) ||
		IsTraceReply(pm.Subject) ||
		strings.EqualFold(strings.TrimSpace(pm.ToName), TraceSubject)
}

// BuildPongReply constructs the automatic PONG reply to an inbound PING
// netmail pm, addressed back to its sender. our is this node's own address
// on the network the PING arrived on; botName is the sender identity used
// for the automatic reply (e.g. "VirtBBS PingBot").
func BuildPongReply(our Addr, botName string, pm *Message) *NetmailMsg {
	return &NetmailMsg{
		FromName:    botName,
		FromAddr:    our.String(),
		ToName:      pm.FromName,
		ToAddr:      pm.OrigAddr.String(),
		Subject:     PongSubject,
		NoSignature: true,
		Body: fmt.Sprintf(
			"Automatic PONG reply from %s.\r\n"+
				"Your PING was received at %s.\r\n"+
				"Original PING timestamp: %s\r\n",
			our.String(), time.Now().Format("02 Jan 06  15:04:05"), pm.DateTime),
	}
}

// AutoRespondPing builds and writes a PONG reply PKT for an inbound PING
// netmail pm, routed via the network's configured uplink. Called by the
// toss pipeline whenever an inbound netmail's Subject is "PING".
func AutoRespondPing(nd *NetworkDef, pm *Message) error {
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return fmt.Errorf("cannot auto-reply to PING: invalid local address %q", nd.Address)
	}
	uplink := nd.UplinkAddr()
	if uplink == (Addr{}) {
		return fmt.Errorf("cannot auto-reply to PING: no uplink configured")
	}

	reply := BuildPongReply(our, "VirtBBS PingBot", pm)
	outDir := OutboundDir(nd.OutboundDir, uplink, uplink, false)
	_, err := WritePKT(our, uplink, nd.Password, outDir, []*NetmailMsg{reply}, nd.Name)
	return err
}

// SendPing composes and writes a PING netmail PKT to toAddr on network nd,
// for sysop-initiated connectivity testing. fromName is the local sender
// identity (typically the sysop's name); toName is looked up by the caller
// (e.g. from the nodelist) or defaults to "Sysop" when unknown.
func SendPing(nd *NetworkDef, fromName, toName, toAddr string) (pktPath string, err error) {
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return "", fmt.Errorf("invalid local address %q", nd.Address)
	}
	dest, err := ParseAddr(toAddr)
	if err != nil {
		return "", fmt.Errorf("invalid destination address %q: %w", toAddr, err)
	}

	msg := &NetmailMsg{
		FromName:    fromName,
		FromAddr:    our.String(),
		ToName:      toName,
		ToAddr:      dest.String(),
		Subject:     PingSubject,
		NoSignature: true,
		Body:        fmt.Sprintf("PING from %s at %s.\r\n", our.String(), time.Now().Format("02 Jan 06  15:04:05")),
	}

	uplink := nd.UplinkAddr()
	if uplink == (Addr{}) {
		return "", fmt.Errorf("no uplink configured")
	}
	outDir := OutboundDir(nd.OutboundDir, uplink, uplink, false)
	return WritePKT(our, uplink, nd.Password, outDir, []*NetmailMsg{msg}, nd.Name)
}
