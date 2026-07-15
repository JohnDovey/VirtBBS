// Package mrc implements Multi-Relay Chat (MRC 1.3) as an in-process VirtBBS hub.
package mrc

import (
	"strconv"
	"strings"
	"unicode"
)

// Config holds MRC hub settings (VirtBBS.DAT [mrc] section).
type Config struct {
	Enabled     bool   `toml:"enabled"      json:"enabled"`
	Host        string `toml:"host"         json:"host"`
	Port        int    `toml:"port"         json:"port"`
	UseTLS      bool   `toml:"use_tls"      json:"use_tls"`
	BBSName     string `toml:"bbs_name"     json:"bbs_name"`
	BBSPretty   string `toml:"bbs_pretty"   json:"bbs_pretty"`
	Sysop       string `toml:"sysop"        json:"sysop"`
	Description string `toml:"description"  json:"description"`
	Telnet      string `toml:"telnet"       json:"telnet"`
	SSH         string `toml:"ssh"          json:"ssh"`
	Website     string `toml:"website"      json:"website"`
	DefaultRoom string `toml:"default_room" json:"default_room"`
	MinSecurity int    `toml:"min_security" json:"min_security"`
}

// DefaultConfig returns MRC defaults (disabled).
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		Host:        "mrc.bottomlessabyss.net",
		Port:        5000,
		UseTLS:      false,
		DefaultRoom: "lobby",
		MinSecurity: 10,
	}
}

// Resolve fills empty identity fields from BBS/sysop defaults and sanitises names.
func (c Config) Resolve(bbsName, sysopName, platform string) Resolved {
	r := Resolved{
		Config:   c,
		Platform: platform,
	}
	if r.Host == "" {
		r.Host = "mrc.bottomlessabyss.net"
	}
	if r.Port <= 0 {
		if r.UseTLS {
			r.Port = 5001
		} else {
			r.Port = 5000
		}
	}
	if r.DefaultRoom == "" {
		r.DefaultRoom = "lobby"
	}
	if r.MinSecurity <= 0 {
		r.MinSecurity = 10
	}
	r.BBSName = SanitizeName(c.BBSName)
	if r.BBSName == "" {
		r.BBSName = SanitizeName(bbsName)
	}
	if r.BBSName == "" {
		r.BBSName = "VirtBBS"
	}
	r.BBSPretty = strings.TrimSpace(c.BBSPretty)
	if r.BBSPretty == "" {
		r.BBSPretty = strings.TrimSpace(bbsName)
	}
	if r.BBSPretty == "" {
		r.BBSPretty = r.BBSName
	}
	r.Sysop = strings.TrimSpace(c.Sysop)
	if r.Sysop == "" {
		r.Sysop = strings.TrimSpace(sysopName)
	}
	if r.Platform == "" {
		r.Platform = "VirtBBS"
	}
	return r
}

// Resolved is Config with identity defaults applied.
type Resolved struct {
	Config
	Platform string
}

// HandshakeString is the first line sent after TCP connect: BBSName~Platform
func (r Resolved) HandshakeString() string {
	return r.BBSName + "~" + SanitizeField(r.Platform)
}

// Addr returns host:port.
func (r Resolved) Addr() string {
	return r.Host + ":" + strconv.Itoa(r.Port)
}

// SanitizeName removes tildes, replaces spaces with underscores, keeps ASCII 32-125.
func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, "~", "")
	name = strings.ReplaceAll(name, " ", "_")
	return SanitizeField(name)
}

// SanitizeField removes tildes and non-printable ASCII outside 32-125.
func SanitizeField(s string) string {
	s = strings.ReplaceAll(s, "~", "")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 32 && r <= 125 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// StripTildesForBody replaces ~ in chat bodies (field separator).
func StripTildesForBody(s string) string {
	return strings.ReplaceAll(s, "~", " ")
}

// SplitMessage splits outbound chat text on word boundaries into <= maxLen chunks.
func SplitMessage(s string, maxLen int) []string {
	s = strings.TrimSpace(StripTildesForBody(s))
	if s == "" {
		return nil
	}
	if maxLen <= 0 {
		maxLen = 140
	}
	var out []string
	for len(s) > 0 {
		if len(s) <= maxLen {
			out = append(out, s)
			break
		}
		chunk := s[:maxLen]
		if i := strings.LastIndexByte(chunk, ' '); i > maxLen/3 {
			chunk = s[:i]
			s = strings.TrimLeftFunc(s[i:], unicode.IsSpace)
		} else {
			s = s[maxLen:]
		}
		chunk = strings.TrimSpace(chunk)
		if chunk != "" {
			out = append(out, chunk)
		}
	}
	return out
}
