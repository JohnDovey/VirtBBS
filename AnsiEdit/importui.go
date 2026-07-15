package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (e *Editor) runImportUI() {
	termCols, termRows := e.term.Size()
	e.term.Clear()
	e.drawBox(1, 1, termCols, termRows, "Import Image")

	path := e.promptAt(3, 2, "Image path: ", 0)
	if path == "" {
		e.status = "Import cancelled"
		return
	}
	path = expandHome(path)

	e.term.MoveTo(5, 2)
	e.term.Print("Mode: [1] ANSI (HBFS truecolor)  [2] ASCII ramp")
	e.term.MoveTo(6, 2)
	e.term.Print("Choice: ")
	mode := ImportANSI
	ev, err := e.term.ReadEvent()
	if err != nil {
		return
	}
	if ev.Kind == KeyRune && ev.Rune == '2' {
		mode = ImportASCII
	}

	widthStr := e.promptAt(8, 2, "Width in cells [40/80/160] (default 80): ", 0)
	width := 80
	if widthStr != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(widthStr)); err == nil && n > 0 {
			width = n
		}
	}
	if width < 8 {
		width = 8
	}
	if width > maxCols {
		width = maxCols
	}

	if e.dirty {
		e.term.MoveTo(10, 2)
		e.term.Print("Replace current canvas? [y/N]: ")
		ev, err = e.term.ReadEvent()
		if err != nil || !(ev.Kind == KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y')) {
			e.status = "Import cancelled"
			return
		}
	}

	e.term.MoveTo(12, 2)
	e.term.Print("Converting...")
	img, err := LoadImage(path)
	if err != nil {
		e.status = fmt.Sprintf("Import failed: %v", err)
		return
	}
	c := ConvertImage(img, mode, width)
	e.canvas = c
	e.cx, e.cy = 0, 0
	e.scrollY = 0
	PrefillSauceForImport(&e.sauce, path, c, mode)
	e.dirty = true
	if e.path == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if mode == ImportASCII {
			e.path = base + ".ASC"
		} else {
			e.path = base + ".ANS"
		}
	}
	e.status = fmt.Sprintf("Imported %s → %dx%d", filepath.Base(path), c.Cols, c.Rows)
}

func (e *Editor) promptAt(row, col int, prompt string, _ int) string {
	e.term.MoveTo(row, col)
	e.term.Print(prompt)
	e.term.ShowCursor()
	line := e.term.PromptLine("")
	e.term.HideCursor()
	return line
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
