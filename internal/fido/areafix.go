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
//   v0.4.0  2026-06-28  %RESCAN backlog export, +TAG,R=N subscribe-with-rescan
//   v0.2.0  2026-06-25  Initial implementation — AreaFix responder (for downlink
//                        subscription requests) and request generator (for
//                        subscribing to our own uplink as a downlink)
//   v0.3.0  2026-06-25  ProcessAreaFixRequest/replyAreaFix/areaFixTagExists take a
//                        *NetworkDef instead of *Config, so the responder works for
//                        any configured network, not just primary
// ============================================================================

package fido

// Package fido — areafix.go
//
// Implements AreaFix, the long-standing FidoNet convention for managing
// echomail area subscriptions by netmail. Two independent roles:
//
//   Responder  — other systems ("downlinks") send netmail to "AreaFix"
//                 at OUR address to subscribe/unsubscribe from the echo
//                 areas we feed them. ProcessAreaFixRequest handles this.
//
//   Requester  — THIS BBS sends netmail to "AreaFix" at our UPLINK's
//                 address to subscribe/unsubscribe from areas we want to
//                 receive. RequestAreaFix handles this.
//
// Command syntax (case-insensitive, one command per line):
//
// Password may appear in the netmail subject (classic AreaFix/FileFix) or as
// the first non-blank body line (VirtBBS requester and some other tossers).
//
//	<password>          (subject and/or first body line)
//	+AREA_TAG       subscribe to AREA_TAG (+TAG,R=N sends N old messages)
//	-AREA_TAG       unsubscribe from AREA_TAG
//	%LIST           list all areas available to subscribe to
//	%QUERY          list areas currently subscribed to
//	%RESCAN         rescan subscribed areas (or set rescan mode for +TAG lines)
//	%RESCAN TAG     rescan backlog for subscribed area TAG
//	%HELP           show this command summary

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// AreaFixRobotName is the netmail ToName that triggers the responder.
const AreaFixRobotName = "AreaFix"

// IsAreaFixRequest reports whether toName addresses the AreaFix robot.
func IsAreaFixRequest(toName string) bool {
	return strings.EqualFold(strings.TrimSpace(toName), AreaFixRobotName)
}

// AreaFixDB manages AreaFix subscription state in SQLite.
type AreaFixDB struct{ db *sql.DB }

// OpenAreaFixDB returns an AreaFixDB using the shared database connection.
func OpenAreaFixDB(db *sql.DB) *AreaFixDB { return &AreaFixDB{db: db} }

// Subscribe records that downlinkAddr (zone:net/node) receives areaTag.
func (a *AreaFixDB) Subscribe(network, downlinkAddr, areaTag string) error {
	_, err := a.db.Exec(`INSERT OR IGNORE INTO fido_areafix_subs (network, downlink_addr, area_tag)
		VALUES (?,?,?)`, network, downlinkAddr, areaTag)
	return err
}

// Unsubscribe removes a downlink's subscription to areaTag.
func (a *AreaFixDB) Unsubscribe(network, downlinkAddr, areaTag string) error {
	_, err := a.db.Exec(`DELETE FROM fido_areafix_subs WHERE network=? AND downlink_addr=? AND area_tag=?`,
		network, downlinkAddr, areaTag)
	return err
}

