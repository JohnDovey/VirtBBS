package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/virtbbs/virtbbs/internal/mrc"
)

// handleMRC renders the browser Multi-Relay Chat page.
func (s *Server) handleMRC(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	hub := s.Deps.MRC
	data := struct {
		pageData
		Enabled     bool
		Connected   bool
		MinSecurity int
		Handle      string
		DefaultRoom string
		CanJoin     bool
		StatusMsg   string
	}{
		pageData: s.page(r),
	}
	if hub == nil || !hub.Enabled() {
		data.StatusMsg = tr(data.Locale, "mrc.disabled")
		s.render(w, "mrc.html", data)
		return
	}
	data.Enabled = true
	data.Connected = hub.Connected()
	data.MinSecurity = hub.MinSecurity()
	data.DefaultRoom = hub.DefaultRoom()
	data.Handle = mrc.SanitizeName(u.Name)
	if hub.Prefs() != nil {
		if p, err := hub.Prefs().Get(u.ID, u.Name); err == nil && p != nil && p.Handle != "" {
			data.Handle = p.Handle
		}
	}
	if u.SecurityLevel < data.MinSecurity {
		data.StatusMsg = tr(data.Locale, "mrc.security")
		s.render(w, "mrc.html", data)
		return
	}
	if !data.Connected {
		data.StatusMsg = tr(data.Locale, "mrc.offline")
		s.render(w, "mrc.html", data)
		return
	}
	data.CanJoin = true
	s.render(w, "mrc.html", data)
}

type mrcClientMsg struct {
	Type   string `json:"type"`
	Handle string `json:"handle,omitempty"`
	Room   string `json:"room,omitempty"`
	Text   string `json:"text,omitempty"`
}

type mrcServerMsg struct {
	Type      string   `json:"type"`
	Kind      string   `json:"kind,omitempty"`
	From      string   `json:"from,omitempty"`
	Site      string   `json:"site,omitempty"`
	Room      string   `json:"room,omitempty"`
	Body      string   `json:"body,omitempty"`
	HTML      string   `json:"html,omitempty"`
	Topic     string   `json:"topic,omitempty"`
	Handle    string   `json:"handle,omitempty"`
	Message   string   `json:"message,omitempty"`
	Names     []string `json:"names,omitempty"`
	Connected bool     `json:"connected,omitempty"`
	TS        string   `json:"ts,omitempty"`
}

