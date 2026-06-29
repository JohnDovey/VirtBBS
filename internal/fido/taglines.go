// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.1.0  2026-06-25  Initial implementation — configurable echomail taglines
// ============================================================================

package fido

import (
	"database/sql"
	"math/rand"
	"os"
	"strings"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// LoadTaglines reads one tagline per line from path, skipping blank lines.
// Returns nil (not an error) if path is empty or the file doesn't exist,
// so an unconfigured taglines feature is silently a no-op.
func LoadTaglines(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// PickTagline returns a random tagline from taglines, or "" if the list is empty.
func PickTagline(taglines []string) string {
	if len(taglines) == 0 {
		return ""
	}
	return taglines[rand.Intn(len(taglines))]
}

// ResolveTaglinesPath returns the taglines_file for a conference's Fido network,
// falling back to the primary network file when the network leaves it blank.
func ResolveTaglinesPath(cfg *Config, conf *conferences.Conference) string {
	if cfg == nil {
		return ""
	}
	path := strings.TrimSpace(cfg.TaglinesFile)
	if conf == nil {
		return path
	}
	netName := conferences.EffectiveNetwork(conf, cfg.EffectivePrimaryName())
	for _, nd := range cfg.AllNetworks() {
		if nd.Name == netName {
			if p := strings.TrimSpace(nd.TaglinesFile); p != "" {
				return p
			}
			break
		}
	}
	return path
}

// resolveNetworkTaglinesPath returns the taglines file for one network definition.
func resolveNetworkTaglinesPath(cfg *Config, nd *NetworkDef) string {
	if nd != nil {
		if p := strings.TrimSpace(nd.TaglinesFile); p != "" {
			return p
		}
	}
	if cfg != nil {
		return strings.TrimSpace(cfg.TaglinesFile)
	}
	return ""
}

// AppendEchoTagline appends a random tagline to a locally originated echomail
// message body when taglines are configured and the body has none yet.
func AppendEchoTagline(m *messages.Message, db *sql.DB, taglinesPath string) {
	if m == nil || !m.Echo || strings.TrimSpace(m.FidoOrigin) != "" {
		return
	}
	existing, _, _ := ParseEchoFooters(m.Body)
	if len(existing) > 0 {
		return
	}
	taglines := LoadTaglinesForUse(db, taglinesPath)
	if tl := PickTagline(taglines); tl != "" {
		body := strings.TrimRight(m.Body, "\r\n")
		m.Body = body + "\r\n\r\n" + tl + "\r\n"
	}
}

// taglineForEchoExport picks a tagline for outbound export of a local echo
// message. Returns "" when the message was received via toss or already carries
// a tagline in its stored body.
func taglineForEchoExport(m *messages.Message, taglines []string) string {
	if m == nil || strings.TrimSpace(m.FidoOrigin) != "" {
		return ""
	}
	existing, _, _ := ParseEchoFooters(m.Body)
	if len(existing) > 0 {
		return ""
	}
	return PickTagline(taglines)
}
