package fido

import (
	"database/sql"
	"strings"
)

// InferNetmailNetwork guesses which configured Fido network an inbound netmail
// belongs to. storedNetwork is used when set at toss time; otherwise origin and
// the nodelist are consulted.
func InferNetmailNetwork(db *sql.DB, storedNetwork, origin, kludges string, networks []NetworkDef) string {
	if n := trimNet(storedNetwork); n != "" {
		return n
	}
	if len(networks) == 0 {
		return ""
	}
	origin = trimNet(origin)
	if origin == "" {
		if _, o := ParseIntlFromKludges(kludges); o != "" {
			origin = o
		}
	}
	if origin == "" {
		return networks[0].Name
	}
	addr, err := ParseAddr(origin)
	if err != nil {
		return networks[0].Name
	}
	if db != nil {
		ndb := OpenNodelistDB(db)
		for _, nd := range networks {
			if e, _ := ndb.LookupAddr(nd.Name, addr); e != nil {
				return nd.Name
			}
		}
	}
	for _, nd := range networks {
		our, err := ParseAddr(nd.Address)
		if err == nil && our.Zone == addr.Zone {
			return nd.Name
		}
	}
	return networks[0].Name
}

func trimNet(s string) string {
	return strings.TrimSpace(s)
}
