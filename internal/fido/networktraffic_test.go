package fido

import (
	"strings"
	"testing"
	"time"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/db"
	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestTrafficMapZipName(t *testing.T) {
	day := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	got := TrafficMapZipName("FIDO_GENERAL", "General Discussion", day)
	want := "FIDO_GENERAL-General_Discussion-2026-06-28-NetworkMap.zip"
	if got != want {
		t.Fatalf("zip name = %q want %q", got, want)
	}
}

func TestCollectEchoTraffic_buildsRoutesAndSeenBy(t *testing.T) {
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

	conf := &conferences.Conference{
		Name: "Test Echo", Echo: true, EchoTag: "TEST_ECHO", Public: true,
	}
	if err := confStore.Create(conf); err != nil {
		t.Fatal(err)
	}

	since := time.Now().Add(-24 * time.Hour)
	if err := msgStore.Post(&messages.Message{
		ConferenceID: conf.ID,
		FromName:     "Alice",
		ToName:       "All",
		Subject:      "Hello",
		Status:       "A",
		Echo:         true,
		Body:         "body\r\n\r\nA tagline here.\r\n--- VirtBBS 1.7.5\r\n * Origin: Test (1:234/1)\r\n",
		FidoOrigin:   "1:234/1",
		FidoPath:     "234/2 100/1",
		FidoSeenBy:   "234/2 234/5",
	}); err != nil {
		t.Fatal(err)
	}

	networks := []NetworkDef{{Name: "FidoNet", Address: "1:100/1"}}
	report, err := CollectEchoTraffic(msgStore, nil, conf, since, networks)
	if err != nil {
		t.Fatal(err)
	}
	if report.MsgCount != 1 {
		t.Fatalf("msg count = %d", report.MsgCount)
	}
	if len(report.RouteEdge) < 2 {
		t.Fatalf("route edges = %v", report.RouteEdge)
	}
	if len(report.SeenEdge) < 2 {
		t.Fatalf("seen edges = %v", report.SeenEdge)
	}
	ascii := buildTrafficASCII(report)
	if !strings.Contains(ascii, "TEST_ECHO") || !strings.Contains(ascii, "1:234/1") {
		t.Fatalf("ascii missing data: %q", ascii)
	}
	if !strings.Contains(ascii, "VirtBBS") || !strings.Contains(ascii, "BBS software") {
		t.Fatalf("ascii missing software table: %q", ascii)
	}
	tdb := OpenTaglineDB(sqlDB)
	if texts := tdb.EnabledTexts(); len(texts) == 0 {
		t.Fatalf("expected harvested tagline")
	}
}

func TestTrafficReportTitleLineMatchesZip(t *testing.T) {
	r := &EchoTrafficReport{
		EchoTag: "FIDO_TEST", EchoName: "My Area",
		PeriodEnd: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	if trafficReportTitleLine(r) != TrafficMapZipName(r.EchoTag, r.EchoName, r.PeriodEnd) {
		t.Fatal("title line should match zip name")
	}
}
