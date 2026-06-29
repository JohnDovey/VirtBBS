package fido

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	statsDB   *sql.DB
	statsDBMu sync.Mutex
)

// BinkpStatsRow holds aggregated counters for one network and period.
type BinkpStatsRow struct {
	Network                 string `json:"network"`
	Period                  string `json:"period"`
	PeriodKey               string `json:"period_key"`
	PollClientOK            int    `json:"poll_client_ok"`
	PollClientFail          int    `json:"poll_client_fail"`
	PollClientFilesSent     int    `json:"poll_client_files_sent"`
	PollClientFilesRecv     int    `json:"poll_client_files_recv"`
	PollServerUplinkOK      int    `json:"poll_server_uplink_ok"`
	PollServerUplinkFail    int    `json:"poll_server_uplink_fail"`
	PollServerUplinkSent    int    `json:"poll_server_uplink_sent"`
	PollServerUplinkRecv    int    `json:"poll_server_uplink_recv"`
	PollServerDownlinkOK    int    `json:"poll_server_downlink_ok"`
	PollServerDownlinkFail  int    `json:"poll_server_downlink_fail"`
	PollServerDownlinkSent  int    `json:"poll_server_downlink_sent"`
	PollServerDownlinkRecv  int    `json:"poll_server_downlink_recv"`
	NetmailRecv             int    `json:"netmail_recv"`
	EchomailRecv            int    `json:"echomail_recv"`
	NetmailSent             int    `json:"netmail_sent"`
	EchomailSent            int    `json:"echomail_sent"`
	TossImported              int    `json:"toss_imported"`
	TossSkipped               int    `json:"toss_skipped"`
	TossSkippedDuplicate      int    `json:"toss_skipped_duplicate"`
	TossSkippedHoldFailed     int    `json:"toss_skipped_hold_failed"`
	TossSkippedInsertFailed   int    `json:"toss_skipped_insert_failed"`
	TossHeld                  int    `json:"toss_held"`
	TossPackets             int    `json:"toss_packets"`
	SessionErrors           int    `json:"session_errors"`
	AreaFixSent             int    `json:"areafix_sent"`
	AreaFixRecv             int    `json:"areafix_recv"`
	FileFixSent             int    `json:"filefix_sent"`
	FileFixRecv             int    `json:"filefix_recv"`
	TICSent                 int    `json:"tic_sent"`
	TICRecv                 int    `json:"tic_recv"`
	TICBytesSent            int64  `json:"tic_bytes_sent"`
	TICBytesRecv            int64  `json:"tic_bytes_recv"`
}

// BinkpLinkStatsRow holds per-peer counters.
type BinkpLinkStatsRow struct {
	Network      string `json:"network"`
	Period       string `json:"period"`
	PeriodKey    string `json:"period_key"`
	LinkType     string `json:"link_type"`
	PeerKey      string `json:"peer_key"`
	PollOK       int    `json:"poll_ok"`
	PollFail     int    `json:"poll_fail"`
	FilesSent    int    `json:"files_sent"`
	FilesRecv    int    `json:"files_recv"`
	NetmailSent  int    `json:"netmail_sent"`
	EchomailSent int    `json:"echomail_sent"`
	NetmailRecv  int    `json:"netmail_recv"`
	EchomailRecv int    `json:"echomail_recv"`
	AreaFixSent  int    `json:"areafix_sent"`
	AreaFixRecv  int    `json:"areafix_recv"`
	FileFixSent  int    `json:"filefix_sent"`
	FileFixRecv  int    `json:"filefix_recv"`
	TICSent      int    `json:"tic_sent"`
	TICRecv      int    `json:"tic_recv"`
	TICBytesSent int64  `json:"tic_bytes_sent"`
	TICBytesRecv int64  `json:"tic_bytes_recv"`
}

