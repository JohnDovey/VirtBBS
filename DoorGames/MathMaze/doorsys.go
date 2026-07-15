package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DoorSession holds caller info from DOOR.SYS.
type DoorSession struct {
	UserName      string
	TimeLeftMins  int
	ANSI          bool
	NodeID        int
	SecurityLevel int
}

// ParseDoorSYS reads a GAP/PCBoard-style DOOR.SYS drop file.
func ParseDoorSYS(path string) (*DoorSession, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) < 10 {
		return nil, fmt.Errorf("DOOR.SYS: expected at least 10 lines, got %d", len(lines))
	}

	s := &DoorSession{
		UserName: strings.TrimSpace(lines[9]),
		ANSI:     true,
	}
	if s.UserName == "" {
		s.UserName = "Unknown"
	}
	if len(lines) >= 4 {
		s.NodeID, _ = strconv.Atoi(strings.TrimSpace(lines[3]))
	}
	if len(lines) >= 15 {
		s.SecurityLevel, _ = strconv.Atoi(strings.TrimSpace(lines[14]))
	}
	if len(lines) >= 19 {
		s.TimeLeftMins, _ = strconv.Atoi(strings.TrimSpace(lines[18]))
	}
	if len(lines) >= 20 {
		// DOOR.SYS: 0 = ANSI, 1 = no ANSI
		s.ANSI = strings.TrimSpace(lines[19]) != "1"
	}
	return s, nil
}

// ResolveDoorFile picks the drop file path from flags/env/args.
func ResolveDoorFile(explicit string, args []string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("DOORFILE"); v != "" {
		return v
	}
	for _, a := range args {
		if strings.Contains(strings.ToUpper(a), "DOOR.SYS") ||
			strings.Contains(strings.ToUpper(a), "DORINFO") {
			return a
		}
	}
	return ""
}
