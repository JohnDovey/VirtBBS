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
//   v0.0.3  2026-06-24  Phase 9: FidoNet toss — import .PKT into message store
// ============================================================================

package fido

// Toss processes all .PKT files in a directory, importing messages into the
// VirtBBS message store according to the area→conference map in Config.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/messages"
)

// TossResult summarises the outcome of a toss run.
type TossResult struct {
	Packets  int // .PKT files processed
	Imported int // messages inserted
	Skipped  int // messages ignored (unknown area, duplicate, etc.)
	Errors   []string
}

// TossDir reads every .PKT file in cfg.InboundDir, imports all recognised
// echomail messages, and moves processed packets to <inbound>/.tossed/.
func TossDir(cfg *Config, store *messages.Store) (*TossResult, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("FidoNet is disabled in config")
	}
	if err := os.MkdirAll(cfg.InboundDir, 0755); err != nil {
		return nil, err
	}
	tossed := filepath.Join(cfg.InboundDir, ".tossed")
	if err := os.MkdirAll(tossed, 0755); err != nil {
		return nil, err
	}

	result := &TossResult{}

	entries, err := os.ReadDir(cfg.InboundDir)
	if err != nil {
		return nil, fmt.Errorf("read inbound dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".pkt") {
			continue
		}

		pktPath := filepath.Join(cfg.InboundDir, e.Name())
		imp, skip, errs := tossFile(cfg, store, pktPath)
		result.Packets++
		result.Imported += imp
		result.Skipped += skip
		result.Errors = append(result.Errors, errs...)

		// Move processed packet to .tossed/.
		dest := filepath.Join(tossed, e.Name())
		_ = os.Rename(pktPath, dest)
	}
	return result, nil
}

// TossFile processes a single .PKT file, importing its messages.
func TossFile(cfg *Config, store *messages.Store, pktPath string) (imported, skipped int, errs []string) {
	return tossFile(cfg, store, pktPath)
}

func tossFile(cfg *Config, store *messages.Store, pktPath string) (imported, skipped int, errs []string) {
	f, err := os.Open(pktPath)
	if err != nil {
		errs = append(errs, fmt.Sprintf("%s: %v", pktPath, err))
		return
	}
	defer f.Close()

	msgs, err := ReadPacket(f)
	if err != nil {
		errs = append(errs, fmt.Sprintf("%s: parse error: %v", pktPath, err))
		return
	}

	for _, pm := range msgs {
		area := pm.AreaTag()

		var confID int
		if area == "" {
			// NetMail — route to conference 0 (General) addressed to the recipient.
			confID = 0
		} else {
			confID = cfg.ConferenceForArea(area)
			if confID < 0 {
				skipped++
				continue // unknown area
			}
		}

		// Parse date from FTS dateTime string "dd Mon yy  hh:mm:ss"
		posted := parseFidoDate(pm.DateTime)

		m := &messages.Message{
			ConferenceID: confID,
			FromName:     pm.FromName,
			ToName:       pm.ToName,
			Subject:      pm.Subject,
			DatePosted:   posted,
			Status:       "A",
			Echo:         area != "",
			Body:         pm.CleanBody(),
		}
		if err := store.Post(m); err != nil {
			errs = append(errs, fmt.Sprintf("insert: %v", err))
			skipped++
			continue
		}
		imported++
	}
	return
}

// parseFidoDate parses an FTS-0001 date string.
// Format: "dd Mon yy  hh:mm:ss" (e.g. "25 Jun 24  14:30:00")
func parseFidoDate(s string) time.Time {
	// Try multiple common FidoNet date formats.
	formats := []string{
		"02 Jan 06  15:04:05",
		"02 Jan 06 15:04:05",
		"_2 Jan 06  15:04:05",
		"Mon Jan  2 15:04:05 2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now()
}
