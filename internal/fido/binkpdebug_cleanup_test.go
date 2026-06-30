package fido

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPurgeOldDebugLogs(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	recentTime := time.Now().Add(-1 * time.Hour)

	oldSession := filepath.Join(dir, "binkp-debug-20260101-120000-FidoNet.log")
	if err := os.WriteFile(oldSession, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldSession, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	oldGlobal := filepath.Join(dir, "binkp-debug.log")
	if err := os.WriteFile(oldGlobal, []byte("old global"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldGlobal, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	recentSession := filepath.Join(dir, "binkp-debug-20260630-120000-FidoNet.log")
	if err := os.WriteFile(recentSession, []byte("recent"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recentSession, recentTime, recentTime); err != nil {
		t.Fatal(err)
	}

	keep := filepath.Join(dir, "binkp.log")
	if err := os.WriteFile(keep, []byte("not debug"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(keep, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	removed, err := PurgeOldDebugLogs(dir, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if _, err := os.Stat(oldSession); !os.IsNotExist(err) {
		t.Fatal("old session log should be removed")
	}
	if _, err := os.Stat(oldGlobal); !os.IsNotExist(err) {
		t.Fatal("old global debug log should be removed when tracing is off")
	}
	if _, err := os.Stat(recentSession); err != nil {
		t.Fatal("recent session log should remain")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("binkp.log should not be removed")
	}
}

func TestPurgeOldDebugLogs_skipsActiveGlobal(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	path := filepath.Join(dir, "binkp-debug.log")
	if err := os.WriteFile(path, []byte("active"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	SetBinkpDebugEnabled(true)
	t.Cleanup(func() { SetBinkpDebugEnabled(false) })

	removed, err := PurgeOldDebugLogs(dir, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0 while tracing enabled", removed)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("active global debug log should be kept")
	}
}
