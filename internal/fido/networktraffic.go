package fido

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

const (
	NetworkTrafficConfName = "Network Traffic"
	NetworkTrafficAreaName = "Network Traffic"
	NetworkTrafficBot      = "NetworkBot"
	networkTrafficPeriod   = 7 * 24 * time.Hour
)

// EchoTrafficReport summarises echomail propagation for one echo conference.
type EchoTrafficReport struct {
	ConfID    int
	EchoTag   string
	EchoName  string
	Network   string
	Zone      int
	PeriodEnd time.Time
	MsgCount  int
	Nodes     map[string]*trafficNode
	RouteEdge map[string]int // "from|to" -> count
	SeenEdge  map[string]int // origin|seen -> count
}

type trafficNode struct {
	Addr       string
	AsOrigin   int
	AsSeen     int
	OnPath     int
	Nodelist   string // optional label from nodelist
}

func trafficEdgeKey(from, to string) string { return from + "|" + to }

// TrafficMapZipName returns [Echo tag]-[echo name]-[date]-NetworkMap.zip
func TrafficMapZipName(echoTag, echoName string, day time.Time) string {
	tag := trafficFileToken(echoTag)
	if tag == "" {
		tag = "ECHO"
	}
	name := trafficFileToken(echoName)
	if name == "" {
		name = "Area"
	}
	return fmt.Sprintf("%s-%s-%s-NetworkMap.zip", tag, name, day.Format("2006-01-02"))
}

func trafficFileToken(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String()
}

func trafficReportTitleLine(r *EchoTrafficReport) string {
	if r == nil {
		return ""
	}
	return TrafficMapZipName(r.EchoTag, r.EchoName, r.PeriodEnd)
}

