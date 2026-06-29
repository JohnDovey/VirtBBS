package fido

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	binkpDebugMu         sync.Mutex
	binkpDebugLogsDir    string
	binkpDebugGlobalPath string
	binkpDebugGlobalFile *os.File
	binkpDebugEnabled    atomic.Bool
)

// InitBinkpDebugLog prepares the logs directory for BinkP protocol debug output.
// Global trace lines (when BinkpDebug is enabled) append to logs/binkp-debug.log.
// One-shot debug polls write to logs/binkp-debug-<timestamp>-<network>.log.
func InitBinkpDebugLog(logsDir string) error {
	binkpDebugMu.Lock()
	defer binkpDebugMu.Unlock()
	binkpDebugLogsDir = strings.TrimSpace(logsDir)
	if binkpDebugLogsDir == "" {
		return fmt.Errorf("empty binkp debug logs directory")
	}
	if err := os.MkdirAll(binkpDebugLogsDir, 0755); err != nil {
		return err
	}
	binkpDebugGlobalPath = filepath.Join(binkpDebugLogsDir, "binkp-debug.log")
	return nil
}

// SetBinkpDebugEnabled turns protocol-level tracing on or off for all BinkP
// client polls (scheduler, menu, admin). Trace lines go to binkp-debug.log.
func SetBinkpDebugEnabled(on bool) {
	binkpDebugEnabled.Store(on)
}

// BinkpDebugEnabled reports whether global BinkP protocol tracing is active.
func BinkpDebugEnabled() bool {
	return binkpDebugEnabled.Load()
}

// BinkpDebugGlobalPath returns the path of the shared debug log, or "".
func BinkpDebugGlobalPath() string {
	binkpDebugMu.Lock()
	defer binkpDebugMu.Unlock()
	return binkpDebugGlobalPath
}

// BinkpDebugLogsDir returns the configured logs directory for debug sessions.
func BinkpDebugLogsDir() string {
	binkpDebugMu.Lock()
	defer binkpDebugMu.Unlock()
	return binkpDebugLogsDir
}

// BinkpDebugSession captures one poll's full wire-level trace to a dedicated file.
type BinkpDebugSession struct {
	path    string
	network string
	f       *os.File
	mu      sync.Mutex
	onLine  func(string)
}

// SetOnLine registers a callback invoked for each trace line (timestamp included).
// Used by the admin debug-poll stream; must be set before the poll starts.
func (s *BinkpDebugSession) SetOnLine(fn func(string)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onLine = fn
	s.mu.Unlock()
}

// BeginBinkpDebugSession opens a timestamped debug log for a single poll.
func BeginBinkpDebugSession(network string) (*BinkpDebugSession, error) {
	binkpDebugMu.Lock()
	dir := binkpDebugLogsDir
	binkpDebugMu.Unlock()
	if dir == "" {
		return nil, fmt.Errorf("binkp debug log not initialized")
	}
	ts := time.Now().Format("20060102-150405")
	safeNet := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, strings.TrimSpace(network))
	if safeNet == "" {
		safeNet = "network"
	}
	path := filepath.Join(dir, fmt.Sprintf("binkp-debug-%s-%s.log", ts, safeNet))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	s := &BinkpDebugSession{path: path, network: network, f: f}
	s.writef("=== BinkP debug session [%s] %s ===", network, time.Now().Format(time.RFC3339))
	return s, nil
}

// Path returns the session log file path.
func (s *BinkpDebugSession) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Close flushes and closes the session log.
func (s *BinkpDebugSession) Close() error {
	if s == nil || s.f == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.f.Close()
	s.f = nil
	return err
}

func (s *BinkpDebugSession) writef(format string, args ...interface{}) {
	if s == nil || s.f == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	full := fmt.Sprintf("%s %s", time.Now().Format("2006/01/02 15:04:05"), line)
	s.mu.Lock()
	if s.f == nil {
		s.mu.Unlock()
		return
	}
	fmt.Fprintf(s.f, "%s\n", full)
	onLine := s.onLine
	s.mu.Unlock()
	if onLine != nil {
		onLine(full)
	}
}

func writeBinkpDebugGlobal(network, line string) {
	binkpDebugMu.Lock()
	defer binkpDebugMu.Unlock()
	if binkpDebugGlobalPath == "" {
		return
	}
	if binkpDebugGlobalFile == nil {
		f, err := os.OpenFile(binkpDebugGlobalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		binkpDebugGlobalFile = f
	}
	prefix := "binkp debug"
	if network != "" {
		prefix = fmt.Sprintf("binkp debug [%s]", network)
	}
	fmt.Fprintf(binkpDebugGlobalFile, "%s %s: %s\n", time.Now().Format("2006/01/02 15:04:05"), prefix, line)
}

// ReadBinkpDebugLogTail returns up to maxLines from the end of path.
func ReadBinkpDebugLogTail(path string, maxLines int) ([]string, string, error) {
	if maxLines <= 0 {
		maxLines = 80
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return []string{}, "", nil
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

func binkpCmdName(cmd byte) string {
	switch cmd {
	case bpM_NUL:
		return "M_NUL"
	case bpM_ADR:
		return "M_ADR"
	case bpM_PWD:
		return "M_PWD"
	case bpM_FILE:
		return "M_FILE"
	case bpM_OK:
		return "M_OK"
	case bpM_EOB:
		return "M_EOB"
	case bpM_GOT:
		return "M_GOT"
	case bpM_ERR:
		return "M_ERR"
	case bpM_BSY:
		return "M_BSY"
	case bpM_GET:
		return "M_GET"
	case bpM_SKIP:
		return "M_SKIP"
	default:
		return fmt.Sprintf("M_%d", cmd)
	}
}

func binkpSanitizeCmdArg(cmd byte, arg string) string {
	if cmd == bpM_PWD {
		if arg == "" || arg == "-" {
			return "(empty)"
		}
		return "***"
	}
	if len(arg) > 200 {
		return arg[:200] + "…"
	}
	return arg
}
