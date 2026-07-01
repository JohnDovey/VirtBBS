package appstats

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/ansi"
)

//go:embed schema.sql
var schema string

const BulletinName = "APPSTATS"

// Open applies the app-usage stats schema to the shared database.
func Open(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

// Report records incremental VirtAnd usage for a user.
func Report(db *sql.DB, userID int64, messagesDownloaded, messagesUploaded, filesDownloaded, filesUploaded int) error {
	if messagesDownloaded == 0 && messagesUploaded == 0 && filesDownloaded == 0 && filesUploaded == 0 {
		return nil
	}
	day := time.Now().Format("2006-01-02")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var seen int
	_ = tx.QueryRow(`SELECT 1 FROM app_usage_users WHERE day=? AND user_id=?`, day, userID).Scan(&seen)
	if seen == 0 {
		if _, err := tx.Exec(`INSERT INTO app_usage_users (day, user_id) VALUES (?,?)`, day, userID); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`
		INSERT INTO app_usage_daily (day, unique_users, messages_downloaded, messages_uploaded, files_downloaded, files_uploaded)
		VALUES (?, CASE WHEN ?=0 THEN 1 ELSE 0 END, ?, ?, ?, ?)
		ON CONFLICT(day) DO UPDATE SET
			unique_users = unique_users + excluded.unique_users,
			messages_downloaded = messages_downloaded + excluded.messages_downloaded,
			messages_uploaded = messages_uploaded + excluded.messages_uploaded,
			files_downloaded = files_downloaded + excluded.files_downloaded,
			files_uploaded = files_uploaded + excluded.files_uploaded`,
		day, seen, messagesDownloaded, messagesUploaded, filesDownloaded, filesUploaded)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO app_usage_totals (id, messages_downloaded, messages_uploaded, files_downloaded, files_uploaded)
		VALUES (1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			messages_downloaded = messages_downloaded + excluded.messages_downloaded,
			messages_uploaded = messages_uploaded + excluded.messages_uploaded,
			files_downloaded = files_downloaded + excluded.files_downloaded,
			files_uploaded = files_uploaded + excluded.files_uploaded`,
		messagesDownloaded, messagesUploaded, filesDownloaded, filesUploaded)
	if err != nil {
		return err
	}
	return tx.Commit()
}

type snapshot struct {
	UniqueUsers        int
	MessagesDownloaded int
	MessagesUploaded   int
	FilesDownloaded    int
	FilesUploaded      int
}

func queryDaily(db *sql.DB, day string) (*snapshot, error) {
	var s snapshot
	err := db.QueryRow(`SELECT COALESCE(unique_users,0), COALESCE(messages_downloaded,0),
		COALESCE(messages_uploaded,0), COALESCE(files_downloaded,0), COALESCE(files_uploaded,0)
		FROM app_usage_daily WHERE day=?`, day).Scan(
		&s.UniqueUsers, &s.MessagesDownloaded, &s.MessagesUploaded, &s.FilesDownloaded, &s.FilesUploaded)
	if err == sql.ErrNoRows {
		return &snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func queryAllTime(db *sql.DB) (*snapshot, error) {
	var s snapshot
	err := db.QueryRow(`SELECT COALESCE(messages_downloaded,0), COALESCE(messages_uploaded,0),
		COALESCE(files_downloaded,0), COALESCE(files_uploaded,0)
		FROM app_usage_totals WHERE id=1`).Scan(
		&s.MessagesDownloaded, &s.MessagesUploaded, &s.FilesDownloaded, &s.FilesUploaded)
	if err == sql.ErrNoRows {
		return &snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	_ = db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM app_usage_users`).Scan(&s.UniqueUsers)
	return &s, nil
}

// WriteBulletin writes APPSTATS.ANS for the sysop dashboard bulletin list.
func WriteBulletin(db *sql.DB, displayDir, bbsName string) error {
	if db == nil {
		return fmt.Errorf("stats database not available")
	}
	if err := os.MkdirAll(displayDir, 0755); err != nil {
		return err
	}
	dayKey := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	dayStats, err := queryDaily(db, dayKey)
	if err != nil {
		return fmt.Errorf("app stats 24h: %w", err)
	}
	allStats, err := queryAllTime(db)
	if err != nil {
		return fmt.Errorf("app stats all-time: %w", err)
	}
	text := renderBulletin(bbsName, dayKey, dayStats, allStats)
	return os.WriteFile(filepath.Join(displayDir, BulletinName+".ANS"), []byte(text), 0644)
}

func renderBulletin(bbsName, dayKey string, dayStats, allStats *snapshot) string {
	var b strings.Builder
	b.WriteString(ansi.ClearScreen())
	b.WriteString(ansi.Header("VirtAnd App Statistics"))
	b.WriteString("\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightBlack, bbsName))
	b.WriteString("\r\n\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightYellow, "Usage reported by the VirtAnd Android client via User API."))
	b.WriteString("\r\n\r\n")

	appendPeriod(&b, fmt.Sprintf("Previous 24 Hours (%s)", dayKey), dayStats)
	b.WriteString("\r\n")
	appendPeriod(&b, "All Time", allStats)

	b.WriteString(ansi.Colorize(ansi.BrightBlack, "  Generated "+time.Now().Format(time.RFC3339)))
	b.WriteString("\r\n")
	return b.String()
}

func appendPeriod(b *strings.Builder, title string, s *snapshot) {
	b.WriteString(ansi.Bold())
	b.WriteString(ansi.Colorize(ansi.BrightCyan, "  "+title))
	b.WriteString(ansi.Reset())
	b.WriteString("\r\n")
	b.WriteString(ansi.Colorize(ansi.BrightBlack, strings.Repeat("-", 62)))
	b.WriteString("\r\n")
	if s == nil || (s.UniqueUsers == 0 && s.MessagesDownloaded == 0 && s.MessagesUploaded == 0 &&
		s.FilesDownloaded == 0 && s.FilesUploaded == 0) {
		b.WriteString(ansi.Colorize(ansi.Yellow, "    No VirtAnd activity recorded for this period."))
		b.WriteString("\r\n")
		return
	}
	statLine(b, "Unique app users", fmt.Sprintf("%d", s.UniqueUsers))
	statLine(b, "Messages downloaded", fmt.Sprintf("%d", s.MessagesDownloaded))
	statLine(b, "Messages uploaded", fmt.Sprintf("%d", s.MessagesUploaded))
	statLine(b, "Files downloaded", fmt.Sprintf("%d", s.FilesDownloaded))
	statLine(b, "Files uploaded", fmt.Sprintf("%d", s.FilesUploaded))
}

func statLine(b *strings.Builder, label, value string) {
	b.WriteString("  ")
	b.WriteString(ansi.Colorize(ansi.Cyan, fmt.Sprintf("%-28s", label)))
	b.WriteString(" ")
	b.WriteString(ansi.Colorize(ansi.White, value))
	b.WriteString("\r\n")
}
