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
//   v0.0.6  2026-06-24  Initial implementation — BinkP TCP client (RFC draft-ietf-fido-binkp)
//   v0.3.0  2026-06-25  Add PollAndToss, combining a poll with an automatic toss of
//                        whatever was received, for the scheduler and sysop/API poll
//                        commands to share
//   v0.4.1  2026-06-30  Handle binkd interleaved M_FILE during M_GOT wait (fixes receive stall)
//   v0.4.0  2026-06-25  Add ServeBinkP — a BinkP server accepting inbound connections
//                        from configured uplinks and downlinks, so other systems can
//                        poll THIS BBS instead of only the reverse
// ============================================================================

// Package fido — binkp.go
//
// BinkP TCP client.  Implements enough of the BinkP/1.1 protocol to:
//   - Connect to an uplink and authenticate with M_ADR / M_PWD
//   - Send outbound PKT/ARQ bundles (M_FILE / M_DATA / M_GOT)
//   - Receive inbound bundles
//   - Handle M_ERR and M_BSY
//
// BinkP framing (2-byte big-endian header):
//   bit 15  = 1 → command frame; bits 14-0 = data length of command
//   bit 15  = 0 → data frame;    bits 14-0 = data length
//
// Command byte occupies the first byte of the data portion of a command frame.
package fido

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// BinkP command bytes.
const (
	bpM_NUL  byte = 0
	bpM_ADR  byte = 1
	bpM_PWD  byte = 2
	bpM_FILE byte = 3
	bpM_OK   byte = 4
	bpM_EOB  byte = 5
	bpM_GOT  byte = 6
	bpM_ERR  byte = 7
	bpM_BSY  byte = 8
	bpM_GET  byte = 9
	bpM_SKIP byte = 10
)

// PollResult describes the outcome of a BinkP poll.
type PollResult struct {
	Sent     []string // basenames of files sent
	Received []string // basenames of files received
	Error    error
}

// Poll dials the uplink, exchanges M_NUL/M_ADR/M_PWD, sends all files
// in outboundDir, receives any inbound files into inboundDir, then hangs up.
// db is used to resolve FidoNet-address uplinks via the imported nodelist.
// When dbg is non-nil, a full protocol trace is written to that session file.
// When global BinkpDebug is enabled, trace lines also go to binkp-debug.log.
func Poll(nd *NetworkDef, outboundFiles []string, inboundDir string, db *sql.DB, dbg *BinkpDebugSession) *PollResult {
	res := &PollResult{}

	if nd.Uplink == "" {
		res.Error = fmt.Errorf("no uplink configured for network %s", nd.Name)
		return res
	}

	dialHost, dialPort, err := ResolveBinkpDialTarget(nd.Name, nd.Uplink, nd.Port(), db)
	if err != nil {
		res.Error = err
		return res
	}
	if dialPort == 0 {
		dialPort = nd.Port()
	}
	target := net.JoinHostPort(dialHost, strconv.Itoa(dialPort))

	conn, err := net.DialTimeout("tcp", target, 30*time.Second)
	if err != nil {
		res.Error = fmt.Errorf("binkp dial %s: %w", target, err)
		return res
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Minute))

	bp := &binkpConn{conn: conn, nd: nd, debug: dbg, inboundDir: inboundDir}
	bp.tracef("dial OK %s (timeout 5m)", target)
	bp.tracef("outbound queue: %d file(s)", len(outboundFiles))
	for _, fpath := range outboundFiles {
		bp.tracef("  %s", fpath)
	}

	// ── Handshake ────────────────────────────────────────────────────────────
	if err := bp.sendCmd(bpM_NUL, "SYS VirtBBS"); err != nil {
		res.Error = err; return res
	}
	if err := bp.sendCmd(bpM_NUL, "ZYZ "+nd.Address); err != nil {
		res.Error = err; return res
	}
	if err := bp.sendCmd(bpM_ADR, strings.Join(nd.AllAddrsString(), " ")); err != nil {
		res.Error = err; return res
	}

	// Wait for remote M_ADR before sending password.
	if err := bp.waitForADR(); err != nil {
		res.Error = fmt.Errorf("handshake ADR (%s): %w", target, err); return res
	}

	pwd := nd.Password
	if pwd == "" {
		pwd = "-"
	}
	if err := bp.sendCmd(bpM_PWD, pwd); err != nil {
		res.Error = fmt.Errorf("handshake PWD (%s): %w", target, err); return res
	}

	// Wait for M_OK or M_ERR.
	if err := bp.waitForAuth(); err != nil {
		res.Error = fmt.Errorf("handshake auth (%s): %w", target, err); return res
	}

	// ── Send outbound files ───────────────────────────────────────────────────
	for _, fpath := range outboundFiles {
		if err := bp.sendFile(fpath); err != nil {
			res.Error = err; return res
		}
		res.Sent = append(res.Sent, filepath.Base(fpath))
	}

	// Signal end-of-batch.
	if err := bp.sendCmd(bpM_EOB, ""); err != nil {
		res.Error = err; return res
	}

	// ── Receive inbound files until remote EOB ────────────────────────────────
	received, err := bp.receiveUntilEOB(inboundDir)
	if err != nil {
		res.Error = fmt.Errorf("receive (%s): %w", target, err); return res
	}
	res.Received = append(bp.earlyReceived, received...)

	// Final EOB / BYE exchange.
	_ = bp.sendCmd(bpM_EOB, "")
	if res.Error != nil {
		bp.tracef("session end: error %v", res.Error)
	} else {
		bp.tracef("session end: sent %d, received %d", len(res.Sent), len(res.Received))
	}
	return res
}

