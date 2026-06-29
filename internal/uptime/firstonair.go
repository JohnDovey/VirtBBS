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

// LongevityBreakdown splits elapsed time into years (365-day), days, hours, and minutes.
func LongevityBreakdown(d time.Duration) (years, days, hours, minutes int) {
	if d < 0 {
		d = 0
	}
	total := int(d.Round(time.Minute).Minutes())
	years = total / (365 * 24 * 60)
	total %= 365 * 24 * 60
	days = total / (24 * 60)
	total %= 24 * 60
	hours = total / 60
	minutes = total % 60
	return years, days, hours, minutes
}

// FirstOnAirMessage returns the historical on-air line for logs and terminals.
func FirstOnAirMessage(bbsName string) string {
	since := FirstOnAir()
	if since.IsZero() {
		return ""
	}
	years, days, hours, minutes := LongevityBreakdown(time.Since(since))
	return fmt.Sprintf("%s first appeared on the air at %s %s. That's %d years, %d days, %d hours and %d minutes ago",
		bbsName,
		since.Format("2006-01-02"), since.Format("15:04:05"),
		years, days, hours, minutes)
}
