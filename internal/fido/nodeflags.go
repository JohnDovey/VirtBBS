// Package fido — nodeflags.go
//
// Sysop-configurable nodelist capability flags (IBN, ITN, CM, etc.) for
// this BBS's own network entry.
package fido

import (
	"fmt"
	"strings"
)

// NodeFlagDef describes one known nodelist capability flag for the GUI.
type NodeFlagDef struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// DefaultNodeFlags are checked by default when a new network is created.
var DefaultNodeFlags = []string{"IBN", "ITN", "BEER", "TRACE", "PING"}

var knownNodeFlags = []NodeFlagDef{
	{Code: "IBN", Description: "Internet BinkP Node — accepts BinkP connections; may include hostname and port"},
	{Code: "INA", Description: "Internet Address — hostname for internet connectivity"},
	{Code: "ITN", Description: "Internet Telnet Node"},
	{Code: "CM", Description: "Continuous Mail — accepts connections 24 hours"},
	{Code: "MO", Description: "Mail Only — no interactive users"},
	{Code: "BEER", Description: "Sysop drinks beer"},
	{Code: "TRACE", Description: "Trace requests honoured"},
	{Code: "PING", Description: "Ping requests honoured"},
}

// KnownNodeFlags returns the full list of supported flags with descriptions.
func KnownNodeFlags() []NodeFlagDef {
	out := make([]NodeFlagDef, len(knownNodeFlags))
	copy(out, knownNodeFlags)
	return out
}

// ValidateNodeFlags checks every flag against the known set and returns a
// de-duplicated, upper-cased slice preserving first-seen order.
func ValidateNodeFlags(flags []string) ([]string, error) {
	known := map[string]struct{}{}
	for _, d := range knownNodeFlags {
		known[d.Code] = struct{}{}
	}
	seen := map[string]struct{}{}
	var out []string
	for _, f := range flags {
		code := strings.ToUpper(strings.TrimSpace(f))
		if code == "" {
			continue
		}
		if _, ok := known[code]; !ok {
			return nil, fmt.Errorf("unknown node flag %q", code)
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out, nil
}

func nodeFlagSet(flags []string) map[string]bool {
	set := map[string]bool{}
	for _, f := range flags {
		set[strings.ToUpper(strings.TrimSpace(f))] = true
	}
	return set
}

// BuildNodelistFlags composes the FTS-0005 flags field from configured
// node_flags plus optional BinkP hostname and telnet port.
func BuildNodelistFlags(flags []string, binkpHost string, binkpPort, telnetPort int) string {
	set := nodeFlagSet(flags)
	var parts []string

	if set["IBN"] {
		parts = append(parts, buildIBNFlag(binkpHost, binkpPort))
	}
	if set["INA"] {
		parts = append(parts, buildINAFlag(binkpHost))
	}
	if set["ITN"] {
		parts = append(parts, buildITNFlag(binkpHost, telnetPort))
	}
	for _, code := range []string{"CM", "MO", "BEER", "TRACE", "PING"} {
		if set[code] {
			parts = append(parts, code)
		}
	}
	return strings.Join(parts, ",")
}

func buildIBNFlag(binkpHost string, binkpPort int) string {
	h, p := splitDialHostPort(binkpHost)
	if p == 0 && binkpPort > 0 {
		p = binkpPort
	}
	if h != "" {
		if p > 0 && !strings.Contains(h, ":") {
			return fmt.Sprintf("IBN:%s:%d", h, p)
		}
		if p > 0 {
			return fmt.Sprintf("IBN:%s", joinHostPort(h, p))
		}
		return "IBN:" + h
	}
	return "IBN"
}

func buildINAFlag(binkpHost string) string {
	h, _ := splitDialHostPort(binkpHost)
	if h != "" {
		return "INA:" + h
	}
	return "INA"
}

func buildITNFlag(binkpHost string, telnetPort int) string {
	if telnetPort > 0 {
		return fmt.Sprintf("ITN:%d", telnetPort)
	}
	h, p := splitDialHostPort(binkpHost)
	if h != "" && p > 0 {
		return fmt.Sprintf("ITN:%s:%d", h, p)
	}
	return "ITN"
}

func joinHostPort(host string, port int) string {
	if strings.Contains(host, ":") {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
}
