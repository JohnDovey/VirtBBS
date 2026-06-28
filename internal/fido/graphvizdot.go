// Package fido — graphvizdot.go
//
// Locates Graphviz's dot binary: optional config path, bundled copy next to
// the virtbbs executable (graphviz/bin/dot), then PATH.
package fido

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var graphvizDotOverride string

// SetGraphvizDotPath sets an explicit dot binary path from VirtBBS.DAT
// (paths.graphviz_dot). Pass "" to clear.
func SetGraphvizDotPath(path string) {
	graphvizDotOverride = strings.TrimSpace(path)
}

// ResolveDotBinary returns the dot executable to use for diagram rendering.
// Search order: config override, bundled graphviz/ next to virtbbs, then PATH.
func ResolveDotBinary() (string, error) {
	if graphvizDotOverride != "" {
		if st, err := os.Stat(graphvizDotOverride); err != nil {
			return "", fmt.Errorf("paths.graphviz_dot %q: %w", graphvizDotOverride, err)
		} else if st.IsDir() {
			return "", fmt.Errorf("paths.graphviz_dot %q is a directory", graphvizDotOverride)
		}
		return graphvizDotOverride, nil
	}
	name := graphvizDotName()
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		for _, cand := range bundledDotCandidates(exeDir, name) {
			if usableBinary(cand) {
				return cand, nil
			}
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		for _, cand := range bundledDotCandidates(cwd, name) {
			if usableBinary(cand) {
				return cand, nil
			}
		}
	}
	if path, err := exec.LookPath("dot"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("graphviz dot not found — bundle it in graphviz/bin/%s next to virtbbs, set paths.graphviz_dot, or install Graphviz on PATH", name)
}

func graphvizDotName() string {
	if runtime.GOOS == "windows" {
		return "dot.exe"
	}
	return "dot"
}

func bundledDotCandidates(baseDir, name string) []string {
	return []string{
		filepath.Join(baseDir, "graphviz", "bin", name),
		filepath.Join(baseDir, "graphviz", name),
	}
}

func usableBinary(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return st.Mode()&0111 != 0
}

func graphvizBundleRoot(dotPath string) string {
	slash := filepath.ToSlash(filepath.Clean(dotPath))
	if idx := strings.Index(slash, "/graphviz/"); idx >= 0 {
		return filepath.FromSlash(slash[:idx+len("/graphviz")])
	}
	return ""
}

// prepareDotCmd builds an exec.Cmd for dot with library paths when using a
// bundled graphviz/ tree (graphviz/bin/dot + graphviz/lib/*.so|dylib).
func prepareDotCmd(dotPath string, args ...string) *exec.Cmd {
	cmd := exec.Command(dotPath, args...)
	root := graphvizBundleRoot(dotPath)
	if root == "" {
		return cmd
	}
	binDir := filepath.Join(root, "bin")
	libDir := filepath.Join(root, "lib")
	if st, err := os.Stat(binDir); err == nil && st.IsDir() {
		cmd.Dir = binDir
	}
	if st, err := os.Stat(libDir); err == nil && st.IsDir() {
		switch runtime.GOOS {
		case "linux":
			cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+prependLibPath("LD_LIBRARY_PATH", libDir))
		case "darwin":
			cmd.Env = append(os.Environ(), "DYLD_LIBRARY_PATH="+prependLibPath("DYLD_LIBRARY_PATH", libDir))
		}
	}
	return cmd
}

func prependLibPath(envKey, dir string) string {
	if prev := os.Getenv(envKey); prev != "" {
		return dir + string(os.PathListSeparator) + prev
	}
	return dir
}
