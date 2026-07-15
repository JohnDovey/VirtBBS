// Package social provides shoutbox, chat rooms, and polls for the web UI.
package social

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

// Shout is one shoutbox entry.
type Shout struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// Room is a chat room.
type Room struct {
	ID           int64
	Name         string
	Description  string
	MinSecurity  int
	CreatedAt    string
}

// ChatMessage is one chat room message.
type ChatMessage struct {
	ID        int64  `json:"id"`
	RoomID    int64  `json:"room_id"`
	UserID    int64  `json:"user_id"`
	UserName  string `json:"user_name"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// Poll is a user poll.
type Poll struct {
	ID        int64
	Question  string
	Open      bool
	CreatedAt string
	ClosedAt  string
}

// PollOption is one answer choice.
type PollOption struct {
	ID        int64
	PollID    int64
	Label     string
	SortOrder int
	Votes     int
}

// Store wraps SQLite tables for social features.
type Store struct {
	db *sql.DB
}

// Open attaches to the shared database and applies schema.
func Open(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("social schema: %w", err)
	}
	s := &Store{db: db}
	if err := s.seedDefaultRoom(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) seedDefaultRoom() error {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM chat_rooms`).Scan(&n)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = s.db.Exec(`
		INSERT INTO chat_rooms (name, description, min_security)
		VALUES ('General', 'Open discussion for all callers', 0)`)
	return err
}

// PostShout adds a shoutbox message.
func (s *Store) PostShout(userID int64, userName, body string) (*Shout, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("empty message")
	}
	if len(body) > 500 {
		body = body[:500]
	}
	res, err := s.db.Exec(`
		INSERT INTO shoutbox (user_id, user_name, body) VALUES (?,?,?)`,
		userID, userName, body)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.getShout(id)
}

func (s *Store) getShout(id int64) (*Shout, error) {
	row := s.db.QueryRow(`SELECT id, user_id, user_name, body, created_at FROM shoutbox WHERE id=?`, id)
	sh := &Shout{}
	if err := row.Scan(&sh.ID, &sh.UserID, &sh.UserName, &sh.Body, &sh.CreatedAt); err != nil {
		return nil, err
	}
	return sh, nil
}

