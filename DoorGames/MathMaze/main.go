package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	local := flag.Bool("local", false, "run without DOOR.SYS (local testing)")
	doorfile := flag.String("doorfile", "", "path to DOOR.SYS drop file")
	dataDir := flag.String("data", "", "data directory for scores/config (default: binary dir)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("MathMaze %s\n", Version)
		return
	}

	base, err := os.Executable()
	if err != nil {
		base = "."
	}
	binDir := filepath.Dir(base)
	// When go run'd, prefer working directory for data
	wd, _ := os.Getwd()
	if *dataDir == "" {
		if _, err := os.Stat(filepath.Join(wd, "scores.json")); err == nil {
			*dataDir = wd
		} else if _, err := os.Stat(filepath.Join(wd, "config.toml")); err == nil {
			*dataDir = wd
		} else if _, err := os.Stat(filepath.Join(wd, "config.toml.example")); err == nil {
			*dataDir = wd
		} else {
			*dataDir = binDir
		}
	}

	cfg := LoadConfig(*dataDir)
	scores := LoadScores(*dataDir)

	var player string
	if *local {
		term := NewTerminal(os.Stdin, os.Stdout)
		defer term.Close()
		term.Clear()
		term.Printf("MathMaze %s — local mode\r\n", Version)
		player = term.PromptLine("Your name: ")
		if player == "" {
			player = "Local Player"
		}
		Run(term, cfg, scores, player)
		return
	}

	path := ResolveDoorFile(*doorfile, flag.Args())
	if path == "" {
		fmt.Fprintln(os.Stderr, "MathMaze: no DOOR.SYS found (use -local or pass drop file)")
		os.Exit(1)
	}
	sess, err := ParseDoorSYS(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MathMaze: read drop file: %v\n", err)
		os.Exit(1)
	}
	player = sess.UserName

	term := NewTerminal(os.Stdin, os.Stdout)
	defer term.Close()
	Run(term, cfg, scores, player)
}
