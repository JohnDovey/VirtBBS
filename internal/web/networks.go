package web

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/fido"
)

// NetworkNavItem is one enabled Fido network in the top nav dropdown.
type NetworkNavItem struct {
	Name  string
	IsHub bool
}

// NetworkAboutView is rendered on /networks/about.
type NetworkAboutView struct {
	Name           string
	Address        string
	Role           string
	Uplink         string
	BinkpHost      string
	BinkpPort      int
	NodelistSource string
	EchoAreas      int
	Members        int
	NodeCount      int
	IsHub          bool
	Enabled        bool
}

// NetworkDiagramView is one selectable map image on /networks/map.
type NetworkDiagramView struct {
	Key      string
	TitleKey string
	NetNum   int
}

func networkNavItems() []NetworkNavItem {
	cfg := config.Get()
	var out []NetworkNavItem
	for _, nd := range cfg.Fido.AllNetworks() {
		if !nd.Enabled || nd.Name == "" {
			continue
		}
		out = append(out, NetworkNavItem{Name: nd.Name, IsHub: nd.IsHub()})
	}
	return out
}

func selectedNetworkFromQuery(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("network"))
}

func (s *Server) requireNetwork(w http.ResponseWriter, r *http.Request) (*fido.NetworkDef, bool) {
	name := selectedNetworkFromQuery(r)
	if name == "" {
		http.Error(w, "network required", http.StatusBadRequest)
		return nil, false
	}
	nd, err := networkDefByName(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return nil, false
	}
	if !nd.Enabled {
		http.Error(w, "network disabled", http.StatusNotFound)
		return nil, false
	}
	return nd, true
}

func buildNetworkAbout(nd *fido.NetworkDef, db *fido.MembersDB, ndb *fido.NodelistDB) NetworkAboutView {
	view := NetworkAboutView{
		Name:      nd.Name,
		Address:   nd.Address,
		BinkpHost: nd.BinkpHost,
		BinkpPort: nd.Port(),
		EchoAreas: len(nd.Areas),
		IsHub:     nd.IsHub(),
		Enabled:   nd.Enabled,
	}
	if view.IsHub {
		view.Role = "hub"
		view.NodelistSource = "local"
		if members, err := db.ListMembers(nd.Name); err == nil {
			view.Members = len(members)
		}
	} else {
		view.Role = "downlink"
		view.Uplink = nd.Uplink
		if nd.NodelistFetchEnabled() {
			view.NodelistSource = nd.EffectiveNodelistURL()
		} else {
			view.NodelistSource = "manual"
		}
	}
	if n, err := ndb.Count(nd.Name); err == nil {
		view.NodeCount = n
	}
	return view
}

func (s *Server) handleNetworkAbout(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	nd, ok := s.requireNetwork(w, r)
	if !ok {
		return
	}
	db := s.Deps.Messages.DB()
	about := buildNetworkAbout(nd, fido.OpenMembersDB(db), fido.OpenNodelistDB(db))
	data := struct {
		pageData
		Network NetworkAboutView
	}{
		pageData: s.page(r),
		Network:  about,
	}
	s.render(w, "network_about.html", data)
}

func (s *Server) handleNetworkMap(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	nd, ok := s.requireNetwork(w, r)
	if !ok {
		return
	}
	var diagrams []NetworkDiagramView
	var mapError string
	locale := localeFromRequest(r)
	if nd.IsHub() {
		cfg := config.Get()
		members, err := fido.OpenMembersDB(s.Deps.Messages.DB()).ListMembers(nd.Name)
		if err != nil {
			mapError = err.Error()
		} else if len(members) == 0 {
			mapError = tr(locale, "networks.map.no_members")
		} else {
			our := nd.NodeAddr()
			pngs, warnings := fido.GenerateDiagrams(our, cfg.BBS.Name, cfg.Sysop.Name, members)
			if len(pngs) == 0 && len(warnings) > 0 {
				mapError = strings.Join(warnings, "; ")
			}
			diagrams = networkDiagramViews(our.Zone, pngs)
		}
	}
	data := struct {
		pageData
		Network   string
		IsHub     bool
		Diagrams  []NetworkDiagramView
		MapError  string
		QueryNet  string
	}{
		pageData: s.page(r),
		Network:  nd.Name,
		IsHub:    nd.IsHub(),
		Diagrams: diagrams,
		MapError: mapError,
		QueryNet: url.QueryEscape(nd.Name),
	}
	s.render(w, "network_map.html", data)
}

func networkDiagramViews(zone int, pngs map[string][]byte) []NetworkDiagramView {
	var out []NetworkDiagramView
	if _, ok := pngs["VirtNet_Full.png"]; ok {
		out = append(out, NetworkDiagramView{Key: "full", TitleKey: "networks.map.full"})
	}
	if _, ok := pngs["VirtNet_Hubs.png"]; ok {
		out = append(out, NetworkDiagramView{Key: "hubs", TitleKey: "networks.map.hubs"})
	}
	var nets []int
	for name := range pngs {
		if !strings.HasPrefix(name, "Hub_") || !strings.HasSuffix(name, ".png") {
			continue
		}
		base := strings.TrimSuffix(strings.TrimPrefix(name, "Hub_"), ".png")
		parts := strings.SplitN(base, "-", 2)
		if len(parts) != 2 {
			continue
		}
		net, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		nets = append(nets, net)
	}
	sort.Ints(nets)
	for _, net := range nets {
		name := fmt.Sprintf("Hub_%d-%d.png", zone, net)
		if _, ok := pngs[name]; ok {
			out = append(out, NetworkDiagramView{
				Key:      fmt.Sprintf("hub-%d", net),
				TitleKey: "networks.map.per_net",
				NetNum:   net,
			})
		}
	}
	return out
}

func (s *Server) handleNetworkDiagram(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	nd, ok := s.requireNetwork(w, r)
	if !ok {
		return
	}
	if !nd.IsHub() {
		http.NotFound(w, r)
		return
	}
	diagram := strings.TrimSpace(r.URL.Query().Get("diagram"))
	if diagram == "" {
		http.Error(w, "diagram required", http.StatusBadRequest)
		return
	}
	cfg := config.Get()
	members, err := fido.OpenMembersDB(s.Deps.Messages.DB()).ListMembers(nd.Name)
	if err != nil || len(members) == 0 {
		http.NotFound(w, r)
		return
	}
	pngs, _ := fido.GenerateDiagrams(nd.NodeAddr(), cfg.BBS.Name, cfg.Sysop.Name, members)
	filename := diagramFilename(nd.NodeAddr().Zone, diagram, pngs)
	if filename == "" {
		http.NotFound(w, r)
		return
	}
	data, ok := pngs[filename]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(data)
}

func diagramFilename(zone int, key string, pngs map[string][]byte) string {
	switch key {
	case "full":
		return "VirtNet_Full.png"
	case "hubs":
		return "VirtNet_Hubs.png"
	case "hub":
		return ""
	}
	if strings.HasPrefix(key, "hub-") {
		netStr := strings.TrimPrefix(key, "hub-")
		if net, err := strconv.Atoi(netStr); err == nil {
			name := fmt.Sprintf("Hub_%d-%d.png", zone, net)
			if _, ok := pngs[name]; ok {
				return name
			}
		}
	}
	return ""
}