// BinkpStatsQueryResult is returned by the management API.
type BinkpStatsQueryResult struct {
	Period    string              `json:"period"`
	PeriodKey string              `json:"period_key"`
	Networks  []BinkpStatsRow     `json:"networks"`
	Links     []BinkpLinkStatsRow `json:"links"`
}

type statsDelta struct {
	pollClientOK, pollClientFail           int
	pollClientSent, pollClientRecv         int
	pollSrvUplinkOK, pollSrvUplinkFail     int
	pollSrvUplinkSent, pollSrvUplinkRecv   int
	pollSrvDownOK, pollSrvDownFail         int
	pollSrvDownSent, pollSrvDownRecv       int
	netmailRecv, echomailRecv              int
	netmailSent, echomailSent              int
	tossImported, tossSkipped, tossHeld              int
	tossSkippedDuplicate, tossSkippedHoldFailed      int
	tossSkippedInsertFailed                            int
	tossPackets, sessionErrors                         int
}

type linkDelta struct {
	pollOK, pollFail           int
	filesSent, filesRecv       int
	netmailSent, echomailSent  int
	netmailRecv, echomailRecv  int
}

// InitBinkpStats attaches the shared database used for counter storage.
func InitBinkpStats(db *sql.DB) {
	statsDBMu.Lock()
	statsDB = db
	statsDBMu.Unlock()
	if db != nil {
		_ = migrateBinkpStats(db)
		_ = migrateRobotStats(db)
	}
}

func migrateBinkpStats(db *sql.DB) error {
	alters := []string{
		`ALTER TABLE fido_binkp_stats ADD COLUMN toss_skipped_duplicate INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN toss_skipped_hold_failed INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN toss_skipped_insert_failed INTEGER NOT NULL DEFAULT 0`,
	}
	for _, stmt := range alters {
		if _, err := db.Exec(stmt); err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "duplicate column") && !strings.Contains(msg, "already exists") {
				return err
			}
		}
	}
	return nil
}

// SkippedBreakdown returns a human-readable skip reason summary for stats rows.
func (r BinkpStatsRow) SkippedBreakdown() string {
	if r.TossSkipped == 0 {
		return ""
	}
	return formatSkippedBreakdown(r.TossSkippedDuplicate, r.TossSkippedHoldFailed, r.TossSkippedInsertFailed)
}

func periodKeys(at time.Time) (day, month, year string) {
	return at.Format("2006-01-02"), at.Format("2006-01"), at.Format("2006")
}

