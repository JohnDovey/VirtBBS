package fido

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveDotBinary_bundled(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exeDir := filepath.Dir(exe)
	name := graphvizDotName()
	bundled := filepath.Join(exeDir, "graphviz", "bin", name)

	origOverride := graphvizDotOverride
	t.Cleanup(func() { graphvizDotOverride = origOverride })

	graphvizDotOverride = ""
	if !usableBinary(bundled) {
		t.Skip("no bundled graphviz in test environment")
	}

	got, err := ResolveDotBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got != bundled {
		t.Fatalf("got %q want bundled %q", got, bundled)
	}
}

func TestResolveDotBinary_override(t *testing.T) {
	dir := t.TempDir()
	dot := filepath.Join(dir, graphvizDotName())
	if err := os.WriteFile(dot, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// Windows does not use execute bits; file exists is enough.
	} else if err := os.Chmod(dot, 0755); err != nil {
		t.Fatal(err)
	}

	origOverride := graphvizDotOverride
	t.Cleanup(func() { graphvizDotOverride = origOverride })

	SetGraphvizDotPath(dot)
	got, err := ResolveDotBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got != dot {
		t.Fatalf("got %q want %q", got, dot)
	}
}

func TestGraphvizBundleRoot(t *testing.T) {
	root := graphvizBundleRoot("/opt/virtbbs/graphviz/bin/dot")
	if root != "/opt/virtbbs/graphviz" && root != filepath.FromSlash("/opt/virtbbs/graphviz") {
		t.Fatalf("unexpected root %q", root)
	}
	if graphvizBundleRoot("/usr/bin/dot") != "" {
		t.Fatal("system dot should not have bundle root")
	}
}
