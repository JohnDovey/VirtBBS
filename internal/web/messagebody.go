package web

import (
	"html"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/fido"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/postname"
)

func bodyHasANSI(raw string) bool {
	return strings.Contains(raw, "\x1b[")
}

// FormatMessageBodyHTML renders a message body for safe HTML display.
// ANSI sequences take precedence, then StyleCodes, else plain escaped text.
func FormatMessageBodyHTML(body string) string {
	if body == "" {
		return ""
	}
	if bodyHasANSI(body) {
		return `<div class="ansi-screen">` + ansiToHTML(body) + `</div>`
	}
	if hasStyleCodes(body) {
		return styleCodesToHTML(body)
	}
	escaped := html.EscapeString(body)
	escaped = strings.ReplaceAll(escaped, "\r\n", "\n")
	escaped = strings.ReplaceAll(escaped, "\r", "\n")
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	return escaped
}

// formatConferenceMessageBody renders a conference message for HTML display,
// including echomail tear line and Origin for locally originated posts.
func formatConferenceMessageBody(c *conferences.Conference, m *messages.Message) string {
	body := m.Body
	if c != nil && c.Echo && m != nil && m.Echo {
		cfg := config.Get()
		body = fido.EchoDisplayText(m, cfg.BBS.Name, postname.EchoOrigAddr(c))
	}
	return FormatMessageBodyHTML(body)
}
