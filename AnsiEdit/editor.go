package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// Editor is the fullscreen ANSI art editor.
type Editor struct {
	term     *Terminal
	canvas   *Canvas
	sauce    Sauce
	path     string
	dirty    bool
	cx, cy   int
	scrollY  int
	fg       Color
	bg       Color
	drawCh   rune
	glyphIdx int
	status   string
	pending  rune // 'F' or 'B' waiting for digit
	escCount int
	quit     bool
}

func NewEditor(term *Terminal, c *Canvas, path string, sauce Sauce) *Editor {
	if c == nil {
		c = NewCanvas(defCols, defRows)
	}
	return &Editor{
		term:     term,
		canvas:   c,
		sauce:    sauce,
		path:     path,
		fg:       classicFG(15),
		bg:       classicBG(0),
		drawCh:   '█',
		glyphIdx: 0,
	}
}

func (e *Editor) Run() error {
	e.term.HideCursor()
	defer e.term.ShowCursor()
	e.redraw()
	for !e.quit {
		ev, err := e.term.ReadEvent()
		if err != nil {
			return err
		}
		e.handle(ev)
		if !e.quit {
			e.redraw()
		}
	}
	e.term.Clear()
	return nil
}

func (e *Editor) viewportRows() int {
	_, rows := e.term.Size()
	vr := rows - 3 // title, help, status
	if vr < 1 {
		vr = 1
	}
	return vr
}

func (e *Editor) redraw() {
	cols, rows := e.term.Size()
	vr := e.viewportRows()

	// Ensure scroll keeps cursor visible
	if e.cy < e.scrollY {
		e.scrollY = e.cy
	}
	if e.cy >= e.scrollY+vr {
		e.scrollY = e.cy - vr + 1
	}
	if e.scrollY < 0 {
		e.scrollY = 0
	}

	name := e.path
	if name == "" {
		name = "(untitled)"
	}
	dirty := ""
	if e.dirty {
		dirty = " *"
	}
	title := fmt.Sprintf(" AnsiEdit %s  %s%s  %d,%d  %dx%d ",
		Version, filepath.Base(name), dirty, e.cx+1, e.cy+1, e.canvas.Cols, e.canvas.Rows)
	e.term.MoveTo(1, 1)
	e.term.Print("\x1b[1;44;97m")
	e.term.Print(padRight(title, cols))
	e.term.Print("\x1b[0m")

	help := " Arrows|Space paint|F/B color|Tab glyph|C picker|S save|O open|I import|M SAUCE|?=help (cmds UPPERCASE) "
	e.term.MoveTo(2, 1)
	e.term.Print("\x1b[1;40;96m")
	e.term.Print(padRight(help, cols))
	e.term.Print("\x1b[0m")

	viewW := cols
	if viewW > e.canvas.Cols {
		viewW = e.canvas.Cols
	}

	for row := 0; row < vr; row++ {
		sy := e.scrollY + row
		termRow := 3 + row
		e.term.MoveTo(termRow, 1)
		if sy >= e.canvas.Rows {
			e.term.Print("\x1b[0m\x1b[K")
			continue
		}
		var lastFG, lastBG *Color
		for x := 0; x < viewW; x++ {
			cell := e.canvas.Get(x, sy)
			onCursor := x == e.cx && sy == e.cy
			fg, bg := cell.FG, cell.BG
			if onCursor {
				// Invert for cursor visibility
				fg, bg = bg, fg
				if !fg.True && !bg.True && fg.Equal(bg) {
					fg = classicFG(15)
					bg = classicBG(4)
				}
			}
			need := lastFG == nil || !lastFG.Equal(fg) || lastBG == nil || !lastBG.Equal(bg)
			if need {
				e.term.Print(sgrSeq(fg, bg))
				f, b := fg, bg
				lastFG, lastBG = &f, &b
			}
			ch := cell.Ch
			if ch == 0 {
				ch = ' '
			}
			e.term.Printf("%c", ch)
		}
		e.term.Print("\x1b[0m\x1b[K")
	}

	status := e.status
	if status == "" {
		status = fmt.Sprintf("FG=%s BG=%s glyph=%c", e.fg.String(), e.bg.String(), e.drawCh)
		if e.sauce.Present {
			status += " SAUCE"
		}
	}
	e.term.MoveTo(rows, 1)
	e.term.Print("\x1b[1;43;30m")
	e.term.Print(padRight(" "+status+" ", cols))
	e.term.Print("\x1b[0m")
	e.status = ""
}

