package mrc

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed schema.sql
var schema string

// UserPrefs are per-BBS-user MRC settings.
type UserPrefs struct {
	UserID      int64
	Handle      string
	HandleColor int
	Prefix      string
	Suffix      string
	TextColor   int
	Theme       int
	Twit        []string
}

// PrefsStore persists mrc_user_prefs.
type PrefsStore struct {
	db *sql.DB
}

// OpenPrefs applies schema and returns a prefs store.
func OpenPrefs(db *sql.DB) (*PrefsStore, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("mrc schema: %w", err)
	}
	return &PrefsStore{db: db}, nil
}

// Get loads prefs for a user, or defaults derived from defaultHandle.
func (s *PrefsStore) Get(userID int64, defaultHandle string) (*UserPrefs, error) {
	p := &UserPrefs{
		UserID:      userID,
		Handle:      SanitizeName(defaultHandle),
		HandleColor: 11,
		TextColor:   7,
		Theme:       1,
	}
	var twit string
	err := s.db.QueryRow(`
		SELECT handle, handle_color, prefix, suffix, text_color, theme, twit
		FROM mrc_user_prefs WHERE user_id = ?`, userID).Scan(
		&p.Handle, &p.HandleColor, &p.Prefix, &p.Suffix, &p.TextColor, &p.Theme, &twit)
	if err == sql.ErrNoRows {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	if p.Handle == "" {
		p.Handle = SanitizeName(defaultHandle)
	}
	p.Twit = splitCSV(twit)
	return p, nil
}

// Save upserts prefs.
func (s *PrefsStore) Save(p *UserPrefs) error {
	if p == nil {
		return fmt.Errorf("nil prefs")
	}
	p.Handle = SanitizeName(p.Handle)
	twit := strings.Join(p.Twit, ",")
	_, err := s.db.Exec(`
		INSERT INTO mrc_user_prefs (user_id, handle, handle_color, prefix, suffix, text_color, theme, twit, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(user_id) DO UPDATE SET
			handle = excluded.handle,
			handle_color = excluded.handle_color,
			prefix = excluded.prefix,
			suffix = excluded.suffix,
			text_color = excluded.text_color,
			theme = excluded.theme,
			twit = excluded.twit,
			updated_at = datetime('now')`,
		p.UserID, p.Handle, p.HandleColor, p.Prefix, p.Suffix, p.TextColor, p.Theme, twit)
	return err
}

// Ignores reports whether name is on the twit list (case-insensitive).
func (p *UserPrefs) Ignores(name string) bool {
	name = strings.ToLower(SanitizeName(name))
	for _, t := range p.Twit {
		if strings.ToLower(SanitizeName(t)) == name {
			return true
		}
	}
	return false
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
