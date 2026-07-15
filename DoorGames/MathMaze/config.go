package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds MathMaze settings.
type Config struct {
	BaseWidth    int
	BaseHeight   int
	BulletinPath string
	TopN         int
}

func defaultConfig(dataDir string) Config {
	return Config{
		BaseWidth:    9,
		BaseHeight:   7,
		BulletinPath: filepath.Join(dataDir, "..", "..", "display", "MATHMAZE.ANS"),
		TopN:         10,
	}
}

// LoadConfig reads an optional simple TOML-like config.toml from dataDir.
func LoadConfig(dataDir string) Config {
	cfg := defaultConfig(dataDir)
	path := filepath.Join(dataDir, "config.toml")
	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"'`)
		switch key {
		case "base_width":
			if n, err := strconv.Atoi(val); err == nil && n >= 5 {
				cfg.BaseWidth = n
			}
		case "base_height":
			if n, err := strconv.Atoi(val); err == nil && n >= 5 {
				cfg.BaseHeight = n
			}
		case "bulletin_path":
			if val != "" {
				if !filepath.IsAbs(val) {
					val = filepath.Join(dataDir, val)
				}
				cfg.BulletinPath = val
			}
		case "top_n":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.TopN = n
			}
		}
	}
	return cfg
}
