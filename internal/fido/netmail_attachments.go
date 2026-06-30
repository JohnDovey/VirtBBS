package fido

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/virtbbs/virtbbs/internal/messages"
)

func migrateNetmailAttachments(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS fido_netmail_attachments (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		netmail_id    INTEGER NOT NULL REFERENCES fido_netmail(id) ON DELETE CASCADE,
		filename      TEXT NOT NULL,
		size_bytes    INTEGER NOT NULL,
		storage_path  TEXT NOT NULL,
		created_at    TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_fido_netmail_attachments_msg ON fido_netmail_attachments(netmail_id)`)
	return err
}

// SaveNetmailAttachments writes queued netmail attachment files to disk.
func (ndb *NetmailDB) SaveNetmailAttachments(root string, netmailID int64, files []messages.AttachmentInput, maxBytes int64) error {
	if len(files) == 0 {
		return nil
	}
	if err := migrateNetmailAttachments(ndb.db); err != nil {
		return err
	}
	if maxBytes <= 0 {
		maxBytes = DefaultAttachmentLimitBytes
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	msgDir := filepath.Join(root, "netmail", fmt.Sprintf("%d", netmailID))
	if err := os.MkdirAll(msgDir, 0o755); err != nil {
		return err
	}
	for i, f := range files {
		if int64(len(f.Data)) > maxBytes {
			return fmt.Errorf("attachment %q exceeds size limit (%d bytes, max %d)", f.Filename, len(f.Data), maxBytes)
		}
		safe := messages.SanitizeAttachmentFilename(f.Filename, i)
		rel := filepath.Join("netmail", fmt.Sprintf("%d", netmailID), safe)
		abs := filepath.Join(root, rel)
		if err := os.WriteFile(abs, f.Data, 0o644); err != nil {
			return err
		}
		_, err := ndb.db.Exec(`INSERT INTO fido_netmail_attachments (netmail_id, filename, size_bytes, storage_path)
			VALUES (?,?,?,?)`, netmailID, safe, len(f.Data), filepath.ToSlash(rel))
		if err != nil {
			return err
		}
	}
	return nil
}

// NetmailAttachmentData loads attachment bytes for outbound export.
func (ndb *NetmailDB) NetmailAttachmentData(root string, netmailID int64) ([]messages.AttachmentInput, error) {
	if err := migrateNetmailAttachments(ndb.db); err != nil {
		return nil, err
	}
	rows, err := ndb.db.Query(`SELECT filename, storage_path FROM fido_netmail_attachments WHERE netmail_id=? ORDER BY id`, netmailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []messages.AttachmentInput
	for rows.Next() {
		var name, rel string
		if err := rows.Scan(&name, &rel); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		out = append(out, messages.AttachmentInput{Filename: name, Data: data})
	}
	return out, rows.Err()
}

// RemoveNetmailAttachments deletes stored files and rows for a sent netmail.
func (ndb *NetmailDB) RemoveNetmailAttachments(root string, netmailID int64) error {
	if err := migrateNetmailAttachments(ndb.db); err != nil {
		return err
	}
	rows, err := ndb.db.Query(`SELECT storage_path FROM fido_netmail_attachments WHERE netmail_id=?`, netmailID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			return err
		}
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
	}
	_, err = ndb.db.Exec(`DELETE FROM fido_netmail_attachments WHERE netmail_id=?`, netmailID)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "netmail", fmt.Sprintf("%d", netmailID))
	_ = os.Remove(dir)
	return nil
}

// netmailBodyForExport returns the body text for a queued netmail, with
// uuencoded attachments when they fit in the FTS message limit.
func netmailBodyForExport(cfg *Config, nd *NetworkDef, ndb *NetmailDB, attachmentsRoot string, netmailID int64, m *NetmailMsg, result *ScanNetmailResult) string {
	body := m.Body
	if ndb == nil || attachmentsRoot == "" {
		return body
	}
	files, err := ndb.NetmailAttachmentData(attachmentsRoot, netmailID)
	if err != nil || len(files) == 0 {
		return body
	}
	withAtt := BodyWithAttachments(m.Body, files, FidoMaxMessageBodyBytes)
	from, _ := ParseAddr(m.FromAddr)
	to, _ := ParseAddr(m.ToAddr)
	export := *m
	export.Body = withAtt
	if len(buildBody(&export, from, to)) <= FidoMaxMessageBodyBytes {
		return withAtt
	}
	if result != nil {
		result.Errors = append(result.Errors,
			fmt.Sprintf("id %d: attachment omitted (export would exceed %d bytes)", netmailID, FidoMaxMessageBodyBytes))
	}
	return body
}

// ParseNetmailComposeAttachment decodes a base64 attachment from API compose.
func ParseNetmailComposeAttachment(name, b64 string, maxBytes int64) (messages.AttachmentInput, error) {
	name = strings.TrimSpace(name)
	b64 = strings.TrimSpace(b64)
	if name == "" || b64 == "" {
		return messages.AttachmentInput{}, fmt.Errorf("attachment name and data required")
	}
	data, err := messages.DecodeBase64Attachment(b64)
	if err != nil {
		return messages.AttachmentInput{}, err
	}
	if maxBytes <= 0 {
		maxBytes = DefaultAttachmentLimitBytes
	}
	if int64(len(data)) > maxBytes {
		return messages.AttachmentInput{}, fmt.Errorf("attachment exceeds size limit (%d bytes, max %d)", len(data), maxBytes)
	}
	return messages.AttachmentInput{Filename: name, Data: data}, nil
}