// SubscriptionsFor returns the area tags downlinkAddr currently subscribes to.
func (a *AreaFixDB) SubscriptionsFor(network, downlinkAddr string) ([]string, error) {
	rows, err := a.db.Query(`SELECT area_tag FROM fido_areafix_subs
		WHERE network=? AND downlink_addr=? ORDER BY area_tag`, network, downlinkAddr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// SubscribedDownlinks returns the addresses of every downlink subscribed to
// areaTag, used by the scanner to fan an outgoing echomail message out to
// downlinks in addition to the conference's normal uplink destination.
func (a *AreaFixDB) SubscribedDownlinks(network, areaTag string) ([]string, error) {
	rows, err := a.db.Query(`SELECT downlink_addr FROM fido_areafix_subs
		WHERE network=? AND area_tag=? ORDER BY downlink_addr`, network, areaTag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var addrs []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, rows.Err()
}

// AllDownlinkAddrs returns the distinct set of downlink addresses with at
// least one subscription on this network. Used by the sysop UI.
func (a *AreaFixDB) AllDownlinkAddrs(network string) ([]string, error) {
	rows, err := a.db.Query(`SELECT DISTINCT downlink_addr FROM fido_areafix_subs
		WHERE network=? ORDER BY downlink_addr`, network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var addrs []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, rows.Err()
}

// ── Responder (downlinks managing their subscriptions with us) ─────────────

// ProcessAreaFixRequest handles an inbound netmail addressed to "AreaFix".
// It validates the sender against the network's configured Downlinks list
// and the password from the subject and/or first non-blank body line, applies any
// +TAG/-TAG/%LIST/%QUERY/%RESCAN/%HELP commands found in the remaining lines,
// and writes an immediate netmail reply summarising the result. When msgStore
// is non-nil, rescan commands queue backlog .pkt files for the downlink.
func ProcessAreaFixRequest(nd *NetworkDef, msgStore *messages.Store, confStore *conferences.Store, networkName, bbsName string, pm *Message) error {
	if msgStore == nil {
		return fmt.Errorf("areafix: message store required")
	}
	db := msgStore.DB()
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return fmt.Errorf("areafix: invalid local address %q", nd.Address)
	}

	dl := nd.DownlinkByAddr(pm.OrigAddr)
	if dl == nil {
		return replyAreaFix(nd, our, pm, "Unknown system — you are not configured as a downlink.\r\n")
	}

	cmdLines, passwordOK := parseFixRequestAuth(pm.Subject, pm.Body, dl.Password)
	if !passwordOK {
		return replyAreaFix(nd, our, pm, "Invalid password.\r\n")
	}
	cmdLines = append(fixRequestSubjectSwitchCommands(pm.Subject, dl.Password), cmdLines...)
	RecordAreaFixRecv(networkName, "downlink", pm.OrigAddr.String())

	areafixDB := OpenAreaFixDB(db)
	areafixDB.ensureDownlinkSchema()
	downlinkAddr := pm.OrigAddr.String()
	dlState, _ := areafixDB.DownlinkStateFor(networkName, downlinkAddr)

	var out strings.Builder
	fmt.Fprintf(&out, "AreaFix response for %s (%s)\r\n\r\n", dl.Name, downlinkAddr)

	if len(cmdLines) == 0 {
		writeAreaFixHelp(&out)
	}

	rescanMode := false

	flushRescan := func(tags []string, maxMsgs int, prefix string) {
		if msgStore == nil || len(tags) == 0 {
			return
		}
		res, err := RescanEchoToDownlink(nd, msgStore, confStore, bbsName, downlinkAddr, tags, maxMsgs)
		if err != nil {
			fmt.Fprintf(&out, "  %srescan ERROR: %v\r\n", prefix, err)
			return
		}
		for _, e := range res.Errors {
			fmt.Fprintf(&out, "  %srescan WARNING: %s\r\n", prefix, e)
		}
		if res.Messages == 0 {
			fmt.Fprintf(&out, "  %srescan — no messages to send\r\n", prefix)
		} else {
			fmt.Fprintf(&out, "  %srescan — %d message(s) queued\r\n", prefix, res.Messages)
		}
	}

	subscribed := func(tag string) bool {
		tags, err := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
		if err != nil {
			return false
		}
		tag = strings.ToUpper(tag)
		for _, t := range tags {
			if t == tag {
				return true
			}
		}
		return false
	}

	for _, rawLine := range cmdLines {
		line := normalizeAreaFixCommandLine(rawLine)
		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case upper == "%LIST" || upper == "LIST":
			subs, _ := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
			writeAreaFixListWithLinks(&out, confStore, networkName, nd, subscribedTagSet(subs))
		case upper == "%QUERY" || upper == "QUERY":
			writeAreaFixQuery(&out, areafixDB, networkName, downlinkAddr, dlState)
		case upper == "%UNLINKED" || upper == "%AVAIL":
			subs, _ := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
			writeAreaFixUnlinked(&out, confStore, networkName, nd, subs)
		case upper == "%PAUSE" || upper == "PAUSE":
			if err := areafixDB.SetDownlinkPaused(networkName, downlinkAddr, true); err != nil {
				fmt.Fprintf(&out, "  %%PAUSE ERROR: %v\r\n", err)
			} else {
				dlState.Paused = true
				out.WriteString("  %PAUSE — mail delivery held (subscriptions unchanged)\r\n")
			}
		case upper == "%RESUME" || upper == "RESUME":
			if err := areafixDB.SetDownlinkPaused(networkName, downlinkAddr, false); err != nil {
				fmt.Fprintf(&out, "  %%RESUME ERROR: %v\r\n", err)
			} else {
				dlState.Paused = false
				out.WriteString("  %RESUME — mail delivery resumed\r\n")
			}
		case upper == "%HELP" || upper == "HELP" || upper == "?":
			writeAreaFixHelp(&out)
		case strings.HasPrefix(upper, "%PASSWD") || strings.HasPrefix(upper, "%PWD") || strings.HasPrefix(upper, "PASSWD") || strings.HasPrefix(upper, "PASSWORD"):
			targetAddr, newPW, ok := parseAreaFixPasswdLine(line)
			if !ok || newPW == "" {
				out.WriteString("  %PASSWD — usage: %PASSWD newpassword  or  %PASSWD from Z:N/NODE newpassword\r\n")
				break
			}
			changeAddr := downlinkAddr
			if targetAddr != "" {
				parsed, err := ParseAddr(targetAddr)
				if err != nil {
					fmt.Fprintf(&out, "  %%PASSWD — invalid address %q\r\n", targetAddr)
					break
				}
				if !dl.MatchesAddr(parsed) {
					out.WriteString("  %PASSWD — you may only change your own password\r\n")
					break
				}
				changeAddr = parsed.String()
			}
			if err := saveDownlinkPassword(networkName, changeAddr, newPW); err != nil {
				fmt.Fprintf(&out, "  %%PASSWD ERROR: %v\r\n", err)
			} else {
				dl.Password = newPW
				out.WriteString("  %PASSWD — password updated\r\n")
			}
		case strings.HasPrefix(upper, "%COMPRESS"):
			comp, listOnly, ok := parseAreaFixCompressLine(line)
			if !ok {
				break
			}
			if listOnly {
				writeAreaFixCompressHelp(&out)
				break
			}
			if !validAreaFixCompressor(comp) {
				fmt.Fprintf(&out, "  %%COMPRESS — unknown compressor %q\r\n", comp)
				writeAreaFixCompressHelp(&out)
				break
			}
			if err := areafixDB.SetDownlinkCompressor(networkName, downlinkAddr, comp); err != nil {
				fmt.Fprintf(&out, "  %%COMPRESS ERROR: %v\r\n", err)
			} else {
				dlState.Compressor = comp
				fmt.Fprintf(&out, "  %%COMPRESS — preference set to %s\r\n", comp)
			}
		case upper == "%RESEND":
			tags, err := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
			if err != nil || len(tags) == 0 {
				out.WriteString("  %RESEND — no subscribed areas\r\n")
			} else {
				flushRescan(tags, 0, "%RESEND ")
			}
		case strings.HasPrefix(upper, "%RESCAN"):
			tag, _ := parseAreaFixRescanLine(line)
			if tag != "" {
				subs, _ := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
				var toRescan []string
				if strings.ContainsAny(tag, "*?") {
					toRescan = expandAreaFixTagPatterns(tag, subs)
				} else if subscribed(tag) {
					toRescan = []string{strings.ToUpper(tag)}
				}
				if len(toRescan) == 0 {
					fmt.Fprintf(&out, "  %-30s NOT SUBSCRIBED — not rescanned\r\n", tag)
				} else {
					flushRescan(toRescan, 0, "")
				}
			} else {
				rescanMode = true
				tags, err := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
				if err != nil || len(tags) == 0 {
					out.WriteString("  %RESCAN — no subscribed areas (subsequent +TAG will rescan)\r\n")
				} else {
					flushRescan(tags, 0, "%RESCAN ")
				}
			}
		case strings.HasPrefix(line, "+") || strings.HasPrefix(line, "="):
			applyAreaFixSubscribeLine(&out, line, rescanMode, networkName, downlinkAddr, nd, confStore, areafixDB, msgStore, bbsName, flushRescan)
		case strings.HasPrefix(line, "-"):
			applyAreaFixUnsubscribeLine(&out, line, networkName, downlinkAddr, nd, confStore, areafixDB)
		default:
			if addTag, _, ok := tryBareAreaFixTagLine(line, confStore, networkName, nd); ok {
				applyAreaFixSubscribeLine(&out, "+"+addTag, rescanMode, networkName, downlinkAddr, nd, confStore, areafixDB, msgStore, bbsName, flushRescan)
			} else {
				fmt.Fprintf(&out, "  Unrecognised command: %q\r\n", rawLine)
			}
		}
	}

	out.WriteString("\r\n")
	writeAreaFixQuery(&out, areafixDB, networkName, downlinkAddr, dlState)

	return replyAreaFix(nd, our, pm, out.String())
}

// parseFixRequestAuth validates an AreaFix/FileFix password from the netmail
// subject and/or body. Classic AreaFix puts the password in the subject
// (optionally followed by switches); VirtBBS and some tossers use the first
// non-blank body line instead. Returns command lines with any consumed
// password line removed.
func parseFixRequestAuth(subject, body, wantPassword string) (cmdLines []string, ok bool) {
	if wantPassword == "" {
		return fixRequestCommandLines(body), true
	}

	subject = fixRequestSubjectForAuth(subject)
	if subject == wantPassword || fixRequestSubjectPassword(subject, wantPassword) {
		cmds := fixRequestCommandLines(body)
		for len(cmds) > 0 && cmds[0] == wantPassword {
			cmds = cmds[1:]
		}
		return cmds, true
	}

	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\r"), "\r")
	var cmds []string
	passwordOK := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !passwordOK {
			if line == wantPassword {
				passwordOK = true
				continue
			}
			return nil, false
		}
		cmds = append(cmds, line)
	}
	if !passwordOK {
		return nil, false
	}
	return cmds, true
}

func fixRequestSubjectForAuth(subject string) string {
	subject = strings.TrimSpace(subject)
	for _, robot := range []string{"AreaFix", "FileFix", "AREAFIX", "FILEFIX", "AreaMgr", "FileMgr"} {
		if strings.EqualFold(subject, robot) {
			return ""
		}
		prefix := robot + " "
		if len(subject) > len(prefix) && strings.EqualFold(subject[:len(robot)], robot) && subject[len(robot)] == ' ' {
			return strings.TrimSpace(subject[len(robot)+1:])
		}
	}
	return subject
}

func fixRequestSubjectPassword(subject, wantPassword string) bool {
	fields := strings.Fields(subject)
	if len(fields) == 0 {
		return false
	}
	return fields[0] == wantPassword
}

func fixRequestCommandLines(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	var cmds []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			cmds = append(cmds, line)
		}
	}
	return cmds
}

