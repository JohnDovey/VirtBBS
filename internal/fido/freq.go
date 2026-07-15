// Package fido — freq.go
//
// Implements FREQ (File REQuest): netmail to "Freq", WaZOO/Bark .REQ, SRIF,
// BinkP session M_GET, and per-file passwords. Matching files are queued as
// raw outbound files for BinkP pickup.
package fido

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// FreqRobotName is the netmail ToName that triggers the FREQ responder.
const FreqRobotName = "Freq"

// DefaultFreqMaxFiles is the default cap on files queued per request.
const DefaultFreqMaxFiles = 5

// DefaultFreqMaxBytes is the default total byte cap per request (5 MiB).
const DefaultFreqMaxBytes int64 = 5 * 1024 * 1024

// IsFreqRequest reports whether toName addresses the FREQ robot.
func IsFreqRequest(toName string) bool {
	n := strings.TrimSpace(toName)
	return strings.EqualFold(n, FreqRobotName) ||
		strings.EqualFold(n, "FileRequest") ||
		strings.EqualFold(n, "FREQ")
}

// IsFreqSubject reports Internet Rex-style FREQ netmail (subject "FREQ", commands in body).
func IsFreqSubject(subject string) bool {
	subject = strings.TrimSpace(subject)
	if strings.EqualFold(subject, "FREQ") {
		return true
	}
	fields := strings.Fields(subject)
	return len(fields) > 0 && strings.EqualFold(fields[0], "FREQ")
}

// ProcessFreqRequest handles inbound FREQ netmail: authorizes the requester,
// parses commands, queues raw files to the requester's outbound .OUT subdir,
// and replies by netmail.
func ProcessFreqRequest(nd *NetworkDef, catalog FreqCatalog, filesRoot string, ndb *NodelistDB, networkName string, pm *Message) error {
	if !nd.EffectiveFreqEnabled() {
		return nil
	}
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return fmt.Errorf("freq: invalid local address %q", nd.Address)
	}

	ok, requesterName := authorizeFreqRequester(nd, ndb, pm.OrigAddr)
	if !ok {
		return replyFreq(nd, our, pm, "Unknown system — you are not authorized to request files.\r\n")
	}

	cmdLines, passwordOK := parseFreqAuth(pm.Subject, pm.Body, nd.FreqPassword)
	if !passwordOK {
		return replyFreq(nd, our, pm, "Invalid password.\r\n")
	}
	// FILE_REQUEST / file_request style: filenames in subject, password in body.
	if pm.Attrib&AttribFileRequest != 0 || strings.EqualFold(strings.TrimSpace(pm.ToName), "FileRequest") {
		if subCmds := freqSubjectFileCommands(pm.Subject, nd.FreqPassword); len(subCmds) > 0 {
			cmdLines = append(subCmds, cmdLines...)
		}
	}
	linkType, peerKey := LinkTypeForAddr(nd, pm.OrigAddr)
	RecordFreqRecv(networkName, linkType, peerKey)

	var out strings.Builder
	fmt.Fprintf(&out, "FREQ response for %s (%s)\r\n\r\n", requesterName, pm.OrigAddr.String())

	if len(cmdLines) == 0 {
		writeFreqHelp(&out)
		return replyFreq(nd, our, pm, out.String())
	}

	queuedTotal, queuedBytes, report, err := FulfillFreqCommands(nd, catalog, filesRoot, networkName, pm.OrigAddr, cmdLines)
	if err != nil {
		return err
	}
	out.WriteString(report)
	if queuedTotal > 0 {
		fmt.Fprintf(&out, "\r\n%d file(s) queued (%d bytes).\r\n", queuedTotal, queuedBytes)
	}

	return replyFreq(nd, our, pm, out.String())
}

// freqSubjectFileCommands extracts requested filenames from a FILE_REQUEST subject.
func freqSubjectFileCommands(subject, globalPassword string) []string {
	subject = strings.TrimSpace(subject)
	if subject == "" || strings.EqualFold(subject, "FREQ") {
		return nil
	}
	subject = freqSubjectForAuth(subject)
	var out []string
	for _, f := range strings.Fields(subject) {
		if globalPassword != "" && f == globalPassword {
			continue
		}
		out = append(out, f)
	}
	return out
}

func authorizeFreqRequester(nd *NetworkDef, ndb *NodelistDB, addr Addr) (bool, string) {
	if dl := nd.DownlinkByAddr(addr); dl != nil {
		name := dl.Name
		if name == "" {
			name = addr.String()
		}
		return true, name
	}
	if ndb != nil {
		if entry, err := ndb.LookupAddr(nd.Name, addr); err == nil && entry != nil {
			name := entry.Sysop
			if name == "" {
				name = entry.Name
			}
			if name == "" {
				name = addr.String()
			}
			return true, name
		}
	}
	return false, ""
}

