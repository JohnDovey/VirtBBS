package web

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/fido"
	"github.com/virtbbs/virtbbs/internal/messages"
)

func (s *Server) attachmentsRoot() string {
	cfg := config.Get()
	return messages.AttachmentsRoot(cfg.Paths.DB, cfg.Paths.Attachments)
}

func parseMultipartAttachment(r *http.Request, field string, maxBytes int64) ([]messages.AttachmentInput, error) {
	if maxBytes <= 0 {
		maxBytes = messages.DefaultAttachmentLimitBytes
	}
	file, header, err := r.FormFile(field)
	if err == http.ErrMissingFile {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("attachment exceeds size limit (max %d bytes)", maxBytes)
	}
	name := header.Filename
	if name == "" {
		name = "attachment.dat"
	}
	return []messages.AttachmentInput{{Filename: name, Data: data}}, nil
}

func parseMultipartFormAttachments(r *http.Request, maxBytes int64) ([]messages.AttachmentInput, error) {
	if err := r.ParseMultipartForm(maxBytes + (1 << 20)); err != nil {
		return nil, err
	}
	return parseMultipartAttachment(r, "attachment", maxBytes)
}

func (s *Server) handleMessageAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	confID, _ := strconv.Atoi(r.URL.Query().Get("conf"))
	msgNum, _ := strconv.Atoi(r.URL.Query().Get("num"))
	attachID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if attachID <= 0 {
		http.Error(w, "missing attachment id", http.StatusBadRequest)
		return
	}
	c, err := s.Deps.Conferences.Get(confID)
	if err != nil || !canReadConference(u, c) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}
	msg, err := s.Deps.Messages.Get(confID, msgNum)
	if err != nil {
		http.Error(w, "message not found", http.StatusNotFound)
		return
	}
	if messages.IsNetmail(msg) && !messages.CanViewNetmail(u.Name, u.Sysop, msg) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}
	a, rc, err := s.Deps.Messages.OpenAttachment(s.attachmentsRoot(), attachID, msg.ID)
	if err != nil {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.Filename))
	_, _ = io.Copy(w, rc)
}

func (s *Server) handleAPINetmailAttachmentDownload(w http.ResponseWriter, r *http.Request) {
	u, ok := s.currentUser(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	msgNum, _ := strconv.Atoi(r.URL.Query().Get("num"))
	attachID, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if attachID <= 0 || msgNum <= 0 {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}
	msg, err := s.Deps.Messages.GetNetmail(u.Name, u.Sysop, msgNum)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	a, rc, err := s.Deps.Messages.OpenAttachment(s.attachmentsRoot(), attachID, msg.ID)
	if err != nil {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.Filename))
	_, _ = io.Copy(w, rc)
}

func attachmentViews(store *messages.Store, root string, messageID int64) ([]attachmentView, error) {
	list, err := store.ListAttachments(messageID)
	if err != nil {
		return nil, err
	}
	out := make([]attachmentView, 0, len(list))
	for _, a := range list {
		out = append(out, attachmentView{
			ID:        a.ID,
			Filename:  a.Filename,
			SizeBytes: a.SizeBytes,
		})
	}
	return out, nil
}

type attachmentView struct {
	ID        int64  `json:"ID"`
	Filename  string `json:"Filename"`
	SizeBytes int64  `json:"SizeBytes"`
}

func savePostedAttachments(s *Server, c *conferences.Conference, messageID int64, files []messages.AttachmentInput) error {
	if len(files) == 0 {
		return nil
	}
	limit := fido.EchoAttachmentLimit(c)
	return s.Deps.Messages.SaveAttachments(s.attachmentsRoot(), messageID, files, limit)
}
