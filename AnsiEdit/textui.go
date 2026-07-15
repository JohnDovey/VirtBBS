package main

import (
	"fmt"
	"strconv"
	"strings"
)

func (e *Editor) runTextInsertUI() {
	cols, rows := e.term.Size()
	e.term.Clear()
	e.drawBox(1, 1, cols, rows, "Insert Text")

	line, ok := e.term.PromptLineEdit(3, 2, "Text: ", "")
	if !ok {
		e.status = "Text insert cancelled"
		return
	}
	text := strings.TrimSpace(line)
	if text == "" {
		e.status = "Text insert cancelled"
		return
	}

	// Font picker
	e.term.MoveTo(5, 2)
	e.term.Print("Font:")
	names := listFontNames()
	for i, n := range names {
		e.term.MoveTo(6+i, 4)
		e.term.Printf("[%d] %s", i+1, n)
	}
	e.term.MoveTo(6+len(names)+1, 2)
	e.term.Print("Choice (1-" + strconv.Itoa(len(names)) + ", default 1): ")
	fontIdx := 0
	ev, err := e.term.ReadEvent()
	if err != nil {
		return
	}
	if ev.Kind == KeyEsc {
		e.status = "Text insert cancelled"
		return
	}
	if ev.Kind == KeyRune && ev.Rune >= '1' && int(ev.Rune-'1') < len(names) {
		fontIdx = int(ev.Rune - '1')
	}
	font := fontByIndex(fontIdx)

	// Size
	sizeStr, ok := e.term.PromptLineEdit(6+len(names)+3, 2, "Size 1-4 (default 1): ", "1")
	if !ok {
		e.status = "Text insert cancelled"
		return
	}
	size := 1
	if n, err := strconv.Atoi(strings.TrimSpace(sizeStr)); err == nil && n >= 1 && n <= 4 {
		size = n
	}

	// Drop shadow
	e.term.MoveTo(6+len(names)+5, 2)
	e.term.Print("Drop shadow? [y/N]: ")
	shadow := false
	ev, err = e.term.ReadEvent()
	if err != nil {
		return
	}
	if ev.Kind == KeyEsc {
		e.status = "Text insert cancelled"
		return
	}
	if ev.Kind == KeyRune && (ev.Rune == 'y' || ev.Rune == 'Y') {
		shadow = true
	}

	grid := font.RenderText(text, size)
	if len(grid) == 0 {
		e.status = "Nothing to insert"
		return
	}
	n := e.stampTextGrid(grid, shadow)
	e.status = fmt.Sprintf("Inserted %q (%s size=%d shadow=%v, %d cells)", text, font.Name, size, shadow, n)
}

// stampTextGrid writes rendered text at the cursor using current FG/BG.
// Shadow uses classic dark gray (8) offset +1,+1 behind the text.
func (e *Editor) stampTextGrid(grid [][]rune, shadow bool) int {
	shadowFG := classicFG(8)
	mainFG := e.fg
	mainBG := e.bg
	var batch []undoEntryCell

	stampCell := func(x, y int, ch rune, fg, bg Color) {
		if ch == ' ' || !e.canvas.InBounds(x, y) {
			return
		}
		batch = append(batch, undoEntryCell{X: x, Y: y, Prev: e.canvas.Get(x, y)})
		e.canvas.Set(x, y, Cell{Ch: ch, FG: fg, BG: bg})
	}

	ox, oy := e.cx, e.cy
	if shadow {
		for y, row := range grid {
			for x, ch := range row {
				if ch == ' ' {
					continue
				}
				stampCell(ox+x+1, oy+y+1, ch, shadowFG, classicBG(0))
			}
		}
	}
	for y, row := range grid {
		for x, ch := range row {
			if ch == ' ' {
				continue
			}
			stampCell(ox+x, oy+y, ch, mainFG, mainBG)
		}
	}

	if len(batch) > 0 {
		e.pushUndoBatch(batch)
		e.dirty = true
	}
	return len(batch)
}
