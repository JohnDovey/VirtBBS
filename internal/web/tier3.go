package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/fido"
	"github.com/virtbbs/virtbbs/internal/fidofiles"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/node"
	"github.com/virtbbs/virtbbs/internal/users"
)

func (s *Server) requireSysop(w http.ResponseWriter, r *http.Request) (*users.User, bool) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return nil, false
	}
	if !u.Sysop {
		http.Error(w, "sysop access required", http.StatusForbidden)
		return nil, false
	}
	return u, true
}

// ── Forgot / reset password (14) ─────────────────────────────────────────────

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	data := struct {
		pageData
		Username  string
		ResetURL  string
	}{
		pageData: s.page(r),
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		data.Username = strings.TrimSpace(r.FormValue("username"))
		if data.Username == "" {
			data.Error = tr(data.Locale, "forgot.error.username_required")
		} else {
			token, err := s.Deps.Users.CreatePasswordResetToken(data.Username)
			if err != nil {
				data.Error = translateAPIError(data.Locale, err.Error())
			} else {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				data.ResetURL = fmt.Sprintf("%s://%s/reset-password?token=%s", scheme, r.Host, token)
				data.Flash = tr(data.Locale, "forgot.flash.link")
			}
		}
	}
	s.render(w, "forgot_password.html", data)
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	data := struct {
		pageData
		Token string
	}{
		pageData: s.page(r),
		Token:    token,
	}
	if token == "" {
		data.Error = tr(data.Locale, "reset.error.token")
		s.render(w, "reset_password.html", data)
		return
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		pass := r.FormValue("password")
		pass2 := r.FormValue("password_confirm")
		if pass != pass2 || pass == "" {
			data.Error = tr(data.Locale, "reset.error.password_mismatch")
		} else if err := s.Deps.Users.ResetPasswordWithToken(token, pass); err != nil {
			data.Error = translateAPIError(data.Locale, err.Error())
		} else {
			data.Flash = tr(data.Locale, "reset.flash.updated")
			data.Token = ""
		}
	}
	s.render(w, "reset_password.html", data)
}

// ── Address book (13) ───────────────────────────────────────────────────────

