package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/virtbbs/virtbbs/internal/node"
	"github.com/virtbbs/virtbbs/internal/social"
)

func (s *Server) socialStore() (*social.Store, error) {
	s.socialOnce.Do(func() {
		s.social, s.socialErr = social.Open(s.Deps.Users.DB())
	})
	return s.social, s.socialErr
}

func (s *Server) handleShoutbox(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	store, err := s.socialStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		pageData
		Shouts []*social.Shout
	}{
		pageData: s.page(r),
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		body := strings.TrimSpace(r.FormValue("body"))
		if body != "" {
			if _, err := store.PostShout(u.ID, u.Name, body); err == nil {
				node.BroadcastAll(u.Name, fmt.Sprintf("[Shoutbox] %s", body))
				data.Flash = tr(data.Locale, "shoutbox.flash.posted")
			} else {
				data.Error = err.Error()
			}
		}
	}
	data.Shouts, _ = store.ListShouts(50)
	s.render(w, "shoutbox.html", data)
}

func (s *Server) handleAPIShoutbox(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	store, err := s.socialStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shouts, err := store.ListShouts(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(shouts)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	store, err := s.socialStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rooms, _ := store.ListRooms()
	var visible []*social.Room
	for _, rm := range rooms {
		if u.SecurityLevel >= rm.MinSecurity {
			visible = append(visible, rm)
		}
	}
	data := struct {
		pageData
		Rooms []*social.Room
	}{
		pageData: s.page(r),
		Rooms:    visible,
	}
	s.render(w, "chat.html", data)
}

func (s *Server) handleChatRoom(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	store, err := s.socialStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	roomID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if roomID == 0 {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}
	room, err := store.GetRoom(roomID)
	if err != nil || u.SecurityLevel < room.MinSecurity {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	data := struct {
		pageData
		Room     *social.Room
		Messages []*social.ChatMessage
	}{
		pageData: s.page(r),
		Room:     room,
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		body := strings.TrimSpace(r.FormValue("body"))
		if body != "" {
			if _, err := store.PostMessage(roomID, u.ID, u.Name, body); err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = tr(data.Locale, "chat.flash.posted")
			}
		}
	}
	data.Messages, _ = store.ListMessages(roomID, 100)
	s.render(w, "chat_room.html", data)
}

func (s *Server) handleAPIChat(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	store, err := s.socialStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	roomID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if roomID == 0 {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	room, err := store.GetRoom(roomID)
	if err != nil || u.SecurityLevel < room.MinSecurity {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	msgs, err := store.ListMessages(roomID, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msgs)
}

func (s *Server) gatherShoutsForMenu() []*social.Shout {
	store, err := s.socialStore()
	if err != nil {
		return nil
	}
	shouts, _ := store.ListShouts(10)
	return shouts
}
