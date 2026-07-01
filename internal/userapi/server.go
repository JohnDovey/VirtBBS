// ============================================================================
// VirtBBS — A modern BBS server inspired by PCBoard BBS
//           (Clark Development Company, 1987-1996)
//
// Copyright (c) 2026 John Dovey <dovey.john@gmail.com>
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.
//
// Change History:
//   v0.6.0  2026-06-26  Phase 0 (VirtAnd): initial implementation —
//                        token-authenticated JSON-over-TCP API for the VirtAnd
//                        Android client, exposing security-filtered conference/
//                        file listings, FidoNet nodelist search, and a nodelist-
//                        version-check endpoint. QWK and file content transfer
//                        land in a later phase.
//   v0.7.0  2026-06-26  Phase 1 (VirtAnd/VirtTerm): qwk.download/qwk.upload
//                        (real binary QWK/REP packets, base64-in-JSON) and
//                        files.download/files.upload (base64-in-JSON file
//                        content transfer, security-filtered by directory ReadSec).
//   v0.9.1  2026-06-26  VirtTerm/VirtTermMac: session.whoami so clients can show
//                        the logged-in user's name and the BBS's name (e.g. in a
//                        window title bar) without scraping the terminal stream.
// ============================================================================

// Package userapi provides a password-authenticated JSON-over-TCP API for
// VirtAnd, the Android point client. Callers authenticate with their normal
// BBS username and password (internal/users.Store.Authenticate) on every
// request. Sysop administration uses the web UI (internal/web, /admin/*),
// not this API.
package userapi

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/fido"
	"github.com/virtbbs/virtbbs/internal/files"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/qwk"
	"github.com/virtbbs/virtbbs/internal/users"
)

// maxLineSize raises bufio.Scanner's default 64KB token limit so larger
// payloads (QWK packets, base64-encoded files) added in a later phase can
// fit on a single newline-delimited JSON line.
const maxLineSize = 16 * 1024 * 1024

// Request is a JSON-RPC-style request (method, params, auth).
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	Auth   AuthParams      `json:"auth"`
}

// AuthParams carries the caller's BBS login on every request.
type AuthParams struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Response wraps the result or an error string.
type Response struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// conferenceListItem is a conference visible to the caller plus read stats
// (same Total/Unread/LastRead badge as the web messages picker).
type conferenceListItem struct {
	conferences.Conference
	Total    int `json:"Total"`
	Unread   int `json:"Unread"`
	LastRead int `json:"LastRead"`
}

// Deps bundles store dependencies.
type Deps struct {
	Users       *users.Store
	Messages    *messages.Store
	Conferences *conferences.Store
	Files       *files.Store
}

// Server listens for user-API connections.
type Server struct {
	Addr string
	Deps Deps
}

// ListenAndServe accepts connections until the listener errors or is closed.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("userapi listen %s: %w", s.Addr, err)
	}
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	enc := json.NewEncoder(c)
	enc.SetEscapeHTML(false)

	for sc.Scan() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					_ = enc.Encode(Response{Error: fmt.Sprintf("internal error: %v", r)})
				}
			}()
			var req Request
			if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
				_ = enc.Encode(Response{Error: "invalid JSON"})
				return
			}
			u, err := s.Deps.Users.Authenticate(req.Auth.Username, req.Auth.Password)
			if err != nil {
				_ = enc.Encode(Response{Error: "unauthorized"})
				return
			}
			result, err := s.dispatch(req, u)
			if err != nil {
				_ = enc.Encode(Response{Error: err.Error()})
			} else {
				_ = enc.Encode(Response{Result: result})
			}
		}()
	}
}

func unmarshalParams(params json.RawMessage, dest any) error {
	if len(params) == 0 {
		return nil
	}
	return json.Unmarshal(params, dest)
}