// PollAndTossResult combines a BinkP poll outcome with the toss that
// automatically follows it.
type PollAndTossResult struct {
	Poll *PollResult
	Toss *TossResult // nil if the poll itself failed, so nothing was tossed
}

// PollAndToss gathers nd's outbound files, polls its uplink, deletes any
// successfully-sent files, and then — regardless of whether anything new
// was received this time — tosses nd's inbound directory, so any mail
// left over from a previous partial failure is also picked up.
//
// This is the single entry point shared by the sysop "[P]oll uplink" menu,
// the "fido.poll" management API, and the automatic scheduler, so all three
// behave identically.
func PollAndToss(cfg *Config, nd *NetworkDef, store *messages.Store, confStore *conferences.Store, sysopName string, fileArea FileArea, filesRoot, attachmentsRoot string) *PollAndTossResult {
	return PollAndTossDebug(cfg, nd, store, confStore, sysopName, fileArea, filesRoot, attachmentsRoot, nil)
}

// PollAndTossDebug is like PollAndToss but writes a full protocol trace when
// dbg is set (admin "Debug poll") and/or when global BinkpDebug is enabled.
func PollAndTossDebug(cfg *Config, nd *NetworkDef, store *messages.Store, confStore *conferences.Store, sysopName string, fileArea FileArea, filesRoot, attachmentsRoot string, dbg *BinkpDebugSession) *PollAndTossResult {
	if store != nil {
		if qr := ScanNetmailQueue(cfg, nd, store.DB(), attachmentsRoot); qr != nil {
			for _, e := range qr.Errors {
				LogBinkp(fmt.Sprintf("netmail queue [%s]: %s", nd.Name, e))
			}
			if qr.Exported > 0 {
				LogBinkp(fmt.Sprintf("netmail queue [%s]: exported %d message(s) to outbound", nd.Name, qr.Exported))
			}
		}
		if confStore != nil {
			bbsName := resolveOutboundBBSName("")
			if sr := ScanNetworkEcho(cfg, nd, store, confStore, bbsName, attachmentsRoot); sr != nil {
				for _, e := range sr.Errors {
					LogBinkp(fmt.Sprintf("echo scan [%s]: %s", nd.Name, e))
				}
				if sr.Scanned > 0 {
					LogBinkp(fmt.Sprintf("echo scan [%s]: exported %d message(s) to outbound (%d pkt file(s))",
						nd.Name, sr.Scanned, sr.PKTFiles))
				}
			}
		}
	}

	uplink := nd.UplinkAddr()
	outFiles := binkpOutboundFilesFor(nd, nil, uplink)

	pollResult := Poll(nd, outFiles, nd.InboundDir, store.DB(), dbg)
	result := &PollAndTossResult{Poll: pollResult}
	uplinkKey := nd.Uplink
	if pollResult.Error != nil {
		logPollResult(nd.Name, "client", len(pollResult.Sent), len(pollResult.Received), pollResult.Error)
		RecordClientPoll(nd.Name, uplinkKey, false, len(pollResult.Sent), len(pollResult.Received))
		RecordBinkpTransferredTICs(nd.Name, "uplink", uplinkKey, nd.OutboundDir, pollResult.Sent, true)
		RecordBinkpTransferredTICs(nd.Name, "uplink", uplinkKey, nd.InboundDir, pollResult.Received, false)
		return result
	}

	sentBase := make(map[string]bool, len(pollResult.Sent))
	for _, f := range pollResult.Sent {
		sentBase[f] = true
	}
	for _, full := range outFiles {
		if sentBase[filepath.Base(full)] {
			_ = os.Remove(full)
		}
	}

	tossResult, err := TossDir(cfg, nd, store, confStore, sysopName, fileArea, filesRoot, attachmentsRoot)
	if err != nil {
		tossResult = &TossResult{Errors: []string{err.Error()}}
	}
	result.Toss = tossResult
	logPollResult(nd.Name, "client", len(pollResult.Sent), len(pollResult.Received), nil)
	logTossResult(nd.Name, "client", result.Toss)
	RecordClientPoll(nd.Name, uplinkKey, true, len(pollResult.Sent), len(pollResult.Received))
	RecordBinkpTransferredTICs(nd.Name, "uplink", uplinkKey, nd.OutboundDir, pollResult.Sent, true)
	RecordBinkpTransferredTICs(nd.Name, "uplink", uplinkKey, nd.InboundDir, pollResult.Received, false)
	for _, w := range RunNodelistMonitorForNetwork(nd, store.DB(), confStore, store, fileArea, sysopName) {
		LogBinkp(fmt.Sprintf("nodelist monitor [%s]: %s", nd.Name, w))
	}
	return result
}

