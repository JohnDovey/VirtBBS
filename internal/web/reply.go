package web

import (
	"strings"

	"github.com/virtbbs/virtbbs/internal/fido"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// quoteReplyBody builds a compose textarea prefill for a reply, following the
// binkterm-php pattern: attribution line plus FSC-0032-style quoted lines.
func quoteReplyBody(orig *messages.Message) string {
	return fido.QuoteEchoReplyBody(orig)
}

func replySubject(origSubject string) string {
	subject := strings.TrimSpace(origSubject)
	if len(subject) >= 3 && strings.EqualFold(subject[:3], "re:") {
		subject = strings.TrimSpace(subject[3:])
	}
	return "Re: " + subject
}
