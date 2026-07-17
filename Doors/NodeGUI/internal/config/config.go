// Package config holds runtime defaults and persisted application settings.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultBaseURL is the pre-filled Zone 1 daily nodelist download location
// (Darkrealms FidoNet Zone 1 hub). Filenames such as Z1DAILY.Z97 are appended.
const DefaultBaseURL = "https://fido-z1.darkrealms.ca/$webfile.send.ZC1./"

// DefaultDomain is the network domain stored with each node.
const DefaultDomain = "FidoNet"

// Settings are persisted next to the SQLite database.
type Settings struct {
	// BaseURL is the directory URL that hosts Z1DAILY.Zjj archives.
	BaseURL string `json:"base_url"`
	// Domain stored on imported nodes (e.g. FidoNet).
	Domain string `json:"domain"`
	// LastImportAt is ISO-8601 when the last successful import finished.
	LastImportAt string `json:"last_import_at,omitempty"`
	// LastNodeDay is the julian day number from the imported list header (if known).
	LastNodeDay int `json:"last_node_day,omitempty"`
	// LastSource is the URL or local path of the last successful import.
	LastSource string `json:"last_source,omitempty"`
	// LastCount is the number of nodes stored after the last import.
	LastCount int `json:"last_count,omitempty"`
}

// Defaults returns factory settings.
func Defaults() Settings {
	return Settings{
		BaseURL: DefaultBaseURL,
		Domain:  DefaultDomain,
	}
}

// Path returns the settings JSON path next to the database file.
func Path(dbPath string) string {
	dir := filepath.Dir(dbPath)
	base := filepath.Base(dbPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	if name == "" {
		name = "nodegui"
	}
	return filepath.Join(dir, name+".settings.json")
}

// Load reads settings from disk, or returns defaults if missing.
func Load(dbPath string) (Settings, error) {
	s := Defaults()
	p := Path(dbPath)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Defaults(), fmt.Errorf("parse settings: %w", err)
	}
	if s.BaseURL == "" {
		s.BaseURL = DefaultBaseURL
	}
	if s.Domain == "" {
		s.Domain = DefaultDomain
	}
	return s, nil
}

// Save writes settings to disk.
func Save(dbPath string, s Settings) error {
	p := Path(dbPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("settings dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// MarkImport updates import metadata after a successful load.
func (s *Settings) MarkImport(source string, nodeDay, count int) {
	s.LastImportAt = time.Now().UTC().Format(time.RFC3339)
	s.LastSource = source
	s.LastNodeDay = nodeDay
	s.LastCount = count
}