// ─── Server (accepting inbound polls) ──────────────────────────────────────────

// ServeBinkP listens on every distinct binkp_port configured among enabled
// networks (a single port shared by several networks is handled — each
// inbound connection's identity, from its M_ADR, is matched against every
// enabled network's uplink and downlink addresses to find which one it
// belongs to). The caller is authenticated by password: the downlink's own
// configured password if it matched a Downlink, or the network's session
// Password if it matched the network's own uplink address.
//
// After exchanging files, any received inbound is tossed immediately
// (matching the "every poll completes with a toss" behaviour of the
// outbound side — see PollAndToss).
//
// Returns a stop function that closes all listeners. Logs session activity
// and errors with the standard logger; never returns an error itself once
// listening has started (per-connection failures are logged, not fatal).
func ServeBinkP(cfg *Config, store *messages.Store, confStore *conferences.Store, sysopName string, fileArea FileArea, filesRoot, attachmentsRoot string) (stop func(), err error) {
	portCandidates := map[int][]NetworkDef{}
	for _, nd := range cfg.AllNetworks() {
		if !nd.Enabled {
			continue
		}
		portCandidates[nd.Port()] = append(portCandidates[nd.Port()], nd)
	}
	if len(portCandidates) == 0 {
		return func() {}, nil
	}

	var listeners []net.Listener
	for port, candidates := range portCandidates {
		addr := fmt.Sprintf(":%d", port)
		ln, lerr := net.Listen("tcp", addr)
		if lerr != nil {
			for _, l := range listeners {
				l.Close()
			}
			return nil, fmt.Errorf("binkp listen %s: %w", addr, lerr)
		}
		listeners = append(listeners, ln)
		LogBinkp(fmt.Sprintf("BinkP listening on %s (%d network(s))", addr, len(candidates)))
		go binkpAcceptLoop(ln, cfg, candidates, store, confStore, sysopName, fileArea, filesRoot, attachmentsRoot)
	}

	return func() {
		for _, l := range listeners {
			l.Close()
		}
	}, nil
}

func binkpAcceptLoop(ln net.Listener, cfg *Config, candidates []NetworkDef, store *messages.Store, confStore *conferences.Store, sysopName string, fileArea FileArea, filesRoot, attachmentsRoot string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		go binkpHandleIncoming(conn, cfg, candidates, store, confStore, sysopName, fileArea, filesRoot, attachmentsRoot)
	}
}

