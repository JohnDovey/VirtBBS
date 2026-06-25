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
//   v0.0.1  2026-06-24  Initial implementation
// ============================================================================

package users

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/virtbbs/virtbbs/pkg/pcbformat"
)

// PCBoard USERS record is exactly 400 bytes.
const pcbUserRecSize = 400

// pcbUserRecord mirrors the on-disk layout of a PCBoard USERS file record.
// All strings are space-padded (not null-terminated).
// Reference: pcboard/Develop 04-15-97/USERS.DOC
type pcbUserRecord struct {
	Name          [25]byte // offset 0
	City          [24]byte // offset 25
	Password      [12]byte // offset 49
	PhoneBusiness [13]byte // offset 61
	PhoneHome     [13]byte // offset 74
	LastLoginDate [6]byte  // offset 87  YYMMDD
	LastLoginTime [5]byte  // offset 93  HH:MM
	ExpertMode    byte     // offset 98  'Y'/'N'
	XferProtocol  byte     // offset 99  'A'-'Z'
	_             [4]byte  // offset 100 reserved
	SecurityLevel uint16   // offset 104 (little-endian)
	TimesOnline   uint16   // offset 106
	PageLength    byte     // offset 108
	_             [7]byte  // offset 109 reserved
	Uploads       uint16   // offset 116
	Downloads     uint16   // offset 118
	_             [8]byte  // offset 120 (bytes dn as 4-byte float at 120, up at 124)
	Comment1      [30]byte // offset 128
	Comment2      [30]byte // offset 158
	_             [2]byte  // offset 188 elapsed time (16-bit)
	ExpireDate    [6]byte  // offset 190 YYMMDD
	_             [5]byte  // offset 196 conference flags byte1..5
	_             [5]byte  // offset 201 user selected conf flags
	_             [24]byte // offset 206 last msg read (float per conf, first 6)
	DeleteFlag    byte     // offset 230 '*' = deleted
	_             [169]byte // offset 231 padding to 400
}

// ImportUSERS reads a PCBoard USERS binary file and upserts records into the store.
// Existing users (matched by name) are skipped to avoid overwriting passwords.
func ImportUSERS(store *Store, path string) (imported, skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open USERS: %w", err)
	}
	defer f.Close()

	for {
		raw := make([]byte, pcbUserRecSize)
		_, err := io.ReadFull(f, raw)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return imported, skipped, err
		}

		name := pcbformat.TrimFixed(raw[0:25])
		if name == "" {
			skipped++
			continue
		}
		deleted := raw[230] == '*'

		// Check if user already exists
		existing, _ := store.GetByName(name)
		if existing != nil {
			skipped++
			continue
		}

		city := pcbformat.TrimFixed(raw[25:49])
		phone1 := pcbformat.TrimFixed(raw[61:74])
		phone2 := pcbformat.TrimFixed(raw[74:87])
		comment1 := pcbformat.TrimFixed(raw[128:158])
		comment2 := pcbformat.TrimFixed(raw[158:188])
		expDate := pcbformat.TrimFixed(raw[190:196])
		secLevel := int(binary.LittleEndian.Uint16(raw[104:106]))
		timesOnline := int(binary.LittleEndian.Uint16(raw[106:108]))
		pageLen := int(raw[108])
		if pageLen == 0 {
			pageLen = 24
		}
		uploads := int(binary.LittleEndian.Uint16(raw[116:118]))
		downloads := int(binary.LittleEndian.Uint16(raw[118:120]))
		expertMode := raw[98] == 'Y'
		xfer := string([]byte{raw[99]})
		if xfer == "\x00" || xfer == " " {
			xfer = "Z"
		}

		// PCBoard stores last login as YYMMDD — convert to YYYY-MM-DD for SQLite
		lastDate := convertYYMMDD(pcbformat.TrimFixed(raw[87:93]))
		lastTime := pcbformat.TrimFixed(raw[93:98])

		u := &User{
			Name:           name,
			City:           city,
			PhoneBusiness:  phone1,
			PhoneHome:      phone2,
			LastLoginDate:  lastDate,
			LastLoginTime:  lastTime,
			SecurityLevel:  secLevel,
			TimesOnline:    timesOnline,
			PageLength:     pageLen,
			Uploads:        uploads,
			Downloads:      downloads,
			Comment1:       comment1,
			Comment2:       comment2,
			ExpirationDate: convertYYMMDD(expDate),
			ExpertMode:     expertMode,
			XferProtocol:   xfer,
			ANSI:           true, // default to ANSI on for imported users
			Deleted:        deleted,
		}

		// Imported users get a random unusable password — sysop must reset.
		if err := store.Create(u, generateRandomPassword()); err != nil {
			return imported, skipped, fmt.Errorf("insert user %q: %w", name, err)
		}
		imported++
	}
	return imported, skipped, nil
}

// convertYYMMDD converts PCBoard YYMMDD to YYYY-MM-DD (assumes 1900s).
func convertYYMMDD(s string) string {
	if len(s) != 6 || s == "000000" {
		return ""
	}
	yy, mm, dd := s[0:2], s[2:4], s[4:6]
	year := "19" + yy
	if yy < "70" {
		year = "20" + yy
	}
	return year + "-" + mm + "-" + dd
}

func generateRandomPassword() string {
	return "IMPORTED_MUST_RESET"
}
