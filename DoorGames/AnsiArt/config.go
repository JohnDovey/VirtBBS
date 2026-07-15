package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds AnsiArt door settings.
type Config struct {
	LibraryDir   string
	BulletinPath string
	DefaultWidth int
}

func defaultConfig(dataDir string) Config {
	return Config{
		LibraryDir:   filepath.Join(dataDir, "LIBRARY"),
		BulletinPath: filepath.Join(dataDir, "..", "..", "display", "ANSIART.ANS"),
		DefaultWidth: 80,
	}
}

func LoadConfig(dataDir string) Config {
	cfg := defaultConfig(dataDir)
	f, err := os.Open(filepath.Join(dataDir, "config.toml"))
	if err != nil {
		return cfg
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch key {
		case "library_dir":
			if val != "" {
				if !filepath.IsAbs(val) {
					val = filepath.Join(dataDir, val)
				}
				cfg.LibraryDir = val
			}
		case "bulletin_path":
			if val != "" {
				if !filepath.IsAbs(val) {
					val = filepath.Join(dataDir, val)
				}
				cfg.BulletinPath = val
			}
		case "default_width":
			if n, err := strconv.Atoi(val); err == nil && n >= 20 {
				cfg.DefaultWidth = n
			}
		}
	}
	return cfg
}
