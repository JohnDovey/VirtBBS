package fido

import (
	"database/sql"
	"time"
)

// FreqFileStatsRow is one per-filename FREQ aggregate.
type FreqFileStatsRow struct {
	Network      string `json:"network"`
	Filename     string `json:"filename"`
	RequestCount int    `json:"request_count"`
	BytesSent    int64  `json:"bytes_sent"`
	UpdatedAt    string `json:"updated_at"`
}

// FreqNodeStatsRow is one per-requester FREQ aggregate.
type FreqNodeStatsRow struct {
	Network       string `json:"network"`
	RequesterAddr string `json:"requester_addr"`
	RequestCount  int    `json:"request_count"`
	FilesSent     int    `json:"files_sent"`
	BytesSent     int64  `json:"bytes_sent"`
	UpdatedAt     string `json:"updated_at"`
}

// FreqStatsResult holds per-file and per-node FREQ statistics.
type FreqStatsResult struct {
	Files []FreqFileStatsRow `json:"files"`
	Nodes []FreqNodeStatsRow `json:"nodes"`
}

func migrateFreqStats(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS fido_freq_file_stats (
			network TEXT NOT NULL,
			filename TEXT NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 0,
			bytes_sent INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (network, filename)
		)`,
		`CREATE TABLE IF NOT EXISTS fido_freq_node_stats (
			network TEXT NOT NULL,
			requester_addr TEXT NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 0,
			files_sent INTEGER NOT NULL DEFAULT 0,
			bytes_sent INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (network, requester_addr)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func recordFreqNodeSent(network, requester string, requests, files int, bytes int64) {
	if statsDB == nil || network == "" || requester == "" {
		return
	}
	now := time.Now().Format(time.RFC3339)
	_, _ = statsDB.Exec(`INSERT INTO fido_freq_node_stats
		(network, requester_addr, request_count, files_sent, bytes_sent, updated_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(network, requester_addr) DO UPDATE SET
		 request_count = request_count + excluded.request_count,
		 files_sent = files_sent + excluded.files_sent,
		 bytes_sent = bytes_sent + excluded.bytes_sent,
		 updated_at = excluded.updated_at`,
		network, requester, requests, files, bytes, now)
}

// RecordFreqFileQueued increments per-file FREQ delivery stats.
func RecordFreqFileQueued(network, filename string, bytes int64) {
	if statsDB == nil || network == "" || filename == "" {
		return
	}
	if bytes < 0 {
		bytes = 0
	}
	now := time.Now().Format(time.RFC3339)
	_, _ = statsDB.Exec(`INSERT INTO fido_freq_file_stats
		(network, filename, request_count, bytes_sent, updated_at)
		VALUES (?,?,1,?,?)
		ON CONFLICT(network, filename) DO UPDATE SET
		 request_count = request_count + 1,
		 bytes_sent = bytes_sent + excluded.bytes_sent,
		 updated_at = excluded.updated_at`,
		network, filename, bytes, now)
}

func recordFreqRobot(network, linkType, peerKey string, sent, recv int) {
	at := time.Now()
	nd := robotStatsDelta{freqSent: sent, freqRecv: recv}
	ld := robotLinkDelta{freqSent: sent, freqRecv: recv}
	applyRobotStats(network, at, nd)
	if linkType != "" && peerKey != "" {
		applyRobotLinkStats(network, linkType, peerKey, at, ld)
	}
}

// RecordFreqSent records an outbound FREQ netmail.
func RecordFreqSent(network, linkType, peerKey string) {
	recordFreqRobot(network, linkType, peerKey, 1, 0)
}

// RecordFreqRecv records an inbound FREQ request.
func RecordFreqRecv(network, linkType, peerKey string) {
	recordFreqRobot(network, linkType, peerKey, 0, 1)
}

// QueryFreqStats returns per-file and per-node FREQ aggregates for a network.
func QueryFreqStats(db *sql.DB, network string, limit int) (*FreqStatsResult, error) {
	if db == nil {
		return nil, sql.ErrConnDone
	}
	if limit <= 0 {
		limit = 100
	}
	res := &FreqStatsResult{
		Files: []FreqFileStatsRow{},
		Nodes: []FreqNodeStatsRow{},
	}

	fileSQL := `SELECT network, filename, request_count, bytes_sent, updated_at
		FROM fido_freq_file_stats`
	fileArgs := []any{}
	if network != "" {
		fileSQL += ` WHERE network=?`
		fileArgs = append(fileArgs, network)
	}
	fileSQL += ` ORDER BY request_count DESC, filename LIMIT ?`
	fileArgs = append(fileArgs, limit)

	rows, err := db.Query(fileSQL, fileArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r FreqFileStatsRow
		if err := rows.Scan(&r.Network, &r.Filename, &r.RequestCount, &r.BytesSent, &r.UpdatedAt); err != nil {
			return nil, err
		}
		res.Files = append(res.Files, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	nodeSQL := `SELECT network, requester_addr, request_count, files_sent, bytes_sent, updated_at
		FROM fido_freq_node_stats`
	nodeArgs := []any{}
	if network != "" {
		nodeSQL += ` WHERE network=?`
		nodeArgs = append(nodeArgs, network)
	}
	nodeSQL += ` ORDER BY request_count DESC, requester_addr LIMIT ?`
	nodeArgs = append(nodeArgs, limit)

	nrows, err := db.Query(nodeSQL, nodeArgs...)
	if err != nil {
		return nil, err
	}
	defer nrows.Close()
	for nrows.Next() {
		var r FreqNodeStatsRow
		if err := nrows.Scan(&r.Network, &r.RequesterAddr, &r.RequestCount, &r.FilesSent, &r.BytesSent, &r.UpdatedAt); err != nil {
			return nil, err
		}
		res.Nodes = append(res.Nodes, r)
	}
	return res, nrows.Err()
}
