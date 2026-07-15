package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/virtbbs/virtbbs/internal/social"
)

func (s *Server) handlePolls(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	store, err := s.socialStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	polls, _ := store.ListOpenPolls()
	data := struct {
		pageData
		Polls []*social.Poll
	}{
		pageData: s.page(r),
		Polls:    polls,
	}
	s.render(w, "polls.html", data)
}

func (s *Server) handlePollView(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	store, err := s.socialStore()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pollID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if pollID == 0 {
		http.Redirect(w, r, "/polls", http.StatusSeeOther)
		return
	}
	poll, err := store.GetPoll(pollID)
	if err != nil {
		http.Error(w, "poll not found", http.StatusNotFound)
		return
	}
	data := struct {
		pageData
		Poll       *social.Poll
		Options    []*social.PollOption
		UserVoteID int64
	}{
		pageData: s.page(r),
		Poll:     poll,
	}
	if r.Method == http.MethodPost && poll.Open {
		_ = r.ParseForm()
		optID, _ := strconv.ParseInt(r.FormValue("option_id"), 10, 64)
		if optID > 0 {
			if err := store.Vote(pollID, u.ID, optID); err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = tr(data.Locale, "polls.flash.voted")
				poll, _ = store.GetPoll(pollID)
				data.Poll = poll
			}
		}
	}
	data.Options, _ = store.ListPollOptions(pollID)
	data.UserVoteID, _ = store.UserVoteOption(pollID, u.ID)
	s.render(w, "poll_view.html", data)
}

func (s *Server) handleAdminPolls(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireSysop(w, r)
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
		Polls []*social.Poll
	}{
		pageData: s.page(r),
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		switch r.FormValue("action") {
		case "create":
			question := strings.TrimSpace(r.FormValue("question"))
			opts := strings.Split(r.FormValue("options"), "\n")
			if _, err := store.CreatePoll(question, opts); err != nil {
				data.Error = err.Error()
			} else {
				data.Flash = tr(data.Locale, "admin_polls.flash.created")
			}
		case "close":
			id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
			if id > 0 {
				_ = store.ClosePoll(id)
				data.Flash = tr(data.Locale, "admin_polls.flash.closed")
			}
		}
	}
	data.Polls, _ = store.ListAllPolls()
	s.render(w, "admin_polls.html", data)
}
