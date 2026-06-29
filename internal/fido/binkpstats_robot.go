package fido

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type robotStatsDelta struct {
	areafixSent, areafixRecv int
	filefixSent, filefixRecv int
	ticSent, ticRecv         int
	ticBytesSent             int64
	ticBytesRecv             int64
}

type robotLinkDelta struct {
	areafixSent, areafixRecv int
	filefixSent, filefixRecv int
	ticSent, ticRecv         int
	ticBytesSent             int64
	ticBytesRecv             int64
}

func migrateRobotStats(db *sql.DB) error {
	netCols := []string{
		`ALTER TABLE fido_binkp_stats ADD COLUMN areafix_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN areafix_recv INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN filefix_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN filefix_recv INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN tic_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN tic_recv INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN tic_bytes_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_stats ADD COLUMN tic_bytes_recv INTEGER NOT NULL DEFAULT 0`,
	}
	linkCols := []string{
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN areafix_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN areafix_recv INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN filefix_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN filefix_recv INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN tic_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN tic_recv INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN tic_bytes_sent INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE fido_binkp_link_stats ADD COLUMN tic_bytes_recv INTEGER NOT NULL DEFAULT 0`,
	}
	for _, stmt := range append(netCols, linkCols...) {
		if _, err := db.Exec(stmt); err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "duplicate column") && !strings.Contains(msg, "already exists") {
				return err
			}
		}
	}
	return nil
}

func robotDeltaEmpty(d robotStatsDelta) bool {
	return d.areafixSent == 0 && d.areafixRecv == 0 && d.filefixSent == 0 && d.filefixRecv == 0 &&
		d.ticSent == 0 && d.ticRecv == 0 && d.ticBytesSent == 0 && d.ticBytesRecv == 0
}

func robotLinkDeltaEmpty(d robotLinkDelta) bool {
	return d.areafixSent == 0 && d.areafixRecv == 0 && d.filefixSent == 0 && d.filefixRecv == 0 &&
		d.ticSent == 0 && d.ticRecv == 0 && d.ticBytesSent == 0 && d.ticBytesRecv == 0
}

func applyRobotStats(network string, at time.Time, d robotStatsDelta) {
	if statsDB == nil || network == "" || robotDeltaEmpty(d) {
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
			 areafix_sent, areafix_recv, filefix_sent, filefix_recv,
			 tic_sent, tic_recv, tic_bytes_sent, tic_bytes_recv)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(network, period, period_key) DO UPDATE SET
			 areafix_sent = areafix_sent + excluded.areafix_sent,
			 areafix_recv = areafix_recv + excluded.areafix_recv,
			 filefix_sent = filefix_sent + excluded.filefix_sent,
			 filefix_recv = filefix_recv + excluded.filefix_recv,
			 tic_sent = tic_sent + excluded.tic_sent,
			 tic_recv = tic_recv + excluded.tic_recv,
			 tic_bytes_sent = tic_bytes_sent + excluded.tic_bytes_sent,
			 tic_bytes_recv = tic_bytes_recv + excluded.tic_bytes_recv`,
			network, pk.period, pk.key,
			d.areafixSent, d.areafixRecv, d.filefixSent, d.filefixRecv,
			d.ticSent, d.ticRecv, d.ticBytesSent, d.ticBytesRecv)
	}
}

