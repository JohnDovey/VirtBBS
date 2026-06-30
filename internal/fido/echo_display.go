package fido

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/virtbbs/virtbbs/internal/messages"
)

// EchoMainBody returns reader-facing message text without taglines or Fido
// footers (tear line, Origin, SEEN-BY, PATH).
func EchoMainBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	lines := strings.Split(body, "\n")

	tearIdx, originIdx, metaIdx := -1, -1, -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "---") {
			tearIdx = i
		}
		if strings.HasPrefix(t, "* Origin:") || strings.HasPrefix(t, " * Origin:") {
			originIdx = i
		}
		if isEchoMetaFooterLine(t) {
			if metaIdx < 0 || i < metaIdx {
				metaIdx = i
			}
		}
	}

	footerEnd := len(lines)
	switch {
	case tearIdx >= 0:
		footerEnd = tearIdx
	case originIdx >= 0:
		footerEnd = originIdx
	case metaIdx >= 0:
		footerEnd = metaIdx
	default:
		blankSep := -1
		for i := 0; i < len(lines)-1; i++ {
			if strings.TrimSpace(lines[i]) == "" {
				blankSep = i
			}
		}
		if blankSep >= 0 {
			footerEnd = blankSep
		}
	}

	main := strings.Join(lines[:footerEnd], "\n")
	return strings.TrimRight(main, "\n")
}

// EchoDisplayText returns the body text shown when reading an echomail message.
// Locally originated posts gain tear line and Origin on display (added at export
// time for the wire); imported messages are shown as stored.
func EchoDisplayText(m *messages.Message, bbsName string, orig Addr) string {
	if m == nil || !m.Echo {
		return m.Body
	}
	if strings.TrimSpace(m.FidoSeenBy) != "" {
		return normalizeDisplayEOL(m.Body)
	}
	taglines, _, _ := ParseEchoFooters(m.Body)
	main := EchoMainBody(m.Body)
	var sb strings.Builder
	sb.WriteString(main)
	if len(taglines) > 0 {
		sb.WriteString("\r\n\r\n")
		sb.WriteString(strings.Join(taglines, "\r\n"))
	}
	if orig != (Addr{}) {
		sb.WriteString("\r\n")
		sb.WriteString(OutboundSignatureLines(bbsName, orig))
	}
	return sb.String()
}

// QuoteEchoReplyBody builds FSC-0032-style quoted text for echo replies.
func QuoteEchoReplyBody(orig *messages.Message) string {
	if orig == nil {
		return ""
	}
	date := orig.DatePosted.Format("January 2 2006")
	attribution := fmt.Sprintf("\nOn %s, %s wrote:\n", date, orig.FromName)
	prefix := " " + echoReplyInitials(orig.FromName) + "> "

	var quoted []string
	for _, line := range strings.Split(normalizeDisplayEOL(EchoMainBody(orig.Body)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			quoted = append(quoted, "")
			continue
		}
		if strings.HasPrefix(trimmed, "\x01") {
			continue
		}
		quoted = append(quoted, prefix+trimmed)
	}
	return attribution + "\n" + strings.Join(quoted, "\n")
}

func echoReplyInitials(name string) string {
	var letters []rune
	for _, r := range name {
		if unicode.IsLetter(r) {
			letters = append(letters, unicode.ToUpper(r))
			if len(letters) == 2 {
				break
			}
		}
	}
	if len(letters) == 0 {
		return "??"
	}
	return string(letters)
}

func normalizeDisplayEOL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
