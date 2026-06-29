package fido

import (
	"strings"
	"testing"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/db"
	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestAppendEchoTagline_addsRandomLine(t *testing.T) {
	sqlDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := messages.Open(sqlDB); err != nil {
		t.Fatal(err)
	}
	if err := MigrateTaglines(sqlDB); err != nil {
		t.Fatal(err)
	}
	tdb := OpenTaglineDB(sqlDB)
	if _, err := tdb.Upsert("Line one.", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.Upsert("Line two.", "test"); err != nil {
		t.Fatal(err)
	}

	m := &messages.Message{Echo: true, Body: "Hello echo world\r\n"}
	AppendEchoTagline(m, sqlDB, "")
	if !strings.Contains(m.Body, "Hello echo world") {
		t.Fatalf("body missing message text: %q", m.Body)
	}
	if !strings.Contains(m.Body, "Line one.") && !strings.Contains(m.Body, "Line two.") {
		t.Fatalf("expected tagline appended: %q", m.Body)
	}
}

func TestAppendEchoTagline_skipsImported(t *testing.T) {
	sqlDB, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := messages.Open(sqlDB); err != nil {
		t.Fatal(err)
	}
	if err := MigrateTaglines(sqlDB); err != nil {
		t.Fatal(err)
	}
	tdb := OpenTaglineDB(sqlDB)
	if _, err := tdb.Upsert("Should not appear.", "test"); err != nil {
		t.Fatal(err)
	}

	m := &messages.Message{
		Echo:       true,
		FidoOrigin: "1:234/1",
		Body:       "Relayed text\r\n",
	}
	before := m.Body
	AppendEchoTagline(m, sqlDB, "")
	if m.Body != before {
		t.Fatalf("imported echo should not get local tagline: %q", m.Body)
	}
}

func TestTaglineForEchoExport_localVsRelay(t *testing.T) {
	taglines := []string{"Alpha", "Beta"}
	local := &messages.Message{Body: "Post body\r\n"}
	if got := taglineForEchoExport(local, taglines); got == "" {
		t.Fatal("expected tagline for local echo")
	}
	relay := &messages.Message{FidoOrigin: "1:2/3", Body: "Relay\r\n"}
	if got := taglineForEchoExport(relay, taglines); got != "" {
		t.Fatalf("relay should not get export tagline, got %q", got)
	}
	withTL := &messages.Message{Body: "Post\r\n\r\nExisting tag\r\n"}
	if got := taglineForEchoExport(withTL, taglines); got != "" {
		t.Fatalf("existing tagline should not pick another, got %q", got)
	}
}

func TestResolveTaglinesPath_networkOverride(t *testing.T) {
	cfg := &Config{
		TaglinesFile: "global.tag",
		Networks: []NetworkDef{{
			Name:         "VirtNet",
			TaglinesFile: "virtnet.tag",
		}},
	}
	conf := &conferences.Conference{Echo: true, Network: "VirtNet"}
	if got := ResolveTaglinesPath(cfg, conf); got != "virtnet.tag" {
		t.Fatalf("ResolveTaglinesPath = %q, want virtnet.tag", got)
	}
}
