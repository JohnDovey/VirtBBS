package fido

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// QueryBinkpStatsForPeriod resolves period aliases and returns enriched stats.
// Supported periods: day, 24h, month, year, all.
func QueryBinkpStatsForPeriod(db *sql.DB, network, period string, at time.Time) (*BinkpStatsQueryResult, error) {
	if db == nil {
		return nil, fmt.Errorf("stats database not available")
	}
	if period == "" {
		period = "day"
	}
	var res *BinkpStatsQueryResult
	var err error
	switch period {
	case "24h":
		res, err = queryBinkpStatsRollingDays(db, network, at, 2)
		res.Period = "24h"
		res.PeriodKey = at.Format("2006-01-02")
	case "day":
		res, err = QueryBinkpStats(db, network, "day", at.Format("2006-01-02"))
	case "month":
		res, err = QueryBinkpStats(db, network, "month", at.Format("2006-01"))
	case "year":
		res, err = QueryBinkpStats(db, network, "year", at.Format("2006"))
	case "all":
		res, err = QueryBinkpStats(db, network, "all", "")
	default:
		return nil, fmt.Errorf("unknown stats period %q", period)
	}
	if err != nil {
		return nil, err
	}
	enrichNetworkStatsFromLinks(res)
	return res, nil
}

func queryBinkpStatsRollingDays(db *sql.DB, network string, at time.Time, days int) (*BinkpStatsQueryResult, error) {
	if days < 1 {
		days = 1
	}
	var merged *BinkpStatsQueryResult
	for i := 0; i < days; i++ {
		day := at.AddDate(0, 0, -i).Format("2006-01-02")
		part, err := QueryBinkpStats(db, network, "day", day)
		if err != nil {
			return nil, err
		}
		merged = mergeBinkpStatsResults(merged, part)
	}
	if merged == nil {
		merged = &BinkpStatsQueryResult{Networks: []BinkpStatsRow{}, Links: []BinkpLinkStatsRow{}}
	}
	return merged, nil
}

func mergeBinkpStatsResults(a, b *BinkpStatsQueryResult) *BinkpStatsQueryResult {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := &BinkpStatsQueryResult{
		Period:    a.Period,
		PeriodKey: a.PeriodKey,
		Networks:  mergeNetworkRows(a.Networks, b.Networks),
		Links:     mergeLinkRows(a.Links, b.Links),
	}
	return out
}

func mergeNetworkRows(a, b []BinkpStatsRow) []BinkpStatsRow {
	byNet := map[string]BinkpStatsRow{}
	for _, r := range a {
		byNet[r.Network] = r
	}
	for _, r := range b {
		if cur, ok := byNet[r.Network]; ok {
			byNet[r.Network] = addNetworkRows(cur, r)
		} else {
			byNet[r.Network] = r
		}
	}
	return sortedNetworkRows(byNet)
}

func mergeLinkRows(a, b []BinkpLinkStatsRow) []BinkpLinkStatsRow {
	type linkKey struct {
		network, linkType, peer string
	}
	byKey := map[linkKey]BinkpLinkStatsRow{}
	for _, r := range a {
		byKey[linkKey{r.Network, r.LinkType, r.PeerKey}] = r
	}
	for _, r := range b {
		k := linkKey{r.Network, r.LinkType, r.PeerKey}
		if cur, ok := byKey[k]; ok {
			byKey[k] = addLinkRows(cur, r)
		} else {
			byKey[k] = r
		}
	}
	out := make([]BinkpLinkStatsRow, 0, len(byKey))
	for _, r := range byKey {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Network != out[j].Network {
			return out[i].Network < out[j].Network
		}
		if out[i].LinkType != out[j].LinkType {
			return out[i].LinkType < out[j].LinkType
		}
		return out[i].PeerKey < out[j].PeerKey
	})
	return out
}

