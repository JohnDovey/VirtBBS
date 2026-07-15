package main

import (
	"strconv"
	"strings"
)

// ParseANSI loads an ANSI/ASCII string into a new canvas.
func ParseANSI(text string, defaultCols, defaultRows int) *Canvas {
	text = expandPCBAnsi(text)
	cols := defaultCols
	if cols < 1 {
		cols = defCols
	}
	rows := defaultRows
	if rows < 1 {
		rows = defRows
	}
	// First pass: discover max cursor position
	cx, cy := 0, 0
	maxX, maxY := cols-1, rows-1
	savedX, savedY := 0, 0
	fg := classicFG(7)
	bg := classicBG(0)
	bold := false

	applySGR := func(params []int) {
		if len(params) == 0 {
			params = []int{0}
		}
		for i := 0; i < len(params); i++ {
			p := params[i]
			switch {
			case p == 0:
				fg, bg, bold = classicFG(7), classicBG(0), false
			case p == 1:
				bold = true
				if !fg.True && fg.Idx < 8 {
					fg.Idx += 8
				}
			case p == 22:
				bold = false
				if !fg.True && fg.Idx >= 8 {
					fg.Idx -= 8
				}
			case p == 39:
				fg = classicFG(7)
				if bold {
					fg.Idx = 15
				}
			case p == 49:
				bg = classicBG(0)
			case p >= 30 && p <= 37:
				idx := uint8(p - 30)
				if bold {
					idx += 8
				}
				fg = classicFG(idx)
			case p >= 40 && p <= 47:
				bg = classicBG(uint8(p - 40))
			case p >= 90 && p <= 97:
				fg = classicFG(uint8(p-90) + 8)
			case p >= 100 && p <= 107:
				bg = classicBG(uint8(p - 100)) // bright BG approximated as 0–7
			case p == 38 && i+1 < len(params):
				if params[i+1] == 2 && i+4 < len(params) {
					fg = rgbColor(uint8(params[i+2]), uint8(params[i+3]), uint8(params[i+4]))
					i += 4
				} else if params[i+1] == 5 && i+2 < len(params) {
					fg = classicFG(uint8(params[i+2] & 15))
					i += 2
				}
			case p == 48 && i+1 < len(params):
				if params[i+1] == 2 && i+4 < len(params) {
					bg = rgbColor(uint8(params[i+2]), uint8(params[i+3]), uint8(params[i+4]))
					i += 4
				} else if params[i+1] == 5 && i+2 < len(params) {
					bg = classicBG(uint8(params[i+2] & 7))
					i += 2
				}
			}
		}
	}

	// Measure bounds
	i := 0
	runes := []rune(text)
	for i < len(runes) {
		r := runes[i]
		if r == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			j := i + 2
			for j < len(runes) && !((runes[j] >= 'A' && runes[j] <= 'Z') || (runes[j] >= 'a' && runes[j] <= 'z')) {
				j++
			}
			if j < len(runes) {
				final := runes[j]
				paramStr := string(runes[i+2 : j])
				params := parseParams(paramStr)
				switch final {
				case 'm':
					applySGR(params)
				case 'H', 'f':
					row, col := 1, 1
					if len(params) >= 1 && params[0] > 0 {
						row = params[0]
					}
					if len(params) >= 2 && params[1] > 0 {
						col = params[1]
					}
					cy, cx = row-1, col-1
				case 'A':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cy -= n
					if cy < 0 {
						cy = 0
					}
				case 'B':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cy += n
				case 'C':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cx += n
				case 'D':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cx -= n
					if cx < 0 {
						cx = 0
					}
				case 's':
					savedX, savedY = cx, cy
				case 'u':
					cx, cy = savedX, savedY
				case 'J':
					// erase — ignore for measure
				case 'K':
					// erase line
				}
				if cx > maxX {
					maxX = cx
				}
				if cy > maxY {
					maxY = cy
				}
				i = j + 1
				continue
			}
		}
		if r == '\r' {
			cx = 0
			i++
			continue
		}
		if r == '\n' {
			cy++
			cx = 0
			if cy > maxY {
				maxY = cy
			}
			i++
			continue
		}
		if r == 0x1A {
			break
		}
		if r >= 32 || r == 0x09 {
			if r == 0x09 {
				cx = (cx + 8) &^ 7
			} else {
				cx++
			}
			if cx-1 > maxX {
				maxX = cx - 1
			}
			if cy > maxY {
				maxY = cy
			}
		}
		i++
	}

	w := maxX + 1
	h := maxY + 1
	if w < cols {
		w = cols
	}
	if h < rows {
		h = rows
	}
	if w > maxCols {
		w = maxCols
	}
	if h > maxRows {
		h = maxRows
	}
	c := NewCanvas(w, h)

	// Second pass: paint
	cx, cy = 0, 0
	fg, bg, bold = classicFG(7), classicBG(0), false
	applySGR = func(params []int) {
		if len(params) == 0 {
			params = []int{0}
		}
		for i := 0; i < len(params); i++ {
			p := params[i]
			switch {
			case p == 0:
				fg, bg, bold = classicFG(7), classicBG(0), false
			case p == 1:
				bold = true
				if !fg.True && fg.Idx < 8 {
					fg.Idx += 8
				}
			case p == 22:
				bold = false
				if !fg.True && fg.Idx >= 8 {
					fg.Idx -= 8
				}
			case p == 39:
				fg = classicFG(7)
				if bold {
					fg.Idx = 15
				}
			case p == 49:
				bg = classicBG(0)
			case p >= 30 && p <= 37:
				idx := uint8(p - 30)
				if bold {
					idx += 8
				}
				fg = classicFG(idx)
			case p >= 40 && p <= 47:
				bg = classicBG(uint8(p - 40))
			case p >= 90 && p <= 97:
				fg = classicFG(uint8(p-90) + 8)
			case p >= 100 && p <= 107:
				bg = classicBG(uint8(p - 100))
			case p == 38 && i+1 < len(params):
				if params[i+1] == 2 && i+4 < len(params) {
					fg = rgbColor(uint8(params[i+2]), uint8(params[i+3]), uint8(params[i+4]))
					i += 4
				} else if params[i+1] == 5 && i+2 < len(params) {
					fg = classicFG(uint8(params[i+2] & 15))
					i += 2
				}
			case p == 48 && i+1 < len(params):
				if params[i+1] == 2 && i+4 < len(params) {
					bg = rgbColor(uint8(params[i+2]), uint8(params[i+3]), uint8(params[i+4]))
					i += 4
				} else if params[i+1] == 5 && i+2 < len(params) {
					bg = classicBG(uint8(params[i+2] & 7))
					i += 2
				}
			}
		}
	}

	put := func(ch rune) {
		if c.InBounds(cx, cy) {
			c.Set(cx, cy, Cell{Ch: ch, FG: fg, BG: bg})
		}
		cx++
		if cx >= c.Cols {
			cx = 0
			cy++
		}
	}

	i = 0
	for i < len(runes) {
		r := runes[i]
		if r == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			j := i + 2
			for j < len(runes) && !((runes[j] >= 'A' && runes[j] <= 'Z') || (runes[j] >= 'a' && runes[j] <= 'z')) {
				j++
			}
			if j < len(runes) {
				final := runes[j]
				params := parseParams(string(runes[i+2 : j]))
				switch final {
				case 'm':
					applySGR(params)
				case 'H', 'f':
					row, col := 1, 1
					if len(params) >= 1 && params[0] > 0 {
						row = params[0]
					}
					if len(params) >= 2 && params[1] > 0 {
						col = params[1]
					}
					cy, cx = row-1, col-1
				case 'A':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cy -= n
					if cy < 0 {
						cy = 0
					}
				case 'B':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cy += n
					if cy >= c.Rows {
						cy = c.Rows - 1
					}
				case 'C':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cx += n
					if cx >= c.Cols {
						cx = c.Cols - 1
					}
				case 'D':
					n := 1
					if len(params) > 0 && params[0] > 0 {
						n = params[0]
					}
					cx -= n
					if cx < 0 {
						cx = 0
					}
				case 's':
					savedX, savedY = cx, cy
				case 'u':
					cx, cy = savedX, savedY
				case 'J':
					mode := 0
					if len(params) > 0 {
						mode = params[0]
					}
					eraseDisplay(c, cx, cy, mode, fg, bg)
				case 'K':
					mode := 0
					if len(params) > 0 {
						mode = params[0]
					}
					eraseLine(c, cx, cy, mode, fg, bg)
				}
				i = j + 1
				continue
			}
		}
		if r == '\r' {
			cx = 0
			i++
			continue
		}
		if r == '\n' {
			cy++
			cx = 0
			i++
			continue
		}
		if r == 0x1A {
			break
		}
		if r == 0x09 {
			for cx%8 != 0 {
				put(' ')
			}
			i++
			continue
		}
		if r >= 32 {
			put(r)
		}
		i++
	}
	return c
}

