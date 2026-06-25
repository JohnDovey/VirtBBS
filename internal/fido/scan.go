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
//   v0.0.3  2026-06-24  Phase 9: FidoNet scan — export messages to .PKT
//   v0.0.6  2026-06-24  Multi-uplink bundling; per-conference uplink_addr override;
//                        network-aware scanning via conferences.Store
// ============================================================================

package fido

// Scan exports echo-flagged messages from the VirtBBS message store into
// outbound .PKT files, one PKT per unique uplink address.
//
// Each echomail conference can override the default uplink via its UplinkAddr
// field.  Messages destined for the same uplink are bundled into one PKT file.
// If no conference overrides apply, all messages go into a single PKT addressed
// to the default uplink.

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// ScanResult summarises the outcome of a scan run.
type ScanResult struct {
	Scanned  int // messages exported
	PKTFiles int // distinct .pkt files written
	Errors   []string
}

// ScanAll exports all echo-flagged messages from every configured echomail
// conference into outbound .PKT files, one file per unique uplink address.
//
// It accepts an optional conferences.Store; when nil, falls back to the
// old cfg.Areas map (compatibility with pre-v0.0.6 setups).
func ScanAll(cfg *Config, store *messages.Store, confStore *conferences.Store) (*ScanResult, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("FidoNet is disabled in config")
	}

	result := &ScanResult{}

	// Process every configured network (primary + additional).
	for _, nd := range cfg.AllNetworks() {
		if !nd.Enabled {
			continue
		}
		if err := scanNetwork(cfg, &nd, store, confStore, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] %v", nd.Name, err))
		}
	}
	return result, nil
}

// scanNetwork processes one network.
func scanNetwork(cfg *Config, nd *NetworkDef, store *messages.Store, confStore *conferences.Store, result *ScanResult) error {
	orig := nd.NodeAddr()
	defaultUplink := nd.UplinkAddr()

	if orig == (Addr{}) {
		return fmt.Errorf("invalid node address %q", nd.Address)
	}
	if defaultUplink == (Addr{}) {
		return fmt.Errorf("invalid uplink address %q", nd.Uplink)
	}

	if err := os.MkdirAll(nd.OutboundDir, 0755); err != nil {
		return err
	}

	// per-uplink message bucket: uplinkAddr.String() → []*Message
	buckets := map[string][]*Message{}
	// uplink address cache for writing PKTs
	uplinkAddrs := map[string]Addr{}

	// ── Build buckets ─────────────────────────────────────────────────────────

	if confStore != nil {
		// Use conference store: iterate echomail conferences for this network.
		confs, err := confStore.ListEcho(nd.Name)
		if err != nil {
			return fmt.Errorf("listing echo confs: %w", err)
		}

		for _, conf := range confs {
			if conf.EchoTag == "" {
				continue
			}

			// Determine uplink for this conference.
			uplinkAddr := defaultUplink
			uplinkStr := nd.Uplink
			if conf.UplinkAddr != "" {
				a, err := ParseAddr(conf.UplinkAddr)
				if err == nil {
					uplinkAddr = a
					uplinkStr = conf.UplinkAddr
				}
			}
			key := uplinkAddr.String()
			uplinkAddrs[key] = uplinkAddr
			_ = uplinkStr

			msgs, err := store.ListEcho(conf.ID, 500, 0)
			if err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("conf %d (%s): %v", conf.ID, conf.Name, err))
				continue
			}
			for _, m := range msgs {
				body := buildEchoBody(conf.EchoTag, orig, m.Body)
				buckets[key] = append(buckets[key], &Message{
					OrigAddr: orig,
					DestAddr: uplinkAddr,
					DateTime: m.DatePosted.Format("02 Jan 06  15:04:05"),
					ToName:   m.ToName,
					FromName: m.FromName,
					Subject:  m.Subject,
					Body:     body,
				})
				result.Scanned++
			}
		}
	} else {
		// Fall back to cfg.Areas map (primary network only).
		for areaTag, confID := range nd.Areas {
			msgs, err := store.ListEcho(confID, 500, 0)
			if err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("area %s conf %d: %v", areaTag, confID, err))
				continue
			}
			key := defaultUplink.String()
			uplinkAddrs[key] = defaultUplink
			for _, m := range msgs {
				body := buildEchoBody(areaTag, orig, m.Body)
				buckets[key] = append(buckets[key], &Message{
					OrigAddr: orig,
					DestAddr: defaultUplink,
					DateTime: m.DatePosted.Format("02 Jan 06  15:04:05"),
					ToName:   m.ToName,
					FromName: m.FromName,
					Subject:  m.Subject,
					Body:     body,
				})
				result.Scanned++
			}
		}
	}

	// ── Write one PKT per uplink bucket ──────────────────────────────────────

	for key, msgs := range buckets {
		if len(msgs) == 0 {
			continue
		}
		uplinkAddr := uplinkAddrs[key]
		pktName := filepath.Join(nd.OutboundDir,
			fmt.Sprintf("%s_%s.pkt", nd.Name, time.Now().Format("20060102150405")))

		f, err := os.Create(pktName)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create pkt %s: %v", pktName, err))
			continue
		}

		password := nd.Password
		if err := WritePacket(f, orig, uplinkAddr, password, msgs); err != nil {
			f.Close()
			result.Errors = append(result.Errors, fmt.Sprintf("write pkt %s: %v", pktName, err))
			continue
		}
		f.Close()
		result.PKTFiles++
	}

	return nil
}

// buildEchoBody prepends the AREA: kludge line and an origin line to the body.
func buildEchoBody(areaTag string, orig Addr, body string) string {
	return fmt.Sprintf("AREA:%s\r%s\r\x01ORIGIN: VirtBBS @ %s\r",
		areaTag, body, orig.String())
}
