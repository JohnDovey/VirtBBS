package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/virtbbs/virtbbs/internal/messages"
)

type threadMessageJSON struct {
	*messages.Message
	DisplayBody string `json:"DisplayBody,omitempty"`
	LangLabel   string `json:"LangLabel,omitempty"`
}

type threadResponse struct {
	Messages []threadMessageJSON `json:"messages"`
}

func messageThreadMeta(store *messages.Store, m *messages.Message) (replyToNum, replyCount int, threadAvailable bool) {
	if m == nil || store == nil {
		return 0, 0, false
	}
	if m.FidoReply != "" {
		if parent, err := store.GetByFidoMsgID(m.ConferenceID, m.FidoReply); err == nil && parent != nil {
			replyToNum = parent.MsgNumber
			threadAvailable = true
		}
	}
	replyCount, _ = store.CountReplies(m.ConferenceID, m.FidoMsgID)
	if replyCount > 0 {
		threadAvailable = true
	}
	return replyToNum, replyCount, threadAvailable
}

func (s *Server) handleAPINetmailThread(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	num, _ := strconv.Atoi(r.URL.Query().Get("num"))
	if num <= 0 {
		http.Error(w, "num required", http.StatusBadRequest)
		return
	}
	thread, err := s.Deps.Messages.FindNetmailThread(u.Name, u.Sysop, num)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	locale := localeFromRequest(r)
	out := make([]threadMessageJSON, 0, len(thread))
	for _, m := range thread {
		out = append(out, threadMessageJSON{
			Message:     m,
			DisplayBody: FormatMessageBodyHTML(m.Body),
			LangLabel:   messageLangLabel(locale, m),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(threadResponse{Messages: out})
}

func (s *Server) handleAPIMessageThread(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	confID, _ := strconv.Atoi(r.URL.Query().Get("conf"))
	num, _ := strconv.Atoi(r.URL.Query().Get("num"))
	if confID < 0 || num <= 0 {
		http.Error(w, "conf and num required", http.StatusBadRequest)
		return
	}
	c, err := s.Deps.Conferences.Get(confID)
	if err != nil || !canReadConference(u, c) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}
	m, err := s.Deps.Messages.Get(confID, num)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if messages.IsNetmail(m) && !messages.CanViewNetmail(u.Name, u.Sysop, m) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}
	thread, err := s.Deps.Messages.FindThread(confID, num)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	locale := localeFromRequest(r)
	out := make([]threadMessageJSON, 0, len(thread))
	for _, tm := range thread {
		out = append(out, threadMessageJSON{
			Message:     tm,
			DisplayBody: FormatMessageBodyHTML(tm.Body),
			LangLabel:   messageLangLabel(locale, tm),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(threadResponse{Messages: out})
}
