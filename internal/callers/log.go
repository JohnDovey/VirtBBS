// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.0.1  2026-06-24  Initial implementation — 64-byte fixed-record format
//   v0.0.5  2026-06-24  Phase 14: Rich structured Entry type, JSON log, search,
//                        callers.list API support, daily stats
// ============================================================================

// Package callers manages the VirtBBS callers log.
//
// Two files are maintained in parallel:
//
//  1. CALLERS.LOG — a fixed-width 80-byte-per-line text file in the spirit of
//     the original PCBoard callers log.  Human-readable and importable by
//     legacy tools.  Format per line (80 chars + CRLF = 82 bytes):
//
//     MM-DD HH:MM  <name:25>  <city:20>  <sec:3>  <action:10>  <node:3>
//
//  2. CALLERS.DAT — newline-delimited JSON (NDJSON) file.  One JSON object
//     per call.  Contains the full structured Entry so that the sysop GUI and
//     callers.list API can present rich information without text parsing.
package callers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	textLineLen = 80 // printable chars per text log line
	textRecSize = 82 // textLineLen + CRLF
)

// Entry holds the structured record for a single caller session.
type Entry struct {
	// Timestamp is when the caller connected (login time).
	Timestamp time.Time `json:"timestamp"`
	// UserName is the caller's full name.
	UserName string `json:"user_name"`
	// City is the caller's city/state.
	City string `json:"city"`
	// RemoteAddr is the caller's IP address.
	RemoteAddr string `json:"remote_addr,omitempty"`
	// SecurityLevel is the caller's security level at login.
	SecurityLevel int `json:"security_level"`
	// Node is the BBS node number for this call.
	Node int `json:"node"`
	// Action describes what happened: LOGIN, LOGOFF, TIMEOUT, KICKED, NEW USER.
	Action string `json:"action"`
	// DurationSecs is the length of the session in seconds (set at logoff).
	DurationSecs int `json:"duration_secs,omitempty"`
	// MsgsRead is the number of messages read this session.
	MsgsRead int `json:"msgs_read,omitempty"`
	// MsgsLeft is the number of messages posted this session.
	MsgsLeft int `json:"msgs_left,omitempty"`
	// FilesDown is the number of files downloaded.
	FilesDown int `json:"files_down,omitempty"`
	// FilesUp is the number of files uploaded.
	FilesUp int `json:"files_up,omitempty"`
}

// Log is an append-only callers log that writes to both the text log and NDJSON log.
type Log struct {
	textPath string // CALLERS.LOG
	jsonPath string // CALLERS.DAT
}

// Open opens (or creates) the callers log at textPath.
// The NDJSON companion file uses the same base name with a .DAT extension.
func Open(textPath string) (*Log, error) {
	// Ensure both files exist (create if missing).
	for _, p := range []string{textPath, jsonPath(textPath)} {
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open callers log %s: %w", p, err)
		}
		f.Close()
	}
	return &Log{textPath: textPath, jsonPath: jsonPath(textPath)}, nil
}

// jsonPath derives the .DAT path from the .LOG path.
func jsonPath(logPath string) string {
	if strings.HasSuffix(strings.ToUpper(logPath), ".LOG") {
		return logPath[:len(logPath)-4] + ".DAT"
	}
	return logPath + ".dat"
}

// Record appends a caller entry to both log files.
func (l *Log) Record(e *Entry) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	if err := l.writeText(e); err != nil {
		return err
	}
	return l.writeJSON(e)
}

// writeText appends an 80-char + CRLF line to CALLERS.LOG.
// Format: MM-DD HH:MM  <name:25>  <city:20>  <sec:3>  <action:10>  Nd:<n>
func (l *Log) writeText(e *Entry) error {
	ts := e.Timestamp.Format("01-02 15:04")
	line := fmt.Sprintf("%-11s  %-25s  %-20s  %3d  %-10s  N%d",
		ts,
		truncate(e.UserName, 25),
		truncate(e.City, 20),
		e.SecurityLevel,
		truncate(e.Action, 10),
		e.Node,
	)
	// Pad / trim to textLineLen.
	for len(line) < textLineLen {
		line += " "
	}
	if len(line) > textLineLen {
		line = line[:textLineLen]
	}
	record := line + "\r\n"

	f, err := os.OpenFile(l.textPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(record)
	return err
}

// writeJSON appends a JSON object (newline-delimited) to CALLERS.DAT.
func (l *Log) writeJSON(e *Entry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.jsonPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// Recent returns the last n entries in reverse chronological order (newest first).
// If n <= 0, all entries are returned.
func (l *Log) Recent(n int) ([]*Entry, error) {
	return l.search("", n)
}

// Search returns entries whose UserName or City contains substr (case-insensitive),
// limited to the most recent n. n <= 0 means no limit.
func (l *Log) Search(substr string, n int) ([]*Entry, error) {
	return l.search(substr, n)
}

func (l *Log) search(substr string, limit int) ([]*Entry, error) {
	f, err := os.Open(l.jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	substr = strings.ToLower(substr)
	var all []*Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if substr != "" {
			haystack := strings.ToLower(e.UserName + " " + e.City + " " + e.Action)
			if !strings.Contains(haystack, substr) {
				continue
			}
		}
		all = append(all, &e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	// Reverse for newest-first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// DailyStats returns the number of unique callers and total calls for today.
func (l *Log) DailyStats() (unique, total int, err error) {
	today := time.Now().Format("2006-01-02")
	entries, err := l.search("", 0)
	if err != nil {
		return 0, 0, err
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Timestamp.Format("2006-01-02") != today {
			continue
		}
		total++
		seen[strings.ToLower(e.UserName)] = true
	}
	return len(seen), total, nil
}

// CountByDay returns call counts keyed by YYYY-MM-DD for the last days calendar
// days (including today). Older entries in the log are ignored.
func (l *Log) CountByDay(days int) map[string]int {
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	out := map[string]int{}
	entries, err := l.search("", 0)
	if err != nil {
		return out
	}
	for _, e := range entries {
		key := e.Timestamp.Format("2006-01-02")
		if key < cutoff {
			continue
		}
		out[key]++
	}
	return out
}

// TextRecords returns the last n raw text-log lines (as recorded in CALLERS.LOG).
// Useful for the terminal-based sysop log viewer.
func (l *Log) TextRecords(n int) ([]string, error) {
	data, err := os.ReadFile(l.textPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []string
	for i := 0; i+textRecSize <= len(data); i += textRecSize {
		records = append(records, strings.TrimRight(string(data[i:i+textLineLen]), " "))
	}
	// Reverse.
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}
	if n > 0 && len(records) > n {
		records = records[:n]
	}
	return records, nil
}

// ── Legacy compatibility shim ─────────────────────────────────────────────────

// RecordSimple is a convenience wrapper for callers that only know the bare
// name/city/action (e.g. existing session code).
func (l *Log) RecordSimple(userName, city, action string) error {
	return l.Record(&Entry{
		Timestamp: time.Now(),
		UserName:  userName,
		City:      city,
		Action:    action,
	})
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