func addNetworkRows(a, b BinkpStatsRow) BinkpStatsRow {
	a.PollClientOK += b.PollClientOK
	a.PollClientFail += b.PollClientFail
	a.PollClientFilesSent += b.PollClientFilesSent
	a.PollClientFilesRecv += b.PollClientFilesRecv
	a.PollServerUplinkOK += b.PollServerUplinkOK
	a.PollServerUplinkFail += b.PollServerUplinkFail
	a.PollServerUplinkSent += b.PollServerUplinkSent
	a.PollServerUplinkRecv += b.PollServerUplinkRecv
	a.PollServerDownlinkOK += b.PollServerDownlinkOK
	a.PollServerDownlinkFail += b.PollServerDownlinkFail
	a.PollServerDownlinkSent += b.PollServerDownlinkSent
	a.PollServerDownlinkRecv += b.PollServerDownlinkRecv
	a.NetmailRecv += b.NetmailRecv
	a.EchomailRecv += b.EchomailRecv
	a.NetmailSent += b.NetmailSent
	a.EchomailSent += b.EchomailSent
	a.TossImported += b.TossImported
	a.TossSkipped += b.TossSkipped
	a.TossSkippedDuplicate += b.TossSkippedDuplicate
	a.TossSkippedHoldFailed += b.TossSkippedHoldFailed
	a.TossSkippedInsertFailed += b.TossSkippedInsertFailed
	a.TossHeld += b.TossHeld
	a.TossPackets += b.TossPackets
	a.SessionErrors += b.SessionErrors
	return a
}

func addLinkRows(a, b BinkpLinkStatsRow) BinkpLinkStatsRow {
	a.PollOK += b.PollOK
	a.PollFail += b.PollFail
	a.FilesSent += b.FilesSent
	a.FilesRecv += b.FilesRecv
	a.NetmailSent += b.NetmailSent
	a.EchomailSent += b.EchomailSent
	a.NetmailRecv += b.NetmailRecv
	a.EchomailRecv += b.EchomailRecv
	return a
}

func sortedNetworkRows(byNet map[string]BinkpStatsRow) []BinkpStatsRow {
	out := make([]BinkpStatsRow, 0, len(byNet))
	for _, r := range byNet {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Network < out[j].Network })
	return out
}

// enrichNetworkStatsFromLinks fills network-level counters from per-link rows when
// the network aggregate row is missing or has zero poll counts while links do not.
func enrichNetworkStatsFromLinks(q *BinkpStatsQueryResult) {
	if q == nil {
		return
	}
	byNet := map[string]int{}
	for i, n := range q.Networks {
		byNet[n.Network] = i
	}
	for _, l := range q.Links {
		idx, ok := byNet[l.Network]
		if !ok {
			q.Networks = append(q.Networks, BinkpStatsRow{
				Network: l.Network,
				Period:  q.Period,
				PeriodKey: q.PeriodKey,
			})
			idx = len(q.Networks) - 1
			byNet[l.Network] = idx
		}
		n := &q.Networks[idx]
		switch l.LinkType {
		case "uplink":
			if n.PollClientOK+n.PollClientFail == 0 && l.PollOK+l.PollFail > 0 {
				n.PollClientOK += l.PollOK
				n.PollClientFail += l.PollFail
				n.PollClientFilesSent += l.FilesSent
				n.PollClientFilesRecv += l.FilesRecv
			}
		case "downlink":
			if n.PollServerDownlinkOK+n.PollServerDownlinkFail == 0 && l.PollOK+l.PollFail > 0 {
				n.PollServerDownlinkOK += l.PollOK
				n.PollServerDownlinkFail += l.PollFail
				n.PollServerDownlinkSent += l.FilesSent
				n.PollServerDownlinkRecv += l.FilesRecv
			}
		}
		if n.NetmailSent == 0 && l.NetmailSent > 0 {
			n.NetmailSent += l.NetmailSent
		}
		if n.NetmailRecv == 0 && l.NetmailRecv > 0 {
			n.NetmailRecv += l.NetmailRecv
		}
		if n.EchomailSent == 0 && l.EchomailSent > 0 {
			n.EchomailSent += l.EchomailSent
		}
		if n.EchomailRecv == 0 && l.EchomailRecv > 0 {
			n.EchomailRecv += l.EchomailRecv
		}
	}
	sort.Slice(q.Networks, func(i, j int) bool { return q.Networks[i].Network < q.Networks[j].Network })
}

