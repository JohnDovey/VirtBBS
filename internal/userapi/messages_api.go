package userapi

import (
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/qwk"
	"github.com/virtbbs/virtbbs/internal/users"
)

type syncMessageAttachment struct {
	ID        int64  `json:"ID"`
	Filename  string `json:"Filename"`
	SizeBytes int64  `json:"SizeBytes"`
}

type syncMessageItem struct {
	ID           int64                   `json:"ID"`
	ConferenceID int                     `json:"ConferenceID"`
	MsgNumber    int                     `json:"MsgNumber"`
	FromName     string                  `json:"FromName"`
	ToName       string                  `json:"ToName"`
	Subject      string                  `json:"Subject"`
	DatePosted   string                  `json:"DatePosted"`
	Body         string                  `json:"Body"`
	HasAttachment bool                   `json:"HasAttachment"`
	Attachments  []syncMessageAttachment `json:"Attachments"`
}

type syncResult struct {
	Messages []syncMessageItem `json:"Messages"`
}

func (s *Server) handleMessagesSync(req Request, u *users.User) (any, error) {
	var p struct {
		ConferenceIDs []int `json:"ConferenceIDs"`
		Limit         int   `json:"Limit"`
	}
	if err := unmarshalParams(req.Params, &p); err != nil {
		return nil, err
	}
	if p.Limit <= 0 {
		p.Limit = 500
	}

	confIDs := p.ConferenceIDs
	if len(confIDs) == 0 {
		all, err := s.Deps.Conferences.List()
		if err != nil {
			return nil, err
		}
		for _, c := range all {
			if u.SecurityLevel >= c.ReadSec {
				confIDs = append(confIDs, c.ID)
			}
		}
	} else {
		var allowed []int
		for _, cid := range confIDs {
			c, err := s.Deps.Conferences.Get(cid)
			if err != nil {
				continue
			}
			if u.SecurityLevel >= c.ReadSec {
				allowed = append(allowed, cid)
			}
		}
		confIDs = allowed
	}

	var out []syncMessageItem
	for _, cid := range confIDs {
		lastApp := s.Deps.Users.GetAppLast(u.ID, cid)
		msgs, err := s.Deps.Messages.ListFrom(cid, lastApp+1, p.Limit)
		if err != nil {
			return nil, err
		}
		highWater := lastApp
		for _, m := range msgs {
			item := syncMessageItem{
				ID:           m.ID,
				ConferenceID: m.ConferenceID,
				MsgNumber:    m.MsgNumber,
				FromName:     m.FromName,
				ToName:       m.ToName,
				Subject:      m.Subject,
				DatePosted:   m.DatePosted.Format(time.RFC3339),
				Body:         m.Body,
			}
			if atts, err := s.Deps.Messages.ListAttachments(m.ID); err == nil && len(atts) > 0 {
				item.HasAttachment = true
				for _, a := range atts {
					item.Attachments = append(item.Attachments, syncMessageAttachment{
						ID: a.ID, Filename: a.Filename, SizeBytes: a.SizeBytes,
					})
				}
			}
			out = append(out, item)
			if m.MsgNumber > highWater {
				highWater = m.MsgNumber
			}
		}
		if highWater > lastApp {
			if err := s.Deps.Users.SetAppLast(u.ID, cid, highWater); err != nil {
				return nil, err
			}
		}
	}
	return syncResult{Messages: nonNilSlice(out)}, nil
}

func (s *Server) handleMessagesPost(req Request, u *users.User) (any, error) {
	var p struct {
		Replies []struct {
			ConferenceID int    `json:"ConferenceID"`
			RefNum       int    `json:"RefNum"`
			ToName       string `json:"ToName"`
			Subject      string `json:"Subject"`
			Body         string `json:"Body"`
		} `json:"Replies"`
	}
	if err := unmarshalParams(req.Params, &p); err != nil {
		return nil, err
	}
	var allowed []*qwk.ReplyMsg
	for _, r := range p.Replies {
		c, err := s.Deps.Conferences.Get(r.ConferenceID)
		if err != nil {
			continue
		}
		if u.SecurityLevel < c.WriteSec {
			continue
		}
		allowed = append(allowed, &qwk.ReplyMsg{
			ConferenceID: r.ConferenceID,
			RefNum:       r.RefNum,
			ToName:       r.ToName,
			Subject:      r.Subject,
			Body:         r.Body,
		})
	}
	posted, err := qwk.PostReplies(s.Deps.Messages, s.Deps.Conferences, u, allowed)
	if err != nil {
		return nil, err
	}
	return map[string]int{"posted": posted, "rejected": len(p.Replies) - posted}, nil
}

func (s *Server) handleMessagesMarkRead(req Request, u *users.User) (any, error) {
	var p struct {
		ConferenceID int `json:"ConferenceID"`
		MsgNumber    int `json:"MsgNumber"`
	}
	if err := unmarshalParams(req.Params, &p); err != nil {
		return nil, err
	}
	c, err := s.Deps.Conferences.Get(p.ConferenceID)
	if err != nil {
		return nil, err
	}
	if u.SecurityLevel < c.ReadSec {
		return nil, fmt.Errorf("access denied")
	}
	if err := s.Deps.Users.SetLastRead(u.ID, p.ConferenceID, p.MsgNumber); err != nil {
		return nil, err
	}
	unread, _ := s.Deps.Users.NewMessageCounts(u.ID)
	return map[string]int{
		"LastRead": p.MsgNumber,
		"Unread":   unread[p.ConferenceID],
	}, nil
}

func (s *Server) handleMessagesAttachmentDownload(req Request, u *users.User) (any, error) {
	var p struct {
		ConferenceID int   `json:"ConferenceID"`
		MsgNumber    int   `json:"MsgNumber"`
		AttachmentID int64 `json:"AttachmentID"`
	}
	if err := unmarshalParams(req.Params, &p); err != nil {
		return nil, err
	}
	c, err := s.Deps.Conferences.Get(p.ConferenceID)
	if err != nil {
		return nil, err
	}
	if u.SecurityLevel < c.ReadSec {
		return nil, fmt.Errorf("access denied")
	}
	msg, err := s.Deps.Messages.Get(p.ConferenceID, p.MsgNumber)
	if err != nil {
		return nil, fmt.Errorf("message not found")
	}
	if messages.IsNetmail(msg) && !messages.CanViewNetmail(u.Name, u.Sysop, msg) {
		return nil, fmt.Errorf("access denied")
	}
	root := messages.AttachmentsRoot(config.Get().Paths.DB, config.Get().Paths.Attachments)
	a, rc, err := s.Deps.Messages.OpenAttachment(root, p.AttachmentID, msg.ID)
	if err != nil {
		return nil, fmt.Errorf("attachment not found")
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"filename": a.Filename,
		"data":     base64.StdEncoding.EncodeToString(data),
	}, nil
}
