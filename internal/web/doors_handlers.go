package web

import (
	"net/http"
	"strconv"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/door"
	"github.com/virtbbs/virtbbs/internal/node"
)

type doorView struct {
	Index       int
	Name        string
	Description string
	MinSecurity int
}

func (s *Server) handleDoors(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	cfg := config.Get()
	var doors []doorView
	for i, d := range cfg.Doors {
		if d.Cmd == "" {
			continue
		}
		if u.SecurityLevel < d.MinSecurity {
			continue
		}
		doors = append(doors, doorView{
			Index:       i,
			Name:        d.Name,
			Description: d.Description,
			MinSecurity: d.MinSecurity,
		})
	}
	data := struct {
		pageData
		Doors []doorView
	}{
		pageData: s.page(r),
		Doors:    doors,
	}
	s.render(w, "doors.html", data)
}

func (s *Server) handleDoorsPlay(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	idx, _ := strconv.Atoi(r.URL.Query().Get("n"))
	cfg := config.Get()
	if idx < 0 || idx >= len(cfg.Doors) {
		http.NotFound(w, r)
		return
	}
	d := cfg.Doors[idx]
	if d.Cmd == "" || u.SecurityLevel < d.MinSecurity {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}
	data := struct {
		pageData
		DoorIndex int
		DoorName  string
	}{
		pageData:  s.page(r),
		DoorIndex: idx,
		DoorName:  d.Name,
	}
	s.render(w, "doors_play.html", data)
}

func (s *Server) handleDoorsWS(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	idx, _ := strconv.Atoi(r.URL.Query().Get("n"))
	cfg := config.Get()
	if idx < 0 || idx >= len(cfg.Doors) {
		http.Error(w, "door not found", http.StatusNotFound)
		return
	}
	dcfg := cfg.Doors[idx]
	if dcfg.Cmd == "" || u.SecurityLevel < dcfg.MinSecurity {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	ws, err := acceptWebSocket(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer ws.Close()

	nodeID, err := s.Deps.Nodes.Register()
	if err != nil {
		return
	}
	defer s.Deps.Nodes.Unregister(nodeID)
	_ = s.Deps.Nodes.Update(nodeID, node.StatusDoor, "Web door: "+dcfg.Name, u.ID, u.Name, u.City)

	sess := door.Session{
		NodeID:        nodeID,
		UserName:      u.Name,
		City:          u.City,
		SecurityLevel: u.SecurityLevel,
		TimesOnline:   u.TimesOnline,
		TimeLeftMins:  config.Get().Session.TimePerCallMins,
		ANSI:          u.ANSI,
		BaudRate:      38400,
		BBSName:       cfg.BBS.Name,
		SysopName:     cfg.Sysop.Name,
		Credits:       u.Credits,
	}
	if err := door.Run(ws, dcfg, sess); err != nil {
		_, _ = ws.Write([]byte("\r\nDoor error: " + err.Error() + "\r\n"))
	}
	_ = s.Deps.Nodes.Update(nodeID, node.StatusWeb, "Web UI", u.ID, u.Name, u.City)
}

func (s *Server) handlePacketBBS(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	pd := s.page(r)
	data := struct {
		pageData
		Message string
	}{
		pageData: pd,
		Message:  tr(pd.Locale, "packetbbs.message"),
	}
	s.render(w, "packetbbs.html", data)
}

func (s *Server) handleAI(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	pd := s.page(r)
	data := struct {
		pageData
		Message string
	}{
		pageData: pd,
		Message:  tr(pd.Locale, "ai.message"),
	}
	s.render(w, "ai.html", data)
}
