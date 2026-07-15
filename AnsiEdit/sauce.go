package main

import (
	"bytes"
	"encoding/binary"
	"strings"
)

const (
	sauceID      = "SAUCE00"
	sauceLen     = 128
	comntID      = "COMNT"
	comntLineLen = 64
	maxComments  = 255
)

// Sauce holds ACiD SAUCE00 metadata and optional COMNT lines.
type Sauce struct {
	Present      bool
	Title        string
	Author       string
	Group        string
	Date         string // CCYYMMDD
	FileSize     uint32
	DataType     uint8
	FileType     uint8
	TInfo1       uint16 // width
	TInfo2       uint16 // height
	TInfo3       uint16
	TInfo4       uint16
	Comments     uint8
	Flags        uint8
	TInfoS       [22]byte
	CommentLines []string // each ≤ 64 chars
}

func NewSauce() Sauce {
	return Sauce{
		Present:  true,
		Date:     formatSauceDate(),
		DataType: 1, // Character
		FileType: 1, // ANSI
	}
}

// SplitSauce separates art body from optional EOF + COMNT + SAUCE00 trailer.
// Layout: [art][0x1A][COMNT + N×64]?[SAUCE00 128]
func SplitSauce(data []byte) (art []byte, sauce Sauce) {
	if len(data) < sauceLen {
		return data, Sauce{}
	}
	rec := data[len(data)-sauceLen:]
	if string(rec[0:7]) != sauceID {
		return data, Sauce{}
	}
	s := parseSauceRecord(rec)

	sauceStart := len(data) - sauceLen
	bodyEnd := sauceStart

	if s.Comments > 0 {
		comntBytes := 5 + int(s.Comments)*comntLineLen
		comntStart := sauceStart - comntBytes
		if comntStart < 0 {
			return data, Sauce{}
		}
		if string(data[comntStart:comntStart+5]) != comntID {
			// Malformed; strip SAUCE only
			if bodyEnd > 0 && data[bodyEnd-1] == 0x1A {
				bodyEnd--
			}
			s.Present = true
			return data[:bodyEnd], s
		}
		s.CommentLines = make([]string, s.Comments)
		off := comntStart + 5
		for i := 0; i < int(s.Comments); i++ {
			line := data[off : off+comntLineLen]
			s.CommentLines[i] = strings.TrimRight(string(line), "\x00 ")
			off += comntLineLen
		}
		bodyEnd = comntStart
	}

	if bodyEnd > 0 && data[bodyEnd-1] == 0x1A {
		bodyEnd--
	}
	s.Present = true
	return data[:bodyEnd], s
}

func parseSauceRecord(rec []byte) Sauce {
	s := Sauce{Present: true}
	s.Title = strings.TrimRight(string(rec[7:42]), "\x00 ")
	s.Author = strings.TrimRight(string(rec[42:62]), "\x00 ")
	s.Group = strings.TrimRight(string(rec[62:82]), "\x00 ")
	s.Date = strings.TrimRight(string(rec[82:90]), "\x00 ")
	s.FileSize = binary.LittleEndian.Uint32(rec[90:94])
	s.DataType = rec[94]
	s.FileType = rec[95]
	s.TInfo1 = binary.LittleEndian.Uint16(rec[96:98])
	s.TInfo2 = binary.LittleEndian.Uint16(rec[98:100])
	s.TInfo3 = binary.LittleEndian.Uint16(rec[100:102])
	s.TInfo4 = binary.LittleEndian.Uint16(rec[102:104])
	s.Comments = rec[104]
	s.Flags = rec[105]
	copy(s.TInfoS[:], rec[106:128])
	return s
}

// AppendSauce appends EOF + optional COMNT + SAUCE00 to art.
func AppendSauce(art []byte, s Sauce) []byte {
	if !s.Present {
		return art
	}
	lines := make([]string, 0, len(s.CommentLines))
	for _, ln := range s.CommentLines {
		if len(ln) > comntLineLen {
			ln = ln[:comntLineLen]
		}
		lines = append(lines, ln)
		if len(lines) >= maxComments {
			break
		}
	}
	s.CommentLines = lines
	s.Comments = uint8(len(lines))
	s.FileSize = uint32(len(art))

	var buf bytes.Buffer
	buf.Write(art)
	buf.WriteByte(0x1A)
	if len(lines) > 0 {
		buf.WriteString(comntID)
		for _, ln := range lines {
			pad := make([]byte, comntLineLen)
			copy(pad, []byte(ln))
			buf.Write(pad)
		}
	}
	buf.Write(encodeSauceRecord(s))
	return buf.Bytes()
}

func encodeSauceRecord(s Sauce) []byte {
	rec := make([]byte, sauceLen)
	copy(rec[0:7], sauceID)
	copyFixed(rec[7:42], s.Title, 35)
	copyFixed(rec[42:62], s.Author, 20)
	copyFixed(rec[62:82], s.Group, 20)
	date := s.Date
	if len(date) != 8 {
		date = formatSauceDate()
	}
	copyFixed(rec[82:90], date, 8)
	binary.LittleEndian.PutUint32(rec[90:94], s.FileSize)
	rec[94] = s.DataType
	rec[95] = s.FileType
	binary.LittleEndian.PutUint16(rec[96:98], s.TInfo1)
	binary.LittleEndian.PutUint16(rec[98:100], s.TInfo2)
	binary.LittleEndian.PutUint16(rec[100:102], s.TInfo3)
	binary.LittleEndian.PutUint16(rec[102:104], s.TInfo4)
	rec[104] = s.Comments
	rec[105] = s.Flags
	copy(rec[106:128], s.TInfoS[:])
	return rec
}

func copyFixed(dst []byte, s string, n int) {
	for i := range dst {
		dst[i] = 0x20
	}
	b := []byte(s)
	if len(b) > n {
		b = b[:n]
	}
	copy(dst, b)
}

func (s Sauce) Summary() string {
	if !s.Present {
		return "(no SAUCE)"
	}
	return strings.TrimSpace(s.Title) + " / " + strings.TrimSpace(s.Author)
}
