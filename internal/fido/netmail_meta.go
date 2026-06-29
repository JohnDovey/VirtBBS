package fido

import (
	"database/sql"
	"strings"
)

// InferNetmailNetwork guesses which configured Fido network an inbound netmail
// belongs to. storedNetwork is used when it matches the origin in the nodelist;
// otherwise origin and configured networks are consulted.
func InferNetmailNetwork(db *sql.DB, storedNetwork, origin, kludges string, networks []NetworkDef) string {
	origin = trimNet(origin)
	if origin == "" {
		if _, o := ParseIntlFromKludges(kludges); o != "" {
			origin = o
		}
	}
	if origin == "" {
		if n := trimNet(storedNetwork); n != "" {
			return n
		}
		if len(networks) > 0 {
			return networks[0].Name
		}
		return ""
	}
	addr, err := ParseAddr(origin)
	if err != nil {
		if n := trimNet(storedNetwork); n != "" {
			return n
		}
		if len(networks) > 0 {
			return networks[0].Name
		}
		return ""
	}
	return InferNetworkForAddr(db, addr, networks, storedNetwork)
}

// InferNetworkForAddr picks the Fido network that owns addr using the nodelist,
// configured networks, and an optional stored hint (e.g. from a tossed message).
func InferNetworkForAddr(db *sql.DB, addr Addr, networks []NetworkDef, storedHint string) string {
	if db == nil {
		if n := trimNet(storedHint); n != "" {
			return n
		}
		if len(networks) > 0 {
			return networks[0].Name
		}
		return ""
	}
	ndb := OpenNodelistDB(db)
	if n := trimNet(storedHint); n != "" {
		if e, _ := ndb.LookupAddr(n, addr); e != nil {
			return n
		}
	}
	for _, nd := range networks {
		if e, _ := ndb.LookupAddr(nd.Name, addr); e != nil {
			return nd.Name
		}
	}
	if e := ndb.LookupAddrAny(addr); e != nil {
		return e.Network
	}
	for _, nd := range networks {
		if our, err := ParseAddr(nd.Address); err == nil && our.Zone == addr.Zone {
			return nd.Name
		}
	}
	if n := trimNet(storedHint); n != "" {
		return n
	}
	if len(networks) > 0 {
		return networks[0].Name
	}
	return ""
}

func trimNet(s string) string {
	return strings.TrimSpace(s)
}
