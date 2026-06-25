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

package files

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ImportDIRLST reads a PCBoard DIR.LST file and populates the file_dirs table.
// DIR.LST has 125-byte fixed-length records:
//   DirPath[30]  DskPath[30]  DirDesc[35]  SortType[1]  padding[29]
//
// Reference: pcboard/Develop 04-15-97/DIRLST.DOC
func ImportDIRLST(db *sql.DB, path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("open DIR.LST: %w", err)
	}

	const recSize = 125
	imported := 0
	for i := 0; i+recSize <= len(data); i += recSize {
		rec := data[i : i+recSize]
		dirPath := strings.TrimRight(string(rec[0:30]), " \x00")
		dskPath := strings.TrimRight(string(rec[30:60]), " \x00")
		desc := strings.TrimRight(string(rec[60:95]), " \x00")
		sortType := int(rec[95])

		if dirPath == "" {
			continue
		}

		_, err := db.Exec(`INSERT OR IGNORE INTO file_dirs (name, description, path, sort_type)
			VALUES (?,?,?,?)`, desc, "Imported from PCBoard: "+dskPath, dirPath, sortType)
		if err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}

// ImportDIRFile reads a PCBoard DIR text file and registers its entries in the
// files table for the given dirID. PCBoard DIR files are column-sensitive:
//   col 0-20:  filename (no spaces, right-padded)
//   col 21:    space
//   col 22-23: size (right-justified to col 21)
//   col 24-31: date MM-DD-YY
//   col 33+:   description
func ImportDIRFile(store *Store, dirID int64, path, uploaderName string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	imported := 0
	sc := bufio.NewScanner(f)
	var lastFile string
	var lastDesc strings.Builder

	flush := func() {
		if lastFile == "" {
			return
		}
		_ = store.RegisterUpload(dirID, lastFile, strings.TrimSpace(lastDesc.String()), uploaderName)
		imported++
		lastFile = ""
		lastDesc.Reset()
	}

	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 || line[0] == ' ' {
			// Continuation description line
			if lastFile != "" {
				lastDesc.WriteString(" " + strings.TrimSpace(line))
			}
			continue
		}
		if strings.HasPrefix(line, "%") {
			// Include directive — skip
			continue
		}

		// Try to parse as a file entry
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		name := fields[0]
		if strings.ContainsAny(name, ":/\\") {
			continue // skip header lines
		}

		// Extract description (everything after col 33, or after size+date)
		desc := ""
		if len(line) > 33 {
			desc = strings.TrimSpace(line[33:])
		} else if len(fields) > 3 {
			desc = strings.Join(fields[3:], " ")
		}

		// Size from column 10-21 region
		sizeStr := ""
		if len(line) >= 22 {
			sizeStr = strings.TrimSpace(line[10:22])
		}
		_ = sizeStr // stored on disk; RegisterUpload reads actual size

		flush()
		lastFile = name
		lastDesc.WriteString(desc)
	}
	flush()
	return imported, sc.Err()
}

// ImportDIRFileByLine parses a simpler space-delimited DIR listing.
func parseSizeField(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
