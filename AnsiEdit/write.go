package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// WriteANSI serializes the canvas to ANSI bytes (UTF-8 with truecolor / classic SGR).
// preferCP437 writes high glyphs as CP437 single bytes when they map.
func WriteANSI(c *Canvas, preferCP437 bool) []byte {
	var buf bytes.Buffer
	var lastFG, lastBG *Color
	reset := true

	emitSGR := func(fg, bg Color) {
		if !reset && lastFG != nil && lastBG != nil && lastFG.Equal(fg) && lastBG.Equal(bg) {
			return
		}
		var parts []string
		if reset || lastFG == nil || !lastFG.Equal(fg) || lastBG == nil || !lastBG.Equal(bg) {
			// Full color update; use reset when either side is classic default
			needReset := reset
			if !fg.True && !bg.True {
				needReset = true
			}
			if needReset {
				parts = append(parts, "0")
			}
			if fg.True {
				parts = append(parts, fmt.Sprintf("38;2;%d;%d;%d", fg.R, fg.G, fg.B))
			} else {
				idx := fg.Idx
				if idx >= 8 {
					parts = append(parts, "1", fmt.Sprintf("%d", 30+idx-8))
				} else {
					parts = append(parts, fmt.Sprintf("%d", 30+idx))
				}
			}
			if bg.True {
				parts = append(parts, fmt.Sprintf("48;2;%d;%d;%d", bg.R, bg.G, bg.B))
			} else {
				parts = append(parts, fmt.Sprintf("%d", 40+bg.Idx))
			}
		}
		if len(parts) > 0 {
			buf.WriteString("\x1b[")
			buf.WriteString(strings.Join(parts, ";"))
			buf.WriteByte('m')
		}
		cpFG, cpBG := fg, bg
		lastFG, lastBG = &cpFG, &cpBG
		reset = false
	}

	for y := 0; y < c.Rows; y++ {
		// Find last non-blank on row for shorter lines (keep attrs for trailing space that has custom BG)
		end := c.Cols
		for end > 0 {
			cell := c.Get(end-1, y)
			if cell.Ch != ' ' || cell.BG.True || (!cell.BG.True && cell.BG.Idx != 0) ||
				(cell.FG.True || (!cell.FG.True && cell.FG.Idx != 7)) {
				break
			}
			end--
		}
		for x := 0; x < end; x++ {
			cell := c.Get(x, y)
			emitSGR(cell.FG, cell.BG)
			ch := cell.Ch
			if ch == 0 {
				ch = ' '
			}
			if preferCP437 {
				if b, ok := cp437FromRune(ch); ok {
					buf.WriteByte(b)
					continue
				}
			}
			buf.WriteRune(ch)
		}
		buf.WriteString("\r\n")
		// After newline, many terminals reset wrap; keep color state
	}
	buf.WriteString("\x1b[0m")
	return buf.Bytes()
}

// SaveFile writes art (+ optional SAUCE) atomically.
func SaveFile(path string, c *Canvas, sauce Sauce, preferCP437 bool) error {
	art := WriteANSI(c, preferCP437)
	if sauce.Present {
		if sauce.TInfo1 == 0 {
			sauce.TInfo1 = uint16(c.Cols)
		}
		if sauce.TInfo2 == 0 {
			sauce.TInfo2 = uint16(c.Rows)
		}
		art = AppendSauce(art, sauce)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, art, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadFile reads an .ANS/.ASC file into canvas + sauce.
func LoadFile(path string) (*Canvas, Sauce, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Sauce{}, err
	}
	art, sauce := SplitSauce(data)
	text := decodeANSBytes(art)
	c := ParseANSI(text, defCols, defRows)
	if sauce.Present {
		if sauce.TInfo1 > 0 && int(sauce.TInfo1) != c.Cols {
			// keep parsed size; SAUCE dims are advisory
		}
	}
	return c, sauce, nil
}
