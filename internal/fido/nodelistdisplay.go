package fido

import (
	"fmt"
	"strings"
)

// NodelistFlagDisplay is one parsed capability flag from a nodelist entry.
type NodelistFlagDisplay struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Value       string `json:"value,omitempty"`
}

// DescribeNodelistFlags expands the raw FTS-0005 flags field into labeled parts.
func DescribeNodelistFlags(raw string) []NodelistFlagDisplay {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	descByCode := map[string]string{}
	for _, d := range knownNodeFlags {
		descByCode[d.Code] = d.Description
	}
	var out []NodelistFlagDisplay
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.Index(part, ":"); i >= 0 {
			code := strings.ToUpper(strings.TrimSpace(part[:i]))
			val := strings.TrimSpace(part[i+1:])
			out = append(out, NodelistFlagDisplay{
				Code:        code,
				Description: flagDescription(descByCode, code, val),
				Value:       val,
			})
			continue
		}
		code := strings.ToUpper(part)
		out = append(out, NodelistFlagDisplay{
			Code:        code,
			Description: flagDescription(descByCode, code, ""),
		})
	}
	return out
}

func flagDescription(known map[string]string, code, value string) string {
	switch code {
	case "IBN":
		if value != "" {
			return fmt.Sprintf("%s — %s", known["IBN"], value)
		}
	case "ITN":
		if value != "" {
			return fmt.Sprintf("%s — port %s", known["ITN"], value)
		}
	case "INA":
		if value != "" {
			return fmt.Sprintf("%s — %s", known["INA"], value)
		}
	}
	if d, ok := known[code]; ok {
		return d
	}
	if value != "" {
		return code + " — " + value
	}
	return code
}
