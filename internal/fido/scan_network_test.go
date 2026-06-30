package fido

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/db"
	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestScanNetworkEcho_exportsPendingMessage(t *testing.T) {
	sqlDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	msgStore, err := messages.Open(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	confStore, err := conferences.Open(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	InitBinkpStats(sqlDB)

	conf := &conferences.Conference{
		Name: "LVLY Test", Echo: true, EchoTag: "LVLY_TEST", Network: "LovlyNet", Public: true,
	}
	if err := confStore.Create(conf); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	nd := &NetworkDef{
		Name: "LovlyNet", Enabled: true, Address: "227:1/1", Uplink: "227:1/0",
		OutboundDir: outDir,
	}
	m := &messages.Message{
		ConferenceID: conf.ID,
		FromName:     "Sysop",
		ToName:       "All",
		Subject:      "Test",
		Body:         "Outbound echo test\r\n",
		Echo:         true,
		FidoMsgID:    "227:1/1 12345678",
	}
	if err := msgStore.Post(m); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Enabled: true, Networks: []NetworkDef{*nd}}
	res := ScanNetworkEcho(cfg, nd, msgStore, confStore, "Lovely BBS", "")
	if res.Scanned != 1 || res.PKTFiles != 1 {
		t.Fatalf("scan result: scanned=%d pkts=%d errors=%v", res.Scanned, res.PKTFiles, res.Errors)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || filepath.Ext(entries[0].Name()) != ".pkt" {
		t.Fatalf("outbound dir: %v", entries)
	}
}
