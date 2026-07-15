package main

import "fmt"

func (e *Editor) runCharPicker() {
	cols, rows := e.term.Size()
	const perRow = 16
	glyphs := drawGlyphs
	sel := e.glyphIdx
	if sel < 0 || sel >= len(glyphs) {
		sel = 0
	}

	for {
		e.term.Clear()
		e.term.MoveTo(1, 1)
		e.term.Print("\x1b[1;44;97m")
		e.term.Print(padRight(" Character picker — Enter select, Esc cancel ", cols))
		e.term.Print("\x1b[0m")

		startRow := 3
		for i, g := range glyphs {
			r := startRow + i/perRow
			c := 2 + (i%perRow)*4
			if r >= rows-2 {
				break
			}
			e.term.MoveTo(r, c)
			if i == sel {
				e.term.Print("\x1b[1;30;46m")
			}
			e.term.Printf(" %c ", g)
			e.term.Print("\x1b[0m")
		}
		e.term.MoveTo(rows, 1)
		e.term.Print("\x1b[1;43;30m")
		e.term.Print(padRight(fmt.Sprintf(" Selected: %c  U+%04X ", glyphs[sel], glyphs[sel]), cols))
		e.term.Print("\x1b[0m")

		ev, err := e.term.ReadEvent()
		if err != nil {
			return
		}
		switch ev.Kind {
		case KeyEsc:
			return
		case KeyEnter:
			e.glyphIdx = sel
			e.drawCh = glyphs[sel]
			e.status = fmt.Sprintf("Draw glyph %c", e.drawCh)
			return
		case KeyLeft:
			if sel > 0 {
				sel--
			}
		case KeyRight:
			if sel+1 < len(glyphs) {
				sel++
			}
		case KeyUp:
			if sel >= perRow {
				sel -= perRow
			}
		case KeyDown:
			if sel+perRow < len(glyphs) {
				sel += perRow
			}
		}
	}
}

func (e *Editor) runHelp() {
	cols, rows := e.term.Size()
	lines := []string{
		"AnsiEdit " + Version + " — ANSI art editor",
		"",
		"Arrows/Home/End/PgUp/PgDn  Move cursor / scroll",
		"Printable / Space            Paint char / draw glyph",
		"U                            Undo last paint/clear (25 deep)",
		"Backspace/Del                Clear cell",
		"F then 0-9/a-f               Set classic foreground (palette)",
		"B then 0-7                   Set classic background (uppercase B)",
		"Tab / Shift+Tab              Cycle draw glyph",
		"C                            Character picker (uppercase)",
		"S / F2                       Save",
		"O / F3                       Open",
		"I                            Import image (HBFS ANSI / ASCII)",
		"M                            SAUCE + COMNT editor",
		"N                            New canvas",
		"Ctrl+L                       Redraw",
		"Ctrl+Q / Esc Esc             Quit",
		"",
		"Bottom palette: blue █ + white 0-f = FG color keys (F then digit)",
		"",
		"Press any key to return…",
	}
	e.term.Clear()
	for i, ln := range lines {
		if i+1 >= rows {
			break
		}
		e.term.MoveTo(i+1, 2)
		e.term.Print(ln)
	}
	_ = cols
	_, _ = e.term.ReadEvent()
}
