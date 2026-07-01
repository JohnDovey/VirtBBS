package userapi

import (
	"github.com/virtbbs/virtbbs/internal/appstats"
	"github.com/virtbbs/virtbbs/internal/config"
)

func (s *Server) handleAppStatsReport(req Request, userID int64) (any, error) {
	var p struct {
		MessagesDownloaded int `json:"messages_downloaded"`
		MessagesUploaded   int `json:"messages_uploaded"`
		FilesDownloaded    int `json:"files_downloaded"`
		FilesUploaded      int `json:"files_uploaded"`
	}
	if err := unmarshalParams(req.Params, &p); err != nil {
		return nil, err
	}
	if err := appstats.Report(s.Deps.Users.DB(), userID,
		p.MessagesDownloaded, p.MessagesUploaded, p.FilesDownloaded, p.FilesUploaded); err != nil {
		return nil, err
	}
	cfg := config.Get()
	_ = appstats.WriteBulletin(s.Deps.Users.DB(), cfg.Session.DisplayDir, cfg.BBS.Name)
	return "ok", nil
}