func parseFreqAuth(subject, body, wantPassword string) (cmdLines []string, ok bool) {
	subject = freqSubjectForAuth(subject)
	return parseFixRequestAuth(subject, body, wantPassword)
}

func freqSubjectForAuth(subject string) string {
	subject = strings.TrimSpace(subject)
	for _, robot := range []string{"Freq", "FileRequest", "FREQ", "File Request"} {
		if strings.EqualFold(subject, robot) {
			return ""
		}
		prefix := robot + " "
		if len(subject) > len(prefix) && strings.EqualFold(subject[:len(robot)], robot) && subject[len(robot)] == ' ' {
			return strings.TrimSpace(subject[len(robot)+1:])
		}
	}
	if strings.EqualFold(subject, "FREQ") {
		return ""
	}
	return subject
}

type freqFileMatch struct {
	srcPath  string
	filename string
	dirName  string
	size     int64
}

type freqQueuedFile struct {
	destName string
	bytes    int64
}

func resolveFreqFiles(catalog FreqCatalog, filesRoot, pattern string) ([]freqFileMatch, error) {
	if catalog == nil || filesRoot == "" {
		return nil, fmt.Errorf("files not configured")
	}
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, nil
	}
	dirs, err := catalog.ListFreqDirs()
	if err != nil {
		return nil, err
	}
	patUpper := strings.ToUpper(pattern)
	var matches []freqFileMatch
	seen := map[string]bool{}
	for _, dir := range dirs {
		catalogFiles, err := catalog.ListFreqFiles(dir.ID)
		if err != nil {
			continue
		}
		dirPath := filepath.Join(filesRoot, dir.RelPath)
		for _, f := range catalogFiles {
			base := filepath.Base(f.Filename)
			if !freqPatternMatch(patUpper, strings.ToUpper(base)) {
				continue
			}
			src := filepath.Join(dirPath, f.Filename)
			if seen[src] {
				continue
			}
			info, err := os.Stat(src)
			if err != nil || info.IsDir() {
				continue
			}
			seen[src] = true
			matches = append(matches, freqFileMatch{
				srcPath:  src,
				filename: base,
				dirName:  dir.Name,
				size:     info.Size(),
			})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].filename != matches[j].filename {
			return matches[i].filename < matches[j].filename
		}
		return matches[i].dirName < matches[j].dirName
	})
	return matches, nil
}

func freqPatternMatch(pattern, name string) bool {
	if pattern == name {
		return true
	}
	if strings.ContainsAny(pattern, "*?") {
		ok, _ := path.Match(pattern, name)
		return ok
	}
	return false
}

func queueFreqRawFiles(nd *NetworkDef, requester Addr, matches []freqFileMatch, maxFiles int, maxBytes int64) ([]freqQueuedFile, []string) {
	outDir := OutboundDir(nd.OutboundDir, requester, nd.UplinkAddr(), true)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return nil, []string{fmt.Sprintf("  queue ERROR: %v\r\n", err)}
	}
	var queued []freqQueuedFile
	var msgs []string
	for _, m := range matches {
		if maxFiles > 0 && len(queued) >= maxFiles {
			break
		}
		if maxBytes > 0 && m.size > maxBytes {
			msgs = append(msgs, fmt.Sprintf("  %s — too large (%d bytes)\r\n", m.filename, m.size))
			continue
		}
		var batchBytes int64
		for _, q := range queued {
			batchBytes += q.bytes
		}
		if maxBytes > 0 && batchBytes+m.size > maxBytes {
			break
		}
		destName := uniqueOutboundName(outDir, m.filename)
		destPath := filepath.Join(outDir, destName)
		if err := copyFile(m.srcPath, destPath); err != nil {
			msgs = append(msgs, fmt.Sprintf("  %s — copy failed: %v\r\n", m.filename, err))
			continue
		}
		queued = append(queued, freqQueuedFile{destName: destName, bytes: m.size})
	}
	return queued, msgs
}

func uniqueOutboundName(outDir, base string) string {
	candidate := base
	for i := 0; ; i++ {
		if _, err := os.Stat(filepath.Join(outDir, candidate)); os.IsNotExist(err) {
			return candidate
		}
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		if i == 0 {
			candidate = stem + "_1" + ext
		} else {
			candidate = fmt.Sprintf("%s_%d%s", stem, i+1, ext)
		}
	}
}