func migrateNetworkTrafficState(db *sql.DB) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS fido_network_traffic_state (
		echo_conf_id INTEGER PRIMARY KEY,
		last_report_week TEXT NOT NULL
	)`)
	return err
}

func trafficWeekKey(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

func alreadyReportedThisWeek(db *sql.DB, confID int, week string) bool {
	if db == nil {
		return false
	}
	var prev string
	err := db.QueryRow(`SELECT last_report_week FROM fido_network_traffic_state WHERE echo_conf_id=?`, confID).Scan(&prev)
	if err == sql.ErrNoRows {
		return false
	}
	return err == nil && prev == week
}

func markTrafficReported(db *sql.DB, confID int, week string) {
	if db == nil {
		return
	}
	_, _ = db.Exec(`INSERT INTO fido_network_traffic_state (echo_conf_id, last_report_week)
		VALUES (?,?) ON CONFLICT(echo_conf_id) DO UPDATE SET last_report_week=excluded.last_report_week`,
		confID, week)
}

func trafficTokenToAddr(zone int, token string) (Addr, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Addr{}, false
	}
	if a, err := ParseAddr(token); err == nil {
		return a, true
	}
	net, node := splitNetNode(token)
	if net == 0 && node == 0 {
		return Addr{}, false
	}
	if zone == 0 {
		return Addr{}, false
	}
	return Addr{Zone: zone, Net: net, Node: node}, true
}

func defaultZoneForConference(conf *conferences.Conference, networks []NetworkDef) int {
	if conf == nil {
		return 0
	}
	netName := strings.TrimSpace(conf.Network)
	for _, nd := range networks {
		if netName != "" && !strings.EqualFold(nd.Name, netName) {
			continue
		}
		if a, err := ParseAddr(nd.Address); err == nil && a.Zone != 0 {
			return a.Zone
		}
	}
	for _, nd := range networks {
		if a, err := ParseAddr(nd.Address); err == nil && a.Zone != 0 {
			return a.Zone
		}
	}
	return 0
}

// CollectEchoTraffic builds a traffic report from echo messages in one conference.
func CollectEchoTraffic(msgStore *messages.Store, ndb *NodelistDB, conf *conferences.Conference, since time.Time, networks []NetworkDef) (*EchoTrafficReport, error) {
	if msgStore == nil || conf == nil {
		return nil, fmt.Errorf("store or conference required")
	}
	msgs, err := msgStore.ListConferenceEchoSince(conf.ID, since, 50000)
	if err != nil {
		return nil, err
	}
	zone := defaultZoneForConference(conf, networks)
	report := &EchoTrafficReport{
		ConfID:    conf.ID,
		EchoTag:   conf.EchoTag,
		EchoName:  conf.Name,
		Network:   conf.Network,
		Zone:      zone,
		PeriodEnd: time.Now().UTC(),
		Nodes:     map[string]*trafficNode{},
		RouteEdge: map[string]int{},
		SeenEdge:  map[string]int{},
	}
	for _, m := range msgs {
		if m == nil {
			continue
		}
		report.MsgCount++
		origin, err := ParseAddr(strings.TrimSpace(m.FidoOrigin))
		if err != nil || origin == (Addr{}) {
			continue
		}
		if origin.Zone != 0 {
			zone = origin.Zone
			report.Zone = zone
		}
		originKey := origin.String()
		report.touchNode(originKey, true, false, false, ndb, conf.Network)
		prev := originKey
		for _, tok := range strings.Fields(m.FidoPath) {
			a, ok := trafficTokenToAddr(zone, tok)
			if !ok {
				continue
			}
			key := a.String()
			report.touchNode(key, false, false, true, ndb, conf.Network)
			report.RouteEdge[trafficEdgeKey(prev, key)]++
			prev = key
		}
		for _, tok := range strings.Fields(m.FidoSeenBy) {
			a, ok := trafficTokenToAddr(zone, tok)
			if !ok {
				continue
			}
			seenKey := a.String()
			report.touchNode(seenKey, false, true, false, ndb, conf.Network)
			report.SeenEdge[trafficEdgeKey(originKey, seenKey)]++
		}
	}
	if report.MsgCount == 0 {
		return report, nil
	}
	return report, nil
}

func (r *EchoTrafficReport) touchNode(addr string, asOrigin, asSeen, onPath bool, ndb *NodelistDB, network string) {
	if addr == "" {
		return
	}
	n, ok := r.Nodes[addr]
	if !ok {
		n = &trafficNode{Addr: addr}
		r.Nodes[addr] = n
		if ndb != nil {
			if a, err := ParseAddr(addr); err == nil {
				if e, _ := ndb.LookupAddr(network, a); e != nil {
					label := strings.TrimSpace(e.Name)
					if label == "" {
						label = strings.TrimSpace(e.Sysop)
					}
					n.Nodelist = label
				} else if e := ndb.LookupAddrAny(a); e != nil {
					n.Nodelist = strings.TrimSpace(e.Name)
				}
			}
		}
	}
	if asOrigin {
		n.AsOrigin++
	}
	if asSeen {
		n.AsSeen++
	}
	if onPath {
		n.OnPath++
	}
}

func (n *trafficNode) label() string {
	if n == nil {
		return ""
	}
	parts := []string{n.Addr}
	if n.Nodelist != "" {
		parts = append(parts, n.Nodelist)
	}
	var stats []string
	if n.AsOrigin > 0 {
		stats = append(stats, fmt.Sprintf("orig %d", n.AsOrigin))
	}
	if n.AsSeen > 0 {
		stats = append(stats, fmt.Sprintf("seen %d", n.AsSeen))
	}
	if n.OnPath > 0 {
		stats = append(stats, fmt.Sprintf("path %d", n.OnPath))
	}
	if len(stats) > 0 {
		parts = append(parts, strings.Join(stats, ", "))
	}
	return strings.Join(parts, " — ")
}

func buildTrafficASCII(r *EchoTrafficReport) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	line := trafficReportTitleLine(r)
	b.WriteString(line)
	b.WriteString("\r\n")
	b.WriteString(strings.Repeat("=", len(line)))
	b.WriteString("\r\n\r\n")
	fmt.Fprintf(&b, "Echo area : %s (%s)\r\n", r.EchoTag, r.EchoName)
	if r.Network != "" {
		fmt.Fprintf(&b, "Network   : %s\r\n", r.Network)
	}
	fmt.Fprintf(&b, "Period    : last %d days ending %s\r\n", int(networkTrafficPeriod/(24*time.Hour)), r.PeriodEnd.Format("2006-01-02"))
	fmt.Fprintf(&b, "Messages  : %d\r\n\r\n", r.MsgCount)

	if r.MsgCount == 0 {
		b.WriteString("No echomail traffic recorded in this period.\r\n")
		return b.String()
	}

	b.WriteString("── Nodes ──────────────────────────────────────────────\r\n")
	addrs := make([]string, 0, len(r.Nodes))
	for a := range r.Nodes {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)
	for _, a := range addrs {
		fmt.Fprintf(&b, "  %s\r\n", r.Nodes[a].label())
	}

	b.WriteString("\r\n── Routes (PATH hops) ─────────────────────────────────\r\n")
	routeKeys := sortedEdgeKeys(r.RouteEdge)
	if len(routeKeys) == 0 {
		b.WriteString("  (no PATH data)\r\n")
	} else {
		for _, k := range routeKeys {
			from, to := splitEdgeKey(k)
			fmt.Fprintf(&b, "  %s -> %s  (%d)\r\n", from, to, r.RouteEdge[k])
		}
	}

	b.WriteString("\r\n── Seen-by coverage (origin -> node) ──────────────────\r\n")
	seenKeys := sortedEdgeKeys(r.SeenEdge)
	if len(seenKeys) == 0 {
		b.WriteString("  (no SEEN-BY data)\r\n")
	} else {
		byOrigin := map[string][]string{}
		for _, k := range seenKeys {
			from, to := splitEdgeKey(k)
			byOrigin[from] = append(byOrigin[from], fmt.Sprintf("%s (%d)", to, r.SeenEdge[k]))
		}
		origins := make([]string, 0, len(byOrigin))
		for o := range byOrigin {
			origins = append(origins, o)
		}
		sort.Strings(origins)
		for _, o := range origins {
			fmt.Fprintf(&b, "  %s\r\n", o)
			sort.Strings(byOrigin[o])
			for _, s := range byOrigin[o] {
				fmt.Fprintf(&b, "    -> %s\r\n", s)
			}
		}
	}
	return b.String()
}

func sortedEdgeKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if m[out[i]] != m[out[j]] {
			return m[out[i]] > m[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func splitEdgeKey(k string) (from, to string) {
	parts := strings.SplitN(k, "|", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return k, ""
}

func buildTrafficDOT(r *EchoTrafficReport) string {
	if r == nil || len(r.Nodes) == 0 {
		return "digraph Traffic {\n\"none\" [label=\"No traffic\"]\n}\n"
	}
	title := sanitizeNetworkFileToken(r.EchoTag)
	if title == "" {
		title = "Traffic"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %s {\r\n", dotEscape(title))
	b.WriteString("node [shape=tab, style=filled]\r\n")
	b.WriteString("edge [fontsize=10]\r\n")
	b.WriteString("rankdir=LR\r\n\r\n")

	addrs := make([]string, 0, len(r.Nodes))
	for a := range r.Nodes {
		addrs = append(addrs, a)
	}
	sort.Strings(addrs)
	for _, a := range addrs {
		n := r.Nodes[a]
		fill := "lightyellow"
		if n.AsOrigin > 0 {
			fill = "lightblue"
		}
		label := strings.ReplaceAll(n.label(), " — ", "\\n")
		label = strings.ReplaceAll(label, ", ", "\\n")
		fmt.Fprintf(&b, "\"%s\" [label=\"%s\", fillcolor=%s]\r\n", dotEscape(a), dotEscape(label), fill)
	}
	for _, k := range sortedEdgeKeys(r.RouteEdge) {
		from, to := splitEdgeKey(k)
		fmt.Fprintf(&b, "\"%s\" -> \"%s\" [label=\"path %d\", color=steelblue, penwidth=2]\r\n",
			dotEscape(from), dotEscape(to), r.RouteEdge[k])
	}
	for _, k := range sortedEdgeKeys(r.SeenEdge) {
		from, to := splitEdgeKey(k)
		fmt.Fprintf(&b, "\"%s\" -> \"%s\" [label=\"seen %d\", color=gray, style=dashed]\r\n",
			dotEscape(from), dotEscape(to), r.SeenEdge[k])
	}
	b.WriteString("}\r\n")
	return b.String()
}

func renderTrafficPNG(r *EchoTrafficReport) ([]byte, error) {
	dot := buildTrafficDOT(r)
	tmpOut, err := os.CreateTemp("", "traffic-*.png")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpOut.Name())
	tmpOut.Close()
	if err := RenderPNG(dot, tmpOut.Name()); err != nil {
		return nil, err
	}
	return os.ReadFile(tmpOut.Name())
}

// PublishEchoTrafficReport posts ASCII summary to Network Traffic and registers zip in file area.
func PublishEchoTrafficReport(msgStore *messages.Store, confStore *conferences.Store,
	fileArea FileArea, r *EchoTrafficReport) []string {
	var warnings []string
	warn := func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) }
	if r == nil || r.MsgCount == 0 {
		return warnings
	}

	trafficConf, err := EnsureConference(confStore, NetworkTrafficConfName, "")
	if err != nil {
		return []string{fmt.Sprintf("ensure Network Traffic conference: %v", err)}
	}

	titleLine := trafficReportTitleLine(r)
	body := buildTrafficASCII(r)
	subject := fmt.Sprintf("%s — %s traffic map", r.EchoTag, r.EchoName)
	if err := msgStore.Post(&messages.Message{
		ConferenceID: trafficConf.ID,
		FromName:     NetworkTrafficBot,
		ToName:       "All",
		Subject:      subject,
		Status:       "A",
		Body:         body,
	}); err != nil {
		warn("post traffic message: %v", err)
	}

	if fileArea == nil {
		warn("file area not available")
		return warnings
	}
	dirID, dirPath, err := fileArea.EnsureDir(NetworkTrafficAreaName, NetworkTrafficAreaName+" (auto-created)")
	if err != nil {
		warn("ensure file area: %v", err)
		return warnings
	}

	pngName := strings.TrimSuffix(titleLine, ".zip") + ".png"
	png, err := renderTrafficPNG(r)
	if err != nil {
		warn("render PNG: %v", err)
		return warnings
	}
	zipName := titleLine
	diz := zipName
	if err := writeMultiZipAndRegister(dirPath, dirID, fileArea, zipName, map[string][]byte{pngName: png}, diz); err != nil {
		warn("zip %s: %v", zipName, err)
	}
	return warnings
}

// RunWeeklyNetworkTrafficReports scans echo conferences and publishes weekly maps.
func RunWeeklyNetworkTrafficReports(db *sql.DB, msgStore *messages.Store, confStore *conferences.Store,
	fileArea FileArea, networks []NetworkDef) []string {
	var warnings []string
	warn := func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) }

	if err := migrateNetworkTrafficState(db); err != nil {
		return []string{err.Error()}
	}
	if msgStore == nil || confStore == nil {
		return []string{"message or conference store not available"}
	}

	week := trafficWeekKey(time.Now())
	since := time.Now().Add(-networkTrafficPeriod)
	ndb := OpenNodelistDB(db)

	echoConfs, err := confStore.ListEcho("")
	if err != nil {
		return []string{err.Error()}
	}

	for _, conf := range echoConfs {
		if conf == nil || !conf.Echo {
			continue
		}
		if strings.EqualFold(conf.Name, NetworkTrafficConfName) {
			continue
		}
		if alreadyReportedThisWeek(db, conf.ID, week) {
			continue
		}
		report, err := CollectEchoTraffic(msgStore, ndb, conf, since, networks)
		if err != nil {
			warn("%s: collect: %v", conf.Name, err)
			continue
		}
		if report.MsgCount == 0 {
			continue
		}
		for _, w := range PublishEchoTrafficReport(msgStore, confStore, fileArea, report) {
			warn("%s: %s", conf.Name, w)
		}
		markTrafficReported(db, conf.ID, week)
	}
	return warnings
}
