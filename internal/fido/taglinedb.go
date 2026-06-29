package fido

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

// TaglineRow is one stored echomail tagline.
type TaglineRow struct {
	ID         int64
	Text       string
	Source     string
	Enabled    bool
	HitCount   int
	CreatedAt  string
}

// TaglineDB stores harvested and sysop-managed taglines.
type TaglineDB struct {
	db *sql.DB
}

func OpenTaglineDB(db *sql.DB) *TaglineDB {
	if db == nil {
		return nil
	}
	return &TaglineDB{db: db}
}

func MigrateTaglines(db *sql.DB) error {
	if db == nil {
		return nil
	}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS fido_taglines (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		text TEXT NOT NULL,
		normalized TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'harvest',
		enabled INTEGER NOT NULL DEFAULT 1,
		hit_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE(normalized)
	)`)
	return err
}

func normalizeTagline(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func (t *TaglineDB) ListAll() ([]TaglineRow, error) {
	if t == nil || t.db == nil {
		return nil, nil
	}
	rows, err := t.db.Query(`SELECT id, text, source, enabled, hit_count, created_at
		FROM fido_taglines ORDER BY enabled DESC, hit_count DESC, text COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTaglineRows(rows)
}

func (t *TaglineDB) EnabledTexts() []string {
	if t == nil || t.db == nil {
		return nil
	}
	rows, err := t.db.Query(`SELECT text FROM fido_taglines WHERE enabled=1 ORDER BY text COLLATE NOCASE`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			continue
		}
		if strings.TrimSpace(text) != "" {
			out = append(out, text)
		}
	}
	return out
}

func (t *TaglineDB) Upsert(text, source string) (added bool, err error) {
	if t == nil || t.db == nil {
		return false, fmt.Errorf("tagline db unavailable")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return false, nil
	}
	if source == "" {
		source = "manual"
	}
	norm := normalizeTagline(text)
	var id int64
	err = t.db.QueryRow(`SELECT id FROM fido_taglines WHERE normalized=?`, norm).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = t.db.Exec(`INSERT INTO fido_taglines (text, normalized, source) VALUES (?,?,?)`,
			text, norm, source)
		return err == nil, err
	}
	if err != nil {
		return false, err
	}
	_, err = t.db.Exec(`UPDATE fido_taglines SET hit_count = hit_count + 1 WHERE id=?`, id)
	return false, err
}

func (t *TaglineDB) SetText(id int64, text string, enabled bool) error {
	if t == nil || t.db == nil {
		return fmt.Errorf("tagline db unavailable")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("tagline text required")
	}
	norm := normalizeTagline(text)
	en := 0
	if enabled {
		en = 1
	}
	_, err := t.db.Exec(`UPDATE fido_taglines SET text=?, normalized=?, enabled=? WHERE id=?`,
		text, norm, en, id)
	return err
}

func (t *TaglineDB) Delete(id int64) error {
	if t == nil || t.db == nil {
		return fmt.Errorf("tagline db unavailable")
	}
	_, err := t.db.Exec(`DELETE FROM fido_taglines WHERE id=?`, id)
	return err
}

func (t *TaglineDB) ImportLines(lines []string, source string) (added int, err error) {
	if t == nil || t.db == nil {
		return 0, fmt.Errorf("tagline db unavailable")
	}
	if source == "" {
		source = "import"
	}
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ok, err := t.Upsert(line, source)
		if err != nil {
			return added, err
		}
		if ok {
			added++
		}
	}
	return added, nil
}

func (t *TaglineDB) ImportFile(path string) (added int, err error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return t.ImportLines(strings.Split(string(data), "\n"), "file")
}

func (t *TaglineDB) HarvestFromBody(body string) {
	if t == nil {
		return
	}
	taglines, _, _ := ParseEchoFooters(body)
	for _, tl := range taglines {
		_, _ = t.Upsert(tl, "harvest")
	}
}

func (t *TaglineDB) CountEnabled() int {
	if t == nil || t.db == nil {
		return 0
	}
	var n int
	_ = t.db.QueryRow(`SELECT COUNT(*) FROM fido_taglines WHERE enabled=1`).Scan(&n)
	return n
}

// LoadTaglinesForUse returns enabled taglines from the database, seeding from
// path once when the table is empty and a legacy taglines_file is configured.
func LoadTaglinesForUse(db *sql.DB, filePath string) []string {
	if db != nil {
		if err := MigrateTaglines(db); err == nil {
			tdb := OpenTaglineDB(db)
			if tdb.CountEnabled() == 0 {
				if path := strings.TrimSpace(filePath); path != "" {
					_, _ = tdb.ImportFile(path)
				}
			}
			if texts := tdb.EnabledTexts(); len(texts) > 0 {
				return texts
			}
		}
	}
	return LoadTaglines(filePath)
}

func scanTaglineRows(rows *sql.Rows) ([]TaglineRow, error) {
	var out []TaglineRow
	for rows.Next() {
		var r TaglineRow
		var en int
		if err := rows.Scan(&r.ID, &r.Text, &r.Source, &en, &r.HitCount, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = en != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// TaglineImportResult summarises a merge import.
type TaglineImportResult struct {
	Added   int
	Total   int
	At      time.Time
}