// SplitFixCommandLines splits a multi-line form field into non-blank command
// lines, accepting CRLF, LF, or CR line endings.
func SplitFixCommandLines(s string) []string {
	return fixRequestCommandLines(s)
}

// BuildFixRequestBody formats AreaFix/FileFix outbound command lines. Area
// tags in adds/removes get a +/- prefix unless the line already starts with
// +, -, =, or % (e.g. %HELP, %LIST).
func BuildFixRequestBody(adds, removes []string) string {
	var body strings.Builder
	for _, line := range adds {
		if out := formatFixRequestAddLine(line); out != "" {
			fmt.Fprintf(&body, "%s\r\n", out)
		}
	}
	for _, line := range removes {
		if out := formatFixRequestRemoveLine(line); out != "" {
			fmt.Fprintf(&body, "%s\r\n", out)
		}
	}
	return body.String()
}

func formatFixRequestAddLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if fixRequestLineHasPrefix(line) {
		return line
	}
	return "+" + strings.ToUpper(line)
}

func formatFixRequestRemoveLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if fixRequestLineHasPrefix(line) {
		return line
	}
	return "-" + strings.ToUpper(line)
}

func fixRequestLineHasPrefix(line string) bool {
	return strings.HasPrefix(line, "+") ||
		strings.HasPrefix(line, "-") ||
		strings.HasPrefix(line, "=") ||
		strings.HasPrefix(line, "%")
}

