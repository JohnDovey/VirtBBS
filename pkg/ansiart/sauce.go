package ansiart

import (
	"bytes"
	"encoding/binary"
	"strings"
	"time"
)

// SauceMeta holds fields written into a SAUCE00 record.
type SauceMeta struct {
	Title   string
	Author  string
	Group   string
	Date    string // CCYYMMDD; empty = today
	Width   int
	Height  int
	ANSI    bool // true = Character/ANSI, false = Character/ASCII
	Comment string
}

// AppendSAUCE appends EOF + optional COMNT + 128-byte SAUCE00 to art content.
func AppendSAUCE(art []byte, meta SauceMeta) []byte {
	if meta.Date == "" {
		meta.Date = time.Now().Format("20060102")
	}
	fileSize := uint32(len(art))

	var comments []byte
	commentLines := 0
	if strings.TrimSpace(meta.Comment) != "" {
		line := padField(meta.Comment, 64)
		comments = append([]byte("COMNT"), []byte(line)...)
		commentLines = 1
	}

	rec := make([]byte, 128)
	copy(rec[0:7], []byte("SAUCE00"))
	copy(rec[7:42], []byte(padField(meta.Title, 35)))
	copy(rec[42:62], []byte(padField(meta.Author, 20)))
	copy(rec[62:82], []byte(padField(meta.Group, 20)))
	copy(rec[82:90], []byte(padField(meta.Date, 8)))
	binary.LittleEndian.PutUint32(rec[90:94], fileSize)
	rec[94] = 1 // DataType: Character
	if meta.ANSI {
		rec[95] = 1 // FileType: ANSI
	} else {
		rec[95] = 0 // FileType: ASCII
	}
	binary.LittleEndian.PutUint16(rec[96:98], uint16(meta.Width))  // TInfo1
	binary.LittleEndian.PutUint16(rec[98:100], uint16(meta.Height)) // TInfo2
	rec[104] = byte(commentLines)                                  // Comments

	var out bytes.Buffer
	out.Write(art)
	out.WriteByte(0x1a)
	if len(comments) > 0 {
		out.Write(comments)
	}
	out.Write(rec)
	return out.Bytes()
}

// SauceInfo is parsed SAUCE metadata for display.
type SauceInfo struct {
	Title   string
	Author  string
	Group   string
	Date    string
	Width   int
	Height  int
	ANSI    bool
	Comment string
	FileSize uint32
}

// ReadSAUCE parses a trailing SAUCE00 record if present.
func ReadSAUCE(data []byte) (SauceInfo, bool) {
	if len(data) < 128 {
		return SauceInfo{}, false
	}
	rec := data[len(data)-128:]
	if string(rec[0:7]) != "SAUCE00" {
		return SauceInfo{}, false
	}
	info := SauceInfo{
		Title:    strings.TrimSpace(string(rec[7:42])),
		Author:   strings.TrimSpace(string(rec[42:62])),
		Group:    strings.TrimSpace(string(rec[62:82])),
		Date:     strings.TrimSpace(string(rec[82:90])),
		FileSize: binary.LittleEndian.Uint32(rec[90:94]),
		Width:    int(binary.LittleEndian.Uint16(rec[96:98])),
		Height:   int(binary.LittleEndian.Uint16(rec[98:100])),
		ANSI:     rec[95] == 1,
	}
	nComments := int(rec[104])
	if nComments > 0 {
		comntLen := 5 + nComments*64
		start := len(data) - 128 - comntLen
		if start >= 0 && start+5 <= len(data) && string(data[start:start+5]) == "COMNT" {
			info.Comment = strings.TrimSpace(string(data[start+5 : start+5+64]))
		}
	}
	return info, true
}

func padField(s string, n int) string {
	if len(s) > n {
		s = s[:n]
	}
	for len(s) < n {
		s += " "
	}
	return s
}
