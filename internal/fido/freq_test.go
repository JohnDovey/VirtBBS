package fido

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/virtbbs/virtbbs/internal/messages"
)

type mockFreqCatalog struct {
	dirs  []FreqDirInfo
	files map[int64][]FreqFileInfo
}

func (m *mockFreqCatalog) ListFreqDirs() ([]FreqDirInfo, error)  { return m.dirs, nil }
func (m *mockFreqCatalog) ListFreqFiles(id int64) ([]FreqFileInfo, error) {
	return m.files[id], nil
}

func setupFreqTest(t *testing.T) (FreqCatalog, *NetworkDef, Addr, string, *sql.DB) {
	t.Helper()
	filesRoot := t.TempDir()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := messages.Open(db); err != nil {
		t.Fatal(err)
	}
	InitBinkpStats(db)

	gamesDir := filepath.Join(filesRoot, "games")
	if err := os.MkdirAll(gamesDir, 0755); err != nil {
		t.Fatal(err)
	}
	catalog := &mockFreqCatalog{
		dirs: []FreqDirInfo{{ID: 1, Name: "Games", RelPath: "games"}},
		files: map[int64][]FreqFileInfo{},
	}
	for _, name := range []string{"readme.txt", "demo.zip", "archive.tar"} {
		content := []byte("payload-" + name)
		if err := os.WriteFile(filepath.Join(gamesDir, name), content, 0644); err != nil {
			t.Fatal(err)
		}
		catalog.files[1] = append(catalog.files[1], FreqFileInfo{
			Filename: name, Size: int64(len(content)), Description: "Test " + name,
		})
	}

	outDir := t.TempDir()
	dlAddr := Addr{Zone: 1, Net: 234, Node: 3}
	nd := &NetworkDef{
		Name:         "TestNet",
		Enabled:      true,
		Address:      "1:234/1",
		Uplink:       "1:234/2",
		OutboundDir:  outDir,
		FreqPassword: "freqpw",
		Downlinks: []Downlink{
			{Name: "Downlink", Address: dlAddr.String(), Password: "secret"},
		},
	}

	return catalog, nd, dlAddr, filesRoot, db
}

func TestBuildFreqFileRequest(t *testing.T) {
	subject, body, err := BuildFreqFileRequest([]string{"readme.txt", "%LIST"}, "remotepw")
	if err != nil {
		t.Fatal(err)
	}
	if subject != "readme.txt ALLFILES" {
		t.Fatalf("subject=%q", subject)
	}
	if body != "remotepw\r\n" {
		t.Fatalf("body=%q", body)
	}
}

func TestRequestFreq_setsFileRequestAttribute(t *testing.T) {
	_, nd, _, _, _ := setupFreqTest(t)
	path, err := RequestFreq(nd, "Sysop", []string{"readme.txt", "ALLFILES"}, "1:234/2")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	msgs, err := ReadPacket(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("msgs=%d", len(msgs))
	}
	m := msgs[0]
	if m.Attrib&AttribFileRequest == 0 {
		t.Fatalf("attrib=%#x want FILE_REQUEST", m.Attrib)
	}
	if m.Subject != "readme.txt ALLFILES" {
		t.Fatalf("subject=%q", m.Subject)
	}
	if !strings.Contains(m.Body, "freqpw") {
		t.Fatalf("body=%q", m.Body)
	}
	if m.ToName != "FileRequest" {
		t.Fatalf("ToName=%q", m.ToName)
	}
}

func TestIsFreqRequest(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"Freq", true},
		{"FREQ", true},
		{"FileRequest", true},
		{"Sysop", false},
	}
	for _, tc := range tests {
		if got := IsFreqRequest(tc.name); got != tc.ok {
			t.Fatalf("%q: got %v want %v", tc.name, got, tc.ok)
		}
	}
}