// areaFixAddCmd holds a parsed +TAG subscribe line.
type areaFixAddCmd struct {
	tag       string
	rescanMax int // -1 = no rescan; 0 = full backlog; N>0 = oldest N messages
}

// parseAreaFixAddLine parses +TAG or =TAG with optional ,R or ,R=N suffix.
func parseAreaFixAddLine(line string) (areaFixAddCmd, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return areaFixAddCmd{}, false
	}
	if line[0] == '+' || line[0] == '=' {
		line = line[1:]
	}
	parts := strings.Split(line, ",")
	tag := strings.ToUpper(strings.TrimSpace(parts[0]))
	cmd := areaFixAddCmd{tag: tag, rescanMax: -1}
	for _, opt := range parts[1:] {
		opt = strings.ToUpper(strings.TrimSpace(opt))
		if opt == "R" {
			cmd.rescanMax = 0
			continue
		}
		if strings.HasPrefix(opt, "R=") {
			n, err := strconv.Atoi(strings.TrimSpace(opt[2:]))
			if err != nil || n < 0 {
				cmd.rescanMax = 0
			} else {
				cmd.rescanMax = n
			}
		}
	}
	return cmd, tag != ""
}

// parseAreaFixRescanLine parses %RESCAN or %RESCAN TAG.
func parseAreaFixRescanLine(line string) (tag string, ok bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToUpper(line), "%RESCAN") {
		return "", false
	}
	rest := strings.TrimSpace(line[len("%RESCAN"):])
	if rest == "" {
		return "", true
	}
	return strings.ToUpper(rest), true
}