// binkpHandleIncoming answers one inbound BinkP connection: handshake,
// identify and authenticate the caller, receive their files, send back
// whatever is queued for them, then toss what was received.
func binkpHandleIncoming(conn net.Conn, cfg *Config, candidates []NetworkDef, store *messages.Store, confStore *conferences.Store, sysopName string, fileArea FileArea, filesRoot, attachmentsRoot string) {
	defer conn.Close()
	// Short deadline for the initial handshake so port scans don't linger.
	_ = conn.SetDeadline(time.Now().Add(binkpHandshakeTimeout))
	bp := &binkpConn{conn: conn}

	_ = bp.sendCmd(bpM_NUL, "SYS VirtBBS")
	if len(candidates) > 0 {
		_ = bp.sendCmd(bpM_NUL, "ZYZ "+candidates[0].Address)
		_ = bp.sendCmd(bpM_ADR, strings.Join(candidates[0].AllAddrsString(), " "))
	}

	peerAddrs, err := binkpWaitForADRAddrs(bp)
	if err != nil {
		if isBinkpProbeError(err) {
			noteBinkpProbe(conn.RemoteAddr(), err)
			return
		}
		LogBinkp(fmt.Sprintf("binkp server: handshake error from %s: %v", conn.RemoteAddr(), err))
		RecordSessionError("")
		return
	}

	nd, dl, isUplink := binkpMatchPeer(candidates, peerAddrs)
	if nd == nil {
		_ = bp.sendCmd(bpM_ERR, "unknown system")
		LogBinkp(fmt.Sprintf("binkp server: rejected unknown peer %v from %s", peerAddrs, conn.RemoteAddr()))
		RecordSessionError("")
		return
	}
	bp.nd = nd
	bp.inboundDir = nd.InboundDir

	wantPassword := nd.Password
	if !isUplink && dl != nil {
		wantPassword = dl.Password
	}
	linkType := "downlink"
	peerKey := ""
	if isUplink {
		linkType = "uplink"
		peerKey = nd.Uplink
	} else if dl != nil {
		if dl.Address != "" {
			peerKey = dl.Address
		} else {
			peerKey = dl.Name
		}
	}
	if peerKey == "" && len(peerAddrs) > 0 {
		peerKey = peerAddrs[0]
	}

	if wantPassword != "" {
		gotPwd, err := binkpWaitForPWD(bp)
		if err != nil {
			if isBinkpProbeError(err) {
				noteBinkpProbe(conn.RemoteAddr(), err)
				return
			}
			LogBinkp(fmt.Sprintf("binkp server [%s]: password handshake error: %v", nd.Name, err))
			RecordSessionError(nd.Name)
			RecordServerSession(nd.Name, linkType, peerKey, false, 0, 0)
			return
		}
		if gotPwd != wantPassword {
			_ = bp.sendCmd(bpM_ERR, "bad password")
			LogBinkp(fmt.Sprintf("binkp server [%s]: bad password from %v", nd.Name, peerAddrs))
			RecordSessionError(nd.Name)
			RecordServerSession(nd.Name, linkType, peerKey, false, 0, 0)
			return
		}
	}
	if err := bp.sendCmd(bpM_OK, ""); err != nil {
		if isBinkpProbeError(err) {
			noteBinkpProbe(conn.RemoteAddr(), err)
		}
		return
	}

	// Authenticated peer — extend deadline for the transfer session.
	_ = conn.SetDeadline(time.Now().Add(binkpSessionTimeout))

	if err := os.MkdirAll(nd.InboundDir, 0755); err != nil {
		LogBinkp(fmt.Sprintf("binkp server [%s]: %v", nd.Name, err))
		RecordSessionError(nd.Name)
		RecordServerSession(nd.Name, linkType, peerKey, false, 0, 0)
		return
	}

	peerAddr, _ := ParseAddr(peerAddrs[0])
	if catalog, ok := fileArea.(FreqCatalog); ok && nd.EffectiveFreqEnabled() {
		ndb := OpenNodelistDB(store.DB())
		bp.onFreqGET = func(arg string) error {
			okAuth, _ := authorizeFreqRequester(nd, ndb, peerAddr)
			if !okAuth {
				return fmt.Errorf("unauthorized FREQ from %s", peerAddr)
			}
			cmds := strings.Fields(arg)
			if len(cmds) == 0 {
				return nil
			}
			RecordFreqRecv(nd.Name, linkType, peerKey)
			_, _, _, err := FulfillFreqCommands(nd, catalog, filesRoot, nd.Name, peerAddr, cmds)
			return err
		}
	}

	received, err := bp.receiveUntilEOB(nd.InboundDir)
	if err != nil {
		LogBinkp(fmt.Sprintf("binkp server [%s]: receive error: %v", nd.Name, err))
		RecordSessionError(nd.Name)
		RecordServerSession(nd.Name, linkType, peerKey, false, 0, len(received))
		return
	}

	if catalog, ok := fileArea.(FreqCatalog); ok && nd.EffectiveFreqEnabled() {
		ndb := OpenNodelistDB(store.DB())
		for _, note := range ProcessInboundREQFiles(nd, catalog, filesRoot, ndb, nd.Name, nd.InboundDir, peerAddr) {
			LogBinkp(fmt.Sprintf("binkp server [%s]: %s", nd.Name, note))
		}
	}

	outFiles := binkpOutboundFilesFor(nd, dl, peerAddr)
	var sent []string
	for _, f := range outFiles {
		if err := bp.sendFile(f); err != nil {
			LogBinkp(fmt.Sprintf("binkp server [%s]: send error: %v", nd.Name, err))
			break
		}
		sent = append(sent, f)
	}
	_ = bp.sendCmd(bpM_EOB, "")
	for _, f := range sent {
		_ = os.Remove(f)
	}

	who := "uplink"
	if !isUplink {
		who = "downlink " + dl.Name
	}
	LogBinkp(fmt.Sprintf("binkp server [%s]: session with %s (%v) complete — received %d, sent %d",
		nd.Name, who, peerAddrs, len(received), len(sent)))
	RecordServerSession(nd.Name, linkType, peerKey, true, len(sent), len(received))
	var sentNames []string
	for _, f := range sent {
		sentNames = append(sentNames, filepath.Base(f))
	}
	RecordBinkpTransferredTICs(nd.Name, linkType, peerKey, nd.OutboundDir, sentNames, true)
	RecordBinkpTransferredTICs(nd.Name, linkType, peerKey, nd.InboundDir, received, false)

	if len(received) > 0 {
		if tr, err := TossDir(cfg, nd, store, confStore, sysopName, fileArea, filesRoot, attachmentsRoot); err != nil {
			LogBinkp(fmt.Sprintf("binkp server [%s]: auto-toss error: %v", nd.Name, err))
		} else {
			logTossResult(nd.Name, "server", tr)
			for _, w := range RunNodelistMonitorForNetwork(nd, store.DB(), confStore, store, fileArea, sysopName) {
				LogBinkp(fmt.Sprintf("nodelist monitor [%s]: %s", nd.Name, w))
			}
		}
	}
}

// binkpWaitForADRAddrs reads frames until the caller's M_ADR arrives,
// returning its space-separated address list.
func binkpWaitForADRAddrs(b *binkpConn) ([]string, error) {
	for {
		isCmd, cmd, payload, err := b.recvFrame()
		if err != nil {
			return nil, err
		}
		if !isCmd {
			continue
		}
		switch cmd {
		case bpM_ADR:
			return strings.Fields(string(payload)), nil
		case bpM_ERR:
			return nil, fmt.Errorf("remote M_ERR during handshake: %s", string(payload))
		case bpM_BSY:
			return nil, fmt.Errorf("remote busy")
		}
	}
}