// nonNilSlice ensures empty Go slices encode as JSON [] instead of null.
func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (s *Server) dispatch(req Request, u *users.User) (any, error) {
	switch req.Method {

	// session.whoami lets a client (VirtAnd) show
	// the logged-in user's name and the BBS's name — e.g. in a window
	// title bar — without needing to scrape it out of the terminal byte
	// stream. No params; identity comes entirely from the authenticated user.
	case "session.whoami":
		return map[string]any{
			"name":           u.Name,
			"security_level": u.SecurityLevel,
			"sysop":          u.Sysop,
			"bbs_name":       config.Get().BBS.Name,
		}, nil

	case "conferences.list":
		all, err := s.Deps.Conferences.List()
		if err != nil {
			return nil, err
		}
		unread, _ := s.Deps.Users.NewMessageCounts(u.ID)
		highMap, _ := s.Deps.Messages.HighMsgNumberByConference()
		lastMap := s.Deps.Users.LastReadMap(u.ID)
		var out []conferenceListItem
		for _, c := range all {
			if u.SecurityLevel < c.ReadSec {
				continue
			}
			total := highMap[c.ID]
			lastRead := lastMap[c.ID]
			if lastRead > total {
				_ = s.Deps.Users.SetLastRead(u.ID, c.ID, total)
				lastRead = total
			}
			out = append(out, conferenceListItem{
				Conference: *c,
				Total:      total,
				Unread:     unread[c.ID],
				LastRead:   lastRead,
			})
		}
		if nmTotal, err := s.Deps.Messages.CountNetmail(u.Name, u.Sysop); err == nil {
			nmLast := lastMap[0]
			nmUnread, _ := s.Deps.Messages.CountNetmailUnread(u.Name, u.Sysop, nmLast)
			out = append(out, conferenceListItem{
				Conference: conferences.Conference{
					ID:          VirtAndNetmailConferenceID,
					Name:        "Netmail",
					Description: "FidoNet personal mail",
					ReadSec:     10,
					WriteSec:    10,
				},
				Total:    nmTotal,
				Unread:   nmUnread,
				LastRead: nmLast,
			})
		}
		return nonNilSlice(out), nil

	case "files.dirs.list":
		all, err := s.Deps.Files.ListDirs()
		if err != nil {
			return nil, err
		}
		var out []*files.Dir
		for _, d := range all {
			if u.SecurityLevel >= d.ReadSec {
				out = append(out, d)
			}
		}
		return nonNilSlice(out), nil

	case "files.list":
		var p struct{ DirID int64 }
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		dir, err := s.Deps.Files.GetDir(p.DirID)
		if err != nil {
			return nil, err
		}
		if u.SecurityLevel < dir.ReadSec {
			return nil, fmt.Errorf("access denied")
		}
		files, err := s.Deps.Files.ListFiles(p.DirID)
		if err != nil {
			return nil, err
		}
		return nonNilSlice(files), nil

	case "files.search":
		var p struct{ Query string }
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		matches, err := s.Deps.Files.Search(p.Query)
		if err != nil {
			return nil, err
		}
		// Filter by the ReadSec of each match's containing directory.
		dirSec := map[int64]int{}
		dirs, err := s.Deps.Files.ListDirs()
		if err != nil {
			return nil, err
		}
		for _, d := range dirs {
			dirSec[d.ID] = d.ReadSec
		}
		var out []*files.File
		for _, f := range matches {
			if sec, ok := dirSec[f.DirID]; ok && u.SecurityLevel >= sec {
				out = append(out, f)
			}
		}
		return nonNilSlice(out), nil

	case "files.download":
		var p struct {
			DirID    int64
			Filename string
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		dir, err := s.Deps.Files.GetDir(p.DirID)
		if err != nil {
			return nil, err
		}
		if u.SecurityLevel < dir.ReadSec {
			return nil, fmt.Errorf("access denied")
		}
		data, err := os.ReadFile(s.Deps.Files.AbsPath(p.DirID, p.Filename))
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
		if dirFiles, err := s.Deps.Files.ListFiles(p.DirID); err == nil {
			for _, f := range dirFiles {
				if f.Filename == p.Filename {
					_ = s.Deps.Files.IncrementDownloads(f.ID)
					break
				}
			}
		}
		return map[string]string{
			"filename": p.Filename,
			"data":     base64.StdEncoding.EncodeToString(data),
		}, nil

	case "files.upload":
		var p struct {
			DirID       int64
			Filename    string
			Description string
			Data        string // base64
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		dir, err := s.Deps.Files.GetDir(p.DirID)
		if err != nil {
			return nil, err
		}
		if u.SecurityLevel < dir.UploadSec {
			return nil, fmt.Errorf("access denied")
		}
		raw, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 data: %w", err)
		}
		if err := s.Deps.Files.EnsureDirPath(p.DirID); err != nil {
			return nil, err
		}
		destPath := s.Deps.Files.AbsPath(p.DirID, p.Filename)
		if err := os.WriteFile(destPath, raw, 0644); err != nil {
			return nil, fmt.Errorf("write file: %w", err)
		}
		if err := s.Deps.Files.RegisterUpload(p.DirID, p.Filename, p.Description, u.Name); err != nil {
			return nil, err
		}
		_ = s.Deps.Files.BuildLocalFile(config.Get().BBS.Name)
		return "uploaded", nil

	// ── QWK / REP offline mail (VirtAnd) ────────────────────────────────────

	case "qwk.download":
		var p struct{ ConferenceIDs []int }
		if err := unmarshalParams(req.Params, &p); err != nil {
			return nil, err
		}
		if len(p.ConferenceIDs) == 0 {
			all, err := s.Deps.Conferences.List()
			if err != nil {
				return nil, err
			}
			for _, c := range all {
				if u.SecurityLevel >= c.ReadSec {
					p.ConferenceIDs = append(p.ConferenceIDs, c.ID)
				}
			}
		} else {
			// Drop any conference the caller isn't allowed to read.
			var allowed []int
			for _, cid := range p.ConferenceIDs {
				c, err := s.Deps.Conferences.Get(cid)
				if err != nil {
					continue
				}
				if u.SecurityLevel >= c.ReadSec {
					allowed = append(allowed, cid)
				}
			}
			p.ConferenceIDs = allowed
		}

		cfg := config.Get()
		meta := qwk.PacketMeta{
			BBSName:   cfg.BBS.Name,
			SysopName: cfg.Sysop.Name,
			BBSID:     "VBBS",
		}
		data, err := qwk.BuildPacket(meta, s.Deps.Users, s.Deps.Messages, s.Deps.Conferences, u.ID, p.ConferenceIDs)
		if err != nil {
			return nil, err
		}
		return map[string]string{"data": base64.StdEncoding.EncodeToString(data)}, nil

	case "qwk.upload":
		var p struct{ Data string }
		if err := unmarshalParams(req.Params, &p); err != nil {
			return nil, err
		}
		raw, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 data: %w", err)
		}
		replies, err := qwk.ParseRep(raw)
		if err != nil {
			return nil, err
		}
		// Drop any reply targeting a conference the caller can't write to.
		var allowed []*qwk.ReplyMsg
		for _, r := range replies {
			c, err := s.Deps.Conferences.Get(r.ConferenceID)
			if err != nil {
				continue
			}
			if u.SecurityLevel >= c.WriteSec {
				allowed = append(allowed, r)
			}
		}
		posted, err := qwk.PostReplies(s.Deps.Messages, s.Deps.Conferences, u, allowed)
		if err != nil {
			return nil, err
		}
		return map[string]int{"posted": posted, "rejected": len(replies) - posted}, nil

	// ── FidoNet nodelist ────────────────────────────────────────────────────

	case "fido.nodes.search":
		var p struct {
			Network string `json:"network"`
			Query   string `json:"query"`
			Page    int    `json:"page"`
			Size    int    `json:"size"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		ndb := fido.OpenNodelistDB(s.Deps.Messages.DB())
		return ndb.Search(p.Network, p.Query, p.Page, p.Size)

	case "fido.nodes.get":
		var p struct {
			Network string `json:"network"`
			Addr    string `json:"addr"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		a, err := fido.ParseAddr(p.Addr)
		if err != nil {
			return nil, err
		}
		ndb := fido.OpenNodelistDB(s.Deps.Messages.DB())
		return ndb.LookupAddr(p.Network, a)

	case "fido.nodelist.version":
		var p struct{ Network string }
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, err
		}
		return fido.GetNodelistVersion(s.Deps.Messages.DB(), p.Network)

	case "fido.networks.list":
		cfg := config.Get()
		var names []string
		for _, nd := range cfg.Fido.AllNetworks() {
			if nd.Name != "" {
				names = append(names, nd.Name)
			}
		}
		return nonNilSlice(names), nil

	// ── VirtAnd message sync (replaces QWK for Android) ───────────────────

	case "messages.sync":
		return s.handleMessagesSync(req, u)

	case "messages.post":
		return s.handleMessagesPost(req, u)

	case "messages.mark_read":
		return s.handleMessagesMarkRead(req, u)

	case "messages.delete":
		return s.handleMessagesDelete(req, u)

	case "messages.attachment.download":
		return s.handleMessagesAttachmentDownload(req, u)

	case "app.stats.report":
		return s.handleAppStatsReport(req, u.ID)

	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
}