func applyStats(network string, at time.Time, d statsDelta) {
	if statsDB == nil || network == "" {
		return
	}
	day, month, year := periodKeys(at)
	keys := []struct {
		period, key string
	}{
		{"day", day},
		{"month", month},
		{"year", year},
		{"all", ""},
	}
	for _, pk := range keys {
		_, _ = statsDB.Exec(`INSERT INTO fido_binkp_stats
			(network, period, period_key,
			 poll_client_ok, poll_client_fail, poll_client_files_sent, poll_client_files_recv,
			 poll_server_uplink_ok, poll_server_uplink_fail, poll_server_uplink_sent, poll_server_uplink_recv,
			 poll_server_downlink_ok, poll_server_downlink_fail, poll_server_downlink_sent, poll_server_downlink_recv,
			 netmail_recv, echomail_recv, netmail_sent, echomail_sent,
			 toss_imported, toss_skipped, toss_skipped_duplicate, toss_skipped_hold_failed, toss_skipped_insert_failed,
			 toss_held, toss_packets, session_errors)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(network, period, period_key) DO UPDATE SET
			 poll_client_ok = poll_client_ok + excluded.poll_client_ok,
			 poll_client_fail = poll_client_fail + excluded.poll_client_fail,
			 poll_client_files_sent = poll_client_files_sent + excluded.poll_client_files_sent,
			 poll_client_files_recv = poll_client_files_recv + excluded.poll_client_files_recv,
			 poll_server_uplink_ok = poll_server_uplink_ok + excluded.poll_server_uplink_ok,
			 poll_server_uplink_fail = poll_server_uplink_fail + excluded.poll_server_uplink_fail,
			 poll_server_uplink_sent = poll_server_uplink_sent + excluded.poll_server_uplink_sent,
			 poll_server_uplink_recv = poll_server_uplink_recv + excluded.poll_server_uplink_recv,
			 poll_server_downlink_ok = poll_server_downlink_ok + excluded.poll_server_downlink_ok,
			 poll_server_downlink_fail = poll_server_downlink_fail + excluded.poll_server_downlink_fail,
			 poll_server_downlink_sent = poll_server_downlink_sent + excluded.poll_server_downlink_sent,
			 poll_server_downlink_recv = poll_server_downlink_recv + excluded.poll_server_downlink_recv,
			 netmail_recv = netmail_recv + excluded.netmail_recv,
			 echomail_recv = echomail_recv + excluded.echomail_recv,
			 netmail_sent = netmail_sent + excluded.netmail_sent,
			 echomail_sent = echomail_sent + excluded.echomail_sent,
			 toss_imported = toss_imported + excluded.toss_imported,
			 toss_skipped = toss_skipped + excluded.toss_skipped,
			 toss_skipped_duplicate = toss_skipped_duplicate + excluded.toss_skipped_duplicate,
			 toss_skipped_hold_failed = toss_skipped_hold_failed + excluded.toss_skipped_hold_failed,
			 toss_skipped_insert_failed = toss_skipped_insert_failed + excluded.toss_skipped_insert_failed,
			 toss_held = toss_held + excluded.toss_held,
			 toss_packets = toss_packets + excluded.toss_packets,
			 session_errors = session_errors + excluded.session_errors`,
			network, pk.period, pk.key,
			d.pollClientOK, d.pollClientFail, d.pollClientSent, d.pollClientRecv,
			d.pollSrvUplinkOK, d.pollSrvUplinkFail, d.pollSrvUplinkSent, d.pollSrvUplinkRecv,
			d.pollSrvDownOK, d.pollSrvDownFail, d.pollSrvDownSent, d.pollSrvDownRecv,
			d.netmailRecv, d.echomailRecv, d.netmailSent, d.echomailSent,
			d.tossImported, d.tossSkipped, d.tossSkippedDuplicate, d.tossSkippedHoldFailed, d.tossSkippedInsertFailed,
			d.tossHeld, d.tossPackets, d.sessionErrors)
	}
}

func applyLinkStats(network, linkType, peerKey string, at time.Time, d linkDelta) {
	if statsDB == nil || network == "" || linkType == "" || peerKey == "" {
		return
	}
	day, month, year := periodKeys(at)
	keys := []struct {
		period, key string
	}{
		{"day", day},
		{"month", month},
		{"year", year},
		{"all", ""},
	}
	for _, pk := range keys {
		_, _ = statsDB.Exec(`INSERT INTO fido_binkp_link_stats
			(network, period, period_key, link_type, peer_key,
			 poll_ok, poll_fail, files_sent, files_recv,
			 netmail_sent, echomail_sent, netmail_recv, echomail_recv)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(network, period, period_key, link_type, peer_key) DO UPDATE SET
			 poll_ok = poll_ok + excluded.poll_ok,
			 poll_fail = poll_fail + excluded.poll_fail,
			 files_sent = files_sent + excluded.files_sent,
			 files_recv = files_recv + excluded.files_recv,
			 netmail_sent = netmail_sent + excluded.netmail_sent,
			 echomail_sent = echomail_sent + excluded.echomail_sent,
			 netmail_recv = netmail_recv + excluded.netmail_recv,
			 echomail_recv = echomail_recv + excluded.echomail_recv`,
			network, pk.period, pk.key, linkType, peerKey,
			d.pollOK, d.pollFail, d.filesSent, d.filesRecv,
			d.netmailSent, d.echomailSent, d.netmailRecv, d.echomailRecv)
	}
}

