package web

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	ansiEscRE  = regexp.MustCompile(`\x1b\[([0-9;]*)m`)
	clearScrRE = regexp.MustCompile(`\x1b\[[0-9;]*[HJ]`)
)

// ansiToHTML converts ANSI SGR sequences to HTML spans (named colors + truecolor).
func ansiToHTML(raw string) string {
	s := clearScrRE.ReplaceAllString(raw, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var out strings.Builder
	var state sgrState
	flush := func(text string) {
		if text == "" {
			return
		}
		escaped := html.EscapeString(text)
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		classes := state.classes()
		style := state.style()
		if classes == "" && style == "" {
			out.WriteString(escaped)
			return
		}
		out.WriteString(`<span`)
		if classes != "" {
			out.WriteString(` class="`)
			out.WriteString(classes)
			out.WriteString(`"`)
		}
		if style != "" {
			out.WriteString(` style="`)
			out.WriteString(style)
			out.WriteString(`"`)
		}
		out.WriteString(`>`)
		out.WriteString(escaped)
		out.WriteString(`</span>`)
	}

	pos := 0
	matches := ansiEscRE.FindAllStringSubmatchIndex(s, -1)
	for _, m := range matches {
		if m[0] > pos {
			flush(s[pos:m[0]])
		}
		state.apply(s[m[2]:m[3]])
		pos = m[1]
	}
	flush(s[pos:])
	return out.String()
}

type sgrState struct {
	bold bool
	fg   string // class name or empty
	bg   string // class name or empty
	fgRGB string // "r,g,b" or empty
	bgRGB string
}

func (s *sgrState) classes() string {
	var classes []string
	if s.bold {
		classes = append(classes, "ansi-bold")
	}
	if s.fg != "" && s.fgRGB == "" {
		classes = append(classes, s.fg)
	}
	if s.bg != "" && s.bgRGB == "" {
		classes = append(classes, s.bg)
	}
	return strings.Join(classes, " ")
}

func (s *sgrState) style() string {
	var parts []string
	if s.fgRGB != "" {
		parts = append(parts, "color:rgb("+s.fgRGB+")")
	}
	if s.bgRGB != "" {
		parts = append(parts, "background-color:rgb("+s.bgRGB+")")
	}
	return strings.Join(parts, ";")
}

func (s *sgrState) apply(code string) {
	if code == "" || code == "0" {
		*s = sgrState{}
		return
	}
	parts := strings.Split(code, ";")
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch p {
		case "1":
			s.bold = true
		case "22":
			s.bold = false
		case "30":
			s.fg, s.fgRGB = "ansi-fg-black", ""
		case "31":
			s.fg, s.fgRGB = "ansi-fg-red", ""
		case "32":
			s.fg, s.fgRGB = "ansi-fg-green", ""
		case "33":
			s.fg, s.fgRGB = "ansi-fg-yellow", ""
		case "34":
			s.fg, s.fgRGB = "ansi-fg-blue", ""
		case "35":
			s.fg, s.fgRGB = "ansi-fg-magenta", ""
		case "36":
			s.fg, s.fgRGB = "ansi-fg-cyan", ""
		case "37":
			s.fg, s.fgRGB = "ansi-fg-white", ""
		case "39":
			s.fg, s.fgRGB = "", ""
		case "40":
			s.bg, s.bgRGB = "ansi-bg-black", ""
		case "41":
			s.bg, s.bgRGB = "ansi-bg-red", ""
		case "42":
			s.bg, s.bgRGB = "ansi-bg-green", ""
		case "43":
			s.bg, s.bgRGB = "ansi-bg-yellow", ""
		case "44":
			s.bg, s.bgRGB = "ansi-bg-blue", ""
		case "45":
			s.bg, s.bgRGB = "ansi-bg-magenta", ""
		case "46":
			s.bg, s.bgRGB = "ansi-bg-cyan", ""
		case "47":
			s.bg, s.bgRGB = "ansi-bg-white", ""
		case "49":
			s.bg, s.bgRGB = "", ""
		case "90":
			s.fg, s.fgRGB = "ansi-fg-bright-black", ""
		case "91":
			s.fg, s.fgRGB = "ansi-fg-bright-red", ""
		case "92":
			s.fg, s.fgRGB = "ansi-fg-bright-green", ""
		case "93":
			s.fg, s.fgRGB = "ansi-fg-bright-yellow", ""
		case "94":
			s.fg, s.fgRGB = "ansi-fg-bright-blue", ""
		case "95":
			s.fg, s.fgRGB = "ansi-fg-bright-magenta", ""
		case "96":
			s.fg, s.fgRGB = "ansi-fg-bright-cyan", ""
		case "97":
			s.fg, s.fgRGB = "ansi-fg-bright-white", ""
		case "38":
			if i+1 < len(parts) && parts[i+1] == "2" && i+4 < len(parts) {
				r, _ := strconv.Atoi(parts[i+2])
				g, _ := strconv.Atoi(parts[i+3])
				b, _ := strconv.Atoi(parts[i+4])
				s.fgRGB = fmt.Sprintf("%d,%d,%d", r, g, b)
				s.fg = ""
				i += 4
			}
		case "48":
			if i+1 < len(parts) && parts[i+1] == "2" && i+4 < len(parts) {
				r, _ := strconv.Atoi(parts[i+2])
				g, _ := strconv.Atoi(parts[i+3])
				b, _ := strconv.Atoi(parts[i+4])
				s.bgRGB = fmt.Sprintf("%d,%d,%d", r, g, b)
				s.bg = ""
				i += 4
			}
		}
	}
}