func queueFreqNodelistFile(nd *NetworkDef, requester Addr, prefix string) (count int, bytes int64, msg string) {
	src := latestNodelistFile(nd.NodelistDir, prefix)
	if src == "" {
		return 0, 0, fmt.Sprintf("  %s — not available\r\n", strings.TrimSuffix(prefix, "."))
	}
	info, err := os.Stat(src)
	if err != nil || info.IsDir() {
		return 0, 0, fmt.Sprintf("  %s — not available\r\n", strings.TrimSuffix(prefix, "."))
	}
	queued, msgs := queueFreqRawFiles(nd, requester, []freqFileMatch{{
		srcPath:  src,
		filename: filepath.Base(src),
		size:     info.Size(),
	}}, 1, info.Size())
	if len(queued) == 0 {
		if len(msgs) > 0 {
			return 0, 0, msgs[0]
		}
		return 0, 0, fmt.Sprintf("  %s — queue failed\r\n", strings.TrimSuffix(prefix, "."))
	}
	return 1, info.Size(), fmt.Sprintf("  queued %s (%d bytes)\r\n", queued[0].destName, info.Size())
}

func writeFreqHelp(out *strings.Builder) {
	out.WriteString("FREQ commands (one per line):\r\n")
	out.WriteString("  filename       request a file by name\r\n")
	out.WriteString("  *.zip          wildcard (* and ?)\r\n")
	out.WriteString("  %LIST / FILES  list available files\r\n")
	out.WriteString("  NODELIST       queue latest nodelist\r\n")
	out.WriteString("  NODEDIFF       queue latest nodediff\r\n")
	out.WriteString("  %HELP          show this help\r\n\r\n")
}

func writeFreqCatalog(out *strings.Builder, catalog FreqCatalog, filesRoot string) {
	if catalog == nil || filesRoot == "" {
		out.WriteString("  (files not configured)\r\n")
		return
	}
	dirs, err := catalog.ListFreqDirs()
	if err != nil {
		fmt.Fprintf(out, "  catalog ERROR: %v\r\n", err)
		return
	}
	var lines []string
	for _, dir := range dirs {
		catalogFiles, err := catalog.ListFreqFiles(dir.ID)
		if err != nil {
			continue
		}
		for _, f := range catalogFiles {
			lines = append(lines, fmt.Sprintf("  %-12s %-24s %8d  %s",
				dir.Name, f.Filename, f.Size, truncateFreqDesc(f.Description, 40)))
		}
	}
	if len(lines) == 0 {
		out.WriteString("  (no files)\r\n")
		return
	}
	sort.Strings(lines)
	for _, l := range lines {
		out.WriteString(l)
		out.WriteString("\r\n")
	}
}

func truncateFreqDesc(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func replyFreq(nd *NetworkDef, our Addr, pm *Message, body string) error {
	uplink := nd.UplinkAddr()
	if uplink == (Addr{}) {
		return fmt.Errorf("freq: no uplink configured to route reply")
	}
	reply := &NetmailMsg{
		FromName:    FreqRobotName,
		FromAddr:    our.String(),
		ToName:      pm.FromName,
		ToAddr:      pm.OrigAddr.String(),
		Subject:     "FREQ response",
		Body:        body,
		NoSignature: true,
	}
	outDir := OutboundDir(nd.OutboundDir, uplink, uplink, false)
	_, err := WritePKT(our, uplink, nd.Password, outDir, []*NetmailMsg{reply}, nd.Name)
	if err == nil {
		linkType, peerKey := LinkTypeForAddr(nd, pm.OrigAddr)
		RecordFreqSent(nd.Name, linkType, peerKey)
	}
	return err
}

// RequestFreq composes and writes an outbound FREQ request to toAddr.
// mode overrides the network freq_outbound setting when non-empty
// (classic, file_request, wazoo, or bark).
func RequestFreq(nd *NetworkDef, fromName string, lines []string, toAddr, mode string) (pktPath string, err error) {
	switch ResolveFreqOutboundMode(nd, mode) {
	case FreqOutboundFileRequest:
		return requestFreqFileRequest(nd, fromName, lines, toAddr)
	case FreqOutboundWaZooMode:
		return requestFreqWaZoo(nd, lines, toAddr, false)
	case FreqOutboundBark:
		return requestFreqWaZoo(nd, lines, toAddr, true)
	default:
		return requestFreqClassic(nd, fromName, lines, toAddr)
	}
}

func requestFreqWaZoo(nd *NetworkDef, lines []string, toAddr string, bark bool) (string, error) {
	dest, err := ParseAddr(toAddr)
	if err != nil {
		return "", fmt.Errorf("invalid destination %q: %w", toAddr, err)
	}
	names, err := freqFileRequestNames(lines)
	if err != nil {
		return "", err
	}
	outDir := OutboundDir(nd.OutboundDir, dest, nd.UplinkAddr(), true)
	var path string
	if bark {
		path, err = WriteBarkREQ(outDir, dest, names, nd.FreqPassword)
	} else {
		path, err = WriteWaZooREQ(outDir, dest, names)
	}
	if err == nil {
		linkType, peerKey := LinkTypeForAddr(nd, dest)
		if linkType == "" {
			linkType = "downlink"
			peerKey = dest.String()
		}
		RecordFreqSent(nd.Name, linkType, peerKey)
	}
	return path, err
}

func requestFreqFileRequest(nd *NetworkDef, fromName string, lines []string, toAddr string) (pktPath string, err error) {
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return "", fmt.Errorf("invalid local address %q", nd.Address)
	}
	dest, err := ParseAddr(toAddr)
	if err != nil {
		return "", fmt.Errorf("invalid destination %q: %w", toAddr, err)
	}

	subject, body, err := BuildFreqFileRequest(lines, nd.FreqPassword)
	if err != nil {
		return "", err
	}

	msg := &NetmailMsg{
		FromName:    fromName,
		FromAddr:    our.String(),
		ToName:      "FileRequest",
		ToAddr:      dest.String(),
		Subject:     subject,
		Body:        body,
		FileRequest: true,
		NoSignature: true,
	}
	return writeFreqRequest(nd, our, dest, msg)
}

