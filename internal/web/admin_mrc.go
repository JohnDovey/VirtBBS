package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/mrc"
	"github.com/virtbbs/virtbbs/internal/version"
)

func (s *Server) handleAdminMRC(w http.ResponseWriter, r *http.Request) {
	_, ok := s.requireSysop(w, r)
	if !ok {
		return
	}
	cfg := config.Get()
	data := struct {
		pageData
		Config *config.Config
		Status mrc.Status
	}{
		pageData: s.page(r),
		Config:   cfg,
	}
	if s.Deps.MRC != nil {
		data.Status = s.Deps.MRC.Status()
	}

	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		merged := *cfg
		merged.MRC.Enabled = formBool(r, "enabled")
		merged.MRC.Host = strings.TrimSpace(r.FormValue("host"))
		merged.MRC.Port = formInt(r, "port", merged.MRC.Port)
		merged.MRC.UseTLS = formBool(r, "use_tls")
		merged.MRC.BBSName = strings.TrimSpace(r.FormValue("bbs_name"))
		merged.MRC.BBSPretty = strings.TrimSpace(r.FormValue("bbs_pretty"))
		merged.MRC.Sysop = strings.TrimSpace(r.FormValue("sysop"))
		merged.MRC.Description = strings.TrimSpace(r.FormValue("description"))
		merged.MRC.Telnet = strings.TrimSpace(r.FormValue("telnet"))
		merged.MRC.SSH = strings.TrimSpace(r.FormValue("ssh"))
		merged.MRC.Website = strings.TrimSpace(r.FormValue("website"))
		merged.MRC.DefaultRoom = strings.TrimSpace(r.FormValue("default_room"))
		merged.MRC.MinSecurity = formInt(r, "min_security", merged.MRC.MinSecurity)
		if err := config.Save(&merged); err != nil {
			data.Error = err.Error()
		} else {
			data.Flash = "MRC settings saved."
			data.Config = config.Get()
			if s.Deps.MRC != nil {
				platform := fmt.Sprintf("VirtBBS_%s/unix", version.Version)
				c := data.Config
				s.Deps.MRC.ApplyConfig(c.MRC.Resolve(c.BBS.Name, c.Sysop.Name, platform))
				data.Status = s.Deps.MRC.Status()
			}
		}
	}

	s.render(w, "admin_mrc.html", data)
}