// areaFixTagExists reports whether tag is a valid, known echomail area —
// either as a conference's EchoTag (preferred, via confStore) or in the
// legacy nd.Areas map.
func areaFixTagExists(confStore *conferences.Store, networkName string, nd *NetworkDef, tag string) bool {
	if confStore != nil {
		if conf, err := confStore.GetByTag(tag, networkName); err == nil && conf != nil {
			return true
		}
	}
	_, ok := nd.Areas[tag]
	return ok
}

func writeAreaFixList(out *strings.Builder, confStore *conferences.Store, networkName string) {
	writeAreaFixListWithLinks(out, confStore, networkName, nil, nil)
}

func writeAreaFixQuery(out *strings.Builder, areafixDB *AreaFixDB, networkName, downlinkAddr string, st DownlinkState) {
	tags, err := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
	out.WriteString("Currently subscribed:\r\n")
	if err != nil || len(tags) == 0 {
		out.WriteString("  (none)\r\n")
	} else {
		for _, t := range tags {
			fmt.Fprintf(out, "  %s\r\n", t)
		}
	}
	writeAreaFixStatus(out, st)
}

func writeAreaFixHelp(out *strings.Builder) {
	out.WriteString("Password in the subject (classic) or as the first body line.\r\n")
	out.WriteString("Subject switches: -l (list), -q (query), -R (rescan all).\r\n")
	out.WriteString("Commands (one per line; %% escapes a leading %):\r\n")
	out.WriteString("  +TAG / =TAG     subscribe or update link (+TAG,R=N sends N old messages)\r\n")
	out.WriteString("  -TAG            unsubscribe (wildcards * and ? supported)\r\n")
	out.WriteString("  TAG             subscribe without + prefix\r\n")
	out.WriteString("  SUBSCRIBE TAG   same as +TAG; UNSUBSCRIBE / CONNECT / DISCONNECT synonyms\r\n")
	out.WriteString("  %LIST / LISTALL list all areas (* marks subscribed)\r\n")
	out.WriteString("  %QUERY          list your subscriptions\r\n")
	out.WriteString("  %UNLINKED / %AVAIL / AVAIL  areas not yet subscribed\r\n")
	out.WriteString("  %PAUSE / PASSIVE  hold mail; %RESUME / ACTIVE resume\r\n")
	out.WriteString("  %PASSWD newpw   change your AreaFix password\r\n")
	out.WriteString("  %COMPRESS name  set compressor preference (none, zlib)\r\n")
	out.WriteString("  %RESCAN         rescan subscribed areas\r\n")
	out.WriteString("  %RESCAN TAG     rescan one area\r\n")
	out.WriteString("  %RESEND         resend backlog for all subscribed areas\r\n")
	out.WriteString("  %HELP           show this help\r\n\r\n")
}

