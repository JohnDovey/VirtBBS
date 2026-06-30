package fido

import (
	"strings"
	"testing"
)

func TestRenderRobotStatsBulletin(t *testing.T) {
	q := &BinkpStatsQueryResult{
		Networks: []BinkpStatsRow{{
			Network:     "FidoNet",
			AreaFixRecv: 2,
			AreaFixSent: 1,
			FreqRecv:    5,
			FreqSent:    3,
			TICRecv:     3,
			TICSent:     4,
			TICBytesRecv: 1024 * 1024,
		}},
		Links: []BinkpLinkStatsRow{{
			Network:     "FidoNet",
			LinkType:    "downlink",
			PeerKey:     "227:1/17",
			AreaFixRecv: 2,
			AreaFixSent: 1,
			FreqRecv:    1,
		}},
	}
	text := renderRobotStatsBulletin(nil, "Test BBS", "2026-06-30", q, q)
	if !strings.Contains(text, "Robot Statistics") {
		t.Fatal("missing title")
	}
	for _, name := range []string{"AreaFix", "FileFix", "FREQ", "TIC"} {
		if !strings.Contains(text, name) {
			t.Fatalf("missing robot %q: %q", name, text)
		}
	}
	if !strings.Contains(text, "227:1/17") {
		t.Fatal("missing per-node detail")
	}
}

func TestRenderRobotStatsBulletin_empty(t *testing.T) {
	text := renderRobotStatsBulletin(nil, "Test BBS", "2026-06-30", &BinkpStatsQueryResult{}, &BinkpStatsQueryResult{})
	if !strings.Contains(text, "No robot activity") {
		t.Fatalf("expected empty message: %q", text)
	}
}