func parseParams(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ";")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			out = append(out, 0)
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			out = append(out, 0)
			continue
		}
		out = append(out, n)
	}
	return out
}

func eraseDisplay(c *Canvas, cx, cy, mode int, fg, bg Color) {
	blank := Cell{Ch: ' ', FG: fg, BG: bg}
	switch mode {
	case 0: // cursor to end
		for y := cy; y < c.Rows; y++ {
			x0 := 0
			if y == cy {
				x0 = cx
			}
			for x := x0; x < c.Cols; x++ {
				c.Set(x, y, blank)
			}
		}
	case 1: // start to cursor
		for y := 0; y <= cy; y++ {
			x1 := c.Cols
			if y == cy {
				x1 = cx + 1
			}
			for x := 0; x < x1; x++ {
				c.Set(x, y, blank)
			}
		}
	case 2:
		for y := 0; y < c.Rows; y++ {
			for x := 0; x < c.Cols; x++ {
				c.Set(x, y, blank)
			}
		}
	}
}

func eraseLine(c *Canvas, cx, cy, mode int, fg, bg Color) {
	blank := Cell{Ch: ' ', FG: fg, BG: bg}
	switch mode {
	case 0:
		for x := cx; x < c.Cols; x++ {
			c.Set(x, cy, blank)
		}
	case 1:
		for x := 0; x <= cx; x++ {
			c.Set(x, cy, blank)
		}
	case 2:
		for x := 0; x < c.Cols; x++ {
			c.Set(x, cy, blank)
		}
	}
}
