package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/fido"
	"github.com/virtbbs/virtbbs/internal/users"
)

func (s *Server) enqueueUserNetmail(u *users.User, m *fido.NetmailMsg) (int64, error) {
	cfg := config.Get()
	if !cfg.Fido.Enabled {
		return 0, fmt.Errorf("FidoNet not enabled")
	}
	netName := strings.TrimSpace(m.Network)
	if netName == "" {
		netName = cfg.Fido.EffectivePrimaryName()
	}
	nd := cfg.Fido.NetworkByName(netName)
	if nd == nil {
		return 0, fmt.Errorf("network not found")
	}
	if strings.TrimSpace(m.ToAddr) == "" {
		return 0, fmt.Errorf("destination address required")
	}
	m.Network = nd.Name
	m.FromName = u.Name
	m.FromAddr = nd.Address
	m.AuthorLang = fido.NormalizeLangCode(u.Locale)
	ndb := fido.OpenNetmailDB(s.Deps.Messages.DB())
	return ndb.Enqueue(m)
}

func (s *Server) redirectNetmailReply(w http.ResponseWriter, r *http.Request, msgNum int) {
	http.Redirect(w, r, fmt.Sprintf("/netmail/app?reply=%d", msgNum), http.StatusSeeOther)
}
