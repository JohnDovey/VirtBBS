package fido

import (
	"fmt"
	"strings"
)

const kludgeFLAGS = "FLAGS"

// NetmailFlagsDefault is the standard FLAGS kludge for private netmail.
const NetmailFlagsDefault = "PVT"

// FlagsKludgeLine returns a ^AFLAGS control line for outbound netmail.
func FlagsKludgeLine(flags string) string {
	flags = strings.TrimSpace(flags)
	if flags == "" {
		flags = NetmailFlagsDefault
	}
	return fmt.Sprintf("\x01%s %s\r", kludgeFLAGS, flags)
}

// ParseFlagsFromKludges extracts the FLAGS value from stored ^A kludge text.
func ParseFlagsFromKludges(kludges string) string {
	for _, line := range strings.Split(kludges, "\r") {
		line = strings.TrimSpace(strings.TrimRight(line, "\n"))
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "\x01FLAGS ") {
			return strings.TrimSpace(line[len("\x01FLAGS "):])
		}
		if strings.HasPrefix(upper, "\x01FLAGS:") {
			return strings.TrimSpace(line[len("\x01FLAGS:"):])
		}
	}
	return ""
}

// HasPrivateFlag reports whether kludges or packet attributes mark the message private.
func HasPrivateFlag(kludges string, attrib uint16) bool {
	flags := strings.ToUpper(ParseFlagsFromKludges(kludges))
	if strings.Contains(flags, "PVT") {
		return true
	}
	return attrib&AttribPrivate != 0
}

// IntlKludgeLine returns the ^AINTL line: destination then origin (boss nodes).
func IntlKludgeLine(from, to Addr) string {
	return fmt.Sprintf("\x01INTL %s %s\r", to.BossString(), from.BossString())
}

// ParseIntlFromKludges returns destination and origin addresses from ^AINTL, if present.
func ParseIntlFromKludges(kludges string) (dest, orig string) {
	for _, line := range strings.Split(kludges, "\r") {
		line = strings.TrimSpace(strings.TrimRight(line, "\n"))
		if !strings.HasPrefix(strings.ToUpper(line), "\x01INTL ") {
			continue
		}
		parts := strings.Fields(strings.TrimSpace(line[len("\x01INTL "):]))
		if len(parts) >= 1 {
			dest = parts[0]
		}
		if len(parts) >= 2 {
			orig = parts[1]
		}
		return dest, orig
	}
	return "", ""
}
