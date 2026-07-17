// Package store provides SQLite persistence for FidoNet nodelist entries.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Node is a single addressable system from the nodelist.
type Node struct {
	Domain   string
	NodeNo   string // e.g. 1:229/426
	Zone     int
	Net      int
	Node     int
	Role     string // Zone, Region, Host, Hub, Node, Pvt, Hold, Down
	BBSName  string
	Location string
	Sysop    string
	Phone    string
	MaxBaud  string
	Flags    string
	NodeDay  int
	Updated  time.Time
}

// Stats summarizes the current database.
type Stats struct {
	Total    int
	ByRole   map[string]int
	NodeDay  int
	Updated  time.Time
	HasData  bool
}

// Store wraps the SQLite database.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens (or creates) the database and runs migrations.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;`); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Close closes the database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS nodes (
	nodeno    TEXT PRIMARY KEY COLLATE NOCASE,
	domain    TEXT NOT NULL DEFAULT 'FidoNet',
	zone      INTEGER NOT NULL DEFAULT 0,
	net       INTEGER NOT NULL DEFAULT 0,
	node      INTEGER NOT NULL DEFAULT 0,
	role      TEXT NOT NULL DEFAULT 'Node',
	bbsname   TEXT NOT NULL DEFAULT '',
	location  TEXT NOT NULL DEFAULT '',
	sysop     TEXT NOT NULL DEFAULT '',
	phone     TEXT NOT NULL DEFAULT '',
	maxbaud   TEXT NOT NULL DEFAULT '',
	flags     TEXT NOT NULL DEFAULT '',
	nodeday   INTEGER NOT NULL DEFAULT 0,
	updated   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_nodes_zone_net ON nodes(zone, net);
CREATE INDEX IF NOT EXISTS idx_nodes_role ON nodes(role);
CREATE INDEX IF NOT EXISTS idx_nodes_sysop ON nodes(sysop);
CREATE INDEX IF NOT EXISTS idx_nodes_bbsname ON nodes(bbsname);
CREATE INDEX IF NOT EXISTS idx_nodes_location ON nodes(location);
`)
	return err
}

// ReplaceAll replaces the entire nodes table with the provided set (one transaction).
func (s *Store) ReplaceAll(nodes []Node) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM nodes`); err != nil {
		return fmt.Errorf("clear nodes: %w", err)
	}

	stmt, err := tx.Prepare(`
INSERT INTO nodes (
	nodeno, domain, zone, net, node, role,
	bbsname, location, sysop, phone, maxbaud, flags, nodeday, updated
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, n := range nodes {
		updated := now
		if !n.Updated.IsZero() {
			updated = n.Updated.UTC().Format(time.RFC3339)
		}
		if _, err := stmt.Exec(
			n.NodeNo, n.Domain, n.Zone, n.Net, n.Node, n.Role,
			n.BBSName, n.Location, n.Sysop, n.Phone, n.MaxBaud, n.Flags,
			n.NodeDay, updated,
		); err != nil {
			return fmt.Errorf("insert %s: %w", n.NodeNo, err)
		}
	}
	return tx.Commit()
}

// List returns nodes ordered by zone, net, node. filter is applied as a
// case-insensitive substring match against address, name, location, sysop, flags.
func (s *Store) List(filter string, limit int) ([]Node, error) {
	if limit <= 0 {
		limit = 50000
	}
	q := `
SELECT nodeno, domain, zone, net, node, role,
       bbsname, location, sysop, phone, maxbaud, flags, nodeday, updated
FROM nodes`
	var args []any
	filter = strings.TrimSpace(filter)
	if filter != "" {
		like := "%" + filter + "%"
		q += ` WHERE
			nodeno LIKE ? COLLATE NOCASE OR
			bbsname LIKE ? COLLATE NOCASE OR
			location LIKE ? COLLATE NOCASE OR
			sysop LIKE ? COLLATE NOCASE OR
			flags LIKE ? COLLATE NOCASE OR
			role LIKE ? COLLATE NOCASE OR
			phone LIKE ? COLLATE NOCASE OR
			domain LIKE ? COLLATE NOCASE`
		args = append(args, like, like, like, like, like, like, like, like)
	}
	q += ` ORDER BY zone, net, node LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

// Get returns a single node by address (nodeno).
func (s *Store) Get(nodeno string) (Node, error) {
	row := s.db.QueryRow(`
SELECT nodeno, domain, zone, net, node, role,
       bbsname, location, sysop, phone, maxbaud, flags, nodeday, updated
FROM nodes WHERE nodeno = ? COLLATE NOCASE`, nodeno)
	return scanNode(row)
}

// Count returns total rows, optionally filtered.
func (s *Store) Count(filter string) (int, error) {
	q := `SELECT COUNT(*) FROM nodes`
	var args []any
	filter = strings.TrimSpace(filter)
	if filter != "" {
		like := "%" + filter + "%"
		q += ` WHERE
			nodeno LIKE ? COLLATE NOCASE OR
			bbsname LIKE ? COLLATE NOCASE OR
			location LIKE ? COLLATE NOCASE OR
			sysop LIKE ? COLLATE NOCASE OR
			flags LIKE ? COLLATE NOCASE OR
			role LIKE ? COLLATE NOCASE OR
			phone LIKE ? COLLATE NOCASE OR
			domain LIKE ? COLLATE NOCASE`
		args = append(args, like, like, like, like, like, like, like, like)
	}
	var n int
	err := s.db.QueryRow(q, args...).Scan(&n)
	return n, err
}

// Stats returns aggregate information about the database.
func (s *Store) Stats() (Stats, error) {
	st := Stats{ByRole: map[string]int{}}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&total); err != nil {
		return st, err
	}
	st.Total = total
	st.HasData = total > 0

	rows, err := s.db.Query(`SELECT role, COUNT(*) FROM nodes GROUP BY role`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		var c int
		if err := rows.Scan(&role, &c); err != nil {
			return st, err
		}
		st.ByRole[role] = c
	}
	if err := rows.Err(); err != nil {
		return st, err
	}

	var day sql.NullInt64
	var updated sql.NullString
	_ = s.db.QueryRow(`SELECT nodeday, updated FROM nodes ORDER BY updated DESC LIMIT 1`).Scan(&day, &updated)
	if day.Valid {
		st.NodeDay = int(day.Int64)
	}
	if updated.Valid && updated.String != "" {
		if t, err := time.Parse(time.RFC3339, updated.String); err == nil {
			st.Updated = t
		}
	}
	return st, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanNode(row scannable) (Node, error) {
	var n Node
	var updated string
	err := row.Scan(
		&n.NodeNo, &n.Domain, &n.Zone, &n.Net, &n.Node, &n.Role,
		&n.BBSName, &n.Location, &n.Sysop, &n.Phone, &n.MaxBaud, &n.Flags,
		&n.NodeDay, &updated,
	)
	if err != nil {
		return n, err
	}
	if updated != "" {
		if t, e := time.Parse(time.RFC3339, updated); e == nil {
			n.Updated = t
		}
	}
	return n, nil
}

func scanNodes(rows *sql.Rows) ([]Node, error) {
	var out []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
