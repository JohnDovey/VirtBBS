package fido

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// DownlinkPasswordSaver persists a downlink AreaFix password in VirtBBS.DAT.
type DownlinkPasswordSaver func(networkName, downlinkAddr, newPassword string) error

var (
	downlinkPasswordMu     sync.Mutex
	downlinkPasswordSaver  DownlinkPasswordSaver
)

// SetDownlinkPasswordSaver registers the callback that updates [[fido.downlinks]]
// after a successful %PASSWD command.
func SetDownlinkPasswordSaver(fn DownlinkPasswordSaver) {
	downlinkPasswordMu.Lock()
	downlinkPasswordSaver = fn
	defer downlinkPasswordMu.Unlock()
}

func saveDownlinkPassword(networkName, downlinkAddr, newPassword string) error {
	downlinkPasswordMu.Lock()
	fn := downlinkPasswordSaver
	downlinkPasswordMu.Unlock()
	if fn == nil {
		return fmt.Errorf("password change not configured on this system")
	}
	return fn(networkName, downlinkAddr, newPassword)
}

// fixRequestSubjectSwitchCommands maps classic subject switches (-l, -q, -R)
// to body commands. Password field in the subject is skipped when present.
func fixRequestSubjectSwitchCommands(subject, wantPassword string) []string {
	subject = fixRequestSubjectForAuth(subject)
	fields := strings.Fields(subject)
	if len(fields) == 0 {
		return nil
	}
	start := 0
	if wantPassword != "" {
		if fields[0] == wantPassword {
			start = 1
		} else if subject == wantPassword {
			return nil
		}
	}
	var cmds []string
	for _, f := range fields[start:] {
		switch strings.ToLower(f) {
		case "-l":
			cmds = append(cmds, "%LIST")
		case "-q":
			cmds = append(cmds, "%QUERY")
		case "-r":
			cmds = append(cmds, "%RESCAN")
		}
	}
	return cmds
}

// normalizeAreaFixCommandLine applies %% escape and synonym mapping.
func normalizeAreaFixCommandLine(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "%%") && len(line) > 1 {
		line = line[1:]
	}
	upper := strings.ToUpper(line)
	switch upper {
	case "LISTALL", "LIST ALL":
		return "%LIST"
	case "AVAIL", "AVAILABLE":
		return "%UNLINKED"
	case "PASSIVE":
		return "%PAUSE"
	case "ACTIVE":
		return "%RESUME"
	}
	if strings.HasPrefix(upper, "SUBSCRIBE ") {
		return "+" + strings.TrimSpace(line[len("SUBSCRIBE "):])
	}
	if strings.HasPrefix(upper, "UNSUBSCRIBE ") {
		return "-" + strings.TrimSpace(line[len("UNSUBSCRIBE "):])
	}
	if strings.HasPrefix(upper, "UNSUB ") {
		return "-" + strings.TrimSpace(line[len("UNSUB "):])
	}
	if strings.HasPrefix(upper, "CONNECT ") {
		return "+" + strings.TrimSpace(line[len("CONNECT "):])
	}
	if strings.HasPrefix(upper, "DISCONNECT ") {
		return "-" + strings.TrimSpace(line[len("DISCONNECT "):])
	}
	if strings.HasPrefix(upper, "PASSWORD ") {
		return "%PASSWD " + strings.TrimSpace(line[len("PASSWORD "):])
	}
	return line
}