func applyRobotLinkStats(network, linkType, peerKey string, at time.Time, d robotLinkDelta) {
	if statsDB == nil || network == "" || linkType == "" || peerKey == "" || robotLinkDeltaEmpty(d) {
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
			 areafix_sent, areafix_recv, filefix_sent, filefix_recv,
			 tic_sent, tic_recv, tic_bytes_sent, tic_bytes_recv)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(network, period, period_key, link_type, peer_key) DO UPDATE SET
			 areafix_sent = areafix_sent + excluded.areafix_sent,
			 areafix_recv = areafix_recv + excluded.areafix_recv,
			 filefix_sent = filefix_sent + excluded.filefix_sent,
			 filefix_recv = filefix_recv + excluded.filefix_recv,
			 tic_sent = tic_sent + excluded.tic_sent,
			 tic_recv = tic_recv + excluded.tic_recv,
			 tic_bytes_sent = tic_bytes_sent + excluded.tic_bytes_sent,
			 tic_bytes_recv = tic_bytes_recv + excluded.tic_bytes_recv`,
			network, pk.period, pk.key, linkType, peerKey,
			d.areafixSent, d.areafixRecv, d.filefixSent, d.filefixRecv,
			d.ticSent, d.ticRecv, d.ticBytesSent, d.ticBytesRecv)
	}
}

func recordRobot(network, linkType, peerKey string, areaFixSent, areaFixRecv, fileFixSent, fileFixRecv int) {
	at := time.Now()
	nd := robotStatsDelta{
		areafixSent: areaFixSent, areafixRecv: areaFixRecv,
		filefixSent: fileFixSent, filefixRecv: fileFixRecv,
	}
	ld := robotLinkDelta{
		areafixSent: areaFixSent, areafixRecv: areaFixRecv,
		filefixSent: fileFixSent, filefixRecv: fileFixRecv,
	}
	applyRobotStats(network, at, nd)
	if linkType != "" && peerKey != "" {
		applyRobotLinkStats(network, linkType, peerKey, at, ld)
	}
}

func RecordAreaFixSent(network, linkType, peerKey string) {
	recordRobot(network, linkType, peerKey, 1, 0, 0, 0)
}

func RecordAreaFixRecv(network, linkType, peerKey string) {
	recordRobot(network, linkType, peerKey, 0, 1, 0, 0)
}

func RecordFileFixSent(network, linkType, peerKey string) {
	recordRobot(network, linkType, peerKey, 0, 0, 1, 0)
}

func RecordFileFixRecv(network, linkType, peerKey string) {
	recordRobot(network, linkType, peerKey, 0, 0, 0, 1)
}

func RecordTICRecv(network, linkType, peerKey string, payloadBytes int64) {
	recordTIC(network, linkType, peerKey, false, payloadBytes)
}

func RecordTICSent(network, linkType, peerKey string, payloadBytes int64) {
	recordTIC(network, linkType, peerKey, true, payloadBytes)
}

func recordTIC(network, linkType, peerKey string, sent bool, payloadBytes int64) {
	if payloadBytes < 0 {
		payloadBytes = 0
	}
	at := time.Now()
	nd := robotStatsDelta{}
	ld := robotLinkDelta{}
	if sent {
		nd.ticSent = 1
		nd.ticBytesSent = payloadBytes
		ld.ticSent = 1
		ld.ticBytesSent = payloadBytes
	} else {
		nd.ticRecv = 1
		nd.ticBytesRecv = payloadBytes
		ld.ticRecv = 1
		ld.ticBytesRecv = payloadBytes
	}
	applyRobotStats(network, at, nd)
	if linkType != "" && peerKey != "" {
		applyRobotLinkStats(network, linkType, peerKey, at, ld)
	}
}

func RecordBinkpTransferredTICs(network, linkType, peerKey, dir string, basenames []string, sent bool) {
	if network == "" || dir == "" || len(basenames) == 0 {
		return
	}
	var totalBytes int64
	var count int
	seenPayload := map[string]bool{}
	for _, base := range basenames {
		path := filepath.Join(dir, base)
		if strings.EqualFold(filepath.Ext(base), ".tic") {
			count++
			totalBytes += fileSizeBytes(path)
			if payload := ticPayloadPath(path); payload != "" {
				seenPayload[filepath.Base(payload)] = true
				totalBytes += fileSizeBytes(payload)
			}
		}
	}
	if count == 0 {
		return
	}
	at := time.Now()
	nd := robotStatsDelta{}
	ld := robotLinkDelta{}
	if sent {
		nd.ticSent = count
		nd.ticBytesSent = totalBytes
		ld.ticSent = count
		ld.ticBytesSent = totalBytes
	} else {
		nd.ticRecv = count
		nd.ticBytesRecv = totalBytes
		ld.ticRecv = count
		ld.ticBytesRecv = totalBytes
	}
	applyRobotStats(network, at, nd)
	if linkType != "" && peerKey != "" {
		applyRobotLinkStats(network, linkType, peerKey, at, ld)
	}
}

func fileSizeBytes(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func LinkTypeForAddr(nd *NetworkDef, addr Addr) (linkType, peerKey string) {
	if nd == nil || addr == (Addr{}) {
		return "", ""
	}
	peerKey = addr.String()
	if uplink := nd.UplinkAddr(); uplink == addr {
		return "uplink", peerKey
	}
	return "downlink", peerKey
}

func FormatTICMegabytes(bytes int64) string {
	if bytes <= 0 {
		return "0.00"
	}
	return fmt.Sprintf("%.2f", float64(bytes)/(1024*1024))
}
