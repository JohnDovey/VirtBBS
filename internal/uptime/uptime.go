// Package uptime records when the VirtBBS server process started and formats
// elapsed uptime for logs, stats, and PPL.
package uptime

import (
	"fmt"
	"sync"
	"time"
)

var (
	mu        sync.RWMutex
	startedAt time.Time
)

// RecordStart marks the current time as the server start (call once from main).
func RecordStart() {
	mu.Lock()
	startedAt = time.Now()
	mu.Unlock()
}

// StartedAt returns when RecordStart was called, or zero if not yet started.
func StartedAt() time.Time {
	mu.RLock()
	defer mu.RUnlock()
	return startedAt
}

// Elapsed returns time since RecordStart, or zero if not yet started.
func Elapsed() time.Duration {
	mu.RLock()
	t := startedAt
	mu.RUnlock()
	if t.IsZero() {
		return 0
	}
	return time.Since(t)
}

// Breakdown splits a duration into years (365-day), days, hours, minutes, and seconds.
func Breakdown(d time.Duration) (years, days, hours, minutes, seconds int) {
	if d < 0 {
		d = 0
	}
	sec := int(d.Round(time.Second).Seconds())
	years = sec / (365 * 86400)
	sec %= 365 * 86400
	days = sec / 86400
	sec %= 86400
	hours = sec / 3600
	sec %= 3600
	minutes = sec / 60
	seconds = sec % 60
	return years, days, hours, minutes, seconds
}

// FormatDuration returns a human-readable years/days/hours/minutes/seconds string.
func FormatDuration(d time.Duration) string {
	y, days, hours, minutes, seconds := Breakdown(d)
	return fmt.Sprintf("%d years, %d days, %d hours, %d minutes and %d seconds",
		y, days, hours, minutes, seconds)
}

// Message returns the standard BBS uptime line for logs and terminals.
func Message(bbsName string) string {
	elapsed := FormatDuration(Elapsed())
	since := StartedAt()
	if since.IsZero() {
		return fmt.Sprintf("This BBS (%s) has been up for %s", bbsName, elapsed)
	}
	return fmt.Sprintf("This BBS (%s) has been up for %s since %s %s",
		bbsName, elapsed, since.Format("2006-01-02"), since.Format("15:04:05"))
}

// MessageLines returns the process uptime line and the first-on-air history line.
func MessageLines(bbsName string) []string {
	lines := []string{Message(bbsName)}
	if msg := FirstOnAirMessage(bbsName); msg != "" {
		lines = append(lines, msg)
	}
	return lines
}