func listEchoAreaTags(confStore *conferences.Store, networkName string, nd *NetworkDef) []string {
	seen := map[string]bool{}
	var tags []string
	if confStore != nil {
		confs, err := confStore.ListEcho(networkName)
		if err == nil {
			for _, c := range confs {
				if c.EchoTag != "" && !seen[c.EchoTag] {
					seen[c.EchoTag] = true
					tags = append(tags, strings.ToUpper(c.EchoTag))
				}
			}
		}
	}
	if nd != nil {
		for tag := range nd.Areas {
			tag = strings.ToUpper(tag)
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	sort.Strings(tags)
	return tags
}

func areaFixTagPatternMatch(pattern, tag string) bool {
	pattern = strings.ToUpper(strings.TrimSpace(pattern))
	tag = strings.ToUpper(strings.TrimSpace(tag))
	if pattern == tag {
		return true
	}
	if strings.ContainsAny(pattern, "*?") {
		ok, _ := path.Match(pattern, tag)
		return ok
	}
	return false
}

func expandAreaFixTagPatterns(pattern string, candidates []string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, c := range candidates {
		if areaFixTagPatternMatch(pattern, c) && !seen[c] {
			seen[c] = true
			out = append(out, strings.ToUpper(c))
		}
	}
	sort.Strings(out)
	return out
}

func subscribedTagSet(tags []string) map[string]bool {
	m := make(map[string]bool, len(tags))
	for _, t := range tags {
		m[strings.ToUpper(t)] = true
	}
	return m
}

func writeAreaFixListWithLinks(out *strings.Builder, confStore *conferences.Store, networkName string, nd *NetworkDef, linked map[string]bool) {
	out.WriteString("Areas available:\r\n")
	tags := listEchoAreaTags(confStore, networkName, nd)
	if len(tags) == 0 {
		out.WriteString("  (none configured)\r\n")
		return
	}
	for _, t := range tags {
		mark := " "
		if linked[t] {
			mark = "*"
		}
		fmt.Fprintf(out, "  %s %s\r\n", mark, t)
	}
	out.WriteString("  (* = currently subscribed)\r\n")
}

func writeAreaFixUnlinked(out *strings.Builder, confStore *conferences.Store, networkName string, nd *NetworkDef, subscribed []string) {
	out.WriteString("Areas not subscribed (available):\r\n")
	linked := subscribedTagSet(subscribed)
	tags := listEchoAreaTags(confStore, networkName, nd)
	var unlinked []string
	for _, t := range tags {
		if !linked[t] {
			unlinked = append(unlinked, t)
		}
	}
	if len(unlinked) == 0 {
		out.WriteString("  (none — all areas subscribed or no areas configured)\r\n")
		return
	}
	for _, t := range unlinked {
		fmt.Fprintf(out, "  %s\r\n", t)
	}
}

func writeAreaFixStatus(out *strings.Builder, st DownlinkState) {
	if st.Paused {
		out.WriteString("  Link status: PAUSED (subscriptions kept; new mail held)\r\n")
	} else {
		out.WriteString("  Link status: ACTIVE\r\n")
	}
	if st.Compressor != "" && st.Compressor != "none" {
		fmt.Fprintf(out, "  Compressor: %s\r\n", st.Compressor)
	}
}

func parseAreaFixPasswdLine(line string) (targetAddr, newPassword string, ok bool) {
	line = strings.TrimSpace(line)
	upper := strings.ToUpper(line)
	for _, prefix := range []string{"%PASSWD", "%PWD", "PASSWD", "PWD"} {
		if strings.HasPrefix(upper, prefix) {
			rest := strings.TrimSpace(line[len(prefix):])
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				return "", "", false
			}
			if strings.EqualFold(fields[0], "FROM") && len(fields) >= 3 {
				return fields[1], strings.Join(fields[2:], " "), true
			}
			if len(fields) == 1 {
				return "", fields[0], true
			}
			return "", "", false
		}
	}
	if strings.HasPrefix(upper, "PASSWORD ") {
		rest := strings.TrimSpace(line[len("PASSWORD"):])
		if rest == "" {
			return "", "", false
		}
		return "", rest, true
	}
	return "", "", false
}

func parseAreaFixCompressLine(line string) (compressor string, listOnly bool, ok bool) {
	upper := strings.ToUpper(strings.TrimSpace(line))
	if !strings.HasPrefix(upper, "%COMPRESS") {
		return "", false, false
	}
	rest := strings.TrimSpace(line[len("%COMPRESS"):])
	if rest == "" || strings.EqualFold(rest, "?") {
		return "", true, true
	}
	return strings.ToLower(rest), false, true
}

func writeAreaFixCompressHelp(out *strings.Builder) {
	out.WriteString("  Supported compressors:\r\n")
	for _, c := range SupportedAreaFixCompressors {
		fmt.Fprintf(out, "    %s\r\n", c)
	}
}

type areaFixRescanFn func(tags []string, maxMsgs int, prefix string)

func applyAreaFixSubscribeLine(out *strings.Builder, line string, rescanMode bool,
	networkName, downlinkAddr string, nd *NetworkDef, confStore *conferences.Store,
	areafixDB *AreaFixDB, msgStore *messages.Store, bbsName string, flush areaFixRescanFn) {

	isUpdate := strings.HasPrefix(strings.TrimSpace(line), "=")
	add, ok := parseAreaFixAddLine(line)
	if !ok || add.tag == "" {
		return
	}
	pattern := add.tag
	allTags := listEchoAreaTags(confStore, networkName, nd)
	var targets []string
	if strings.ContainsAny(pattern, "*?") {
		targets = expandAreaFixTagPatterns(pattern, allTags)
	} else if areaFixTagExists(confStore, networkName, nd, pattern) {
		targets = []string{pattern}
	}
	if len(targets) == 0 {
		fmt.Fprintf(out, "  +%-30s UNKNOWN AREA — not added\r\n", pattern)
		return
	}
	subs, _ := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
	linked := subscribedTagSet(subs)
	for _, tag := range targets {
		already := linked[tag]
		if !already {
			if err := areafixDB.Subscribe(networkName, downlinkAddr, tag); err != nil {
				fmt.Fprintf(out, "  +%-30s ERROR: %v\r\n", tag, err)
				continue
			}
			fmt.Fprintf(out, "  +%-30s subscribed\r\n", tag)
			linked[tag] = true
		} else if isUpdate {
			fmt.Fprintf(out, "  =%-30s updated\r\n", tag)
		} else {
			fmt.Fprintf(out, "  +%-30s already subscribed\r\n", tag)
		}
		doRescan := add.rescanMax >= 0 || rescanMode || (isUpdate && add.rescanMax < 0)
		if isUpdate && add.rescanMax < 0 && !rescanMode {
			doRescan = true
		}
		if doRescan && linked[tag] {
			max := add.rescanMax
			if isUpdate && max < 0 {
				max = 0
			}
			flush([]string{tag}, max, "")
		}
	}
}

func applyAreaFixUnsubscribeLine(out *strings.Builder, line string,
	networkName, downlinkAddr string, nd *NetworkDef, confStore *conferences.Store, areafixDB *AreaFixDB) {

	pattern := strings.TrimSpace(line[1:])
	if pattern == "" {
		return
	}
	pattern = strings.ToUpper(pattern)
	subs, err := areafixDB.SubscriptionsFor(networkName, downlinkAddr)
	if err != nil {
		return
	}
	var targets []string
	if strings.ContainsAny(pattern, "*?") {
		targets = expandAreaFixTagPatterns(pattern, subs)
	} else {
		targets = []string{pattern}
	}
	if len(targets) == 0 {
		fmt.Fprintf(out, "  -%-30s not subscribed\r\n", pattern)
		return
	}
	for _, tag := range targets {
		if err := areafixDB.Unsubscribe(networkName, downlinkAddr, tag); err != nil {
			fmt.Fprintf(out, "  -%-30s ERROR: %v\r\n", tag, err)
			continue
		}
		fmt.Fprintf(out, "  -%-30s unsubscribed\r\n", tag)
	}
}

func tryBareAreaFixTagLine(line string, confStore *conferences.Store, networkName string, nd *NetworkDef) (add, remove string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") ||
		strings.HasPrefix(line, "=") || strings.HasPrefix(line, "%") {
		return "", "", false
	}
	if strings.Contains(line, " ") {
		return "", "", false
	}
	upper := strings.ToUpper(line)
	if strings.ContainsAny(upper, "*?") {
		return "", "", false
	}
	if areaFixTagExists(confStore, networkName, nd, upper) {
		return upper, "", true
	}
	return "", "", false
}
