package fido

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestParseFixRequestAuth(t *testing.T) {
	const pw = "secret"
	tests := []struct {
		name    string
		subject string
		body    string
		want    []string
		wantOK  bool
	}{
		{
			name:    "body password",
			subject: "AreaFix",
			body:    "secret\r\n+GENERAL\r\n",
			want:    []string{"+GENERAL"},
			wantOK:  true,
		},
		{
			name:    "subject password",
			subject: "secret",
			body:    "+GENERAL\r\n",
			want:    []string{"+GENERAL"},
			wantOK:  true,
		},
		{
			name:    "subject password with switch",
			subject: "secret -R",
			body:    "+GENERAL\r\n",
			want:    []string{"+GENERAL"},
			wantOK:  true,
		},
		{
			name:    "subject password skips redundant body line",
			subject: "secret",
			body:    "secret\r\n+GENERAL\r\n",
			want:    []string{"+GENERAL"},
			wantOK:  true,
		},
		{
			name:    "subject AreaFix prefix",
			subject: "AreaFix secret",
			body:    "+GENERAL\r\n",
			want:    []string{"+GENERAL"},
			wantOK:  true,
		},
		{
			name:    "bad password",
			subject: "AreaFix",
			body:    "wrong\r\n+GENERAL\r\n",
			wantOK:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseFixRequestAuth(tc.subject, tc.body, pw)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("cmds=%v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("cmds[%d]=%q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}

	t.Run("empty password accepts body commands", func(t *testing.T) {
		got, ok := parseFixRequestAuth("AreaFix", "+GENERAL\r\n", "")
		if !ok {
			t.Fatal("expected ok")
		}
		if len(got) != 1 || got[0] != "+GENERAL" {
			t.Fatalf("cmds=%v", got)
		}
	})
}

func TestFixRequestCommandLines_lineEndings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "lf", in: "%help\n%list", want: []string{"%help", "%list"}},
		{name: "crlf", in: "%help\r\n%list", want: []string{"%help", "%list"}},
		{name: "cr", in: "%help\r%list", want: []string{"%help", "%list"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fixRequestCommandLines(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("lines=%v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("lines[%d]=%q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestBuildFixRequestBody(t *testing.T) {
	body := BuildFixRequestBody([]string{"%help", "%list"}, nil)
	want := "%help\r\n%list\r\n"
	if body != want {
		t.Fatalf("body=%q want %q", body, want)
	}
	body = BuildFixRequestBody([]string{"general", "+FSX_GEN"}, []string{"games"})
	want = "+GENERAL\r\n+FSX_GEN\r\n-GAMES\r\n"
	if body != want {
		t.Fatalf("body=%q want %q", body, want)
	}
}

func TestParseAreaFixAddLine(t *testing.T) {
	tests := []struct {
		line    string
		tag     string
		rescan  int
		wantOK  bool
	}{
		{"+GENERAL", "GENERAL", -1, true},
		{"=GENERAL,R=50", "GENERAL", 50, true},
		{"+FOO,R", "FOO", 0, true},
		{"+BAR,R=0", "BAR", 0, true},
		{"+", "", -1, false},
	}
	for _, tc := range tests {
		got, ok := parseAreaFixAddLine(tc.line)
		if ok != tc.wantOK {
			t.Fatalf("%q: ok=%v want %v", tc.line, ok, tc.wantOK)
		}
		if !ok {
			continue
		}
		if got.tag != tc.tag || got.rescanMax != tc.rescan {
			t.Fatalf("%q: got %+v want tag=%q rescan=%d", tc.line, got, tc.tag, tc.rescan)
		}
	}
}

func TestParseAreaFixRescanLine(t *testing.T) {
	tag, ok := parseAreaFixRescanLine("%RESCAN")
	if !ok || tag != "" {
		t.Fatalf("bare %%RESCAN: tag=%q ok=%v", tag, ok)
	}
	tag, ok = parseAreaFixRescanLine("%RESCAN GENERAL")
	if !ok || tag != "GENERAL" {
		t.Fatalf("%%RESCAN TAG: tag=%q ok=%v", tag, ok)
	}
}

func setupAreaFixTest(t *testing.T) (*messages.Store, *conferences.Store, *NetworkDef, Addr) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	msgStore, err := messages.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	confStore, err := conferences.Open(db)
	if err != nil {
		t.Fatal(err)
	}

	conf := &conferences.Conference{
		Name: "General", Echo: true, EchoTag: "GENERAL", Network: "TestNet", Public: true,
	}
	if err := confStore.Create(conf); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	dlAddr := Addr{Zone: 1, Net: 234, Node: 3}
	nd := &NetworkDef{
		Name:        "TestNet",
		Address:     "1:234/1",
		Uplink:      "1:234/2",
		OutboundDir: outDir,
		Downlinks: []Downlink{
			{Name: "Downlink", Address: dlAddr.String(), Password: "secret"},
		},
	}

	return msgStore, confStore, nd, dlAddr
}

func postEchoMessages(t *testing.T, store *messages.Store, confID int, bodies ...string) {
	t.Helper()
	for i, body := range bodies {
		m := &messages.Message{
			ConferenceID: confID,
			FromName:     "Alice",
			ToName:       "All",
			Subject:      "Test",
			Echo:         true,
			Body:         body,
			DatePosted:   time.Now().Add(time.Duration(i) * time.Minute),
		}
		if err := store.Post(m); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRescanEchoToDownlink_includesExportedMessages(t *testing.T) {
	msgStore, confStore, nd, dlAddr := setupAreaFixTest(t)

	postEchoMessages(t, msgStore, 1, "one", "two", "three")
	msgs, err := msgStore.ListEcho(1, 10, 0)
	if err != nil || len(msgs) != 3 {
		t.Fatalf("ListEcho: %d msgs err=%v", len(msgs), err)
	}
	if err := msgStore.MarkExported(msgs[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := msgStore.MarkExported(msgs[1].ID); err != nil {
		t.Fatal(err)
	}

	areafixDB := OpenAreaFixDB(msgStore.DB())
	if err := areafixDB.Subscribe("TestNet", dlAddr.String(), "GENERAL"); err != nil {
		t.Fatal(err)
	}

	res, err := RescanEchoToDownlink(nd, msgStore, confStore, "TestBBS", dlAddr.String(), []string{"GENERAL"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Messages != 3 {
		t.Fatalf("rescan messages=%d want 3", res.Messages)
	}
	if res.PKTPath == "" {
		t.Fatal("expected rescan pkt path")
	}
	if !strings.Contains(filepath.Base(res.PKTPath), "_rescan_") {
		t.Fatalf("unexpected pkt name %s", res.PKTPath)
	}

	// Export state unchanged — still only 2 marked exported from before.
	again, err := msgStore.ListEcho(1, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 {
		t.Fatalf("ListEcho after rescan=%d want 1 unexported", len(again))
	}
}

func TestRescanEchoToDownlink_respectsMaxMsgs(t *testing.T) {
	msgStore, confStore, nd, dlAddr := setupAreaFixTest(t)
	postEchoMessages(t, msgStore, 1, "a", "b", "c", "d", "e")

	areafixDB := OpenAreaFixDB(msgStore.DB())
	_ = areafixDB.Subscribe("TestNet", dlAddr.String(), "GENERAL")

	res, err := RescanEchoToDownlink(nd, msgStore, confStore, "TestBBS", dlAddr.String(), []string{"GENERAL"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.Messages != 2 {
		t.Fatalf("rescan messages=%d want 2", res.Messages)
	}
}

func TestProcessAreaFixRequest_rescanOnSubscribe(t *testing.T) {
	msgStore, confStore, nd, dlAddr := setupAreaFixTest(t)
	postEchoMessages(t, msgStore, 1, "hello")

	pm := &Message{
		OrigAddr: dlAddr,
		FromName: "Sysop",
		ToName:   AreaFixRobotName,
		Subject:  "AreaFix",
		Body:     "secret\r\n+GENERAL,R=1\r\n",
	}

	if err := ProcessAreaFixRequest(nd, msgStore, confStore, "TestNet", "TestBBS", pm); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(nd.OutboundDir)
	if err != nil {
		t.Fatal(err)
	}
	var rescanPKT bool
	for _, e := range entries {
		if strings.Contains(e.Name(), "_rescan_") && strings.HasSuffix(e.Name(), ".pkt") {
			rescanPKT = true
		}
	}
	if !rescanPKT {
		t.Fatalf("expected rescan pkt in %v", entries)
	}
}

func TestProcessAreaFixRequest_subjectPassword(t *testing.T) {
	msgStore, confStore, nd, dlAddr := setupAreaFixTest(t)

	pm := &Message{
		OrigAddr: dlAddr,
		FromName: "Sysop",
		ToName:   AreaFixRobotName,
		Subject:  "secret",
		Body:     "%QUERY\r\n",
	}

	if err := ProcessAreaFixRequest(nd, msgStore, confStore, "TestNet", "TestBBS", pm); err != nil {
		t.Fatal(err)
	}
}

func TestAreaFixExtendedCommands(t *testing.T) {
	msgStore, confStore, nd, dlAddr := setupAreaFixTest(t)
	SetDownlinkPasswordSaver(func(network, addr, pw string) error {
		if network != "TestNet" || addr != dlAddr.String() || pw != "newsecret" {
			t.Fatalf("password saver: network=%s addr=%s pw=%s", network, addr, pw)
		}
		nd.Downlinks[0].Password = pw
		return nil
	})

	areafixDB := OpenAreaFixDB(msgStore.DB())
	_ = areafixDB.Subscribe("TestNet", dlAddr.String(), "GENERAL")

	pm := func(body string) *Message {
		return &Message{
			OrigAddr: dlAddr,
			FromName: "Sysop",
			ToName:   AreaFixRobotName,
			Subject:  "secret",
			Body:     body,
		}
	}

	if err := ProcessAreaFixRequest(nd, msgStore, confStore, "TestNet", "TestBBS", pm("secret\r\n%UNLINKED\r\n")); err != nil {
		t.Fatal(err)
	}

	if err := ProcessAreaFixRequest(nd, msgStore, confStore, "TestNet", "TestBBS", pm("secret\r\n%PAUSE\r\n")); err != nil {
		t.Fatal(err)
	}
	if !areafixDB.IsDownlinkPaused("TestNet", dlAddr.String()) {
		t.Fatal("expected paused")
	}

	if err := ProcessAreaFixRequest(nd, msgStore, confStore, "TestNet", "TestBBS", pm("secret\r\n%RESUME\r\n")); err != nil {
		t.Fatal(err)
	}
	if areafixDB.IsDownlinkPaused("TestNet", dlAddr.String()) {
		t.Fatal("expected active after resume")
	}

	if err := ProcessAreaFixRequest(nd, msgStore, confStore, "TestNet", "TestBBS", pm("secret\r\n%PASSWD newsecret\r\n")); err != nil {
		t.Fatal(err)
	}
	if nd.Downlinks[0].Password != "newsecret" {
		t.Fatalf("password=%q", nd.Downlinks[0].Password)
	}
}

func TestNormalizeAreaFixCommandLine(t *testing.T) {
	if got := normalizeAreaFixCommandLine("%%LIST"); got != "%LIST" {
		t.Fatalf("escape: %q", got)
	}
	if got := normalizeAreaFixCommandLine("listall"); got != "%LIST" {
		t.Fatalf("listall: %q", got)
	}
	if got := normalizeAreaFixCommandLine("SUBSCRIBE GENERAL"); got != "+GENERAL" {
		t.Fatalf("subscribe: %q", got)
	}
}

func TestExpandAreaFixTagPatterns(t *testing.T) {
	got := expandAreaFixTagPatterns("GEN*", []string{"GENERAL", "GAMES", "FSX_GEN"})
	if len(got) != 1 || got[0] != "GENERAL" {
		t.Fatalf("got %v", got)
	}
}

func TestFixRequestSubjectSwitchCommands(t *testing.T) {
	cmds := fixRequestSubjectSwitchCommands("secret -l -q", "secret")
	if len(cmds) != 2 || cmds[0] != "%LIST" || cmds[1] != "%QUERY" {
		t.Fatalf("cmds=%v", cmds)
	}
}

func TestProcessAreaFixRequest_passwordInSubject(t *testing.T) {
	msgStore, confStore, nd, dlAddr := setupAreaFixTest(t)
	postEchoMessages(t, msgStore, 1, "hello")

	pm := &Message{
		OrigAddr: dlAddr,
		FromName: "Sysop",
		ToName:   AreaFixRobotName,
		Subject:  "secret",
		Body:     "+GENERAL,R=1\r\n",
	}

	if err := ProcessAreaFixRequest(nd, msgStore, confStore, "TestNet", "TestBBS", pm); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(nd.OutboundDir)
	if err != nil {
		t.Fatal(err)
	}
	var rescanPKT bool
	for _, e := range entries {
		if strings.Contains(e.Name(), "_rescan_") && strings.HasSuffix(e.Name(), ".pkt") {
			rescanPKT = true
		}
	}
	if !rescanPKT {
		t.Fatalf("expected rescan pkt in %v", entries)
	}
}
