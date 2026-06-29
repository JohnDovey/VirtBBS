package fido

import "strings"

// ParseTearLine extracts software and version from a Fido tear line (--- ...).
func ParseTearLine(line string) (software, version string) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "---") {
		return "", ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "---"))
	if rest == "" {
		return "", ""
	}
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", ""
	}
	software = parts[0]
	if len(parts) > 1 {
		version = parts[1]
	}
	return software, version
}

// ParseEchoFooters extracts taglines and tear-line software from an echomail body.
func ParseEchoFooters(body string) (taglines []string, software, version string) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	lines := strings.Split(body, "\n")

	tearIdx := -1
	originIdx := -1
	metaIdx := -1
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
		software, version = ParseTearLine(strings.TrimSpace(lines[tearIdx]))
	case originIdx >= 0:
		footerEnd = originIdx
	case metaIdx >= 0:
		footerEnd = metaIdx
	}

	if tearIdx < 0 && originIdx < 0 && metaIdx < 0 {
		blankSep := -1
		for i := 0; i < len(lines)-1; i++ {
			if strings.TrimSpace(lines[i]) == "" {
				blankSep = i
			}
		}
		if blankSep < 0 {
			return nil, "", ""
		}
		for i := blankSep + 1; i < len(lines); i++ {
			if t := strings.TrimSpace(lines[i]); t != "" {
				taglines = append(taglines, t)
			}
		}
		return taglines, "", ""
	}

	for i := footerEnd - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			if len(taglines) > 0 {
				break
			}
			continue
		}
		if isEchoMetaFooterLine(trimmed) || strings.HasPrefix(trimmed, "---") {
			break
		}
		taglines = append([]string{trimmed}, taglines...)
	}
	return taglines, software, version
}

func isEchoMetaFooterLine(s string) bool {
	u := strings.ToUpper(strings.TrimSpace(s))
	return strings.HasPrefix(u, "SEEN-BY:") ||
		strings.HasPrefix(u, "^ASEEN-BY:") ||
		strings.HasPrefix(u, "PATH ") ||
		strings.HasPrefix(u, "^APATH:") ||
		strings.HasPrefix(u, "AREA:")
}

func softwareCountKey(software, version string) string {
	software = strings.TrimSpace(software)
	version = strings.TrimSpace(version)
	if software == "" {
		return ""
	}
	if version == "" {
		return software
	}
	return software + " " + version
}