// binkpWaitForPWD reads frames until the caller's M_PWD arrives.
func binkpWaitForPWD(b *binkpConn) (string, error) {
	for {
		isCmd, cmd, payload, err := b.recvFrame()
		if err != nil {
			return "", err
		}
		if !isCmd {
			continue
		}
		switch cmd {
		case bpM_PWD:
			return string(payload), nil
		case bpM_ERR:
			return "", fmt.Errorf("remote M_ERR: %s", string(payload))
		}
	}
}

// binkpMatchPeer finds which candidate network (and, if applicable, which
// configured Downlink) a caller's announced addresses belong to: either
// the network's own uplink address (isUplink=true), or one of its
// configured downlinks (isUplink=false, dl set).
func binkpMatchPeer(candidates []NetworkDef, peerAddrs []string) (nd *NetworkDef, dl *Downlink, isUplink bool) {
	for i := range candidates {
		c := candidates[i]
		uplink := c.UplinkAddr()
		for _, pa := range peerAddrs {
			a, err := ParseAddr(pa)
			if err != nil {
				continue
			}
			if uplink != (Addr{}) && a.Zone == uplink.Zone && a.Net == uplink.Net && a.Node == uplink.Node {
				return &c, nil, true
			}
			if found := c.DownlinkByAddr(a); found != nil {
				return &c, found, false
			}
		}
	}
	return nil, nil, false
}

// binkpOutboundFilesFor returns the paths of files queued for a specific
// peer: if dl is set (the peer is a known downlink), only files whose name
// was tagged with that downlink's address by scan/file-scan fan-out; otherwise
// (the peer is our uplink) every file NOT specifically tagged for one of our
// own downlinks. Includes .pkt, .tic, and TIC payload files.
func binkpOutboundFilesFor(nd *NetworkDef, dl *Downlink, peerAddr Addr) []string {
	var out []string
	entries, _ := os.ReadDir(nd.OutboundDir)
	downlinkTags := downlinkOutboundTags(nd)

	if dl != nil {
		tag := sanitizeAddrForFilename(peerAddr)
		out = append(out, filterTaggedOutbound(nd.OutboundDir, entries, tag, true)...)
		appendOutboundSubdirFiles(&out, nd.OutboundDir, peerAddr)
	} else {
		out = append(out, filterTaggedOutbound(nd.OutboundDir, entries, "", false, downlinkTags)...)
		for _, e := range entries {
			if !e.IsDir() || !strings.HasSuffix(strings.ToUpper(e.Name()), ".OUT") {
				continue
			}
			subDir := filepath.Join(nd.OutboundDir, e.Name())
			if subEntries, err := os.ReadDir(subDir); err == nil {
				for _, se := range subEntries {
					if !se.IsDir() {
						out = append(out, filepath.Join(subDir, se.Name()))
					}
				}
			}
		}
	}

	if nd.IsHub() {
		if latest := latestFullNodelistPath(nd.NodelistDir); latest != "" {
			out = append(out, latest)
		}
		if latest := latestNodelistDiffPath(nd.NodelistDir); latest != "" {
			out = append(out, latest)
		}
	}
	return dedupePaths(out)
}

func downlinkOutboundTags(nd *NetworkDef) map[string]bool {
	tags := map[string]bool{}
	for _, dl := range nd.Downlinks {
		if a, err := ParseAddr(dl.Address); err == nil {
			tags[sanitizeAddrForFilename(a)] = true
		}
	}
	return tags
}

func filterTaggedOutbound(dir string, entries []os.DirEntry, wantTag string, requireTag bool, excludeTags ...map[string]bool) []string {
	var excl map[string]bool
	if len(excludeTags) > 0 {
		excl = excludeTags[0]
	}
	var out []string
	var ticFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isBinkpOutboundCandidate(name) {
			continue
		}
		if requireTag {
			if !strings.Contains(name, wantTag) {
				continue
			}
		} else if excl != nil {
			tagged := false
			for tag := range excl {
				if strings.Contains(name, tag) {
					tagged = true
					break
				}
			}
			if tagged {
				continue
			}
		}
		full := filepath.Join(dir, name)
		out = append(out, full)
		if strings.EqualFold(filepath.Ext(name), ".tic") {
			ticFiles = append(ticFiles, full)
		}
	}
	for _, ticPath := range ticFiles {
		if payload := ticPayloadPath(ticPath); payload != "" {
			out = append(out, payload)
		}
	}
	return out
}

func isBinkpOutboundCandidate(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".pkt", ".tic", ".zip", ".rar", ".lha", ".lzh", ".arj", ".zoo", ".gz", ".7z":
		return true
	}
	return ext == "" || strings.Contains(name, "_")
}

func ticPayloadPath(ticPath string) string {
	body, err := os.ReadFile(ticPath)
	if err != nil {
		return ""
	}
	ticket, err := ParseTIC(body)
	if err != nil || ticket.File == "" {
		return ""
	}
	payload := filepath.Join(filepath.Dir(ticPath), ticket.File)
	if _, err := os.Stat(payload); err != nil {
		return ""
	}
	return payload
}

