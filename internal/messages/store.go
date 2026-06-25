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
//   v0.0.1  2026-06-24  Initial implementation
// ============================================================================

// Package messages manages the VirtBBS message base.
package messages

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Message represents a single BBS message.
type Message struct {
	ID           int64
	ConferenceID int
	MsgNumber    int
	FromName     string
	ToName       string
	Subject      string
	DatePosted   time.Time
	Status       string
	Echo         bool
	Body         string
}

// Store manages messages in SQLite.
type Store struct {
	db *sql.DB
}

// Open opens the database and applies the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("messages schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB returns the underlying *sql.DB for packages that need direct access
// (e.g. fido nodelist operations that share the same database file).
func (s *Store) DB() *sql.DB { return s.db }

// PostWithNumber inserts a message preserving its existing MsgNumber.
// Used by importers that need to retain original PCBoard message numbers.
// On conflict (duplicate msg_number in the same conference) the message is skipped.
func (s *Store) PostWithNumber(m *Message) error {
	if m.DatePosted.IsZero() {
		m.DatePosted = time.Now()
	}
	res, err := s.db.Exec(`
		INSERT OR IGNORE INTO messages
		  (conference_id, msg_number, from_name, to_name, subject, date_posted, status, echo, body)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		m.ConferenceID, m.MsgNumber, m.FromName, m.ToName, m.Subject,
		m.DatePosted.Format(time.RFC3339), m.Status, boolInt(m.Echo), m.Body,
	)
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

// Post inserts a new message into the given conference.
func (s *Store) Post(m *Message) error {
	var nextNum int
	row := s.db.QueryRow(`SELECT COALESCE(MAX(msg_number),0)+1 FROM messages WHERE conference_id=?`, m.ConferenceID)
	if err := row.Scan(&nextNum); err != nil {
		return err
	}
	m.MsgNumber = nextNum
	if m.DatePosted.IsZero() {
		m.DatePosted = time.Now()
	}
	res, err := s.db.Exec(`
		INSERT INTO messages (conference_id, msg_number, from_name, to_name, subject, date_posted, status, echo, body)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		m.ConferenceID, m.MsgNumber, m.FromName, m.ToName, m.Subject,
		m.DatePosted.Format(time.RFC3339), m.Status, boolInt(m.Echo), m.Body,
	)
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

// List returns messages in a conference, newest first.
func (s *Store) List(conferenceID, limit, offset int) ([]*Message, error) {
	rows, err := s.db.Query(`
		SELECT id, conference_id, msg_number, from_name, to_name, subject, date_posted, status, echo, body
		FROM messages WHERE conference_id=? AND status!='D'
		ORDER BY msg_number DESC LIMIT ? OFFSET ?`,
		conferenceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

// ListFrom returns messages in a conference with msg_number >= startNum, oldest first.
func (s *Store) ListFrom(conferenceID, startNum, limit int) ([]*Message, error) {
	rows, err := s.db.Query(`
		SELECT id, conference_id, msg_number, from_name, to_name, subject, date_posted, status, echo, body
		FROM messages WHERE conference_id=? AND msg_number>=? AND status!='D'
		ORDER BY msg_number ASC LIMIT ?`,
		conferenceID, startNum, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

// Get fetches a single message by conference + number.
func (s *Store) Get(conferenceID, msgNumber int) (*Message, error) {
	row := s.db.QueryRow(`
		SELECT id, conference_id, msg_number, from_name, to_name, subject, date_posted, status, echo, body
		FROM messages WHERE conference_id=? AND msg_number=?`,
		conferenceID, msgNumber)
	return scanMessage(row)
}

// Delete marks a message as deleted.
func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`UPDATE messages SET status='D' WHERE id=?`, id)
	return err
}

// ListEcho returns echo-flagged messages in a conference, oldest first.
// Used by the FidoNet scanner when building outbound packets.
func (s *Store) ListEcho(conferenceID, limit, offset int) ([]*Message, error) {
	rows, err := s.db.Query(`
		SELECT id, conference_id, msg_number, from_name, to_name, subject, date_posted, status, echo, body
		FROM messages WHERE conference_id=? AND echo=1 AND status!='D'
		ORDER BY msg_number ASC LIMIT ? OFFSET ?`,
		conferenceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}

// HighMsgNumber returns the highest message number in a conference.
func (s *Store) HighMsgNumber(conferenceID int) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(msg_number),0) FROM messages WHERE conference_id=?`, conferenceID).Scan(&n)
	return n, err
}

type scanner interface{ Scan(...any) error }

func scanMessage(sc scanner) (*Message, error) {
	var m Message
	var dateStr string
	var echo int
	err := sc.Scan(&m.ID, &m.ConferenceID, &m.MsgNumber, &m.FromName, &m.ToName,
		&m.Subject, &dateStr, &m.Status, &echo, &m.Body)
	if err != nil {
		return nil, err
	}
	m.DatePosted, _ = time.Parse(time.RFC3339, dateStr)
	m.Echo = echo != 0
	return &m, nil
}

func scanMessages(rows *sql.Rows) ([]*Message, error) {
	var out []*Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
