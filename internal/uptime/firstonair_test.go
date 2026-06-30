package uptime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLongevityBreakdown(t *testing.T) {
	y, d, h, m, s := LongevityBreakdown(365*24*time.Hour + 2*24*time.Hour + 3*time.Hour + 15*time.Minute + 20*time.Second)
	if y != 1 || d != 2 || h != 3 || m != 15 || s != 20 {
		t.Fatalf("got %d years %d days %d hours %d minutes %d seconds", y, d, h, m, s)
	}
}

func TestInitFirstOnAir_createsAndReloads(t *testing.T) {
	dir := t.TempDir()
	if err := InitFirstOnAir(dir); err != nil {
		t.Fatal(err)
	}
	first := FirstOnAir()
	if first.IsZero() {
		t.Fatal("expected first on air time")
	}
	path := filepath.Join(dir, firstOnAirFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	firstOnAirMu.Lock()
	firstOnAir = time.Time{}
	firstOnAirMu.Unlock()
	if err := InitFirstOnAir(dir); err != nil {
		t.Fatal(err)
	}
	if FirstOnAir().Unix() != first.Unix() {
		t.Fatalf("reload: got %v want %v", FirstOnAir(), first)
	}
}

func TestFirstOnAirMessage_format(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().Add(-2*time.Hour - 30*time.Minute - 5*time.Second)
	if err := os.WriteFile(filepath.Join(dir, firstOnAirFile), []byte(past.Format(time.RFC3339)), 0644); err != nil {
		t.Fatal(err)
	}
	firstOnAirMu.Lock()
	firstOnAir = time.Time{}
	firstOnAirMu.Unlock()
	if err := InitFirstOnAir(dir); err != nil {
		t.Fatal(err)
	}
	msg := FirstOnAirMessage("Dev BBS")
	if !strings.Contains(msg, "Dev BBS first appeared on the air at") {
		t.Fatalf("missing intro: %q", msg)
	}
	if !strings.Contains(msg, "hours") || !strings.Contains(msg, "seconds ago") {
		t.Fatalf("missing full duration: %q", msg)
	}
}

func TestMessageLines_includesFirstOnAir(t *testing.T) {
	dir := t.TempDir()
	if err := InitFirstOnAir(dir); err != nil {
		t.Fatal(err)
	}
	RecordStart()
	lines := MessageLines("Test BBS")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	if !strings.Contains(lines[1], "first appeared on the air") {
		t.Fatalf("second line: %q", lines[1])
	}
}
