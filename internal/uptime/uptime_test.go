package uptime

import (
	"strings"
	"testing"
	"time"
)

func TestBreakdown(t *testing.T) {
	d, m, s := Breakdown(2*24*time.Hour + 45*time.Minute + 30*time.Second)
	if d != 2 || m != 45 || s != 30 {
		t.Fatalf("got %d days %d min %d sec", d, m, s)
	}
}

func TestMessage_includesSince(t *testing.T) {
	RecordStart()
	msg := Message("Test BBS")
	if !strings.Contains(msg, "Test BBS") {
		t.Fatalf("missing name: %q", msg)
	}
	if !strings.Contains(msg, "since") {
		t.Fatalf("missing since: %q", msg)
	}
}
