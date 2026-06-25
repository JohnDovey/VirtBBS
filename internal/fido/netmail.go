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
//   v0.0.6  2026-06-24  Initial implementation — netmail compose, route, PKT write
// ============================================================================

// Package fido — netmail.go
//
// Composes and routes FidoNet netmail (personal messages between nodes).
//
// Routing rules (zone-aware):
//   Crash flag  → write PKT directly to outbound/<destAddr>/  (direct delivery)
//   Same zone   → route via uplink (uplink handles local delivery)
//   Other zone  → route via uplink (uplink contacts zone gate)
//   Point addr  → strip point, deliver to boss node (zone:net/node)
//
// PKT format: FTS-0001 Type-2 packet
//   58-byte packet header
//   N × message records (null-terminated fields)
//   0x0000 end-of-packet marker
package fido

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NetmailMsg holds the fields for one outbound netmail message.
type NetmailMsg struct {
	// Sender
	FromName string
	FromAddr string // e.g. "1:234/567"

	// Recipient
	ToName string
	ToAddr string // e.g. "1:234/568" or "1:234/567.1" (point)

	Subject string
	Body    string

	// Crash = true: bypass routing, write PKT directly to dest outbound dir.
	Crash bool

	// Network name (blank = primary).
	Network string
}

// NetmailDB wraps the database for netmail queue operations.
type NetmailDB struct{ db *sql.DB }

// OpenNetmailDB returns a NetmailDB using the shared database connection.
func OpenNetmailDB(db *sql.DB) *NetmailDB { return &NetmailDB{db: db} }