func dedupePaths(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range in {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func appendOutboundSubdirFiles(out *[]string, baseOutbound string, peerAddr Addr) {
	sub := fmt.Sprintf("%04X%04X.OUT", peerAddr.Zone*0x100+peerAddr.Net, peerAddr.Node)
	crashDir := filepath.Join(baseOutbound, sub)
	if crashEntries, err := os.ReadDir(crashDir); err == nil {
		for _, e := range crashEntries {
			if !e.IsDir() {
				*out = append(*out, filepath.Join(crashDir, e.Name()))
			}
		}
	}
}

// latestNodelistFile finds the most recently modified file in dir whose
// name starts with prefix (e.g. "NODELIST.Z" or "NODEDIFF.Z").
func latestNodelistFile(dir, prefix string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestTime int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().UnixNano() > bestTime {
			bestTime = fi.ModTime().UnixNano()
			best = filepath.Join(dir, e.Name())
		}
	}
	return best
}

// ─── Internal BinkP connection ─────────────────────────────────────────────────

type binkpConn struct {
	conn          net.Conn
	nd            *NetworkDef
	debug         *BinkpDebugSession
	inboundDir    string
	earlyReceived []string
	inbound       *inboundXfer
	// onFreqGET handles session FREQ via bare-name M_GET (optional).
	onFreqGET func(arg string) error
}

type inboundXfer struct {
	file     *os.File
	name     string
	size     int64
	mtime    int64
	received int64
}

func (b *binkpConn) abortInboundXfer() {
	if b == nil || b.inbound == nil {
		return
	}
	if b.inbound.file != nil {
		_ = b.inbound.file.Close()
	}
	b.inbound = nil
}

func (b *binkpConn) beginInboundFILE(arg, destDir string) error {
	b.abortInboundXfer()
	name, size, mtime, offset, ok := parseBinkpFileArg(arg)
	if !ok || name == "" || size <= 0 {
		return fmt.Errorf("invalid M_FILE: %q", arg)
	}
	destPath := filepath.Join(destDir, name)
	var f *os.File
	var err error
	received := int64(0)
	if offset > 0 {
		f, err = os.OpenFile(destPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("open inbound %s: %w", destPath, err)
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			return err
		}
		received = offset
	} else {
		f, err = os.Create(destPath)
		if err != nil {
			return fmt.Errorf("create inbound %s: %w", destPath, err)
		}
	}
	b.inbound = &inboundXfer{
		file:     f,
		name:     name,
		size:     size,
		mtime:    mtime,
		received: received,
	}
	return nil
}

// writeInboundDATA appends one data frame to the current inbound file.
// When the file is complete it sends M_GOT and returns the basename.
func (b *binkpConn) writeInboundDATA(data []byte) (string, error) {
	if b == nil || b.inbound == nil || b.inbound.file == nil {
		return "", nil
	}
	if len(data) == 0 {
		return "", nil
	}
	if _, err := b.inbound.file.Write(data); err != nil {
		b.abortInboundXfer()
		return "", err
	}
	b.inbound.received += int64(len(data))
	if b.inbound.received < b.inbound.size {
		return "", nil
	}
	name := b.inbound.name
	size := b.inbound.size
	mtime := b.inbound.mtime
	if err := b.inbound.file.Close(); err != nil {
		b.abortInboundXfer()
		return "", err
	}
	b.inbound = nil
	gotArg := fmt.Sprintf("%s %d %d", name, size, mtime)
	if err := b.sendCmd(bpM_GOT, gotArg); err != nil {
		return "", err
	}
	return name, nil
}

func (b *binkpConn) noteInboundFile(name string) {
	if name == "" {
		return
	}
	b.earlyReceived = append(b.earlyReceived, name)
}

func (b *binkpConn) tracing() bool {
	return b != nil && (b.debug != nil || BinkpDebugEnabled())
}

func (b *binkpConn) tracef(format string, args ...interface{}) {
	if b == nil || !b.tracing() {
		return
	}
	line := fmt.Sprintf(format, args...)
	network := ""
	if b.nd != nil {
		network = b.nd.Name
	}
	if b.debug != nil {
		b.debug.writef("%s", line)
	}
	if BinkpDebugEnabled() {
		writeBinkpDebugGlobal(network, line)
	}
}

// sendCmd sends a command frame: header (bit15=1, len=1+len(arg)) + cmd byte + arg bytes.
func (b *binkpConn) sendCmd(cmd byte, arg string) error {
	if b.tracing() {
		b.tracef("TX %s %q", binkpCmdName(cmd), binkpSanitizeCmdArg(cmd, arg))
	}
	data := append([]byte{cmd}, []byte(arg)...)
	hdr := uint16(0x8000) | uint16(len(data))
	if err := binary.Write(b.conn, binary.BigEndian, hdr); err != nil {
		return err
	}
	_, err := b.conn.Write(data)
	return err
}

// sendData sends a data frame.
func (b *binkpConn) sendData(data []byte) error {
	if b.tracing() {
		b.tracef("TX data %d bytes", len(data))
	}
	hdr := uint16(len(data))
	if err := binary.Write(b.conn, binary.BigEndian, hdr); err != nil {
		return err
	}
	_, err := b.conn.Write(data)
	return err
}

// recvFrame reads one BinkP frame.  Returns (isCmd, cmdByte, payload, err).
func (b *binkpConn) recvFrame() (isCmd bool, cmd byte, payload []byte, err error) {
	var hdr uint16
	if err = binary.Read(b.conn, binary.BigEndian, &hdr); err != nil {
		return
	}
	isCmd = hdr&0x8000 != 0
	length := int(hdr & 0x7FFF)
	payload = make([]byte, length)
	if _, err = io.ReadFull(b.conn, payload); err != nil {
		return
	}
	if isCmd && len(payload) > 0 {
		cmd = payload[0]
		payload = payload[1:]
	}
	if b.tracing() {
		if isCmd {
			b.tracef("RX %s %q", binkpCmdName(cmd), binkpSanitizeCmdArg(cmd, string(payload)))
		} else {
			b.tracef("RX data %d bytes", len(payload))
		}
	}
	return
}

// waitForADR reads frames until M_ADR is received.
func (b *binkpConn) waitForADR() error {
	for {
		isCmd, cmd, _, err := b.recvFrame()
		if err != nil {
			return err
		}
		if isCmd && cmd == bpM_ADR {
			return nil
		}
		if isCmd && cmd == bpM_ERR {
			return fmt.Errorf("remote M_ERR during ADR")
		}
		if isCmd && cmd == bpM_BSY {
			return fmt.Errorf("remote busy (M_BSY)")
		}
	}
}

// waitForAuth reads frames until M_OK or M_ERR.
func (b *binkpConn) waitForAuth() error {
	for {
		isCmd, cmd, payload, err := b.recvFrame()
		if err != nil {
			return err
		}
		if isCmd {
			switch cmd {
			case bpM_OK:
				return nil
			case bpM_ERR:
				return fmt.Errorf("authentication failed: %s", string(payload))
			case bpM_BSY:
				return fmt.Errorf("remote busy: %s", string(payload))
			}
		}
	}
}

// errGetRestart signals the peer requested a resume via M_GET.
type errGetRestart struct {
	offset int64
}

func (e errGetRestart) Error() string {
	return fmt.Sprintf("M_GET restart at offset %d", e.offset)
}

var errRemoteSkip = fmt.Errorf("remote skipped file")

// parseBinkpFileArg parses "name size mtime [offset]" from M_FILE / M_GET.
func parseBinkpFileArg(arg string) (name string, size, mtime, offset int64, ok bool) {
	parts := strings.Fields(arg)
	if len(parts) < 1 {
		return "", 0, 0, 0, false
	}
	name = parts[0]
	if len(parts) >= 2 {
		fmt.Sscanf(parts[1], "%d", &size)
	}
	if len(parts) >= 3 {
		fmt.Sscanf(parts[2], "%d", &mtime)
	}
	if len(parts) >= 4 {
		fmt.Sscanf(parts[3], "%d", &offset)
	}
	return name, size, mtime, offset, true
}

// isResumeGET reports whether arg is an FTS-1026 resume M_GET (name size time offset)
// rather than a bare filename FREQ request.
func isResumeGET(arg string) bool {
	parts := strings.Fields(arg)
	return len(parts) >= 4
}

// sendFile sends one file using M_FILE + M_DATA frames then waits for M_GOT.
// Honours peer M_GET resume requests (FTS-1026).
func (b *binkpConn) sendFile(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	size := fi.Size()
	mtime := fi.ModTime().Unix()
	offset := int64(0)
	for {
		err := b.sendFileFrom(path, size, mtime, offset)
		if err == nil {
			return nil
		}
		if err == errRemoteSkip {
			b.tracef("remote skipped %s", filepath.Base(path))
			return nil
		}
		var gr errGetRestart
		if errors.As(err, &gr) {
			if gr.offset < 0 || gr.offset > size {
				return fmt.Errorf("M_GET %s: invalid offset %d", filepath.Base(path), gr.offset)
			}
			offset = gr.offset
			b.tracef("resuming %s at offset %d", filepath.Base(path), offset)
			continue
		}
		return err
	}
}

func (b *binkpConn) sendFileFrom(path string, size, mtime, offset int64) error {
	name := filepath.Base(path)
	fileArg := fmt.Sprintf("%s %d %d %d", name, size, mtime, offset)
	b.tracef("sending file %s (%d bytes, offset %d)", name, size, offset)
	if err := b.sendCmd(bpM_FILE, fileArg); err != nil {
		return fmt.Errorf("M_FILE %s: %w", name, err)
	}

	// Brief window for early M_GET / M_SKIP after the announce.
	if err := b.pollOutboundCtrl(name, size, mtime, 200*time.Millisecond); err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	buf := make([]byte, 8192)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			if serr := b.sendData(buf[:n]); serr != nil {
				return fmt.Errorf("M_DATA %s: %w", name, serr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if err := b.waitForGOT(name, size, mtime); err != nil {
		return err
	}
	return nil
}

// pollOutboundCtrl checks for peer control frames without blocking longer than wait.
// wait=0 means a zero-deadline non-blocking peek (best-effort).
func (b *binkpConn) pollOutboundCtrl(name string, size, mtime int64, wait time.Duration) error {
	if b.conn == nil {
		return nil
	}
	deadline := time.Now().Add(wait)
	if wait == 0 {
		deadline = time.Now()
	}
	_ = b.conn.SetReadDeadline(deadline)
	defer func() { _ = b.conn.SetReadDeadline(time.Time{}) }()

	isCmd, cmd, payload, err := b.recvFrame()
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return nil
		}
		// Some platforms wrap deadline differently.
		if strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "timeout") {
			return nil
		}
		return err
	}
	return b.handleOutboundCtrl(name, size, mtime, isCmd, cmd, payload)
}

