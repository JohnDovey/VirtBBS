package mrc

import (
	"strconv"
	"strings"

	"github.com/virtbbs/virtbbs/internal/ansi"
)

// mysticPipe maps |00-|23 (and common extensions) to ANSI SGR.
// Compatible with Mystic/Synchronet pipe color conventions used on MRC.
var mysticFG = map[string]int{
	"00": ansi.Black,
	"01": ansi.Blue,
	"02": ansi.Green,
	"03": ansi.Cyan,
	"04": ansi.Red,
	"05": ansi.Magenta,
	"06": ansi.Yellow, // brown-ish → yellow
	"07": ansi.White,
	"08": ansi.BrightBlack,
	"09": ansi.BrightBlue,
	"10": ansi.BrightGreen,
	"11": ansi.BrightCyan,
	"12": ansi.BrightRed,
	"13": ansi.BrightMagenta,
	"14": ansi.BrightYellow,
	"15": ansi.BrightWhite,
	"16": ansi.Black,
	"17": ansi.Blue,
	"18": ansi.Green,
	"19": ansi.Cyan,
	"20": ansi.Red,
	"21": ansi.Magenta,
	"22": ansi.Yellow,
	"23": ansi.White,
}

// PipeToANSI converts Mystic |NN pipe codes to ANSI escapes.
func PipeToANSI(s string) string {
	if !strings.Contains(s, "|") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	for i := 0; i < len(s); {
		if s[i] == '|' && i+2 < len(s) {
			code := s[i+1 : i+3]
			if fg, ok := mysticFG[code]; ok {
				b.WriteString(ansi.Color(fg))
				i += 3
				continue
			}
			// |[X for backgrounds / attributes — skip unknown pipe of form |XY when XY digits
			if isTwoDigits(code) {
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	b.WriteString(ansi.Reset())
	return b.String()
}

func isTwoDigits(s string) bool {
	if len(s) != 2 {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

// StripPipe removes pipe codes for plain-text matching (mentions, etc.).
func StripPipe(s string) string {
	if !strings.Contains(s, "|") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '|' && i+2 < len(s) && isTwoDigits(s[i+1:i+3]) {
			i += 3
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
