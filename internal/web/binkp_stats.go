package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/virtbbs/virtbbs/internal/fido"
)

const binkpChartDays = 30

// BinkpNetworkView is one network block on the BinkP stats page.
type BinkpNetworkView struct {
	Network  string
	Stats    fido.BinkpStatsRow
	Links    []fido.BinkpLinkStatsRow
	ChartID  string
	ChartJSON string
}

type binkpStatsPageData struct {
	pageData
	Networks      []string
	Period        string
	PeriodLabel   string
	NetworkFilter string
	Stats         *fido.BinkpStatsQueryResult
	NetworkViews  []BinkpNetworkView
	LogLines      []string
	LogPath       string
	Flash         string
	Error         string
}

func (s *Server) gatherBinkpStatsPage(r *http.Request, flash, errMsg string) binkpStatsPageData {
	locale := localeFromRequest(r)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}
	networkFilter := r.URL.Query().Get("network")

	data := binkpStatsPageData{
		pageData:      s.page(r),
		Networks:      fidoNetworkNamesList(),
		Period:        period,
		PeriodLabel:   binkpPeriodLabel(locale, period),
		NetworkFilter: networkFilter,
		Flash:         flash,
		Error:         errMsg,
	}
	if lines, path, err := fido.ReadBinkpLogTail(40); err == nil {
		data.LogLines, data.LogPath = lines, path
	}
	db := s.Deps.Messages.DB()
	if st, err := fido.QueryBinkpStatsForPeriod(db, networkFilter, period, time.Now()); err == nil {
		data.Stats = st
		for _, n := range st.Networks {
			view := BinkpNetworkView{
				Network: n.Network,
				Stats:   n,
				Links:   fido.LinksForNetwork(st, n.Network),
				ChartID: fido.SanitizeChartID(n.Network),
			}
			if series, err := fido.QueryBinkpDailySeries(db, n.Network, binkpChartDays); err == nil {
				view.ChartJSON = binkpChartJSON(series)
			}
			data.NetworkViews = append(data.NetworkViews, view)
		}
	}
	return data
}

func binkpPeriodLabel(locale, period string) string {
	key := "admin_binkp.period." + period
	if label := tr(locale, key); label != key {
		return label
	}
	return period
}

func binkpChartJSON(s *fido.BinkpDailySeries) string {
	if s == nil {
		return "{}"
	}
	payload := map[string]any{
		"labels": s.Labels,
		"datasets": map[string][]int{
			"pollsOK":      s.PollsOK,
			"pollsFail":    s.PollsFail,
			"filesSent":    s.FilesSent,
			"filesRecv":    s.FilesRecv,
			"netmailSent":  s.NetmailSent,
			"netmailRecv":  s.NetmailRecv,
			"echomailSent": s.EchomailSent,
			"echomailRecv": s.EchomailRecv,
			"tossImported": s.TossImported,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}