func sgrSeq(fg, bg Color) string {
	var parts []string
	parts = append(parts, "0")
	if fg.True {
		parts = append(parts, fmt.Sprintf("38;2;%d;%d;%d", fg.R, fg.G, fg.B))
	} else {
		idx := fg.Idx
		if idx >= 8 {
			parts = append(parts, "1", fmt.Sprintf("%d", 30+int(idx)-8))
		} else {
			parts = append(parts, fmt.Sprintf("%d", 30+idx))
		}
	}
	if bg.True {
		parts = append(parts, fmt.Sprintf("48;2;%d;%d;%d", bg.R, bg.G, bg.B))
	} else {
		parts = append(parts, fmt.Sprintf("%d", 40+bg.Idx))
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

func (e *Editor) handle(ev Event) {
	if e.pending != 0 {
		e.handleColorDigit(ev)
		return
	}

	switch ev.Kind {
	case KeyCtrlQ:
		e.tryQuit()
	case KeyEsc:
		e.escCount++
		if e.escCount >= 2 {
			e.tryQuit()
		} else {
			e.status = "Press Esc again to quit"
		}
		return
	case KeyCtrlL:
		// redraw only
	case KeyCtrlS, KeyF2:
		e.doSave()
	case KeyF3:
		e.doOpen()
	case KeyF1:
		e.runHelp()
	case KeyUp:
		if e.cy > 0 {
			e.cy--
		}
	case KeyDown:
		if e.cy+1 < e.canvas.Rows {
			e.cy++
		}
	case KeyLeft:
		if e.cx > 0 {
			e.cx--
		}
	case KeyRight:
		if e.cx+1 < e.canvas.Cols {
			e.cx++
		}
	case KeyHome:
		e.cx = 0
	case KeyEnd:
		e.cx = e.canvas.Cols - 1
	case KeyPgUp:
		e.cy -= e.viewportRows()
		if e.cy < 0 {
			e.cy = 0
		}
	case KeyPgDn:
		e.cy += e.viewportRows()
		if e.cy >= e.canvas.Rows {
			e.cy = e.canvas.Rows - 1
		}
	case KeyTab:
		e.glyphIdx = (e.glyphIdx + 1) % len(drawGlyphs)
		e.drawCh = drawGlyphs[e.glyphIdx]
	case KeyShiftTab:
		e.glyphIdx--
		if e.glyphIdx < 0 {
			e.glyphIdx = len(drawGlyphs) - 1
		}
		e.drawCh = drawGlyphs[e.glyphIdx]
	case KeyBackspace, KeyDelete:
		e.canvas.Set(e.cx, e.cy, blankCell())
		e.dirty = true
		if ev.Kind == KeyBackspace && e.cx > 0 {
			e.cx--
		}
	case KeyRune:
		e.handleRune(ev.Rune)
	}
	e.escCount = 0
}

func (e *Editor) handleRune(r rune) {
	// Letter commands are uppercase / ? so lowercase letters still paint.
	switch r {
	case '?':
		e.runHelp()
		return
	case 'C':
		e.runCharPicker()
		return
	case 'F':
		e.pending = 'F'
		e.status = "Foreground: type 0-9 or a-f (0-15)"
		return
	case 'B':
		e.pending = 'B'
		e.status = "Background: type 0-7"
		return
	case 'S':
		e.doSave()
		return
	case 'O':
		e.doOpen()
		return
	case 'I':
		e.runImportUI()
		return
	case 'M':
		e.runSauceUI()
		return
	case 'N':
		e.doNew()
		return
	case ' ':
		e.paint(e.drawCh)
		return
	}
	if unicode.IsPrint(r) {
		e.paint(r)
	}
}

func (e *Editor) handleColorDigit(ev Event) {
	pend := e.pending
	e.pending = 0
	if ev.Kind != KeyRune {
		e.status = "Color cancelled"
		return
	}
	r := unicode.ToLower(ev.Rune)
	var n int
	switch {
	case r >= '0' && r <= '9':
		n = int(r - '0')
	case r >= 'a' && r <= 'f':
		n = int(r-'a') + 10
	default:
		e.status = "Invalid color"
		return
	}
	if pend == 'F' {
		if n > 15 {
			n = 15
		}
		e.fg = classicFG(uint8(n))
		e.status = fmt.Sprintf("FG=%s", e.fg.String())
	} else {
		if n > 7 {
			n = 7
		}
		e.bg = classicBG(uint8(n))
		e.status = fmt.Sprintf("BG=%s", e.bg.String())
	}
}

func (e *Editor) paint(ch rune) {
	e.canvas.Set(e.cx, e.cy, Cell{Ch: ch, FG: e.fg, BG: e.bg})
	e.dirty = true
	if e.cx+1 < e.canvas.Cols {
		e.cx++
	}
}

func (e *Editor) tryQuit() {
	if e.dirty {
		cols, rows := e.term.Size()
		e.term.MoveTo(rows, 1)
		e.term.Print("\x1b[1;41;97m")
		e.term.Print(padRight(" Unsaved changes! Quit anyway? [y/N] ", cols))
		e.term.Print("\x1b[0m")
		ev, err := e.term.ReadEvent()
		if err != nil || !(ev.Kind == KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y')) {
			e.status = "Quit cancelled"
			e.escCount = 0
			return
		}
	}
	e.quit = true
}

func (e *Editor) doSave() {
	path := e.path
	if path == "" {
		e.term.MoveTo(e.termRows(), 1)
		e.term.Print("\x1b[K Save as: ")
		path = e.term.PromptLine("")
		if path == "" {
			e.status = "Save cancelled"
			return
		}
		path = expandHome(path)
		if !hasAnsExt(path) {
			path += ".ANS"
		}
	}
	if err := SaveFile(path, e.canvas, e.sauce, false); err != nil {
		e.status = fmt.Sprintf("Save failed: %v", err)
		return
	}
	e.path = path
	e.dirty = false
	e.status = "Saved " + filepath.Base(path)
}

func (e *Editor) doOpen() {
	if e.dirty {
		e.term.MoveTo(e.termRows(), 1)
		e.term.Print("\x1b[K Discard changes and open? [y/N] ")
		ev, _ := e.term.ReadEvent()
		if !(ev.Kind == KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y')) {
			e.status = "Open cancelled"
			return
		}
	}
	e.term.MoveTo(e.termRows(), 1)
	e.term.Print("\x1b[K Open: ")
	path := e.term.PromptLine("")
	if path == "" {
		e.status = "Open cancelled"
		return
	}
	path = expandHome(path)
	c, sauce, err := LoadFile(path)
	if err != nil {
		e.status = fmt.Sprintf("Open failed: %v", err)
		return
	}
	e.canvas = c
	e.sauce = sauce
	e.path = path
	e.cx, e.cy, e.scrollY = 0, 0, 0
	e.dirty = false
	e.status = "Loaded " + filepath.Base(path)
}

func (e *Editor) doNew() {
	if e.dirty {
		e.term.MoveTo(e.termRows(), 1)
		e.term.Print("\x1b[K Discard and new? [y/N] ")
		ev, _ := e.term.ReadEvent()
		if !(ev.Kind == KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y')) {
			e.status = "New cancelled"
			return
		}
	}
	e.canvas = NewCanvas(defCols, defRows)
	e.sauce = Sauce{}
	e.path = ""
	e.cx, e.cy, e.scrollY = 0, 0, 0
	e.dirty = false
	e.status = "New canvas 80x25"
}

func (e *Editor) drawBox(_, _, cols, rows int, title string) {
	e.term.MoveTo(1, 1)
	e.term.Print("\x1b[1;44;97m")
	e.term.Print(padRight(" "+title+" ", cols))
	e.term.Print("\x1b[0m")
	_ = rows
}

func hasAnsExt(path string) bool {
	ext := strings.ToUpper(filepath.Ext(path))
	return ext == ".ANS" || ext == ".ASC" || ext == ".TXT"
}