func (b *binkpConn) handleOutboundCtrl(name string, size, mtime int64, isCmd bool, cmd byte, payload []byte) error {
	if !isCmd {
		done, err := b.writeInboundDATA(payload)
		if err != nil {
			return err
		}
		b.noteInboundFile(done)
		return nil
	}
	switch cmd {
	case bpM_GET:
		gName, gSize, gMtime, gOff, ok := parseBinkpFileArg(string(payload))
		if !ok {
			return nil
		}
		if gName == name && (gSize == 0 || gSize == size) && (gMtime == 0 || gMtime == mtime) {
			return errGetRestart{offset: gOff}
		}
		return nil
	case bpM_SKIP:
		parts := strings.Fields(string(payload))
		if len(parts) > 0 && parts[0] == name {
			return errRemoteSkip
		}
		return nil
	case bpM_FILE:
		if b.inboundDir == "" {
			return nil
		}
		return b.beginInboundFILE(string(payload), b.inboundDir)
	case bpM_GOT:
		// Unexpected during send; ignore non-matching.
		return nil
	case bpM_ERR:
		return fmt.Errorf("remote error: %s", string(payload))
	default:
		return nil
	}
}

// waitForGOT reads frames until M_GOT for the named file arrives.
// binkd and other hosts may send their own M_FILE batches while we wait;
// those must be received and acknowledged or the session stalls.
// Peer M_GET triggers errGetRestart so sendFile can resume from offset.
func (b *binkpConn) waitForGOT(name string, size, mtime int64) error {
	destDir := b.inboundDir
	for {
		isCmd, cmd, payload, err := b.recvFrame()
		if err != nil {
			return err
		}
		if !isCmd {
			done, err := b.writeInboundDATA(payload)
			if err != nil {
				return err
			}
			b.noteInboundFile(done)
			continue
		}
		switch cmd {
		case bpM_GOT:
			parts := strings.SplitN(string(payload), " ", 2)
			if parts[0] == name {
				return nil
			}
		case bpM_FILE:
			if destDir == "" {
				continue
			}
			if err := b.beginInboundFILE(string(payload), destDir); err != nil {
				return err
			}
		case bpM_SKIP:
			parts := strings.Fields(string(payload))
			if len(parts) > 0 && parts[0] == name {
				return errRemoteSkip
			}
		case bpM_ERR:
			return fmt.Errorf("remote error: %s", string(payload))
		case bpM_GET:
			gName, gSize, gMtime, gOff, ok := parseBinkpFileArg(string(payload))
			if !ok {
				continue
			}
			if gName == name && (gSize == 0 || gSize == size) && (gMtime == 0 || gMtime == mtime) {
				return errGetRestart{offset: gOff}
			}
		}
	}
}

