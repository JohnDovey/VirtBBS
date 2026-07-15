package mrc

import (
	"strings"
)

// Packet is one 7-field MRC tilde-framed line.
type Packet struct {
	FromUser string // F1
	FromSite string // F2
	FromRoom string // F3
	ToUser   string // F4
	MsgExt   string // F5
	ToRoom   string // F6
	Body     string // F7
}

// Encode builds F1~F2~F3~F4~F5~F6~F7~\n
func (p Packet) Encode() string {
	fields := []string{
		SanitizeField(p.FromUser),
		SanitizeField(p.FromSite),
		SanitizeField(p.FromRoom),
		SanitizeField(p.ToUser),
		SanitizeField(p.MsgExt),
		SanitizeField(p.ToRoom),
		SanitizeField(p.Body),
	}
	return strings.Join(fields, "~") + "~\n"
}

// ParsePacket parses one MRC line (trailing newline optional).
func ParsePacket(line string) (Packet, bool) {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return Packet{}, false
	}
	parts := strings.Split(line, "~")
	// 8 parts (7 fields + trailing empty) or 7 fields without trailing ~
	switch len(parts) {
	case 7:
		// ok
	case 8:
		parts = parts[:7]
	default:
		if len(parts) < 7 {
			return Packet{}, false
		}
		// More tildes in body — rejoin extras into body
		extra := strings.Join(parts[6:], "~")
		parts = append(parts[:6], extra)
	}
	return Packet{
		FromUser: parts[0],
		FromSite: parts[1],
		FromRoom: parts[2],
		ToUser:   parts[3],
		MsgExt:   parts[4],
		ToRoom:   parts[5],
		Body:     parts[6],
	}, true
}

// IsServerPing reports a keep-alive PING from the relay.
func (p Packet) IsServerPing() bool {
	b := strings.ToUpper(strings.TrimSpace(p.Body))
	return b == "PING" || strings.HasPrefix(b, "PING:") ||
		strings.EqualFold(p.FromUser, "SERVER") && strings.EqualFold(p.Body, "PING")
}

// IsChatMessage is true when body is not an uppercase server verb-style command.
func (p Packet) IsChatMessage() bool {
	b := strings.TrimSpace(p.Body)
	if b == "" {
		return false
	}
	up := strings.ToUpper(b)
	for _, verb := range []string{
		"IAMHERE", "IMALIVE", "LOGOFF", "NEWROOM", "PING", "PONG",
		"USERLIST", "LIST", "STATS", "MOTD", "SHUTDOWN", "NOTME",
		"BBSMETA", "CAPABILITIES", "TOPIC", "CHATTERS", "WHOON",
		"USERS", "CHANNEL", "BBSES", "BANNERS", "ROUTING", "CHANGELOG",
		"TOPICS", "QUICKSTATS", "INFO",
	} {
		if up == verb || strings.HasPrefix(up, verb+":") {
			return false
		}
	}
	return true
}
