package web

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/version"
)

// handleSniff is an unauthenticated MeshSniff / LAN discover endpoint.
// Returns board identity, services, and Fido network addresses.
func (s *Server) handleSniff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := config.Get()
	host := requestHost(r)
	webURL := fmt.Sprintf("http://%s/", net.JoinHostPort(host, fmt.Sprintf("%d", cfg.Network.WebPort)))
	if cfg.Network.WebPort == 80 {
		webURL = fmt.Sprintf("http://%s/", host)
	}

	type svc struct {
		Name string `json:"name"`
		Port int    `json:"port"`
		URL  string `json:"url,omitempty"`
	}
	type netInfo struct {
		Name      string `json:"name"`
		Address   string `json:"address"`
		BinkpPort int    `json:"binkpPort"`
		Role      string `json:"role"`
		Uplink    string `json:"uplink,omitempty"`
	}

	services := []svc{
		{Name: "VirtBBS Web", Port: cfg.Network.WebPort, URL: webURL},
		{Name: "VirtBBS Telnet", Port: cfg.Network.TelnetPort},
		{Name: "VirtBBS SSH", Port: cfg.Network.SSHPort},
		{Name: "VirtBBS API", Port: cfg.Network.UserAPIPort},
	}
	var networks []netInfo
	for _, n := range cfg.Fido.AllNetworks() {
		if !n.Enabled || n.Address == "" {
			continue
		}
		role := "downlink"
		if n.IsHub() {
			role = "hub"
		}
		port := n.Port()
		label := "VirtBBS BinkP"
		if n.Name != "" {
			label = "VirtBBS BinkP " + n.Name
		}
		services = append(services, svc{Name: label, Port: port})
		networks = append(networks, netInfo{
			Name:      n.Name,
			Address:   n.Address,
			BinkpPort: port,
			Role:      role,
			Uplink:    n.Uplink,
		})
	}

	out := map[string]any{
		"meshId":     "",
		"name":       cfg.BBS.Name,
		"platform":   "virtbbs",
		"appVersion": version.Version,
		"sysop":      cfg.Sysop.Name,
		"software":   "VirtBBS",
		"urls": map[string]string{
			"web": webURL,
		},
		"services": services,
		"networks": networks,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_ = json.NewEncoder(w).Encode(out)
}

func requestHost(r *http.Request) string {
	h := r.Host
	if h == "" {
		return "127.0.0.1"
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	// IPv6 literal without port may be bracketed
	return strings.Trim(h, "[]")
}
