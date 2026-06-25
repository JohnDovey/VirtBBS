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
//   v0.0.3  2026-06-24  Phase 9: PCBoard MSGS binary importer (MSGS.DOC format)
// ============================================================================

package messages

// ImportMSGS imports messages from a PCBoard binary MSGS file into the
// given conference in VirtBBS's SQLite message store.
//
// Format reference: MSGS.DOC (PCBoard Developer Documentation, 03/24/93)
//
// File layout:
//   Byte 0: 128-byte message base header (skipped)
//   Then: repeated message records, each consisting of a 128-byte header
//         block followed by N body blocks of 128 bytes each.
//
// Message header block (128 bytes):
//   [0]     : status flag (' '=normal, '*'=deleted, '~'=read, etc.)
//   [1-4]   : bsreal — message number
//   [5-8]   : bsreal — reference (reply-to) message number
//   [9]     : byte   — count of 128-byte body blocks following this header
//   [10-17] : str[8] — date "mm-dd-yy"
//   [18-22] : str[5] — time "hh:mm"
//   [23-47] : str[25]— TO name
//   [48-51] : bsreal — date of reply (yymmdd, ignored)
//   [52-56] : str[5] — time of reply
//   [57]    : char   — 'R' if has reply
//   [58-82] : str[25]— FROM name
//   [83-107]: str[25]— subject
//   [108-119]: str[12]— password
//   [120]   : byte   — active status: 225 (0xE1) = active, 226 (0xE2) = inactive
//   [121]   : byte   — 'E' if echoed
//   [122-127]: reserved

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/virtbbs/virtbbs/pkg/pcbformat"
)

const (
	blockSize    = 128
	activeStatus = 225 // 0xE1
)

// ImportMSGS reads a PCBoard MSGS binary file and posts all active messages
// into the given conference. Returns the count of imported and skipped messages.
func ImportMSGS(store *Store, conferenceID int, path string) (imported, skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open MSGS: %w", err)
	}
	defer f.Close()

	// Skip the 128-byte message base header.
	if _, err := f.Seek(blockSize, io.SeekStart); err != nil {
		return 0, 0, fmt.Errorf("seek past MSGS header: %w", err)
	}

	hdr := make([]byte, blockSize)
	for {
		// Read message header block.
		if _, err := io.ReadFull(f, hdr); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return imported, skipped, fmt.Errorf("read header: %w", err)
		}

		status := hdr[0]
		activeFlag := hdr[120]
		bodyBlocks := int(hdr[9])

		// Read the body blocks (even if we skip the message, we must consume them).
		var bodyData []byte
		if bodyBlocks > 0 {
			bodyData = make([]byte, bodyBlocks*blockSize)
			if _, err := io.ReadFull(f, bodyData); err != nil {
				// Truncated file — stop cleanly.
				break
			}
		}

		// Skip deleted or inactive messages.
		if status == '*' || activeFlag != activeStatus {
			skipped++
			continue
		}

		msgNum  := int(pcbformat.Float4ToInt([4]byte(hdr[1:5])))
		refNum  := int(pcbformat.Float4ToInt([4]byte(hdr[5:9])))
		date    := pcbformat.TrimFixed(hdr[10:18]) // "mm-dd-yy"
		timeStr := pcbformat.TrimFixed(hdr[18:23]) // "hh:mm"
		toName  := pcbformat.TrimFixed(hdr[23:48])
		from    := pcbformat.TrimFixed(hdr[58:83])
		subject := pcbformat.TrimFixed(hdr[83:108])

		// Parse date: "mm-dd-yy" → time.Time
		posted := parseMMDDYY(date, timeStr)

		// Decode body text: null-terminated ASCII within 128-byte blocks.
		// PCBoard uses \r as line separator within the body.
		body := decodeBody(bodyData)

		_ = refNum // stored for future reply threading

		msgStatus := "A"
		if status == '~' || status == '`' {
			msgStatus = "R" // already-read flag
		}

		m := &Message{
			ConferenceID: conferenceID,
			MsgNumber:    msgNum,
			FromName:     from,
			ToName:       toName,
			Subject:      subject,
			DatePosted:   posted,
			Status:       msgStatus,
			Body:         body,
		}

		if err := store.PostWithNumber(m); err != nil {
			skipped++
			continue
		}
		imported++
	}
	return imported, skipped, nil
}

// parseMMDDYY converts "mm-dd-yy" + "hh:mm" into a time.Time.
// Year 70+ is treated as 19xx; below 70 as 20xx.
func parseMMDDYY(date, timeStr string) time.Time {
	t, err := time.Parse("01-02-06 15:04", date+" "+timeStr)
	if err == nil {
		return t
	}
	// Fallback: try without time.
	t, err = time.Parse("01-02-06", date)
	if err == nil {
		return t
	}
	return time.Now()
}

// decodeBody converts raw PCBoard body block bytes into a clean string.
// Body uses \r (0x0D) as line separators; text ends at first 0x00.
func decodeBody(raw []byte) string {
	// Find null terminator.
	end := len(raw)
	for i, b := range raw {
		if b == 0x00 {
			end = i
			break
		}
	}
	// Replace \r with \r\n, strip non-printable chars.
	var sb strings.Builder
	for _, b := range raw[:end] {
		if b == '\r' {
			sb.WriteString("\r\n")
		} else if b == '\n' {
			// ignore bare LF
		} else if b >= 0x20 || unicode.IsPrint(rune(b)) {
			sb.WriteByte(b)
		}
	}
	return strings.TrimSpace(sb.String())
}
