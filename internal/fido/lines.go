package fido

import "strings"

// NetmailReaderText returns the reader-facing body for a stored netmail message.
func NetmailReaderText(body string) string {
	if strings.Contains(body, "\x01MSGID:") || strings.Contains(body, "\x01REPLY:") {
		return (&Message{Body: body}).Parse().Text
	}
	return body
}
func NormalizeDisplayEOL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

// fidoLines splits a message body on FTS/FidoSoft line endings (\r, \r\n, \n).
func fidoLines(body string) []string {
	body = normalizeFidoBodyForParse(body)
	if body == "" {
		return nil
	}
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, strings.TrimRight(line, "\r"))
	}
	return out
}

// normalizeFidoBodyForParse ensures kludge lines start on their own row before splitting.
func normalizeFidoBodyForParse(body string) string {
	if body == "" {
		return ""
	}
	var out strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] == '\x01' && i > 0 {
			prev := body[i-1]
			if prev != '\r' && prev != '\n' {
				out.WriteByte('\n')
			}
		}
		out.WriteByte(body[i])
	}
	s := out.String()
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}
