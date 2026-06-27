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
//   v0.14.0 2026-06-27  VirtNet: ROUTES.BBS-style static routing table —
//                        wildcard address patterns mapped to a next-hop,
//                        with default net->Host (/0) routes auto-seeded,
//                        wired into RouteAddr/OutboundDir for real
//                        outbound netmail routing.
// ============================================================================

// Package fido — routes.go
//
// A ROUTES.BBS-style static routing table — the BinkleyTerm/FrontDoor
// convention for indirect mail routing: wildcard address patterns (most
// specific wins) mapped to a "route via this address instead" next-hop.
// Distinct from routingtable.go's per-member host:port/password table,
// which is connection info for direct members, not relay routing.
package fido

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Route is one routing-table entry.
type Route struct {
	ID        int64  `json:"ID"`
	Network   string `json:"Network"`
	Pattern   string `json:"Pattern"`
	RouteTo   string `json:"RouteTo"`
	IsDefault bool   `json:"IsDefault"`
	CreatedAt string `json:"CreatedAt"`
}

// SeedDefaultHubRoute ensures a default route exists for net's Host:
// "zone:net/*" -> "zone:net/0". Never clobbers an existing route for the
// same pattern (INSERT OR IGNORE) — a sysop/import-added override always
// wins. Call whenever a member is marked is_host (ApproveJoinRequest,
// ApplyNodeAnnounceInfo) — this is "the default routing of hubs."
func SeedDefaultHubRoute(db *sql.DB, network string, zone, net int) error {
	pattern := fmt.Sprintf("%d:%d/*", zone, net)
	routeTo := fmt.Sprintf("%d:%d/0", zone, net)
	_, err := db.Exec(`INSERT OR IGNORE INTO fido_routes (network, pattern, route_to, is_default) VALUES (?,?,?,1)`,
		network, pattern, routeTo)
	return err
}

// AddRoute adds or updates an explicit (non-default) route.
func AddRoute(db *sql.DB, network, pattern, routeTo string) error {
	_, err := db.Exec(`INSERT INTO fido_routes (network, pattern, route_to, is_default)
		VALUES (?,?,?,0)
		ON CONFLICT(network, pattern) DO UPDATE SET route_to=excluded.route_to, is_default=0`,
		network, pattern, routeTo)
	return err
}

// RemoveRoute deletes a route by its exact pattern.
func RemoveRoute(db *sql.DB, network, pattern string) error {
	_, err := db.Exec(`DELETE FROM fido_routes WHERE network=? AND pattern=?`, network, pattern)
	return err
}

// ListRoutes returns every route for network, most specific first.
func ListRoutes(db *sql.DB, network string) ([]*Route, error) {
	rows, err := db.Query(`SELECT id, network, pattern, route_to, is_default, created_at
		FROM fido_routes WHERE network=?`, network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Route
	for rows.Next() {
		r := &Route{}
		var isDefault int
		if err := rows.Scan(&r.ID, &r.Network, &r.Pattern, &r.RouteTo, &isDefault, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.IsDefault = isDefault != 0
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return patternSpecificity(out[i].Pattern) > patternSpecificity(out[j].Pattern) })
	return out, rows.Err()
}

// patternSpecificity scores a pattern for MatchRoute's "most specific
// wins" rule: exact node > "zone:net/*" > "zone:*" > "*".
func patternSpecificity(pattern string) int {
	switch {
	case pattern == "*":
		return 0
	case strings.HasSuffix(pattern, ":*"):
		return 1
	case strings.HasSuffix(pattern, "/*"):
		return 2
	default:
		return 3 // exact zone:net/node
	}
}

// patternMatches reports whether pattern matches dest. Supported forms:
// "*" (catch-all), "zone:*", "zone:net/*", "zone:net/node" (exact).
func patternMatches(pattern string, dest Addr) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ":*") {
		zoneStr := strings.TrimSuffix(pattern, ":*")
		zone, err := strconv.Atoi(zoneStr)
		return err == nil && zone == dest.Zone
	}
	if strings.HasSuffix(pattern, "/*") {
		zoneNet := strings.TrimSuffix(pattern, "/*")
		a, err := ParseAddr(zoneNet + "/0")
		return err == nil && a.Zone == dest.Zone && a.Net == dest.Net
	}
	a, err := ParseAddr(pattern)
	return err == nil && a.Zone == dest.Zone && a.Net == dest.Net && a.Node == dest.Node
}

// MatchRoute returns the most specific route in routes matching dest, or
// nil if none match. routes should already be specificity-sorted (as
// ListRoutes returns them); this re-checks regardless so callers can pass
// an unsorted slice safely too.
func MatchRoute(routes []*Route, dest Addr) *Route {
	var best *Route
	bestScore := -1
	for _, r := range routes {
		if !patternMatches(r.Pattern, dest) {
			continue
		}
		score := patternSpecificity(r.Pattern)
		if score > bestScore {
			best, bestScore = r, score
		}
	}
	return best
}

// RouteFor loads network's routes and returns the resolved next-hop
// address for dest, or ok=false if no route matches.
func RouteFor(db *sql.DB, network string, dest Addr) (Addr, bool, error) {
	routes, err := ListRoutes(db, network)
	if err != nil {
		return Addr{}, false, err
	}
	match := MatchRoute(routes, dest)
	if match == nil {
		return Addr{}, false, nil
	}
	a, err := ParseAddr(match.RouteTo)
	if err != nil {
		return Addr{}, false, fmt.Errorf("route %q -> invalid address %q: %w", match.Pattern, match.RouteTo, err)
	}
	return a, true, nil
}

// ─── ROUTES.BBS text format ─────────────────────────────────────────────────

// ExportRoutesBBS writes network's routing table in the classic
// BinkleyTerm/FrontDoor two-column ROUTES.BBS text format, most specific
// pattern first.
func ExportRoutesBBS(db *sql.DB, network string) ([]byte, error) {
	routes, err := ListRoutes(db, network)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "; ROUTES.BBS for %q, generated %s\r\n", network, time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "; %-20s %s\r\n", "Pattern", "Route-to")
	for _, r := range routes {
		fmt.Fprintf(&b, "%-20s %s\r\n", r.Pattern, r.RouteTo)
	}
	return []byte(b.String()), nil
}

// RoutesImportResult summarises a ROUTES.BBS import.
type RoutesImportResult struct {
	Added  int
	Errors []string
}

// ImportRoutesBBS parses data (the format ExportRoutesBBS writes) and adds
// each line as an explicit route (AddRoute, is_default=0) — an imported
// line always overrides an auto-seeded default for the same pattern.
func ImportRoutesBBS(db *sql.DB, network string, data []byte) (*RoutesImportResult, error) {
	result := &RoutesImportResult{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			result.Errors = append(result.Errors, fmt.Sprintf("malformed line: %q", line))
			continue
		}
		if err := AddRoute(db, network, fields[0], fields[1]); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", fields[0], err))
			continue
		}
		result.Added++
	}
	return result, sc.Err()
}
