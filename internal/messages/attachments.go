package messages

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DefaultAttachmentLimitBytes is the default max attachment size (5 MiB).
const DefaultAttachmentLimitBytes = 5 * 1024 * 1024

// Attachment describes a file stored outside the message body.
type Attachment struct {
	ID          int64
	MessageID   int64
	Filename    string
	SizeBytes   int64
	StoragePath string // relative to attachments root
	CreatedAt   time.Time
}

var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// SanitizeAttachmentFilename returns a filesystem-safe attachment basename.
func SanitizeAttachmentFilename(name string, index int) string {
	safe := safeFilenameRe.ReplaceAllString(filepath.Base(name), "_")
	if safe == "" {
		return fmt.Sprintf("file%d.dat", index+1)
	}
	return safe
}

// DecodeBase64Attachment decodes standard or URL-safe base64 attachment data.
func DecodeBase64Attachment(b64 string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(b64); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(b64)
}

// AttachmentsRoot returns the directory for message attachment files.
func AttachmentsRoot(dbPath, configured string) string {
	if p := strings.TrimSpace(configured); p != "" {
		return p
	}
	return filepath.Join(filepath.Dir(dbPath), "attachments")
}

func migrateAttachments(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS message_attachments (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		message_id    INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
		filename      TEXT NOT NULL,
		size_bytes    INTEGER NOT NULL,
		storage_path  TEXT NOT NULL,
		created_at    TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_message_attachments_msg ON message_attachments(message_id)`)
	return err
}

// SaveAttachments writes files to disk and records metadata. maxBytes is per-file limit.
func (s *Store) SaveAttachments(root string, messageID int64, files []AttachmentInput, maxBytes int64) error {
	if len(files) == 0 {
		return nil
	}
	if maxBytes <= 0 {
		maxBytes = DefaultAttachmentLimitBytes
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	msgDir := filepath.Join(root, fmt.Sprintf("%d", messageID))
	if err := os.MkdirAll(msgDir, 0o755); err != nil {
		return err
	}
	for i, f := range files {
		if int64(len(f.Data)) > maxBytes {
			return fmt.Errorf("attachment %q exceeds size limit (%d bytes, max %d)", f.Filename, len(f.Data), maxBytes)
		}
		safe := SanitizeAttachmentFilename(f.Filename, i)
		rel := filepath.Join(fmt.Sprintf("%d", messageID), safe)
		abs := filepath.Join(root, rel)
		if err := os.WriteFile(abs, f.Data, 0o644); err != nil {
			return err
		}
		_, err := s.db.Exec(`INSERT INTO message_attachments (message_id, filename, size_bytes, storage_path)
			VALUES (?,?,?,?)`, messageID, safe, len(f.Data), filepath.ToSlash(rel))
		if err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`UPDATE messages SET has_attachment=1 WHERE id=?`, messageID)
	return err
}

// AttachmentInput is one file to save with a message.
type AttachmentInput struct {
	Filename string
	Data     []byte
}

// ListAttachments returns attachments for a message.
func (s *Store) ListAttachments(messageID int64) ([]Attachment, error) {
	rows, err := s.db.Query(`SELECT id, message_id, filename, size_bytes, storage_path, created_at
		FROM message_attachments WHERE message_id=? ORDER BY id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attachment
	for rows.Next() {
		var a Attachment
		var created string
		if err := rows.Scan(&a.ID, &a.MessageID, &a.Filename, &a.SizeBytes, &a.StoragePath, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, created)
		if a.CreatedAt.IsZero() {
			a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// OpenAttachment returns a reader for an attachment after verifying message ownership via id.
func (s *Store) OpenAttachment(root string, attachmentID, messageID int64) (Attachment, io.ReadCloser, error) {
	var a Attachment
	var created string
	err := s.db.QueryRow(`SELECT id, message_id, filename, size_bytes, storage_path, created_at
		FROM message_attachments WHERE id=? AND message_id=?`, attachmentID, messageID).Scan(
		&a.ID, &a.MessageID, &a.Filename, &a.SizeBytes, &a.StoragePath, &created)
	if err != nil {
		return Attachment{}, nil, err
	}
	path := filepath.Join(root, filepath.FromSlash(a.StoragePath))
	f, err := os.Open(path)
	if err != nil {
		return Attachment{}, nil, err
	}
	return a, f, nil
}

// AttachmentData loads all attachment bytes for export/uuencode.
func (s *Store) AttachmentData(root string, messageID int64) ([]AttachmentInput, error) {
	list, err := s.ListAttachments(messageID)
	if err != nil {
		return nil, err
	}
	var out []AttachmentInput
	for _, a := range list {
		path := filepath.Join(root, filepath.FromSlash(a.StoragePath))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, AttachmentInput{Filename: a.Filename, Data: data})
	}
	return out, nil
}

// HasAttachments reports whether a message has stored attachments.
func (s *Store) HasAttachments(messageID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM message_attachments WHERE message_id=?`, messageID).Scan(&n)
	return n > 0, err
}