func requestFreqClassic(nd *NetworkDef, fromName string, lines []string, toAddr string) (pktPath string, err error) {
	our := nd.NodeAddr()
	if our == (Addr{}) {
		return "", fmt.Errorf("invalid local address %q", nd.Address)
	}
	dest, err := ParseAddr(toAddr)
	if err != nil {
		return "", fmt.Errorf("invalid destination %q: %w", toAddr, err)
	}

	subject, body, err := BuildClassicFreqRequest(lines, nd.FreqPassword)
	if err != nil {
		return "", err
	}

	msg := &NetmailMsg{
		FromName:    fromName,
		FromAddr:    our.String(),
		ToName:      FreqRobotName,
		ToAddr:      dest.String(),
		Subject:     subject,
		Body:        body,
		NoSignature: true,
	}
	return writeFreqRequest(nd, our, dest, msg)
}

func writeFreqRequest(nd *NetworkDef, our, dest Addr, msg *NetmailMsg) (string, error) {
	outDir := OutboundDir(nd.OutboundDir, dest, nd.UplinkAddr(), true)
	path, err := WritePKT(our, dest, nd.Password, outDir, []*NetmailMsg{msg}, nd.Name)
	if err == nil {
		linkType, peerKey := LinkTypeForAddr(nd, dest)
		if linkType == "" {
			linkType = "downlink"
			peerKey = dest.String()
		}
		RecordFreqSent(nd.Name, linkType, peerKey)
	}
	return path, err
}

// BuildClassicFreqRequest formats classic FREQ netmail: commands in the body,
// optional remote password in the subject (same as AreaFix/FileFix robots).
func BuildClassicFreqRequest(lines []string, remotePassword string) (subject, body string, err error) {
	var b strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			fmt.Fprintf(&b, "%s\r\n", line)
		}
	}
	body = b.String()
	if body == "" {
		return "", "", fmt.Errorf("no commands to request")
	}
	subject = FreqRobotName
	if pw := strings.TrimSpace(remotePassword); pw != "" {
		subject = pw
	}
	return subject, body, nil
}

// BuildFreqFileRequest formats FILE_REQUEST netmail: requested names in the
// in the subject (space-separated), optional remote password as the body.
func BuildFreqFileRequest(lines []string, remotePassword string) (subject, body string, err error) {
	names, err := freqFileRequestNames(lines)
	if err != nil {
		return "", "", err
	}
	subject = strings.Join(names, " ")
	if pw := strings.TrimSpace(remotePassword); pw != "" {
		body = pw + "\r\n"
	}
	return subject, body, nil
}

func freqFileRequestNames(lines []string) ([]string, error) {
	var names []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		switch upper {
		case "%HELP", "HELP":
			continue
		case "%LIST", "LIST", "FILES", "ALLFILES":
			names = append(names, "ALLFILES")
		case "NODELIST", "%NODELIST":
			names = append(names, "NODELIST.")
		case "NODEDIFF", "%NODEDIFF":
			names = append(names, "NODEDIFF.")
		default:
			if strings.HasPrefix(line, "%") {
				line = strings.TrimSpace(line[1:])
			}
			if line != "" {
				names = append(names, line)
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no file names to request")
	}
	return names, nil
}
