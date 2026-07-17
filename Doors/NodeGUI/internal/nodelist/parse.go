// Package nodelist downloads, extracts, and parses FidoNet distribution nodelists.
package nodelist

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/JohnDovey/NodeGUI/internal/store"
)

// Entry is one non-comment line from a distribution nodelist.
type Entry struct {
	Keyword  string // Zone, Region, Host, Hub, Pvt, Hold, Down, or empty
	Number   int
	Name     string
	Location string
	Sysop    string
	Phone    string
	MaxBaud  string
	Flags    string
	Raw      string
}

// Document is a fully parsed nodelist with hierarchical addresses resolved.
type Document struct {
	// HeaderDay is the julian day from the first ";A ... Day number NNN" line, if present.
	HeaderDay int
	// HeaderDate is a human-readable date from the header, if present.
	HeaderDate string
	// Nodes are resolved address records ready for storage.
	Nodes []store.Node
	// LineCount is the number of non-comment data lines parsed.
	LineCount int
	// Skipped is the number of unparseable data lines.
	Skipped int
}

// Parse reads a distribution nodelist (FTS-0005 style) from r.
func Parse(r io.Reader, domain string) (*Document, error) {
	if domain == "" {
		domain = "FidoNet"
	}
	doc := &Document{}
	sc := bufio.NewScanner(r)
	// Nodelist lines can be long (many flags).
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	zone, net, node := 0, 0, 0
	now := time.Now().UTC()

	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r\n")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ";") {
			if doc.HeaderDay == 0 {
				if day, date := parseHeaderDay(line); day > 0 {
					doc.HeaderDay = day
					doc.HeaderDate = date
				}
			}
			continue
		}

		ent, err := parseLine(line)
		if err != nil {
			doc.Skipped++
			continue
		}
		doc.LineCount++

		role := "Node"
		switch strings.ToLower(ent.Keyword) {
		case "zone":
			zone = ent.Number
			net = 0
			node = 0
			role = "Zone"
		case "region":
			net = ent.Number
			node = 0
			role = "Region"
		case "host":
			net = ent.Number
			node = 0
			role = "Host"
		case "hub":
			node = ent.Number
			role = "Hub"
		case "pvt":
			node = ent.Number
			role = "Pvt"
		case "hold":
			node = ent.Number
			role = "Hold"
		case "down":
			node = ent.Number
			role = "Down"
		case "":
			node = ent.Number
			role = "Node"
		default:
			// Unknown keyword with a number: treat as node-like under current zone/net.
			node = ent.Number
			if ent.Keyword != "" {
				role = ent.Keyword
			}
		}

		// Zone coordinator address is zone:zone/0 by convention in some tools;
		// we store zone:0/0 for Zone lines and zone:region/0 for Region, zone:net/0 for Host.
		addrZone, addrNet, addrNode := zone, net, node
		switch role {
		case "Zone":
			addrNet, addrNode = 0, 0
		case "Region", "Host":
			addrNode = 0
		}

		nodeno := formatAddress(addrZone, addrNet, addrNode)
		doc.Nodes = append(doc.Nodes, store.Node{
			Domain:   domain,
			NodeNo:   nodeno,
			Zone:     addrZone,
			Net:      addrNet,
			Node:     addrNode,
			Role:     role,
			BBSName:  humanize(ent.Name),
			Location: humanize(ent.Location),
			Sysop:    humanize(ent.Sysop),
			Phone:    ent.Phone,
			MaxBaud:  ent.MaxBaud,
			Flags:    ent.Flags,
			NodeDay:  doc.HeaderDay,
			Updated:  now,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read nodelist: %w", err)
	}
	return doc, nil
}

func parseLine(line string) (Entry, error) {
	// FTS fields are comma-separated; underscores represent spaces in text fields.
	// Flags field and beyond may contain commas — join remainder.
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return Entry{}, fmt.Errorf("too few fields")
	}

	e := Entry{Raw: line}
	e.Keyword = strings.TrimSpace(parts[0])
	num, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return Entry{}, fmt.Errorf("bad node number %q", parts[1])
	}
	e.Number = num

	field := func(i int) string {
		if i >= len(parts) {
			return ""
		}
		return strings.TrimSpace(parts[i])
	}
	e.Name = field(2)
	e.Location = field(3)
	e.Sysop = field(4)
	e.Phone = field(5)
	e.MaxBaud = field(6)
	if len(parts) > 7 {
		e.Flags = strings.Join(parts[7:], ",")
	}
	return e, nil
}

func parseHeaderDay(line string) (day int, date string) {
	// Example:
	// ;A Fidonet Nodelist for Thursday, July 16, 2026 -- Day number 197 : 25761
	lower := strings.ToLower(line)
	const marker = "day number"
	idx := strings.Index(lower, marker)
	if idx < 0 {
		return 0, ""
	}
	rest := strings.TrimSpace(line[idx+len(marker):])
	// rest starts with "197 : 25761" or "197"
	var n int
	for i, r := range rest {
		if unicode.IsDigit(r) {
			n = n*10 + int(r-'0')
			continue
		}
		if i == 0 {
			return 0, ""
		}
		break
	}
	if n <= 0 || n > 366 {
		return 0, ""
	}
	// Optional date between "for " and " --"
	if i := strings.Index(lower, " for "); i >= 0 {
		from := i + len(" for ")
		to := strings.Index(lower[from:], " --")
		if to > 0 {
			date = strings.TrimSpace(line[from : from+to])
		}
	}
	return n, date
}

func formatAddress(zone, net, node int) string {
	return fmt.Sprintf("%d:%d/%d", zone, net, node)
}

// humanize converts nodelist underscore-spacing to normal spaces.
func humanize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	return strings.TrimSpace(s)
}
