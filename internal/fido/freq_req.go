package fido

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WaZooREQName returns the BinkleyTerm-style outbound request filename for dest
// (NNNNMMMM.req hex net/node).
func WaZooREQName(dest Addr) string {
	return fmt.Sprintf("%04x%04x.req", uint16(dest.Net), uint16(dest.Node))
}

// WriteWaZooREQ writes filenames (one per line) into a WaZOO .REQ in outDir.
func WriteWaZooREQ(outDir string, dest Addr, names []string) (string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, WaZooREQName(dest))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, err := fmt.Fprintln(f, n); err != nil {
			return path, err
		}
	}
	return path, nil
}

// ParseREQFile reads WaZOO/Bark-style .REQ lines (filename [!password]).
func ParseREQFile(path string) (cmds []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		cmds = append(cmds, line)
	}
	return cmds, sc.Err()
}

// ProcessWaZooREQ fulfills an inbound .REQ from peer using the same engine as netmail FREQ.
func ProcessWaZooREQ(nd *NetworkDef, catalog FreqCatalog, filesRoot string, ndb *NodelistDB, networkName string, peer Addr, reqPath string) error {
	cmds, err := ParseREQFile(reqPath)
	if err != nil {
		return err
	}
	pm := &Message{
		OrigAddr: peer,
		FromName: peer.String(),
		ToName:   FreqRobotName,
		Subject:  "FREQ",
		Body:     strings.Join(cmds, "\r\n") + "\r\n",
	}
	err = ProcessFreqRequest(nd, catalog, filesRoot, ndb, networkName, pm)
	_ = os.Remove(reqPath)
	return err
}

// ProcessInboundREQFiles scans inboundDir for *.req / *.REQ and fulfills them.
func ProcessInboundREQFiles(nd *NetworkDef, catalog FreqCatalog, filesRoot string, ndb *NodelistDB, networkName string, inboundDir string, peer Addr) []string {
	var notes []string
	entries, err := os.ReadDir(inboundDir)
	if err != nil {
		return []string{err.Error()}
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		path := filepath.Join(inboundDir, name)
		switch {
		case ext == ".req":
			if err := ProcessWaZooREQ(nd, catalog, filesRoot, ndb, networkName, peer, path); err != nil {
				notes = append(notes, fmt.Sprintf("REQ %s: %v", name, err))
			} else {
				notes = append(notes, fmt.Sprintf("REQ %s processed", name))
			}
		case ext == ".srf" || strings.EqualFold(name, "SRIF.TXT") || strings.HasPrefix(strings.ToUpper(name), "SRIF"):
			if err := ProcessSRIFRequest(nd, catalog, filesRoot, ndb, networkName, path); err != nil {
				notes = append(notes, fmt.Sprintf("SRIF %s: %v", name, err))
			} else {
				notes = append(notes, fmt.Sprintf("SRIF %s processed", name))
				_ = os.Remove(path)
			}
		}
	}
	return notes
}

