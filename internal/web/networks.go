package web

import (
	"net/http"
	"net/url"
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
	Src      string
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
	cfg := config.Get()
	wwwRoot := cfg.Paths.WWW
	if strings.TrimSpace(wwwRoot) == "" {
		wwwRoot = s.Root
	}

	nodes, err := fido.OpenNodelistDB(s.Deps.Messages.DB()).ListAll(nd.Name)
	if err != nil {
		mapError = err.Error()
	} else if len(nodes) == 0 {
		mapError = tr(locale, "networks.map.no_nodes")
	} else {
		zone := nd.NodeAddr().Zone
		if zone == 0 && len(nodes) > 0 {
			zone = nodes[0].Zone
		}
		cached := fido.ListNetworkDiagramCache(wwwRoot, nd.Name, zone)
		if len(cached) == 0 {
			pngs, warnings := fido.GenerateDiagramsFromNodes(nd.Name, nd, cfg.BBS.Name, cfg.Sysop.Name, nodes)
			if len(pngs) == 0 && len(warnings) > 0 {
				mapError = strings.Join(warnings, "; ")
			} else if err := fido.WriteNetworkDiagramCache(wwwRoot, nd.Name, zone, pngs); err != nil {
				mapError = err.Error()
			} else {
				cached = fido.ListNetworkDiagramCache(wwwRoot, nd.Name, zone)
			}
		}
		diagrams = networkDiagramViewsFromCache(nd.Name, cached)
		if len(diagrams) == 0 && mapError == "" {
			mapError = tr(locale, "networks.map.none")
		}
	}
	data := struct {
		pageData
		Network  string
		Diagrams []NetworkDiagramView
		MapError string
		QueryNet string
	}{
		pageData: s.page(r),
		Network:  nd.Name,
		Diagrams: diagrams,
		MapError: mapError,
		QueryNet: url.QueryEscape(nd.Name),
	}
	s.render(w, "network_map.html", data)
}

func networkDiagramViewsFromCache(network string, cached []fido.DiagramCacheEntry) []NetworkDiagramView {
	var out []NetworkDiagramView
	for _, e := range cached {
		view := NetworkDiagramView{
			Key: e.Key,
			Src: fido.DiagramCacheWebURL(network, e.Key),
		}
		switch e.Key {
		case "full":
			view.TitleKey = "networks.map.full"
		case "hubs":
			view.TitleKey = "networks.map.hubs"
		default:
			if strings.HasPrefix(e.Key, "hub-") && e.NetNum > 0 {
				view.TitleKey = "networks.map.per_net"
				view.NetNum = e.NetNum
			} else {
				continue
			}
		}
		if view.Src == "" {
			continue
		}
		out = append(out, view)
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
	diagram := strings.TrimSpace(r.URL.Query().Get("diagram"))
	if diagram == "" {
		http.Error(w, "diagram required", http.StatusBadRequest)
		return
	}
	src := fido.DiagramCacheWebURL(nd.Name, diagram)
	if src == "" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, src, http.StatusMovedPermanently)
}