// handleMRCWS is a live WebSocket session attached to the process MRC hub.
func (s *Server) handleMRCWS(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	hub := s.Deps.MRC
	if hub == nil || !hub.Enabled() {
		http.Error(w, "MRC disabled", http.StatusServiceUnavailable)
		return
	}
	if u.SecurityLevel < hub.MinSecurity() {
		http.Error(w, "insufficient security", http.StatusForbidden)
		return
	}
	if !hub.Connected() {
		http.Error(w, "MRC offline", http.StatusServiceUnavailable)
		return
	}

	ws, err := acceptWebSocket(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer ws.Close()

	handle := mrc.SanitizeName(u.Name)
	if hub.Prefs() != nil {
		if p, err := hub.Prefs().Get(u.ID, u.Name); err == nil && p != nil && p.Handle != "" {
			handle = p.Handle
		}
	}
	room := hub.DefaultRoom()

	_ = ws.conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	if first, err := readWSLine(ws); err == nil {
		var msg mrcClientMsg
		if json.Unmarshal(first, &msg) == nil && msg.Type == "join" {
			if h := mrc.SanitizeName(msg.Handle); h != "" {
				handle = h
				if hub.Prefs() != nil {
					p, _ := hub.Prefs().Get(u.ID, u.Name)
					if p == nil {
						p = &mrc.UserPrefs{UserID: u.ID, Handle: handle, HandleColor: 11, TextColor: 7, Theme: 1}
					}
					p.Handle = handle
					_ = hub.Prefs().Save(p)
				}
			}
			if rr := mrc.SanitizeName(msg.Room); rr != "" {
				room = rr
			}
		}
	}
	_ = ws.conn.SetReadDeadline(time.Time{})

	att, err := hub.Attach(u.ID, handle, room)
	if err != nil {
		_ = writeWSJSON(ws, mrcServerMsg{Type: "error", Message: err.Error()})
		return
	}
	defer hub.Detach(att)

	_ = writeWSJSON(ws, mrcServerMsg{
		Type:      "status",
		Connected: true,
		Handle:    handle,
		Room:      att.CurrentRoom(),
		Topic:     att.Topic(),
		TS:        time.Now().Format("15:04"),
	})
	_ = writeWSJSON(ws, mrcServerMsg{
		Type: "event",
		Kind: "notice",
		Body: "Connected to MRC — type /help for commands.",
		HTML: mrc.PipeToHTML("|15Connected to MRC|07 — type |11/help|07 for commands."),
		TS:   time.Now().Format("15:04"),
	})

	fanoutDone := make(chan struct{})
	go func() {
		defer close(fanoutDone)
		for ev := range att.Inbox {
			msg := eventToServerMsg(ev)
			if err := writeWSJSON(ws, msg); err != nil {
				_ = ws.Close()
				return
			}
			if ev.Kind == mrc.EventTopic {
				_ = writeWSJSON(ws, mrcServerMsg{
					Type:   "status",
					Handle: att.Handle,
					Room:   att.CurrentRoom(),
					Topic:  att.Topic(),
					TS:     time.Now().Format("15:04"),
				})
			}
			if ev.Kind == mrc.EventUserList {
				_ = writeWSJSON(ws, mrcServerMsg{Type: "nicks", Names: att.Nicks()})
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := ws.Read(buf)
		if n > 0 {
			payload := trimSpaceBytes(buf[:n])
			if processClientJSON(hub, att, payload, ws) {
				return
			}
		}
		if err != nil {
			select {
			case <-fanoutDone:
			default:
			}
			return
		}
	}
}

func processClientJSON(hub *mrc.Hub, att *mrc.Attachment, raw []byte, ws *wsConn) (quit bool) {
	raw = trimSpaceBytes(raw)
	if len(raw) == 0 {
		return false
	}
	var msg mrcClientMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		msg = mrcClientMsg{Type: "chat", Text: string(raw)}
	}
	switch strings.ToLower(msg.Type) {
	case "quit":
		return true
	case "chat", "cmd", "message":
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			return false
		}
		if strings.HasPrefix(text, "/") {
			return handleWebSlash(hub, att, text, ws)
		}
		if err := hub.SendChat(att, text); err != nil {
			_ = writeWSJSON(ws, mrcServerMsg{Type: "error", Message: err.Error()})
			return false
		}
		_ = writeWSJSON(ws, mrcServerMsg{
			Type: "event",
			Kind: "chat",
			From: att.Handle,
			Room: att.CurrentRoom(),
			Body: mrc.StripTildesForBody(text),
			HTML: mrc.PipeToHTML(mrc.StripTildesForBody(text)),
			TS:   time.Now().Format("15:04"),
		})
	case "join":
		room := mrc.SanitizeName(msg.Room)
		if room == "" {
			return false
		}
		if err := hub.JoinRoom(att, room); err != nil {
			_ = writeWSJSON(ws, mrcServerMsg{Type: "error", Message: err.Error()})
			return false
		}
		_ = writeWSJSON(ws, mrcServerMsg{
			Type:   "status",
			Handle: att.Handle,
			Room:   att.CurrentRoom(),
			Topic:  att.Topic(),
			TS:     time.Now().Format("15:04"),
		})
		_ = hub.SendServerCmd(att, "USERLIST")
	}
	return false
}

func handleWebSlash(hub *mrc.Hub, att *mrc.Attachment, text string, ws *wsConn) bool {
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}
	notice := func(s string) {
		_ = writeWSJSON(ws, mrcServerMsg{
			Type: "event", Kind: "notice", Body: mrc.StripPipe(s),
			HTML: mrc.PipeToHTML(s), TS: time.Now().Format("15:04"),
		})
	}
	switch cmd {
	case "quit", "q", "exit":
		notice("|12Leaving MRC…")
		_ = writeWSJSON(ws, mrcServerMsg{Type: "quit"})
		return true
	case "help", "?":
		notice("|15/join /list /chatters /whoon /msg /me /topic /motd /bbses /info /quit")
	case "join", "j":
		if arg == "" {
			notice("|12Usage: /join <room>")
			return false
		}
		if err := hub.JoinRoom(att, arg); err != nil {
			notice("|12" + err.Error())
			return false
		}
		notice("|10Joined " + att.CurrentRoom())
		_ = writeWSJSON(ws, mrcServerMsg{
			Type: "status", Handle: att.Handle, Room: att.CurrentRoom(), Topic: att.Topic(),
			TS: time.Now().Format("15:04"),
		})
		_ = hub.SendServerCmd(att, "USERLIST")
	case "list", "rooms":
		_ = hub.SendServerCmd(att, "LIST")
	case "chatters", "who":
		_ = hub.SendServerCmd(att, "CHATTERS")
		_ = hub.SendServerCmd(att, "USERLIST")
	case "whoon":
		_ = hub.SendServerCmd(att, "WHOON")
	case "motd":
		_ = hub.SendServerCmd(att, "MOTD")
	case "bbses":
		_ = hub.SendServerCmd(att, "BBSES")
	case "info":
		if arg == "" {
			_ = hub.SendServerCmd(att, "INFO")
		} else {
			_ = hub.SendServerCmd(att, "INFO "+arg)
		}
	case "topic":
		if arg == "" {
			notice("|14Topic: " + att.Topic())
			return false
		}
		_ = hub.SetTopic(att, arg)
	case "msg", "t", "tell", "pm":
		sp := strings.SplitN(arg, " ", 2)
		if len(sp) < 2 {
			notice("|12Usage: /msg <user> <text>")
			return false
		}
		if err := hub.SendPrivate(att, sp[0], sp[1]); err != nil {
			notice("|12" + err.Error())
			return false
		}
		notice(fmt.Sprintf("|13-> %s|07 %s", sp[0], sp[1]))
	case "me":
		if arg == "" {
			return false
		}
		_ = hub.SendAction(att, arg)
		notice(fmt.Sprintf("|13* %s %s", att.Handle, arg))
	default:
		_ = hub.SendServerCmd(att, strings.TrimPrefix(text, "/"))
	}
	return false
}

