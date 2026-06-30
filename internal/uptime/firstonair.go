package uptime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const firstOnAirFile = "bbs_first_on_air.txt"

var (
	firstOnAirMu sync.RWMutex
	firstOnAir   time.Time
)

// InitFirstOnAir loads or creates the persistent BBS first-on-air timestamp in dataDir.
// On first run the current time is recorded and kept across restarts.
func InitFirstOnAir(dataDir string) error {
	firstOnAirMu.Lock()
	defer firstOnAirMu.Unlock()

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return fmt.Errorf("uptime: data directory required")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dataDir, firstOnAirFile)
	if data, err := os.ReadFile(path); err == nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			firstOnAir = t
			return nil
		}
	}
	now := time.Now()
	if err := os.WriteFile(path, []byte(now.Format(time.RFC3339)), 0644); err != nil {
		return err
	}
	firstOnAir = now
	return nil
}

// FirstOnAir returns when this BBS was first recorded on the air, or zero if unset.
func FirstOnAir() time.Time {
	firstOnAirMu.RLock()
	defer firstOnAirMu.RUnlock()
	return firstOnAir
}

// LongevityBreakdown splits elapsed time into years, days, hours, minutes, and seconds.
func LongevityBreakdown(d time.Duration) (years, days, hours, minutes, seconds int) {
	return Breakdown(d)
}

// FirstOnAirMessage returns the historical on-air line for logs and terminals.
func FirstOnAirMessage(bbsName string) string {
	since := FirstOnAir()
	if since.IsZero() {
		return ""
	}
	ago := FormatDuration(time.Since(since))
	return fmt.Sprintf("%s first appeared on the air at %s %s. That's %s ago",
		bbsName,
		since.Format("2006-01-02"), since.Format("15:04:05"),
		ago)
}
