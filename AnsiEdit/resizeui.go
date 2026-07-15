package main

import (
	"fmt"
	"strconv"
	"strings"
)

// canvasSizePresets are offered by Z resize (cols × rows). First entry is default.
var canvasSizePresets = []struct {
	Label string
	Cols  int
	Rows  int
}{
	{"80×25  classic (default)", 80, 25},
	{"40×25  narrow", 40, 25},
	{"80×40  tall", 80, 40},
	{"80×50  double height", 80, 50},
	{"100×40 wide", 100, 40},
	{"132×25 wide terminal", 132, 25},
	{"160×50 max width", 160, 50},
	{"160×100 large", 160, 100},
}

func (e *Editor) runResizeUI() {
	cols, rows := e.term.Size()
	e.term.Clear()
	e.drawBox(1, 1, cols, rows, "Resize canvas")

	e.term.MoveTo(3, 2)
	e.term.Printf("Current size: %d×%d  (cols×rows)", e.canvas.Cols, e.canvas.Rows)
	e.term.MoveTo(4, 2)
	e.term.Print("Choose a size (content is kept where it fits):")

	for i, p := range canvasSizePresets {
		e.term.MoveTo(6+i, 4)
		cur := ""
		if e.canvas.Cols == p.Cols && e.canvas.Rows == p.Rows {
			cur = "  ← current"
		}
		e.term.Printf("[%d] %s%s", i+1, p.Label, cur)
	}
	customIdx := len(canvasSizePresets) + 1
	e.term.MoveTo(6+len(canvasSizePresets), 4)
	e.term.Printf("[%d] Custom width×height", customIdx)
	e.term.MoveTo(6+len(canvasSizePresets)+2, 2)
	e.term.Print("Choice (Esc cancel): ")

	ev, err := e.term.ReadEvent()
	if err != nil || ev.Kind == KeyEsc {
		e.status = "Resize cancelled"
		return
	}
	if ev.Kind != KeyRune {
		e.status = "Resize cancelled"
		return
	}

	choice := int(ev.Rune - '0')
	if ev.Rune >= 'a' && ev.Rune <= 'z' {
		choice = int(ev.Rune-'a') + 10
	}
	var newCols, newRows int
	switch {
	case choice >= 1 && choice <= len(canvasSizePresets):
		p := canvasSizePresets[choice-1]
		newCols, newRows = p.Cols, p.Rows
	case choice == customIdx:
		wStr, ok := e.term.PromptLineEdit(6+len(canvasSizePresets)+4, 2, "Width (cols): ", strconv.Itoa(e.canvas.Cols))
		if !ok {
			e.status = "Resize cancelled"
			return
		}
		hStr, ok := e.term.PromptLineEdit(6+len(canvasSizePresets)+5, 2, "Height (rows): ", strconv.Itoa(e.canvas.Rows))
		if !ok {
			e.status = "Resize cancelled"
			return
		}
		w, errW := strconv.Atoi(strings.TrimSpace(wStr))
		h, errH := strconv.Atoi(strings.TrimSpace(hStr))
		if errW != nil || errH != nil || w < 1 || h < 1 {
			e.popupError("Invalid size", "Enter positive integers for width and height.")
			e.status = "Resize cancelled"
			return
		}
		newCols, newRows = w, h
	default:
		e.status = "Resize cancelled"
		return
	}

	if newCols > maxCols {
		newCols = maxCols
	}
	if newRows > maxRows {
		newRows = maxRows
	}
	if newCols == e.canvas.Cols && newRows == e.canvas.Rows {
		e.status = fmt.Sprintf("Already %d×%d", newCols, newRows)
		return
	}

	e.canvas.Resize(newCols, newRows)
	if e.cx >= e.canvas.Cols {
		e.cx = e.canvas.Cols - 1
	}
	if e.cy >= e.canvas.Rows {
		e.cy = e.canvas.Rows - 1
	}
	if e.cx < 0 {
		e.cx = 0
	}
	if e.cy < 0 {
		e.cy = 0
	}
	e.scrollY = 0
	e.clearUndo()
	if e.sauce.Present {
		e.sauce.TInfo1 = uint16(e.canvas.Cols)
		e.sauce.TInfo2 = uint16(e.canvas.Rows)
	}
	e.dirty = true
	e.status = fmt.Sprintf("Resized to %d×%d", e.canvas.Cols, e.canvas.Rows)
}
