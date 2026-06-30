package uptime

import (
	"strings"
	"testing"
	"time"
)

func TestBreakdown(t *testing.T) {
	y, d, h, m, s := Breakdown(365*24*time.Hour + 2*24*time.Hour + 3*time.Hour + 45*time.Minute + 30*time.Second)
	if y != 1 || d != 2 || h != 3 || m != 45 || s != 30 {
		t.Fatalf("got %d years %d days %d hours %d min %d sec", y, d, h, m, s)
	}
}

func TestFormatDuration_includesHours(t *testing.T) {
	msg := FormatDuration(2*time.Hour + 5*time.Minute + 7*time.Second)
	if !strings.Contains(msg, "2 hours") {
		t.Fatalf("missing hours: %q", msg)
	}
	if !strings.Contains(msg, "5 minutes") {
		t.Fatalf("missing minutes: %q", msg)
	}
	if !strings.Contains(msg, "7 seconds") {
		t.Fatalf("missing seconds: %q", msg)
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
	if !strings.Contains(msg, "hours") {
		t.Fatalf("missing hours component: %q", msg)
	}
}