// RecordClientPoll records an outbound poll to the network uplink.
func RecordClientPoll(network, uplinkAddr string, ok bool, filesSent, filesRecv int) {
	at := time.Now()
	d := statsDelta{pollClientSent: filesSent, pollClientRecv: filesRecv}
	if ok {
		d.pollClientOK = 1
	} else {
		d.pollClientFail = 1
	}
	applyStats(network, at, d)
	if uplinkAddr != "" {
		ld := linkDelta{filesSent: filesSent, filesRecv: filesRecv}
		if ok {
			ld.pollOK = 1
		} else {
			ld.pollFail = 1
		}
		applyLinkStats(network, "uplink", uplinkAddr, at, ld)
	}
}

// RecordServerSession records an inbound BinkP session from uplink or downlink.
func RecordServerSession(network, linkType, peerKey string, ok bool, filesSent, filesRecv int) {
	at := time.Now()
	d := statsDelta{}
	switch linkType {
	case "uplink":
		if ok {
			d.pollSrvUplinkOK = 1
		} else {
			d.pollSrvUplinkFail = 1
		}
		d.pollSrvUplinkSent, d.pollSrvUplinkRecv = filesSent, filesRecv
	case "downlink":
		if ok {
			d.pollSrvDownOK = 1
		} else {
			d.pollSrvDownFail = 1
		}
		d.pollSrvDownSent, d.pollSrvDownRecv = filesSent, filesRecv
	}
	applyStats(network, at, d)
	if peerKey != "" {
		ld := linkDelta{filesSent: filesSent, filesRecv: filesRecv}
		if ok {
			ld.pollOK = 1
		} else {
			ld.pollFail = 1
		}
		applyLinkStats(network, linkType, peerKey, at, ld)
	}
}

// RecordSessionError increments the session error counter for a network.
func RecordSessionError(network string) {
	applyStats(network, time.Now(), statsDelta{sessionErrors: 1})
}

// RecordToss records toss outcomes including netmail/echomail breakdown.
func RecordToss(network string, tr *TossResult) {
	if tr == nil {
		return
	}
	applyStats(network, time.Now(), statsDelta{
		netmailRecv:             tr.NetmailImported,
		echomailRecv:            tr.EchomailImported,
		tossImported:            tr.Imported,
		tossSkipped:             tr.Skipped,
		tossSkippedDuplicate:    tr.SkippedDuplicate,
		tossSkippedHoldFailed:   tr.SkippedHoldFailed,
		tossSkippedInsertFailed: tr.SkippedInsertFailed,
		tossHeld:                tr.Orphaned,
		tossPackets:             tr.Packets,
	})
}

// RecordScan records outbound echomail exported by scan.
func RecordScan(network string, scanned int) {
	if scanned <= 0 {
		return
	}
	applyStats(network, time.Now(), statsDelta{echomailSent: scanned})
}

// RecordNetmailSent records outbound netmail queued to outbound.
func RecordNetmailSent(network, destAddr string, count int) {
	if count <= 0 {
		return
	}
	at := time.Now()
	applyStats(network, at, statsDelta{netmailSent: count})
	if destAddr != "" {
		applyLinkStats(network, "uplink", destAddr, at, linkDelta{netmailSent: count})
	}
}