// receiveUntilEOB reads files until a remote M_EOB frame or error.
// Files are written to destDir.
func (b *binkpConn) receiveUntilEOB(destDir string) ([]string, error) {
	var received []string
	b.abortInboundXfer()

	for {
		isCmd, cmd, payload, err := b.recvFrame()
		if err != nil {
			b.abortInboundXfer()
			if err == io.EOF {
				// Some BinkP hosts (e.g. Synchronet/sbbs, binkd) close the TCP
				// session after the batch instead of sending a final M_EOB.
				return received, nil
			}
			return received, err
		}

		if !isCmd {
			done, err := b.writeInboundDATA(payload)
			if err != nil {
				return received, err
			}
			if done != "" {
				received = append(received, done)
			}
			continue
		}

		switch cmd {
		case bpM_EOB:
			b.abortInboundXfer()
			return received, nil

		case bpM_FILE:
			if err := b.beginInboundFILE(string(payload), destDir); err != nil {
				return received, err
			}

		case bpM_GET:
			// Session FREQ: bare filename(s), not FTS-1026 resume (handled in send).
			if !isResumeGET(string(payload)) && b.onFreqGET != nil {
				if err := b.onFreqGET(string(payload)); err != nil {
					b.tracef("FREQ M_GET: %v", err)
				}
			}

		case bpM_ERR:
			b.abortInboundXfer()
			return received, fmt.Errorf("remote M_ERR: %s", string(payload))

		case bpM_GOT:
			// Sent by remote for our files — already handled in sendFile.

		case bpM_NUL, bpM_ADR, bpM_OK:
			// Informational during transfer — ignore.
		}
	}
}
