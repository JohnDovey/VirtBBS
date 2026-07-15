package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	local := flag.Bool("local", false, "run without DOOR.SYS")
	doorfile := flag.String("doorfile", "", "path to DOOR.SYS")
	dataDir := flag.String("data", "", "data directory")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("AnsiArt %s\n", Version)
		return
	}

	wd, _ := os.Getwd()
	base, _ := os.Executable()
	binDir := filepath.Dir(base)
	if *dataDir == "" {
		if _, err := os.Stat(filepath.Join(wd, "config.toml.example")); err == nil {
			*dataDir = wd
		} else {
			*dataDir = binDir
		}
	}
	cfg := LoadConfig(*dataDir)

	var player string
	if *local {
		term := NewTerminal(os.Stdin, os.Stdout)
		defer term.Close()
		term.Clear()
		term.Printf("AnsiArt %s — local mode\r\n", Version)
		player = term.PromptLine("Your name: ")
		if player == "" {
			player = "Local Player"
		}
		Run(term, cfg, player)
		return
	}

	path := ResolveDoorFile(*doorfile, flag.Args())
	if path == "" {
		fmt.Fprintln(os.Stderr, "AnsiArt: no DOOR.SYS (use -local or pass drop file)")
		os.Exit(1)
	}
	sess, err := ParseDoorSYS(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "AnsiArt: %v\n", err)
		os.Exit(1)
	}
	term := NewTerminal(os.Stdin, os.Stdout)
	defer term.Close()
	Run(term, cfg, sess.UserName)
}