// QueryBinkpStats returns stats for one or all networks for the given period.
func QueryBinkpStats(db *sql.DB, network, period, periodKey string) (*BinkpStatsQueryResult, error) {
	if db == nil {
		return nil, fmt.Errorf("stats database not available")
	}
	if period == "" {
		period = "day"
	}
	if periodKey == "" {
		periodKey = statsPeriodKey(period, time.Now())
	}
	res := &BinkpStatsQueryResult{
		Period:    period,
		PeriodKey: periodKey,
		Networks:  []BinkpStatsRow{},
		Links:     []BinkpLinkStatsRow{},
	}

	netSQL := `SELECT network, period, period_key,
		poll_client_ok, poll_client_fail, poll_client_files_sent, poll_client_files_recv,
		poll_server_uplink_ok, poll_server_uplink_fail, poll_server_uplink_sent, poll_server_uplink_recv,
		poll_server_downlink_ok, poll_server_downlink_fail, poll_server_downlink_sent, poll_server_downlink_recv,
		netmail_recv, echomail_recv, netmail_sent, echomail_sent,
		toss_imported, toss_skipped, toss_skipped_duplicate, toss_skipped_hold_failed, toss_skipped_insert_failed,
		toss_held, toss_packets, session_errors,
		areafix_sent, areafix_recv, filefix_sent, filefix_recv,
		tic_sent, tic_recv, tic_bytes_sent, tic_bytes_recv
		FROM fido_binkp_stats WHERE period=? AND period_key=?`
	args := []any{period, periodKey}
	if network != "" {
		netSQL += ` AND network=?`
		args = append(args, network)
	}
	netSQL += ` ORDER BY network`

	rows, err := db.Query(netSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r BinkpStatsRow
		if err := rows.Scan(&r.Network, &r.Period, &r.PeriodKey,
			&r.PollClientOK, &r.PollClientFail, &r.PollClientFilesSent, &r.PollClientFilesRecv,
			&r.PollServerUplinkOK, &r.PollServerUplinkFail, &r.PollServerUplinkSent, &r.PollServerUplinkRecv,
			&r.PollServerDownlinkOK, &r.PollServerDownlinkFail, &r.PollServerDownlinkSent, &r.PollServerDownlinkRecv,
			&r.NetmailRecv, &r.EchomailRecv, &r.NetmailSent, &r.EchomailSent,
			&r.TossImported, &r.TossSkipped,
			&r.TossSkippedDuplicate, &r.TossSkippedHoldFailed, &r.TossSkippedInsertFailed,
			&r.TossHeld, &r.TossPackets, &r.SessionErrors,
			&r.AreaFixSent, &r.AreaFixRecv, &r.FileFixSent, &r.FileFixRecv,
			&r.TICSent, &r.TICRecv, &r.TICBytesSent, &r.TICBytesRecv); err != nil {
			return nil, err
		}
		res.Networks = append(res.Networks, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	linkSQL := `SELECT network, period, period_key, link_type, peer_key,
		poll_ok, poll_fail, files_sent, files_recv,
		netmail_sent, echomail_sent, netmail_recv, echomail_recv,
		areafix_sent, areafix_recv, filefix_sent, filefix_recv,
		tic_sent, tic_recv, tic_bytes_sent, tic_bytes_recv
		FROM fido_binkp_link_stats WHERE period=? AND period_key=?`
	linkArgs := []any{period, periodKey}
	if network != "" {
		linkSQL += ` AND network=?`
		linkArgs = append(linkArgs, network)
	}
	linkSQL += ` ORDER BY network, link_type, peer_key`

	lrows, err := db.Query(linkSQL, linkArgs...)
	if err != nil {
		return nil, err
	}
	defer lrows.Close()
	for lrows.Next() {
		var r BinkpLinkStatsRow
		if err := lrows.Scan(&r.Network, &r.Period, &r.PeriodKey, &r.LinkType, &r.PeerKey,
			&r.PollOK, &r.PollFail, &r.FilesSent, &r.FilesRecv,
			&r.NetmailSent, &r.EchomailSent, &r.NetmailRecv, &r.EchomailRecv,
			&r.AreaFixSent, &r.AreaFixRecv, &r.FileFixSent, &r.FileFixRecv,
			&r.TICSent, &r.TICRecv, &r.TICBytesSent, &r.TICBytesRecv); err != nil {
			return nil, err
		}
		res.Links = append(res.Links, r)
	}
	return res, lrows.Err()
}

func statsPeriodKey(period string, at time.Time) string {
	day, month, year := periodKeys(at)
	switch period {
	case "month":
		return month
	case "year":
		return year
	case "all":
		return ""
	default:
		return day
	}
}
