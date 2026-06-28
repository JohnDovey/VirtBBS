package web

import (
	"fmt"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
)

func nodelistNetworkNames() []string {
	cfg := config.Get()
	var names []string
	for _, nd := range cfg.Fido.AllNetworks() {
		if nd.Enabled && nd.Name != "" {
			names = append(names, nd.Name)
		}
	}
	if len(names) == 0 {
		return []string{"FidoNet"}
	}
	return names
}

func safeNodelistFilename(network string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, network)
	if safe == "" {
		safe = "nodelist"
	}
	return fmt.Sprintf("NODELIST-%s.txt", safe)
}