func eventToServerMsg(ev mrc.Event) mrcServerMsg {
	kind := "chat"
	switch ev.Kind {
	case mrc.EventPrivate:
		kind = "private"
	case mrc.EventSystem:
		kind = "system"
	case mrc.EventNotice:
		kind = "notice"
	case mrc.EventTopic:
		kind = "topic"
	case mrc.EventUserList:
		kind = "userlist"
	}
	body := ev.PipeRaw
	if body == "" {
		body = ev.Body
	}
	return mrcServerMsg{
		Type: "event",
		Kind: kind,
		From: ev.From,
		Site: ev.Site,
		Room: ev.Room,
		Body: mrc.StripPipe(body),
		HTML: mrc.PipeToHTML(body),
		TS:   time.Now().Format("15:04"),
	}
}

func writeWSJSON(ws *wsConn, msg mrcServerMsg) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = ws.Write(b)
	return err
}

func readWSLine(ws *wsConn) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		_ = ws.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := ws.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
			if len(out) > 0 && out[0] == '{' {
				return trimSpaceBytes(out), nil
			}
		}
		if err != nil {
			if len(out) > 0 {
				return trimSpaceBytes(out), nil
			}
			return nil, err
		}
	}
	if len(out) == 0 {
		return nil, io.EOF
	}
	return trimSpaceBytes(out), nil
}

func trimSpaceBytes(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
