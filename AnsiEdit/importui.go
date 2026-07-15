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
	pathDraft := ""

	for {
		e.term.Clear()
		e.drawBox(1, 1, termCols, termRows, "Import Image")
		e.term.MoveTo(termRows, 1)
		e.term.Print("\x1b[1;40;96m")
		e.term.Print(padRight(" Enter=ok  Esc=cancel  path keeps text after errors ", termCols))
		e.term.Print("\x1b[0m")

		line, ok := e.term.PromptLineEdit(3, 2, "Image path: ", pathDraft)
		if !ok {
			e.status = "Import cancelled"
			return
		}
		pathDraft = strings.TrimSpace(line)
		if pathDraft == "" {
			e.status = "Import cancelled"
			return
		}

		path := expandHome(pathDraft)
		img, err := LoadImage(path)
		if err != nil {
			e.popupError("Cannot open image", err.Error())
			// pathDraft already holds the untrimmed-then-trimmed input; keep for edit
			pathDraft = strings.TrimSpace(line)
			continue
		}

		e.term.Clear()
		e.drawBox(1, 1, termCols, termRows, "Import Image")
		e.term.MoveTo(3, 2)
		e.term.Printf("File: %s", path)

		e.term.MoveTo(5, 2)
		e.term.Print("Mode: [1] ANSI (HBFS truecolor)  [2] ASCII ramp")
		e.term.MoveTo(6, 2)
		e.term.Print("Choice: ")
		mode := ImportANSI
		ev, err := e.term.ReadEvent()
		if err != nil {
			return
		}
		if ev.Kind == KeyEsc {
			e.status = "Import cancelled"
			return
		}
		if ev.Kind == KeyRune && ev.Rune == '2' {
			mode = ImportASCII
		}

		maxW := 80
		maxH := 25
		widthStr := e.promptAt(8, 2, "Max width in cells (default 80): ", 0)
		if widthStr != "" {
			if n, err := strconv.Atoi(strings.TrimSpace(widthStr)); err == nil && n > 0 {
				maxW = n
			}
		}
		heightStr := e.promptAt(9, 2, "Max height in cells (default 25): ", 0)
		if heightStr != "" {
			if n, err := strconv.Atoi(strings.TrimSpace(heightStr)); err == nil && n > 0 {
				maxH = n
			}
		}
		if maxW < 8 {
			maxW = 8
		}
		if maxW > maxCols {
			maxW = maxCols
		}
		if maxH < 1 {
			maxH = 1
		}
		if maxH > maxRows {
			maxH = maxRows
		}

		if e.dirty {
			e.term.MoveTo(11, 2)
			e.term.Print("Replace current canvas? [y/N]: ")
			ev, err = e.term.ReadEvent()
			if err != nil || !(ev.Kind == KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y')) {
				e.status = "Import cancelled"
				return
			}
		}

		e.term.MoveTo(13, 2)
		e.term.Print("Scaling + converting (fit entire image)...")
		c := ConvertImage(img, mode, maxW, maxH)
		e.canvas = c
		e.cx, e.cy = 0, 0
		e.scrollY = 0
		e.clearUndo()
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
		e.status = fmt.Sprintf("Imported %s → %dx%d (fit in %dx%d)", filepath.Base(path), c.Cols, c.Rows, maxW, maxH)
		return
	}
}

// popupError shows a modal message and waits for a key.
func (e *Editor) popupError(title, msg string) {
	cols, rows := e.term.Size()
	if cols < 40 {
		cols = 40
	}
	boxW := cols - 8
	if boxW > 72 {
		boxW = 72
	}
	if boxW < 30 {
		boxW = cols - 2
	}
	lines := wrapWords(msg, boxW-4)
	boxH := 4 + len(lines)
	top := (rows - boxH) / 2
	if top < 2 {
		top = 2
	}
	left := (cols - boxW) / 2
	if left < 1 {
		left = 1
	}

	e.term.MoveTo(top, left)
	e.term.Print("\x1b[1;41;97m")
	e.term.Print(padRight(" "+title+" ", boxW))
	e.term.Print("\x1b[0m")
	for i := 0; i < len(lines); i++ {
		e.term.MoveTo(top+1+i, left)
		e.term.Print("\x1b[1;47;30m")
		e.term.Print(padRight(" "+lines[i]+" ", boxW))
		e.term.Print("\x1b[0m")
	}
	e.term.MoveTo(top+1+len(lines), left)
	e.term.Print("\x1b[1;43;30m")
	e.term.Print(padRight(" Press any key to edit path… ", boxW))
	e.term.Print("\x1b[0m")
	_, _ = e.term.ReadEvent()
}

func wrapWords(s string, width int) []string {
	if width < 8 {
		width = 8
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{s}
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) <= width {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	lines = append(lines, cur)
	return lines
}

func (e *Editor) promptAt(row, col int, prompt string, _ int) string {
	line, ok := e.term.PromptLineEdit(row, col, prompt, "")
	if !ok {
		return ""
	}
	return strings.TrimSpace(line)
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