func TestParseFreqAuth(t *testing.T) {
	cmds, ok := parseFreqAuth("freqpw", "readme.txt\r\n", "freqpw")
	if !ok {
		t.Fatal("expected ok")
	}
	if len(cmds) != 1 || cmds[0] != "readme.txt" {
		t.Fatalf("cmds=%v", cmds)
	}

	_, ok = parseFreqAuth("Freq", "wrong\r\nreadme.txt\r\n", "freqpw")
	if ok {
		t.Fatal("expected bad password")
	}
}

func TestResolveFreqFiles_wildcard(t *testing.T) {
	catalog, _, _, filesRoot, _ := setupFreqTest(t)
	matches, err := resolveFreqFiles(catalog, filesRoot, "*.zip")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].filename != "demo.zip" {
		t.Fatalf("matches=%+v", matches)
	}
}

func TestQueueFreqRawFiles_outboundSubdir(t *testing.T) {
	catalog, nd, dlAddr, filesRoot, _ := setupFreqTest(t)
	matches, err := resolveFreqFiles(catalog, filesRoot, "readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	queued, msgs := queueFreqRawFiles(nd, dlAddr, matches, 5, DefaultFreqMaxBytes)
	if len(msgs) > 0 {
		t.Fatalf("msgs=%v", msgs)
	}
	if len(queued) != 1 {
		t.Fatalf("queued=%v", queued)
	}
	sub := filepath.Join(nd.OutboundDir, outboundSubdirName(dlAddr))
	path := filepath.Join(sub, queued[0].destName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("queued file missing: %v", err)
	}
}

func TestProcessFreqRequest_authAndQueue(t *testing.T) {
	catalog, nd, dlAddr, filesRoot, db := setupFreqTest(t)
	ndb := OpenNodelistDB(db)

	pm := &Message{
		FromName: "Caller",
		ToName:   FreqRobotName,
		OrigAddr: dlAddr,
		DestAddr: nd.NodeAddr(),
		Subject:  "freqpw",
		Body:     "readme.txt\r\n",
	}
	if err := ProcessFreqRequest(nd, catalog, filesRoot, ndb, nd.Name, pm); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(nd.OutboundDir, outboundSubdirName(dlAddr))
	entries, err := os.ReadDir(sub)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("outbound entries=%d", len(entries))
	}
	if !strings.Contains(entries[0].Name(), "readme") {
		t.Fatalf("unexpected file %q", entries[0].Name())
	}

	res, err := QueryBinkpStats(db, nd.Name, "day", statsPeriodKey("day", testNow()))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Networks) == 0 || res.Networks[0].FreqRecv != 1 {
		t.Fatalf("freq recv stats: %+v", res.Networks)
	}
}

func TestProcessFreqRequest_rejectsUnknown(t *testing.T) {
	catalog, nd, _, filesRoot, db := setupFreqTest(t)
	ndb := OpenNodelistDB(db)
	unknown := Addr{Zone: 1, Net: 999, Node: 1}
	pm := &Message{
		FromName: "Stranger",
		ToName:   FreqRobotName,
		OrigAddr: unknown,
		Subject:  "freqpw",
		Body:     "readme.txt\r\n",
	}
	if err := ProcessFreqRequest(nd, catalog, filesRoot, ndb, nd.Name, pm); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(nd.OutboundDir, outboundSubdirName(unknown))
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Fatal("expected no outbound for unauthorized requester")
	}
}

func TestRobotStats_freqCounters(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := messages.Open(sqlDB); err != nil {
		t.Fatal(err)
	}
	InitBinkpStats(sqlDB)

	RecordFreqSent("FidoNet", "uplink", "1:2/3")
	RecordFreqRecv("FidoNet", "downlink", "1:4/5")

	res, err := QueryBinkpStats(sqlDB, "FidoNet", "day", statsPeriodKey("day", testNow()))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Networks) != 1 || res.Networks[0].FreqSent != 1 || res.Networks[0].FreqRecv != 1 {
		t.Fatalf("stats: %+v", res.Networks)
	}
}