// Enqueue stores a netmail in the queue for the next poll cycle.
func (ndb *NetmailDB) Enqueue(m *NetmailMsg) (int64, error) {
	crash := 0
	if m.Crash {
		crash = 1
	}
	res, err := ndb.db.Exec(`INSERT INTO fido_netmail
		(from_name, from_addr, to_name, to_addr, subject, body, crash, network)
		VALUES (?,?,?,?,?,?,?,?)`,
		m.FromName, m.FromAddr, m.ToName, m.ToAddr,
		m.Subject, m.Body, crash, m.Network)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Pending returns all unsent netmails.
func (ndb *NetmailDB) Pending() ([]*NetmailMsg, []int64, error) {
	rows, err := ndb.db.Query(`SELECT id, from_name, from_addr, to_name, to_addr, subject, body, crash, network
		FROM fido_netmail WHERE sent_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var msgs []*NetmailMsg
	var ids []int64
	for rows.Next() {
		m := &NetmailMsg{}
		var id int64
		var crash int
		if err := rows.Scan(&id, &m.FromName, &m.FromAddr, &m.ToName, &m.ToAddr,
			&m.Subject, &m.Body, &crash, &m.Network); err != nil {
			return nil, nil, err
		}
		m.Crash = crash != 0
		msgs = append(msgs, m)
		ids = append(ids, id)
	}
	return msgs, ids, rows.Err()
}

// MarkSent marks a queued netmail as sent.
func (ndb *NetmailDB) MarkSent(id int64) error {
	_, err := ndb.db.Exec(`UPDATE fido_netmail SET sent_at=datetime('now') WHERE id=?`, id)
	return err
}

// ─── PKT writer ──────────────────────────────────────────────────────────────

// WritePKT writes a single FTS-0001 Type-2 PKT file containing the given
// messages.  Returns the path of the created file.
//
//   origAddr  — address of the sending system (us)
//   destAddr  — address of the next-hop system (uplink or direct dest)
//   password  — session password for the PKT header
//   outDir    — directory to write the .pkt file into (created if absent)
func WritePKT(origAddr, destAddr Addr, password, outDir string, msgs []*NetmailMsg) (string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	fname := fmt.Sprintf("%08X.pkt", time.Now().UnixNano()&0xFFFFFFFF)
	path := filepath.Join(outDir, fname)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// ── PKT header (58 bytes, FTS-0001 §2.2) ─────────────────────────────────
	now := time.Now()
	hdr := make([]byte, 58)

	le := binary.LittleEndian
	le.PutUint16(hdr[0:], uint16(origAddr.Node))
	le.PutUint16(hdr[2:], uint16(destAddr.Node))
	le.PutUint16(hdr[4:], uint16(now.Year()))
	hdr[6] = byte(now.Month() - 1) // month 0-based
	hdr[7] = byte(now.Day())
	hdr[8] = byte(now.Hour())
	hdr[9] = byte(now.Minute())
	hdr[10] = byte(now.Second())
	// hdr[11] padding
	le.PutUint16(hdr[12:], 0)                // baud = 0
	le.PutUint16(hdr[14:], 2)                // packet type 2
	le.PutUint16(hdr[16:], uint16(origAddr.Net))
	le.PutUint16(hdr[18:], uint16(destAddr.Net))

	// Password (8 bytes, padded with nulls).
	pw := []byte(password)
	if len(pw) > 8 {
		pw = pw[:8]
	}
	copy(hdr[20:28], pw)

	le.PutUint16(hdr[28:], uint16(origAddr.Zone))
	le.PutUint16(hdr[30:], uint16(destAddr.Zone))

	// Product code, serial, capability — all zero is fine for a basic PKT.
	// hdr[32..57] zeroed

	if _, err := f.Write(hdr); err != nil {
		return "", err
	}

	// ── Message records ───────────────────────────────────────────────────────
	for _, m := range msgs {
		from, _ := ParseAddr(m.FromAddr)
		to, _ := ParseAddr(m.ToAddr)
		if err := writeMsgRecord(f, m, from, to, origAddr.Zone); err != nil {
			return "", err
		}
	}

	// End-of-packet marker: 0x0000
	if _, err := f.Write([]byte{0, 0}); err != nil {
		return "", err
	}

	return path, nil
}

// writeMsgRecord writes one FTS-0001 message record.
// Record layout:
//   uint16  type       = 2
//   uint16  origNode
//   uint16  destNode
//   uint16  origNet
//   uint16  destNet
//   uint16  attribute flags
//   uint16  cost
//   [20]byte dateTime  (ASCII, like "Tue 24 Jun 2026 12:00:00")
//   NUL-terminated: toName, fromName, subject, body
func writeMsgRecord(w *os.File, m *NetmailMsg, from, to Addr, localZone int) error {
	le := binary.LittleEndian
	rec := make([]byte, 14)
	le.PutUint16(rec[0:], 2)                   // message type
	le.PutUint16(rec[2:], uint16(from.Node))
	le.PutUint16(rec[4:], uint16(to.Node))
	le.PutUint16(rec[6:], uint16(from.Net))
	le.PutUint16(rec[8:], uint16(to.Net))

	// Attribute: Private (0x0002) always set for netmail; Crash (0x0100) if flagged.
	attr := uint16(0x0002)
	if m.Crash {
		attr |= 0x0100
	}
	le.PutUint16(rec[10:], attr)
	le.PutUint16(rec[12:], 0) // cost

	if _, err := w.Write(rec); err != nil {
		return err
	}

	// Date/time: 20 bytes, "DD Mmm YY  HH:MM:SS"
	dt := time.Now().Format("02 Jan 06  15:04:05")
	dtBuf := make([]byte, 20)
	copy(dtBuf, dt)
	if _, err := w.Write(dtBuf); err != nil {
		return err
	}

	// NUL-terminated strings.
	for _, s := range []string{m.ToName, m.FromName, m.Subject} {
		if _, err := w.Write(append([]byte(s), 0)); err != nil {
			return err
		}
	}

	// Body: include KLUDGE lines for zone addressing if cross-zone.
	body := buildBody(m, from, to, localZone)
	if _, err := w.Write(append([]byte(body), 0)); err != nil {
		return err
	}

	return nil
}

// buildBody prepends FTS KLUDGE lines and MSGID to the body.
func buildBody(m *NetmailMsg, from, to Addr, localZone int) string {
	var sb strings.Builder

	// MSGID kludge
	msgID := fmt.Sprintf("\x01MSGID: %s %08X\r\n", m.FromAddr, time.Now().UnixNano()&0xFFFFFFFF)
	sb.WriteString(msgID)

	// INTL kludge: required when source/dest zones differ or either is non-local.
	if from.Zone != to.Zone || from.Zone != localZone {
		intl := fmt.Sprintf("\x01INTL %d:%d/%d %d:%d/%d\r\n",
			to.Zone, to.Net, to.Node,
			from.Zone, from.Net, from.Node)
		sb.WriteString(intl)
	}

	// FMPT kludge for point source.
	if from.Point != 0 {
		sb.WriteString(fmt.Sprintf("\x01FMPT %d\r\n", from.Point))
	}
	// TOPT kludge for point destination.
	if to.Point != 0 {
		sb.WriteString(fmt.Sprintf("\x01TOPT %d\r\n", to.Point))
	}

	sb.WriteString(m.Body)
	return sb.String()
}

// ─── Routing helper ─────────────────────────────────────────────────────────

// RouteAddr returns the next-hop address for a netmail message:
//   - Crash: direct to destination (strip point → boss node)
//   - Otherwise: route through uplink
//   - Point: strip point from destination address
func RouteAddr(m *NetmailMsg, nd *NetworkDef) (Addr, error) {
	dest, err := ParseAddr(m.ToAddr)
	if err != nil {
		return Addr{}, fmt.Errorf("invalid destination address %q: %w", m.ToAddr, err)
	}

	if m.Crash {
		// Crash netmail: go directly to the boss (strip point).
		boss := Addr{Zone: dest.Zone, Net: dest.Net, Node: dest.Node}
		return boss, nil
	}

	// Routed: go via uplink.
	uplink := nd.UplinkAddr()
	if uplink.Zone == 0 {
		return Addr{}, fmt.Errorf("no uplink configured for network %s", nd.Name)
	}
	return uplink, nil
}

// OutboundDir returns the per-dest outbound subdirectory for crash netmail,
// or the general outbound dir for routed netmail.
func OutboundDir(baseOutbound string, nextHop Addr, crash bool) string {
	if crash {
		sub := fmt.Sprintf("%04X%04X.OUT", nextHop.Zone*0x100+nextHop.Net, nextHop.Node)
		return filepath.Join(baseOutbound, sub)
	}
	return baseOutbound
}
