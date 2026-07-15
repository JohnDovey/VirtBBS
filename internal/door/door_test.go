package door

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDoorCmd(t *testing.T) {
	root := t.TempDir()
	doorDir := filepath.Join(root, "DoorGames", "MathMaze")
	if err := os.MkdirAll(doorDir, 0755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(doorDir, "mathmaze")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho ok\n"), 0755); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	// Relative path from BBS root + work_dir (the previous failure mode).
	got, err := resolveDoorCmd("DoorGames/MathMaze/mathmaze", doorDir)
	if err != nil {
		t.Fatalf("resolve relative cmd: %v", err)
	}
	got, _ = filepath.EvalSymlinks(got)
	want, _ := filepath.EvalSymlinks(bin)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}

	got, err = resolveDoorCmd("./mathmaze", doorDir)
	if err != nil {
		t.Fatalf("resolve basename: %v", err)
	}
	got, _ = filepath.EvalSymlinks(got)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}