// replyAreaFix writes an immediate netmail reply from the AreaFix robot
// back to the requester, routed via the network's configured uplink.
func replyAreaFix(nd *NetworkDef, our Addr, pm *Message, body string) error {
	uplink := nd.UplinkAddr()
	if uplink == (Addr{}) {
		return fmt.Errorf("areafix: no uplink configured to route reply")
	}
	reply := &NetmailMsg{
		FromName:    AreaFixRobotName,
		FromAddr:    our.String(),
		ToName:      pm.FromName,
		ToAddr:      pm.OrigAddr.String(),
		Subject:     "AreaFix response",
		Body:        body,
		NoSignature: true,
	}
	outDir := OutboundDir(nd.OutboundDir, uplink, uplink, false)
	_, err := WritePKT(our, uplink, nd.Password, outDir, []*NetmailMsg{reply}, nd.Name)
	if err == nil {
		RecordAreaFixSent(nd.Name, "downlink", pm.OrigAddr.String())
	}
	return err
}

// ── Requester (us subscribing to our own uplink's AreaFix) ─────────────────

// RequestAreaFix composes and writes a netmail to "AreaFix" at nd's own
// uplink, requesting subscription changes (adds/removes are AREA: tags,
// without +/- prefixes). Used when VirtBBS itself is a downlink of nd.
func RequestAreaFix(nd *NetworkDef, fromName string, adds, removes []string) (pktPath string, err error) {
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return "", fmt.Errorf("invalid local address %q", nd.Address)
	}
	uplink := nd.UplinkAddr()
	if uplink == (Addr{}) {
		return "", fmt.Errorf("no uplink configured")
	}

	var body strings.Builder
	body.WriteString(BuildFixRequestBody(adds, removes))

	subject := AreaFixRobotName
	if nd.AreaFixPassword != "" {
		subject = nd.AreaFixPassword
	}

	msg := &NetmailMsg{
		FromName:    fromName,
		FromAddr:    our.String(),
		ToName:      AreaFixRobotName,
		ToAddr:      uplink.String(),
		Subject:     subject,
		Body:        body.String(),
		NoSignature: true,
	}

	outDir := OutboundDir(nd.OutboundDir, uplink, uplink, false)
	path, err := WritePKT(our, uplink, nd.Password, outDir, []*NetmailMsg{msg}, nd.Name)
	if err == nil {
		RecordAreaFixSent(nd.Name, "uplink", uplink.String())
	}
	return path, err
}
