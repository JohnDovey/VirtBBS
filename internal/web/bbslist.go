package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/virtbbs/virtbbs/internal/fido"
)

const bbsListPageSize = 15

type bbsListNodeDetail struct {
	Node  *nodeDetailJSON  `json:"node"`
	Users []fido.BBSListUser `json:"users"`
	Stats struct {
		EchomailCount int `json:"echomail_count"`
		NetmailCount  int `json:"netmail_count"`
	} `json:"stats"`
}

func bbsListPageJSON(locale string) template.JS {
	keys := []struct{ jsKey, locKey string }{
		{"title", "bbslist.title"},
		{"section_echomail", "bbslist.section.echomail"},
		{"section_netmail", "bbslist.section.netmail"},
		{"section_network", "bbslist.section.network"},
		{"col_address", "nodelist.col.address"},
		{"col_name", "common.name"},
		{"col_location", "nodelist.col.location"},
		{"col_sysop", "nodelist.col.sysop"},
		{"col_echomail", "bbslist.col.echomail"},
		{"col_netmail", "bbslist.col.netmail"},
		{"col_last_seen", "bbslist.col.last_seen"},
		{"col_user", "common.user"},
		{"col_user_addr", "bbslist.col.user_addr"},
		{"view", "nodelist.view_btn"},
		{"empty", "bbslist.empty"},
		{"loading", "common.loading"},
		{"page", "common.page"},
		{"of", "common.of"},
		{"previous", "common.previous"},
		{"next", "common.next"},
		{"close", "common.close"},
		{"network", "common.network"},
		{"users_heading", "bbslist.users_heading"},
		{"add_contact", "addressbook.add_contact"},
		{"contact_added", "addressbook.flash.added"},
		{"chart_echomail", "bbslist.chart.echomail"},
		{"chart_netmail", "bbslist.chart.netmail"},
		{"chart_insufficient", "stats.chart_insufficient"},
		{"stats_heading", "bbslist.stats_heading"},
		{"no_nodelist", "bbslist.no_nodelist"},
		{"search_label", "bbslist.search_label"},
		{"search_btn", "bbslist.search_btn"},
		{"search_clear", "bbslist.search_clear"},
		{"search_placeholder", "bbslist.search_placeholder"},
	}
	i18n := make(map[string]string, len(keys))
	for _, pair := range keys {
		i18n[pair.jsKey] = tr(locale, pair.locKey)
	}
	b, err := json.Marshal(map[string]any{
		"i18n":     i18n,
		"pageSize": bbsListPageSize,
	})
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(b)
}

func (s *Server) handleBBSList(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	networks, _ := fido.ListBBSNetworkNames(s.Deps.Messages.DB())
	data := struct {
		pageData
		Networks []string
	}{
		pageData: s.page(r),
		Networks: networks,
	}
	s.render(w, "bbslist.html", data)
}

func (s *Server) handleAPIBBSList(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	db := s.Deps.Messages.DB()
	section := strings.TrimSpace(r.URL.Query().Get("section"))
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	w.Header().Set("Content-Type", "application/json")
	var (
		result any
		err    error
	)
	switch section {
	case "echomail":
		result, err = fido.ListBBSNodesEchomail(db, page, bbsListPageSize, search)
	case "netmail":
		result, err = fido.ListBBSNodesNetmail(db, page, bbsListPageSize, search)
	case "network":
		network := strings.TrimSpace(r.URL.Query().Get("network"))
		result, err = fido.ListBBSNodesByNetwork(db, network, page, bbsListPageSize, search)
	default:
		http.Error(w, "section required (echomail, netmail, network)", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) handleAPIBBSListNode(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	network := strings.TrimSpace(r.URL.Query().Get("network"))
	addr := strings.TrimSpace(r.URL.Query().Get("addr"))
	if network == "" || addr == "" {
		http.Error(w, "network and addr required", http.StatusBadRequest)
		return
	}
	db := s.Deps.Messages.DB()

	_ = s.maybeRebuildHubNodelist(network)
	nodeDetail, err := s.lookupNodeDetail(network, addr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if nodeDetail != nil && nodeDetail.Network != "" {
		network = nodeDetail.Network
	}
	var echoCount, netmailCount int
	_ = db.QueryRow(`SELECT echomail_count, netmail_count FROM fido_bbs_nodes
		WHERE network=? AND node_addr=?`, network, addr).Scan(&echoCount, &netmailCount)
	if nodeDetail == nil {
		nodeDetail = &nodeDetailJSON{
			Network: network,
			Address: addr,
		}
	}
	users, err := fido.ListBBSUsersForNode(db, network, addr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := bbsListNodeDetail{
		Node:  nodeDetail,
		Users: users,
	}
	out.Stats.EchomailCount = echoCount
	out.Stats.NetmailCount = netmailCount

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleAPIBBSListCharts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	charts, err := fido.QueryBBSListCharts(s.Deps.Messages.DB(), 30)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(charts)
}
