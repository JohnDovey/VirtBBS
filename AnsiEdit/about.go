package main

import (
	"fmt"
	"strings"
)

func (e *Editor) runAbout() {
	cols, rows := e.term.Size()
	e.term.Clear()

	lines := []string{
		"AnsiEdit " + Version,
		"",
		CopyrightNotice,
		"License: " + LicenseName,
		"",
		"GitHub: " + GitHubRepoURL,
		"  (AnsiEdit/ in the VirtBBS repository)",
		"",
	}
	lines = append(lines, wrapWords(AboutBlurb, cols-4)...)
	lines = append(lines,
		"",
		"Features: paint · palette · undo · image import · SAUCE/COMNT · ANSI fonts",
		"",
		"Press any key to return…",
	)

	e.term.MoveTo(1, 1)
	e.term.Print("\x1b[1;44;97m")
	e.term.Print(padRight(" About AnsiEdit ", cols))
	e.term.Print("\x1b[0m")

	for i, ln := range lines {
		row := 3 + i
		if row >= rows {
			break
		}
		e.term.MoveTo(row, 2)
		if strings.HasPrefix(ln, "Copyright") || strings.HasPrefix(ln, "GitHub:") {
			e.term.Print("\x1b[1;96m")
			e.term.Print(ln)
			e.term.Print("\x1b[0m")
		} else if i == 0 {
			e.term.Print("\x1b[1;97m")
			e.term.Print(ln)
			e.term.Print("\x1b[0m")
		} else {
			e.term.Print(ln)
		}
	}
	_, _ = e.term.ReadEvent()
}

func printVersion() {
	fmt.Printf("AnsiEdit %s\n", Version)
	fmt.Println(CopyrightNotice)
	fmt.Printf("%s\n", GitHubRepoURL)
}
