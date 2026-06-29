package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/fido"
)

func (s *Server) handleAdminFidoDebugPoll(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireSysop(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	network := strings.TrimSpace(r.URL.Query().Get("network"))
	if network == "" {
		http.Error(w, "network required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(event string, data any) {
		if r.Context().Err() != nil {
			return
		}
		payload, err := json.Marshal(data)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
		flusher.Flush()
	}

	send("status", "starting")

	cfg := config.Get()
	nd := cfg.Fido.NetworkByName(network)
	if nd == nil {
		send("status", "failed")
		send("done", map[string]any{"ok": false, "error": "network not found"})
		return
	}

	dbg, err := fido.BeginBinkpDebugSession(network)
	if err != nil {
		send("status", "failed")
		send("done", map[string]any{"ok": false, "error": err.Error()})
		return
	}
	defer dbg.Close()

	dbg.SetOnLine(func(line string) {
		send("line", line)
	})

	send("meta", map[string]string{"logPath": dbg.Path(), "network": network})
	send("status", "polling")

	res := fido.PollAndTossDebug(nd, s.Deps.Messages, s.Deps.Conferences, cfg.Sysop.Name, s.Deps.Files, cfg.Paths.Files, dbg)

	tossed := 0
	if res.Toss != nil {
		tossed = res.Toss.Imported
	}
	sent := 0
	received := 0
	if res.Poll != nil {
		sent = len(res.Poll.Sent)
		received = len(res.Poll.Received)
	}

	done := map[string]any{
		"ok":       res.Poll != nil && res.Poll.Error == nil,
		"sent":     sent,
		"received": received,
		"tossed":   tossed,
		"logPath":  dbg.Path(),
	}
	if res.Poll != nil && res.Poll.Error != nil {
		done["error"] = res.Poll.Error.Error()
		send("status", "failed")
	} else {
		send("status", "complete")
	}
	send("done", done)
}
