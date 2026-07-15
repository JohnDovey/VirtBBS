package main

import (
	"strings"
	"unicode/utf8"
)

// cp437ToUnicode maps each CP437 byte to its Unicode glyph.
var cp437ToUnicode = buildCP437ToUnicode()

// unicodeToCP437 is the reverse map for glyphs we care about.
var unicodeToCP437 map[rune]byte

func buildCP437ToUnicode() [256]rune {
	var t [256]rune
	for i := 0; i < 0x20; i++ {
		t[i] = rune(i)
	}
	for i := 0x20; i <= 0x7E; i++ {
		t[i] = rune(i)
	}
	t[0x7F] = '⌂'

	writeRange := func(base byte, glyphs string) {
		i := int(base)
		for _, ch := range glyphs {
			t[i] = ch
			i++
		}
	}
	writeRange(0x80, "ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒ")
	writeRange(0xA0, "áíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐")
	writeRange(0xC0, "└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀")
	writeRange(0xE0, "αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ ")
	return t
}

func init() {
	unicodeToCP437 = make(map[rune]byte, 256)
	for i := 0; i < 256; i++ {
		r := cp437ToUnicode[i]
		if _, ok := unicodeToCP437[r]; !ok {
			unicodeToCP437[r] = byte(i)
		}
	}
	// Common block-drawing aliases used by art / HBFS.
	extras := map[rune]byte{
		'▀': 0xDF, '▄': 0xDC, '█': 0xDB, '▌': 0xDD, '▐': 0xDE,
		'░': 0xB0, '▒': 0xB1, '▓': 0xB2,
		'─': 0xC4, '│': 0xB3, '┌': 0xDA, '┐': 0xBF, '└': 0xC0, '┘': 0xD9,
		'├': 0xC3, '┤': 0xB4, '┬': 0xC2, '┴': 0xC1, '┼': 0xC5,
		'═': 0xCD, '║': 0xBA, '╔': 0xC9, '╗': 0xBB, '╚': 0xC8, '╝': 0xBC,
		'╠': 0xCC, '╣': 0xB9, '╦': 0xCB, '╩': 0xCA, '╬': 0xCE,
	}
	for r, b := range extras {
		unicodeToCP437[r] = b
	}
}

func runeFromCP437(b byte) rune {
	return cp437ToUnicode[b]
}

func cp437FromRune(r rune) (byte, bool) {
	if r < 128 {
		return byte(r), true
	}
	b, ok := unicodeToCP437[r]
	return b, ok
}

// decodeANSBytes converts raw file bytes to a Unicode string suitable for parsing.
// CP437 high bytes become Unicode; valid UTF-8 with multi-byte runes passes through.
func decodeANSBytes(raw []byte) string {
	if utf8.Valid(raw) {
		s := string(raw)
		for _, r := range s {
			if r > 0xFF {
				return s
			}
		}
		// All runes ≤ U+00FF: may still be mis-decoded CP437; re-decode from bytes.
	}
	needs := false
	for _, b := range raw {
		if b >= 0x80 {
			needs = true
			break
		}
	}
	if !needs {
		return string(raw)
	}
	var sb strings.Builder
	sb.Grow(len(raw))
	for _, b := range raw {
		if b < 0x80 {
			sb.WriteByte(b)
			continue
		}
		sb.WriteRune(runeFromCP437(b))
	}
	return sb.String()
}

// expandPCBAnsi converts PCBoard-style "[1;36m" into real ESC sequences.
func expandPCBAnsi(s string) string {
	needs := false
	for i := 0; i < len(s); i++ {
		if s[i] == '[' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s) + 16)
	for i := 0; i < len(s); i++ {
		if s[i] == '[' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '?' {
				j++
			}
			if j < len(s) && ((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				sb.WriteByte(0x1B)
				sb.WriteByte('[')
				sb.WriteString(s[i+1 : j+1])
				i = j
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// Curated draw glyphs for Tab cycling and picker (Unicode display forms).
var drawGlyphs = []rune{
	'█', '▀', '▄', '▌', '▐', '░', '▒', '▓',
	'═', '║', '╔', '╗', '╚', '╝', '╠', '╣', '╦', '╩', '╬',
	'─', '│', '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼',
	'·', '∙', '°', '■', '±', '≈',
}
