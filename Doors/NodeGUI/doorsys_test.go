package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDoorSYS_Security(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DOOR.SYS")
	// Minimal 20-line DOOR.SYS with security on line 15.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = ""
	}
	lines[3] = "1"      // node
	lines[9] = "TestUser"
	lines[14] = "110"    // security
	lines[18] = "60"     // minutes
	lines[19] = "0"      // ANSI
	body := ""
	for _, l := range lines {
		body += l + "\r\n"
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	sess, err := ParseDoorSYS(path)
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserName != "TestUser" {
		t.Fatalf("UserName=%q", sess.UserName)
	}
	if sess.SecurityLevel != 110 {
		t.Fatalf("SecurityLevel=%d", sess.SecurityLevel)
	}
	if !sess.IsSysop() {
		t.Fatal("expected IsSysop")
	}
}

func TestParseDoorSYS_NonSysop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DOOR.SYS")
	lines := make([]string, 20)
	lines[9] = "Caller"
	lines[14] = "10"
	body := ""
	for _, l := range lines {
		body += l + "\r\n"
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	sess, err := ParseDoorSYS(path)
	if err != nil {
		t.Fatal(err)
	}
	if sess.IsSysop() {
		t.Fatal("expected non-sysop")
	}
}