// BinkpDailySeries holds per-day counters for charting.
type BinkpDailySeries struct {
	Labels       []string
	PollsOK      []int
	PollsFail    []int
	FilesSent    []int
	FilesRecv    []int
	NetmailSent  []int
	NetmailRecv  []int
	EchomailSent []int
	EchomailRecv []int
	TossImported []int
}

// QueryBinkpDailySeries returns daily stats for the last days entries.
func QueryBinkpDailySeries(db *sql.DB, network string, days int) (*BinkpDailySeries, error) {
	if db == nil {
		return nil, fmt.Errorf("stats database not available")
	}
	if days < 1 {
		days = 30
	}
	labels := make([]string, days)
	keys := make([]string, days)
	for i := 0; i < days; i++ {
		d := time.Now().AddDate(0, 0, -(days - 1 - i))
		keys[i] = d.Format("2006-01-02")
		labels[i] = d.Format("Jan 2")
	}
	series := &BinkpDailySeries{
		Labels:       labels,
		PollsOK:      make([]int, days),
		PollsFail:    make([]int, days),
		FilesSent:    make([]int, days),
		FilesRecv:    make([]int, days),
		NetmailSent:  make([]int, days),
		NetmailRecv:  make([]int, days),
		EchomailSent: make([]int, days),
		EchomailRecv: make([]int, days),
		TossImported: make([]int, days),
	}
	keyIndex := map[string]int{}
	for i, k := range keys {
		keyIndex[k] = i
	}

	sqlText := `SELECT period_key,
		poll_client_ok, poll_client_fail,
		poll_client_files_sent, poll_client_files_recv,
		poll_server_uplink_ok, poll_server_uplink_fail,
		poll_server_uplink_sent, poll_server_uplink_recv,
		poll_server_downlink_ok, poll_server_downlink_fail,
		poll_server_downlink_sent, poll_server_downlink_recv,
		netmail_sent, netmail_recv, echomail_sent, echomail_recv,
		toss_imported
		FROM fido_binkp_stats WHERE period='day' AND period_key >= ?`
	args := []any{keys[0]}
	if network != "" {
		sqlText += ` AND network=?`
		args = append(args, network)
	}
	rows, err := db.Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var cOK, cFail, cSent, cRecv int
		var uOK, uFail, uSent, uRecv int
		var dOK, dFail, dSent, dRecv int
		var nmSent, nmRecv, echoSent, echoRecv, toss int
		if err := rows.Scan(&key, &cOK, &cFail, &cSent, &cRecv,
			&uOK, &uFail, &uSent, &uRecv, &dOK, &dFail, &dSent, &dRecv,
			&nmSent, &nmRecv, &echoSent, &echoRecv, &toss); err != nil {
			return nil, err
		}
		i, ok := keyIndex[key]
		if !ok {
			continue
		}
		series.PollsOK[i] += cOK + uOK + dOK
		series.PollsFail[i] += cFail + uFail + dFail
		series.FilesSent[i] += cSent + uSent + dSent
		series.FilesRecv[i] += cRecv + uRecv + dRecv
		series.NetmailSent[i] += nmSent
		series.NetmailRecv[i] += nmRecv
		series.EchomailSent[i] += echoSent
		series.EchomailRecv[i] += echoRecv
		series.TossImported[i] += toss
	}
	return series, rows.Err()
}

// LinksForNetwork returns link stats for one network from a query result.
func LinksForNetwork(q *BinkpStatsQueryResult, network string) []BinkpLinkStatsRow {
	if q == nil {
		return nil
	}
	var out []BinkpLinkStatsRow
	for _, l := range q.Links {
		if l.Network == network {
			out = append(out, l)
		}
	}
	return out
}

// SanitizeChartID returns a safe HTML id fragment for a network name.
func SanitizeChartID(network string) string {
	s := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, network)
	if s == "" {
		return "net"
	}
	return s
}
