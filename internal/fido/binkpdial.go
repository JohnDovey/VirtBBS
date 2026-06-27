// Package fido — binkpdial.go
//
// Resolves a configured uplink (FidoNet address, host:port, or addr@host)
// to a TCP dial target using the imported nodelist and, for VirtNet,
// the fido_members routing table.
package fido

import (
	"database/sql"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ResolveBinkpDialTarget turns nd.Uplink into a hostname/IP and BinkP port.
// When uplink is a FidoNet address (e.g. "4:902/19"), the imported nodelist
// is consulted for IBN/INA flags; VirtNet members fall back to fido_members.
func ResolveBinkpDialTarget(network, uplink string, defaultPort int, db *sql.DB) (host string, port int, err error) {
	uplink = strings.TrimSpace(uplink)
	if uplink == "" {
		return "", 0, fmt.Errorf("empty uplink")
	}

	if idx := strings.Index(uplink, "@"); idx >= 0 {
		hostPart := strings.TrimSpace(uplink[idx+1:])
		h, p := splitDialHostPort(hostPart)
		if h == "" {
			return "", 0, fmt.Errorf("no hostname after @ in uplink %q", uplink)
		}
		if p == 0 {
			p = defaultPort
		}
		return h, p, nil
	}

	addr, parseErr := ParseAddr(uplink)
	if parseErr != nil {
		h, p := splitDialHostPort(uplink)
		if h == "" {
			return "", 0, parseErr
		}
		if p == 0 {
			p = defaultPort
		}
		return h, p, nil
	}

	if db == nil {
		return "", 0, fmt.Errorf("uplink %q is a FidoNet address; nodelist lookup requires a database connection", uplink)
	}

	ndb := OpenNodelistDB(db)
	entry, err := ndb.LookupAddr(network, addr)
	if err != nil {
		return "", 0, fmt.Errorf("nodelist lookup for %s: %w", uplink, err)
	}
	if entry != nil {
		h, p := dialFromNodeFlags(entry.Flags)
		if h != "" {
			if p == 0 {
				p = defaultPort
			}
			return h, p, nil
		}
	}

	mdb := OpenMembersDB(db)
	member, merr := mdb.GetMemberByAddr(network, addr)
	if merr != nil && !isMissingTable(merr) {
		return "", 0, fmt.Errorf("member lookup for %s: %w", uplink, merr)
	}
	if member != nil && strings.TrimSpace(member.BinkpHost) != "" {
		h, p := splitDialHostPort(member.BinkpHost)
		if h == "" && p > 0 {
			return "", 0, fmt.Errorf("member %s has port-only BinkP host %q", uplink, member.BinkpHost)
		}
		if h != "" {
			if p == 0 {
				p = defaultPort
			}
			return h, p, nil
		}
	}

	if entry == nil {
		return "", 0, fmt.Errorf("uplink %s: address not found in nodelist for network %q", uplink, network)
	}
	return "", 0, fmt.Errorf("uplink %s (%s): no IBN/INA host in nodelist entry", uplink, entry.Name)
}

// dialFromNodeFlags extracts a BinkP dial target from a nodelist flags field
// (FTS-0005). Prefers IBN:, then INA:; ITN: supplies a port when IBN omits one.
func dialFromNodeFlags(flags string) (host string, port int) {
	var hasBareIBN bool
	for _, f := range strings.Split(flags, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		upper := strings.ToUpper(f)
		switch {
		case upper == "IBN":
			hasBareIBN = true
		case strings.HasPrefix(upper, "IBN:"):
			val := strings.TrimSpace(f[4:])
			if h, p := splitDialHostPort(val); h != "" {
				host = h
				if p > 0 {
					port = p
				}
			} else if p > 0 && port == 0 {
				port = p
			}
		case strings.HasPrefix(upper, "INA:"):
			if host != "" {
				continue
			}
			val := strings.TrimSpace(f[4:])
			if h, p := splitDialHostPort(val); h != "" {
				host = h
				if p > 0 && port == 0 {
					port = p
				}
			}
		case strings.HasPrefix(upper, "ITN:"):
			val := strings.TrimSpace(f[4:])
			if h, p := splitDialHostPort(val); h != "" {
				if host == "" {
					host = h
				}
				if p > 0 && port == 0 {
					port = p
				}
			} else if p, err := strconv.Atoi(val); err == nil && port == 0 {
				port = p
			}
		}
	}
	if host == "" && hasBareIBN {
		// IBN present without hostname — INA may have been parsed above.
	}
	return host, port
}

// splitDialHostPort parses "host", "host:port", a bare port number, or an IP.
func splitDialHostPort(s string) (host string, port int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0
	}
	if p, err := strconv.Atoi(s); err == nil {
		return "", p
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		port, _ = strconv.Atoi(p)
		return h, port
	}
	return s, 0
}

func isMissingTable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no such table")
}
