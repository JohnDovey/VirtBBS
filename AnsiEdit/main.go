package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	version := flag.Bool("version", false, "print version and exit")
	rows := flag.Int("rows", defRows, "rows for new canvas")
	cols := flag.Int("cols", defCols, "columns for new canvas")
	flag.Parse()

	if *version {
		printVersion()
		os.Exit(0)
	}

	term := NewTerminal(os.Stdin, os.Stdout)
	defer term.Close()

	if !term.raw {
		fmt.Fprintln(os.Stderr, "AnsiEdit requires a TTY (interactive terminal).")
		os.Exit(1)
	}

	var (
		canvas *Canvas
		sauce  Sauce
		path   string
	)

	args := flag.Args()
	if len(args) > 0 {
		path = args[0]
		c, s, err := LoadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load %s: %v\n", path, err)
			os.Exit(1)
		}
		canvas, sauce = c, s
	} else {
		canvas = NewCanvas(*cols, *rows)
	}

	ed := NewEditor(term, canvas, path, sauce)
	if err := ed.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "editor: %v\n", err)
		os.Exit(1)
	}
}
