package fido

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	ampLegacyDecRE = regexp.MustCompile(`&([0-9]{1,3});`)
	ampHTMLDecRE   = regexp.MustCompile(`&#([0-9]{1,5});`)
	ampHTMLHexRE   = regexp.MustCompile(`(?i)&#x([0-9a-f]{1,4});`)
	ampNamedRE     = regexp.MustCompile(`&([a-zA-Z]+);`)
)

var htmlNamedEntities = map[string]string{
	"quot": `"`,
	"amp":  "&",
	"lt":   "<",
	"gt":   ">",
	"apos": "'",
}

// DecodeMessageEntities converts Fido/legacy and HTML character references to
// Unicode runes. hpt and some older netmail software encode quotes as &34;
// (decimal ASCII without #) instead of &#34; or &quot;.
func DecodeMessageEntities(s string) string {
	if s == "" || !strings.Contains(s, "&") {
		return s
	}
	s = ampHTMLHexRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := ampHTMLHexRE.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		n, err := strconv.ParseInt(sub[1], 16, 32)
		if err != nil || !safeDecodedRune(rune(n)) {
			return m
		}
		return string(rune(n))
	})
	s = ampHTMLDecRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := ampHTMLDecRE.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		n, err := strconv.Atoi(sub[1])
		if err != nil || !safeDecodedRune(rune(n)) {
			return m
		}
		return string(rune(n))
	})
	s = ampLegacyDecRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := ampLegacyDecRE.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		n, err := strconv.Atoi(sub[1])
		if err != nil || !safeDecodedRune(rune(n)) {
			return m
		}
		return string(rune(n))
	})
	s = ampNamedRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := ampNamedRE.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		if rep, ok := htmlNamedEntities[strings.ToLower(sub[1])]; ok {
			return rep
		}
		return m
	})
	return s
}

func safeDecodedRune(r rune) bool {
	return r == '\t' || r == '\n' || r == '\r' || (r >= 32 && r <= 126) || r >= 160
}

// NetmailDisplayText returns netmail body text ready for terminal display.
func NetmailDisplayText(body string) string {
	return NormalizeDisplayEOL(DecodeMessageEntities(NetmailReaderText(body)))
}