// ListShouts returns recent shouts, newest first.
func (s *Store) ListShouts(limit int) ([]*Shout, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, user_name, body, created_at
		FROM shoutbox ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Shout
	for rows.Next() {
		sh := &Shout{}
		if err := rows.Scan(&sh.ID, &sh.UserID, &sh.UserName, &sh.Body, &sh.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// LatestShoutID returns the highest shout id, or 0 if none.
func (s *Store) LatestShoutID() (int64, error) {
	var id sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(id) FROM shoutbox`).Scan(&id)
	if err != nil {
		return 0, err
	}
	if !id.Valid {
		return 0, nil
	}
	return id.Int64, nil
}

// ListRooms returns all chat rooms ordered by name.
func (s *Store) ListRooms() ([]*Room, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, min_security, created_at
		FROM chat_rooms ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Room
	for rows.Next() {
		rm := &Room{}
		if err := rows.Scan(&rm.ID, &rm.Name, &rm.Description, &rm.MinSecurity, &rm.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rm)
	}
	return out, rows.Err()
}

// GetRoom fetches a room by id.
func (s *Store) GetRoom(id int64) (*Room, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, min_security, created_at
		FROM chat_rooms WHERE id=?`, id)
	rm := &Room{}
	if err := row.Scan(&rm.ID, &rm.Name, &rm.Description, &rm.MinSecurity, &rm.CreatedAt); err != nil {
		return nil, err
	}
	return rm, nil
}

// PostMessage adds a chat message to a room.
func (s *Store) PostMessage(roomID, userID int64, userName, body string) (*ChatMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("empty message")
	}
	if len(body) > 2000 {
		body = body[:2000]
	}
	res, err := s.db.Exec(`
		INSERT INTO chat_messages (room_id, user_id, user_name, body) VALUES (?,?,?,?)`,
		roomID, userID, userName, body)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	row := s.db.QueryRow(`
		SELECT id, room_id, user_id, user_name, body, created_at
		FROM chat_messages WHERE id=?`, id)
	msg := &ChatMessage{}
	if err := row.Scan(&msg.ID, &msg.RoomID, &msg.UserID, &msg.UserName, &msg.Body, &msg.CreatedAt); err != nil {
		return nil, err
	}
	return msg, nil
}

// ListMessages returns recent messages for a room, oldest first for display.
func (s *Store) ListMessages(roomID int64, limit int) ([]*ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, room_id, user_id, user_name, body, created_at
		FROM chat_messages WHERE room_id=?
		ORDER BY id DESC LIMIT ?`, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rev []*ChatMessage
	for rows.Next() {
		msg := &ChatMessage{}
		if err := rows.Scan(&msg.ID, &msg.RoomID, &msg.UserID, &msg.UserName, &msg.Body, &msg.CreatedAt); err != nil {
			return nil, err
		}
		rev = append(rev, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*ChatMessage, len(rev))
	for i, m := range rev {
		out[len(rev)-1-i] = m
	}
	return out, nil
}

// CreatePoll inserts a poll with options.
func (s *Store) CreatePoll(question string, options []string) (int64, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return 0, fmt.Errorf("question required")
	}
	if len(options) < 2 {
		return 0, fmt.Errorf("at least two options required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO polls (question) VALUES (?)`, question)
	if err != nil {
		return 0, err
	}
	pollID, _ := res.LastInsertId()
	for i, opt := range options {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO poll_options (poll_id, label, sort_order) VALUES (?,?,?)`,
			pollID, opt, i); err != nil {
			return 0, err
		}
	}
	return pollID, tx.Commit()
}

// ClosePoll marks a poll closed.
func (s *Store) ClosePoll(pollID int64) error {
	_, err := s.db.Exec(`
		UPDATE polls SET open=0, closed_at=? WHERE id=?`,
		time.Now().Format("2006-01-02 15:04:05"), pollID)
	return err
}

// ListOpenPolls returns open polls.
func (s *Store) ListOpenPolls() ([]*Poll, error) {
	rows, err := s.db.Query(`
		SELECT id, question, open, created_at, COALESCE(closed_at, '')
		FROM polls WHERE open=1 ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPolls(rows)
}

// ListAllPolls returns every poll for sysop admin.
func (s *Store) ListAllPolls() ([]*Poll, error) {
	rows, err := s.db.Query(`
		SELECT id, question, open, created_at, COALESCE(closed_at, '')
		FROM polls ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPolls(rows)
}

func scanPolls(rows *sql.Rows) ([]*Poll, error) {
	var out []*Poll
	for rows.Next() {
		p := &Poll{}
		var open int
		if err := rows.Scan(&p.ID, &p.Question, &open, &p.CreatedAt, &p.ClosedAt); err != nil {
			return nil, err
		}
		p.Open = open != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPoll returns a poll by id.
func (s *Store) GetPoll(id int64) (*Poll, error) {
	row := s.db.QueryRow(`
		SELECT id, question, open, created_at, COALESCE(closed_at, '')
		FROM polls WHERE id=?`, id)
	p := &Poll{}
	var open int
	if err := row.Scan(&p.ID, &p.Question, &open, &p.CreatedAt, &p.ClosedAt); err != nil {
		return nil, err
	}
	p.Open = open != 0
	return p, nil
}

// ListPollOptions returns options with vote counts.
func (s *Store) ListPollOptions(pollID int64) ([]*PollOption, error) {
	rows, err := s.db.Query(`
		SELECT o.id, o.poll_id, o.label, o.sort_order,
		       (SELECT COUNT(*) FROM poll_votes v WHERE v.option_id=o.id) AS votes
		FROM poll_options o WHERE o.poll_id=? ORDER BY o.sort_order, o.id`, pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PollOption
	for rows.Next() {
		o := &PollOption{}
		if err := rows.Scan(&o.ID, &o.PollID, &o.Label, &o.SortOrder, &o.Votes); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// UserVoteOption returns the option id the user voted for, or 0.
func (s *Store) UserVoteOption(pollID, userID int64) (int64, error) {
	var optID int64
	err := s.db.QueryRow(`
		SELECT option_id FROM poll_votes WHERE poll_id=? AND user_id=?`,
		pollID, userID).Scan(&optID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return optID, err
}

// Vote records a user's vote (one per poll).
func (s *Store) Vote(pollID, userID, optionID int64) error {
	p, err := s.GetPoll(pollID)
	if err != nil {
		return err
	}
	if !p.Open {
		return fmt.Errorf("poll is closed")
	}
	var ok int
	err = s.db.QueryRow(`
		SELECT 1 FROM poll_options WHERE id=? AND poll_id=?`, optionID, pollID).Scan(&ok)
	if err != nil {
		return fmt.Errorf("invalid option")
	}
	_, err = s.db.Exec(`
		INSERT INTO poll_votes (poll_id, user_id, option_id) VALUES (?,?,?)
		ON CONFLICT(poll_id, user_id) DO UPDATE SET option_id=excluded.option_id`,
		pollID, userID, optionID)
	return err
}
