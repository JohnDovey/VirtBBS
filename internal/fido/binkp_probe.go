package fido

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	binkpHandshakeTimeout = 20 * time.Second
	binkpSessionTimeout   = 5 * time.Minute
	binkpProbeLogInterval = 1 * time.Minute
)

var (
	binkpProbeMu     sync.Mutex
	binkpProbeWindow = time.Time{}
	binkpProbeByHost = map[string]int{}
	binkpProbeTotal  int
)

// isBinkpProbeError reports early-disconnect / scan noise during handshake
// (EOF, reset by peer, broken pipe, timeouts). These are not session errors.
func isBinkpProbeError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"connection reset by peer",
		"broken pipe",
		"connection refused",
		"use of closed network connection",
		"i/o timeout",
		"deadline exceeded",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// noteBinkpProbe records a scan-style disconnect and rate-limits a summary log line.
func noteBinkpProbe(remote net.Addr, err error) {
	host := remoteHost(remote)
	reason := probeReason(err)

	binkpProbeMu.Lock()
	now := time.Now()
	if binkpProbeWindow.IsZero() || now.Sub(binkpProbeWindow) >= binkpProbeLogInterval {
		flushBinkpProbesLocked(now)
		binkpProbeWindow = now
	}
	binkpProbeByHost[host]++
	binkpProbeTotal++
	// Keep the map from growing unboundedly under sustained scans.
	if len(binkpProbeByHost) > 64 {
		flushBinkpProbesLocked(now)
		binkpProbeWindow = now
	}
	_ = reason // included in periodic flush summary if useful later
	binkpProbeMu.Unlock()
}

func flushBinkpProbesLocked(now time.Time) {
	if binkpProbeTotal == 0 {
		return
	}
	// Pick top hosts for a short summary (max 3).
	type pair struct {
		host  string
		count int
	}
	var top []pair
	for h, c := range binkpProbeByHost {
		top = append(top, pair{h, c})
	}
	for i := 0; i < len(top); i++ {
		for j := i + 1; j < len(top); j++ {
			if top[j].count > top[i].count {
				top[i], top[j] = top[j], top[i]
			}
		}
	}
	var parts []string
	limit := 3
	if len(top) < limit {
		limit = len(top)
	}
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s×%d", top[i].host, top[i].count))
	}
	extra := ""
	if len(parts) > 0 {
		extra = " (" + strings.Join(parts, ", ")
		if len(top) > limit {
			extra += ", …"
		}
		extra += ")"
	}
	LogBinkp(fmt.Sprintf("binkp server: ignored %d port-scan disconnect(s) in the last %s%s",
		binkpProbeTotal, now.Sub(binkpProbeWindow).Round(time.Second), extra))
	binkpProbeByHost = map[string]int{}
	binkpProbeTotal = 0
}

func remoteHost(addr net.Addr) string {
	if addr == nil {
		return "?"
	}
	s := addr.String()
	if host, _, err := net.SplitHostPort(s); err == nil {
		return host
	}
	return s
}

func probeReason(err error) string {
	if err == nil {
		return "disconnect"
	}
	if errors.Is(err, io.EOF) {
		return "EOF"
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "reset"):
		return "reset"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timeout"
	default:
		return "disconnect"
	}
}
