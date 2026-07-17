// Package about holds attribution and program descriptions for the NodeGUI TUI.
package about

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Contributor is credited in the About screen.
type Contributor struct {
	Name string
	Note string
	URL  string
}

const (
	ProjectName = "NodeGUI"
	License     = "GPL-3.0"
	RepoURL     = "https://github.com/JohnDovey/NodeGUI"
	// DefaultWidth is used when the UI does not supply a column width.
	DefaultWidth = 42
)

// Version is set from main at startup (defaults if unset).
var Version = "0.2.0"

// Contributors lists original author and maintainers.
func Contributors() []Contributor {
	return []Contributor{
		{
			Name: "John Dovey (BoonDock)",
			Note: "Original author — Gambas NodeGUI for FidoNet Z1DAILY → SQLite; " +
				"FidoNet 4:92/1, 4:920/1",
			URL: "https://github.com/JohnDovey",
		},
	}
}

// ProjectBlurb is a short overview of the project.
func ProjectBlurb() string {
	return "NodeGUI downloads the FidoNet Zone 1 daily nodelist (Z1DAILY), " +
		"stores it in SQLite, and lets you browse nodes offline. " +
		"It started as a Gambas desktop app and was rebuilt as a pure Go " +
		"console UI in the ServiceMonitor style."
}

// AppBlurb describes this program.
func AppBlurb() string {
	return "This VirtBBS door imports Z1DAILY over HTTP (zip), parses the " +
		"distribution nodelist hierarchy (Zone / Region / Host / Hub / Node), " +
		"and shows address, sysop, location, phone, and flags. " +
		"Anyone can browse and filter; importing and changing the download " +
		"source URL are sysop-only (security level 100+)."
}

// PlainText renders a full about document for the terminal UI.
// width is the content column width in runes (viewport inner width).
func PlainText(width int) string {
	if width < 24 {
		width = DefaultWidth
	}

	var b strings.Builder
	title := fmt.Sprintf("%s %s", ProjectName, Version)
	fmt.Fprintf(&b, "%s\n", title)
	ruleLen := utf8.RuneCountInString(title) + 2
	if ruleLen > width {
		ruleLen = width
	}
	fmt.Fprintf(&b, "%s\n\n", strings.Repeat("─", ruleLen))

	b.WriteString("About this program\n")
	b.WriteString(Wrap(AppBlurb(), width) + "\n\n")

	b.WriteString("About NodeGUI\n")
	b.WriteString(Wrap(ProjectBlurb(), width) + "\n\n")

	b.WriteString("Default source\n")
	b.WriteString(WrapIndent("Zone 1 daily list (Darkrealms hub)", width, "  ") + "\n")
	b.WriteString(WrapIndent("https://fido-z1.darkrealms.ca/$webfile.send.ZC1./", width, "  ") + "\n\n")

	b.WriteString("Repository\n")
	b.WriteString(WrapIndent(RepoURL, width, "  ") + "\n\n")

	b.WriteString("Contributors\n")
	for _, c := range Contributors() {
		b.WriteString(WrapIndent("• "+c.Name, width, "  ") + "\n")
		b.WriteString(WrapIndent(c.Note, width, "    ") + "\n")
		if c.URL != "" {
			b.WriteString(WrapIndent(c.URL, width, "    ") + "\n")
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "License: %s\n", License)
	b.WriteString("\n" + Wrap("Press esc or ? to close", width))
	return b.String()
}

// Wrap word-wraps s to the given rune width, breaking long tokens (URLs) as needed.
func Wrap(s string, width int) string {
	return wrapLines(s, width, "")
}

// WrapIndent wraps s and prefixes every line with indent (indent counts toward width).
func WrapIndent(s string, width int, indent string) string {
	return wrapLines(s, width, indent)
}

func wrapLines(s string, width int, indent string) string {
	if width < 12 {
		width = 12
	}
	indentW := utf8.RuneCountInString(indent)
	bodyW := width - indentW
	if bodyW < 8 {
		bodyW = 8
	}

	words := strings.Fields(s)
	if len(words) == 0 {
		if strings.TrimSpace(s) == "" {
			return ""
		}
		return indent + hardBreak(strings.TrimSpace(s), bodyW, indent)
	}

	var lines []string
	var line string
	for _, word := range words {
		if utf8.RuneCountInString(word) > bodyW {
			if line != "" {
				lines = append(lines, indent+line)
				line = ""
			}
			lines = append(lines, hardBreakLines(word, bodyW, indent)...)
			continue
		}
		if line == "" {
			line = word
			continue
		}
		if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) > bodyW {
			lines = append(lines, indent+line)
			line = word
			continue
		}
		line += " " + word
	}
	if line != "" {
		lines = append(lines, indent+line)
	}
	return strings.Join(lines, "\n")
}

func hardBreakLines(s string, bodyW int, indent string) []string {
	var out []string
	runes := []rune(s)
	for len(runes) > 0 {
		n := bodyW
		if n > len(runes) {
			n = len(runes)
		}
		// Prefer break after path separators when hard-wrapping URLs.
		if n < len(runes) {
			for i := n; i > bodyW/3; i-- {
				switch runes[i-1] {
				case '/', '\\', '-', '_', '.', '?', '&', '=', ':', ',', ';', '$':
					n = i
					goto cut
				}
			}
		}
	cut:
		out = append(out, indent+string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

func hardBreak(s string, bodyW int, indent string) string {
	return strings.Join(hardBreakLines(s, bodyW, indent), "\n")
}
