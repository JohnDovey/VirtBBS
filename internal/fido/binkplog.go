package fido

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	binkpLogMu   sync.Mutex
	binkpLogPath string
	binkpLogFile *os.File
)

// InitBinkpLog opens (or creates) the BinkP session log at path, typically
// <paths.logs>/binkp.log. Safe to call once at server startup.
func InitBinkpLog(path string) error {
	binkpLogMu.Lock()
	defer binkpLogMu.Unlock()

	if binkpLogFile != nil {
		_ = binkpLogFile.Close()
		binkpLogFile = nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	binkpLogPath = path
	binkpLogFile = f
	return nil
}

// LogBinkp appends a timestamped line to binkp.log and mirrors it to the
// server stdout log (without the timestamp prefix).
func LogBinkp(msg string) {
	line := fmt.Sprintf("%s %s\n", time.Now().Format(time.RFC3339), msg)

	binkpLogMu.Lock()
	if binkpLogFile != nil {
		_, _ = binkpLogFile.WriteString(line)
	}
	binkpLogMu.Unlock()

	log.Print(msg)
}

// BinkpLogPath returns the configured log file path, or "" if not initialized.
func BinkpLogPath() string {
	binkpLogMu.Lock()
	defer binkpLogMu.Unlock()
	return binkpLogPath
}

// ReadBinkpLogTail returns up to maxLines from the end of the BinkP log.
func ReadBinkpLogTail(maxLines int) (lines []string, path string, err error) {
	if maxLines <= 0 {
		maxLines = 200
	}
	if maxLines > 2000 {
		maxLines = 2000
	}

	binkpLogMu.Lock()
	path = binkpLogPath
	binkpLogMu.Unlock()

	if path == "" {
		return nil, "", fmt.Errorf("BinkP log not initialized")
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, path, nil
		}
		return nil, path, err
	}
	defer f.Close()

	var all []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		all = append(all, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, path, err
	}

	if len(all) <= maxLines {
		return all, path, nil
	}
	return all[len(all)-maxLines:], path, nil
}

// logPollResult writes a standard poll summary line for client or scheduler polls.
func logPollResult(network, kind string, sent, received int, pollErr error) {
	if pollErr != nil {
		LogBinkp(fmt.Sprintf("binkp %s [%s]: poll error: %v", kind, network, pollErr))
		return
	}
	LogBinkp(fmt.Sprintf("binkp %s [%s]: poll complete — sent %d, received %d",
		kind, network, sent, received))
}

func logTossResult(network, kind string, tr *TossResult) {
	if tr == nil {
		return
	}
	LogBinkp(fmt.Sprintf("binkp %s [%s]: auto-toss — %d imported, %d skipped, %d held",
		kind, network, tr.Imported, tr.Skipped, tr.Orphaned))
	for _, e := range tr.Errors {
		if strings.TrimSpace(e) != "" {
			LogBinkp(fmt.Sprintf("binkp %s [%s]: toss error: %s", kind, network, e))
		}
	}
}
