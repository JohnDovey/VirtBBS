package main

import "fmt"

const (
	maxCols = 160
	maxRows = 200
	defCols = 80
	defRows = 25
)

// Color is either a classic ANSI index or truecolor RGB.
type Color struct {
	True bool
	Idx  uint8 // 0–15 FG / 0–7 BG when !True
	R, G, B uint8
}

func classicFG(idx uint8) Color { return Color{Idx: idx & 15} }
func classicBG(idx uint8) Color { return Color{Idx: idx & 7} }
func rgbColor(r, g, b uint8) Color {
	return Color{True: true, R: r, G: g, B: b}
}

func (c Color) Equal(o Color) bool {
	if c.True != o.True {
		return false
	}
	if c.True {
		return c.R == o.R && c.G == o.G && c.B == o.B
	}
	return c.Idx == o.Idx
}

// Cell is one character cell on the canvas.
type Cell struct {
	Ch rune
	FG Color
	BG Color
}

func blankCell() Cell {
	return Cell{Ch: ' ', FG: classicFG(7), BG: classicBG(0)}
}

// Canvas is a rectangular art buffer.
type Canvas struct {
	Cols, Rows int
	Cells      []Cell // row-major
}

func NewCanvas(cols, rows int) *Canvas {
	if cols < 1 {
		cols = defCols
	}
	if rows < 1 {
		rows = defRows
	}
	if cols > maxCols {
		cols = maxCols
	}
	if rows > maxRows {
		rows = maxRows
	}
	c := &Canvas{Cols: cols, Rows: rows, Cells: make([]Cell, cols*rows)}
	for i := range c.Cells {
		c.Cells[i] = blankCell()
	}
	return c
}

func (c *Canvas) idx(x, y int) int { return y*c.Cols + x }

func (c *Canvas) InBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < c.Cols && y < c.Rows
}

func (c *Canvas) Get(x, y int) Cell {
	if !c.InBounds(x, y) {
		return blankCell()
	}
	return c.Cells[c.idx(x, y)]
}

func (c *Canvas) Set(x, y int, cell Cell) {
	if !c.InBounds(x, y) {
		return
	}
	if cell.Ch == 0 {
		cell.Ch = ' '
	}
	c.Cells[c.idx(x, y)] = cell
}

func (c *Canvas) Clear() {
	for i := range c.Cells {
		c.Cells[i] = blankCell()
	}
}

func (c *Canvas) Resize(cols, rows int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if cols > maxCols {
		cols = maxCols
	}
	if rows > maxRows {
		rows = maxRows
	}
	n := NewCanvas(cols, rows)
	for y := 0; y < rows && y < c.Rows; y++ {
		for x := 0; x < cols && x < c.Cols; x++ {
			n.Set(x, y, c.Get(x, y))
		}
	}
	*c = *n
}

func (c Color) String() string {
	if c.True {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	names := []string{"Blk", "Red", "Grn", "Yel", "Blu", "Mag", "Cyn", "Wht",
		"BBlk", "BRed", "BGrn", "BYel", "BBlu", "BMag", "BCyn", "BWht"}
	if int(c.Idx) < len(names) {
		return names[c.Idx]
	}
	return fmt.Sprintf("%d", c.Idx)
}

// classicPalette maps VGA index 0–15 to approximate RGB for truecolor emit/display.
var classicPalette = [16][3]uint8{
	{0, 0, 0}, {170, 0, 0}, {0, 170, 0}, {170, 85, 0},
	{0, 0, 170}, {170, 0, 170}, {0, 170, 170}, {170, 170, 170},
	{85, 85, 85}, {255, 85, 85}, {85, 255, 85}, {255, 255, 85},
	{85, 85, 255}, {255, 85, 255}, {85, 255, 255}, {255, 255, 255},
}

func (c Color) RGB() (r, g, b uint8) {
	if c.True {
		return c.R, c.G, c.B
	}
	p := classicPalette[c.Idx&15]
	return p[0], p[1], p[2]
}