// FulfillFreqCommands queues files for the given command lines without netmail reply.
// Used by BinkP session FREQ (M_GET) and SRIF.
func FulfillFreqCommands(nd *NetworkDef, catalog FreqCatalog, filesRoot string, networkName string, peer Addr, cmdLines []string) (queuedTotal int, queuedBytes int64, report string, err error) {
	if !nd.EffectiveFreqEnabled() {
		return 0, 0, "", nil
	}
	var out strings.Builder
	maxFiles := nd.EffectiveFreqMaxFiles()
	maxBytes := nd.EffectiveFreqMaxBytes()
	for _, line := range cmdLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, filePW := splitFreqFilePassword(line)
		upper := strings.ToUpper(name)
		switch {
		case upper == "%HELP" || upper == "HELP":
			writeFreqHelp(&out)
		case upper == "%LIST" || upper == "LIST" || upper == "FILES":
			writeFreqCatalog(&out, catalog, filesRoot)
		case upper == "NODELIST" || upper == "%NODELIST":
			n, b, msg := queueFreqNodelistFile(nd, peer, "NODELIST.")
			out.WriteString(msg)
			queuedTotal += n
			queuedBytes += b
		case upper == "NODEDIFF" || upper == "%NODEDIFF":
			n, b, msg := queueFreqNodelistFile(nd, peer, "NODEDIFF.")
			out.WriteString(msg)
			queuedTotal += n
			queuedBytes += b
		default:
			if want := nd.FreqFilePassword(name); want != "" && want != filePW {
				fmt.Fprintf(&out, "  %s — bad file password\r\n", name)
				continue
			}
			if queuedTotal >= maxFiles || queuedBytes >= maxBytes {
				fmt.Fprintf(&out, "  %s — limit reached\r\n", name)
				continue
			}
			matches, rerr := resolveFreqFiles(catalog, filesRoot, name)
			if rerr != nil {
				fmt.Fprintf(&out, "  %s — ERROR: %v\r\n", name, rerr)
				continue
			}
			if len(matches) == 0 {
				fmt.Fprintf(&out, "  %s — not found\r\n", name)
				continue
			}
			remainingFiles := maxFiles - queuedTotal
			remainingBytes := maxBytes - queuedBytes
			queued, msgs := queueFreqRawFiles(nd, peer, matches, remainingFiles, remainingBytes)
			for _, m := range msgs {
				out.WriteString(m)
			}
			for _, q := range queued {
				fmt.Fprintf(&out, "  queued %s (%d bytes)\r\n", q.destName, q.bytes)
				RecordFreqFileQueued(networkName, q.destName, q.bytes)
				queuedTotal++
				queuedBytes += q.bytes
			}
		}
	}
	if queuedTotal > 0 {
		recordFreqNodeSent(networkName, peer.String(), 1, queuedTotal, queuedBytes)
	}
	return queuedTotal, queuedBytes, out.String(), nil
}

// splitFreqFilePassword splits "name" or "name password" or "name !password" (Bark).
func splitFreqFilePassword(line string) (name, password string) {
	line = strings.TrimSpace(line)
	if i := strings.IndexByte(line, '!'); i > 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	fields := strings.Fields(line)
	if len(fields) >= 2 && !strings.ContainsAny(fields[0], "*?") {
		// "filename password" — only when first token looks like a single file.
		return fields[0], fields[1]
	}
	if len(fields) == 0 {
		return "", ""
	}
	return fields[0], ""
}

// ProcessSRIFRequest parses an SRIF request control file and fulfills file requests.
// SRIF keys recognized: RequestFilename / RequestFile, Password, SystemAddress.
// External helper spawning is optional via nd.SRIFHelper (shell command with %s = path).
func ProcessSRIFRequest(nd *NetworkDef, catalog FreqCatalog, filesRoot string, ndb *NodelistDB, networkName, srifPath string) error {
	f, err := os.Open(srifPath)
	if err != nil {
		return err
	}
	defer f.Close()
	var names []string
	var password string
	peer := Addr{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		key, val, ok := strings.Cut(line, " ")
		if !ok {
			key, val, ok = strings.Cut(line, "=")
		}
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch strings.ToLower(key) {
		case "requestfilename", "requestfile", "file":
			names = append(names, val)
		case "password":
			password = val
		case "systemaddress", "address":
			if a, err := ParseAddr(val); err == nil {
				peer = a
			}
		}
	}
	if peer == (Addr{}) {
		return fmt.Errorf("srif: missing SystemAddress")
	}
	ok, _ := authorizeFreqRequester(nd, ndb, peer)
	if !ok {
		return fmt.Errorf("srif: unauthorized %s", peer)
	}
	if nd.FreqPassword != "" && password != nd.FreqPassword {
		return fmt.Errorf("srif: bad password")
	}
	cmds := names
	if password != "" && nd.FreqPassword == "" {
		// per-line passwords already in names via Bark form
	}
	_, _, _, err = FulfillFreqCommands(nd, catalog, filesRoot, networkName, peer, cmds)
	return err
}

// WriteBarkREQ writes a Bark-compatible request (filenames with optional !password).
func WriteBarkREQ(outDir string, dest Addr, names []string, password string) (string, error) {
	lines := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if password != "" {
			lines = append(lines, n+" !"+password)
		} else {
			lines = append(lines, n)
		}
	}
	return WriteWaZooREQ(outDir, dest, lines)
}