func (s *Server) handleAddressBook(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		switch r.FormValue("action") {
		case "add":
			err := s.Deps.Users.AddAddressBookEntry(u.ID,
				r.FormValue("name"), r.FormValue("fido_addr"), r.FormValue("email"), r.FormValue("notes"), r.FormValue("language"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "delete":
			id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
			_ = s.Deps.Users.DeleteAddressBookEntry(u.ID, id)
		case "edit":
			id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
			err := s.Deps.Users.UpdateAddressBookEntry(u.ID, id,
				r.FormValue("name"), r.FormValue("fido_addr"), r.FormValue("email"), r.FormValue("notes"), r.FormValue("language"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		q := r.URL.Query().Get("q")
		http.Redirect(w, r, "/addressbook?q="+q, http.StatusSeeOther)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	entries, _ := s.Deps.Users.ListAddressBook(u.ID, query)
	data := struct {
		pageData
		Entries []*users.AddressBookEntry
		Query   string
	}{
		pageData: s.page(r),
		Entries:  entries,
		Query:    query,
	}
	s.render(w, "addressbook.html", data)
}

// ── Netmail SPA (12) ─────────────────────────────────────────────────────────

func (s *Server) handleNetmailApp(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	pd := s.page(r)
	data := struct {
		pageData
		EditorType      string
		ComposeI18nJSON string
		Networks        []string
	}{
		pageData:        pd,
		EditorType:      u.EditorType,
		ComposeI18nJSON: composeI18nJSON(pd.Locale),
		Networks:        fidoNetworkNamesList(),
	}
	s.render(w, "netmail_app.html", data)
}

func (s *Server) handleAPINetmail(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	num, _ := strconv.Atoi(r.URL.Query().Get("num"))
	if num > 0 {
		m, err := s.Deps.Messages.GetNetmail(u.Name, u.Sysop, num)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = s.Deps.Users.SetLastRead(u.ID, netmailConferenceID, num)
		writeNetmailMessageJSON(w, s.Deps.Messages, s.Deps.Messages.DB(), localeFromRequest(r), u, m, u.Sysop && r.URL.Query().Get("source") == "1")
		return
	}
	msgs, err := s.Deps.Messages.ListNetmail(u.Name, u.Sysop, 0, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []*messages.Message{}
	}
	lastRead := s.netmailLastRead(u.ID)
	_ = json.NewEncoder(w).Encode(netmailListItems(msgs, lastRead))
}

func (s *Server) handleAPINetmailCompose(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := config.Get()
	if !cfg.Fido.Enabled {
		http.Error(w, "FidoNet not enabled", http.StatusBadRequest)
		return
	}

	var (
		toAddr, toName, subject, bodyText, network, replyMsgID string
		crash                                                  bool
		attachFiles                                            []messages.AttachmentInput
	)

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		if err := r.ParseMultipartForm(messages.DefaultAttachmentLimitBytes + (1 << 20)); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		toAddr = r.FormValue("to_addr")
		toName = r.FormValue("to_name")
		subject = r.FormValue("subject")
		bodyText = r.FormValue("body")
		network = r.FormValue("network")
		replyMsgID = r.FormValue("reply_msgid")
		crash = r.FormValue("crash") == "1" || r.FormValue("crash") == "true"
	} else {
		var body struct {
			ToAddr          string `json:"to_addr"`
			ToName          string `json:"to_name"`
			Subject         string `json:"subject"`
			Body            string `json:"body"`
			Network         string `json:"network"`
			Crash           bool   `json:"crash"`
			ReplyMsgID      string `json:"reply_msgid"`
			AttachmentName  string `json:"attachment_name"`
			AttachmentData  string `json:"attachment_data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		toAddr = body.ToAddr
		toName = body.ToName
		subject = body.Subject
		bodyText = body.Body
		network = body.Network
		crash = body.Crash
		replyMsgID = body.ReplyMsgID
		if body.AttachmentName != "" && body.AttachmentData != "" {
			att, err := fido.ParseNetmailComposeAttachment(body.AttachmentName, body.AttachmentData, 0)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			attachFiles = []messages.AttachmentInput{att}
		}
	}

	netName := strings.TrimSpace(network)
	if netName == "" {
		netName = cfg.Fido.EffectivePrimaryName()
	}
	nd := cfg.Fido.NetworkByName(netName)
	if nd == nil {
		http.Error(w, "network not found", http.StatusBadRequest)
		return
	}
	limit := fido.NetmailAttachmentLimit(&cfg.Fido, nd)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		files, err := parseMultipartAttachment(r, "attachment", limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		attachFiles = files
	} else if len(attachFiles) > 0 {
		if int64(len(attachFiles[0].Data)) > limit {
			http.Error(w, fmt.Sprintf("attachment exceeds size limit (max %d bytes)", limit), http.StatusBadRequest)
			return
		}
	}

	m := fido.NetmailMsg{
		Network:    nd.Name,
		FromName:   u.Name,
		FromAddr:   nd.Address,
		ToName:     toName,
		ToAddr:     toAddr,
		Subject:    subject,
		Body:       bodyText,
		Crash:      crash,
		ReplyMsgID: strings.TrimSpace(replyMsgID),
		AuthorLang: authorLangCode(u, r),
	}
	ndb := fido.OpenNetmailDB(s.Deps.Messages.DB())
	id, err := ndb.Enqueue(&m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(attachFiles) > 0 {
		if err := ndb.SaveNetmailAttachments(s.attachmentsRoot(), id, attachFiles, limit); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "queued": true})
}

// ── SSE stream (17) ──────────────────────────────────────────────────────────

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	lastNew := -1
	lastNet := -1
	lastShout := int64(0)
	for i := 0; i < 120; i++ {
		if r.Context().Err() != nil {
			return
		}
		newTotal := 0
		if counts, err := s.Deps.Users.NewMessageCounts(u.ID); err == nil {
			for _, n := range counts {
				newTotal += n
			}
		}
		netmail := s.netmailUnreadCount(u)
		shoutID := int64(0)
		if store, err := s.socialStore(); err == nil {
			shoutID, _ = store.LatestShoutID()
		}
		if newTotal != lastNew || netmail != lastNet || shoutID != lastShout {
			payload, _ := json.Marshal(map[string]int64{
				"new_messages": int64(newTotal),
				"netmail":      int64(netmail),
				"shouts":       shoutID,
			})
			fmt.Fprintf(w, "event: notify\ndata: %s\n\n", payload)
			flusher.Flush()
			lastNew, lastNet, lastShout = newTotal, netmail, shoutID
		} else {
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}

// ── Sysop admin (4, 5) ───────────────────────────────────────────────────────

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireSysop(w, r)
	if !ok {
		return
	}
	cfg := config.Get()
	data := struct {
		pageData
		FidoEnabled bool
	}{
		pageData: s.page(r),
		FidoEnabled: cfg.Fido.Enabled,
	}
	s.render(w, "admin.html", data)
}

func (s *Server) handleAdminBinkP(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireSysop(w, r)
	if !ok {
		return
	}
	cfg := config.Get()
	locale := localeFromRequest(r)
	var flash, errMsg string

	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		netName := strings.TrimSpace(r.FormValue("network"))
		if netName == "" {
			netName = cfg.Fido.EffectivePrimaryName()
		}
		nd := cfg.Fido.NetworkByName(netName)
		if nd == nil {
			errMsg = tr(locale, "admin_binkp.error.network")
		} else {
			res := fido.PollAndToss(&cfg.Fido, nd, s.Deps.Messages, s.Deps.Conferences, cfg.Sysop.Name, fidofiles.Adapt(s.Deps.Files), cfg.Paths.Files, cfg.AttachmentsDir())
			if res.Poll.Error != nil {
				errMsg = res.Poll.Error.Error()
			} else {
				tossed := 0
				if res.Toss != nil {
					tossed = res.Toss.Imported
				}
				flash = trf(locale, "admin_binkp.flash.poll_ok",
					len(res.Poll.Sent), len(res.Poll.Received), tossed)
			}
		}
	}

	data := s.gatherBinkpStatsPage(r, flash, errMsg)
	data.User = u
	s.render(w, "admin_binkp.html", data)
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireSysop(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
		switch r.FormValue("action") {
		case "delete":
			_ = s.Deps.Users.Delete(id)
		case "password":
			if pass := r.FormValue("password"); pass != "" {
				_ = s.Deps.Users.SetPassword(id, pass)
			}
		}
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}
	list, _ := s.Deps.Users.List()
	data := struct {
		pageData
		Users []*users.User
	}{
		pageData: s.page(r),
		Users:    list,
	}
	s.render(w, "admin_users.html", data)
}

func (s *Server) handleAdminNodes(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireSysop(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		if r.FormValue("action") == "kick" {
			nid, _ := strconv.Atoi(r.FormValue("node_id"))
			_ = node.KickNode(nid)
		}
		http.Redirect(w, r, "/admin/nodes", http.StatusSeeOther)
		return
	}
	nodes, _ := s.Deps.Nodes.List()
	data := struct {
		pageData
		Nodes []*node.NodeInfo
	}{
		pageData: s.page(r),
		Nodes:    nodes,
	}
	s.render(w, "admin_nodes.html", data)
}
