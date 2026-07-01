package userapi

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/virtbbs/virtbbs/internal/appstats"
	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/db"
	"github.com/virtbbs/virtbbs/internal/files"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/users"
)

func TestMessagesSyncIncremental(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	filesRoot := filepath.Join(dir, "files")
	_ = os.MkdirAll(filesRoot, 0755)

	if _, err := config.Load(filepath.Join(dir, "virtbbs.dat")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	userStore, _ := users.Open(sqlDB)
	msgStore, _ := messages.Open(sqlDB)
	confStore, _ := conferences.Open(sqlDB)
	fileStore, _ := files.Open(sqlDB, filesRoot)
	_ = appstats.Open(sqlDB)

	_ = confStore.Create(&conferences.Conference{ID: 1, Name: "General", ReadSec: 10, WriteSec: 10})
	u := &users.User{Name: "SyncUser", SecurityLevel: 20, PageLength: 24, XferProtocol: "Z", ANSI: true}
	_ = userStore.Create(u, "password123")

	post := func(num int, body string) {
		m := &messages.Message{
			ConferenceID: 1, MsgNumber: num, FromName: "Sysop", ToName: "All",
			Subject: "Test", Body: body, Status: "A",
		}
		_ = msgStore.PostWithNumber(m)
	}
	post(1, "first")
	post(2, "second")
	post(3, "third")

	addr := startTestServer(t, userStore, msgStore, confStore, fileStore)
	client := newTestClient(addr, "SyncUser", "password123")

	// First sync — all three messages.
	r1 := client.call(t, "messages.sync", nil)
	msgs1 := r1["Messages"].([]any)
	if len(msgs1) != 3 {
		t.Fatalf("first sync: got %d messages, want 3", len(msgs1))
	}

	// Second sync — nothing new.
	r2 := client.call(t, "messages.sync", nil)
	msgs2 := r2["Messages"].([]any)
	if len(msgs2) != 0 {
		t.Fatalf("second sync: got %d messages, want 0", len(msgs2))
	}

	post(4, "fourth")
	r3 := client.call(t, "messages.sync", nil)
	msgs3 := r3["Messages"].([]any)
	if len(msgs3) != 1 {
		t.Fatalf("third sync: got %d messages, want 1", len(msgs3))
	}
}

func TestMessagesMarkRead(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	filesRoot := filepath.Join(dir, "files")
	_ = os.MkdirAll(filesRoot, 0755)
	if _, err := config.Load(filepath.Join(dir, "virtbbs.dat")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	sqlDB, _ := db.Open(dbPath)
	defer sqlDB.Close()
	userStore, _ := users.Open(sqlDB)
	msgStore, _ := messages.Open(sqlDB)
	confStore, _ := conferences.Open(sqlDB)
	fileStore, _ := files.Open(sqlDB, filesRoot)

	_ = confStore.Create(&conferences.Conference{ID: 1, Name: "General", ReadSec: 10, WriteSec: 10})
	u := &users.User{Name: "Reader", SecurityLevel: 20, PageLength: 24, XferProtocol: "Z", ANSI: true}
	_ = userStore.Create(u, "password123")
	_ = msgStore.PostWithNumber(&messages.Message{
		ConferenceID: 1, MsgNumber: 1, FromName: "Sysop", ToName: "All",
		Subject: "Hi", Body: "Hello", Status: "A",
	})

	addr := startTestServer(t, userStore, msgStore, confStore, fileStore)
	client := newTestClient(addr, "Reader", "password123")

	client.call(t, "messages.mark_read", map[string]any{"ConferenceID": 1, "MsgNumber": 1})
	if got := userStore.GetLastRead(u.ID, 1); got != 1 {
		t.Fatalf("last read = %d, want 1", got)
	}
}

func TestMessagesDeleteNetmailOnly(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	filesRoot := filepath.Join(dir, "files")
	_ = os.MkdirAll(filesRoot, 0755)
	if _, err := config.Load(filepath.Join(dir, "virtbbs.dat")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	sqlDB, _ := db.Open(dbPath)
	defer sqlDB.Close()
	userStore, _ := users.Open(sqlDB)
	msgStore, _ := messages.Open(sqlDB)
	confStore, _ := conferences.Open(sqlDB)
	fileStore, _ := files.Open(sqlDB, filesRoot)

	_ = confStore.Create(&conferences.Conference{ID: 0, Name: "General", ReadSec: 10, WriteSec: 10})
	_ = confStore.Create(&conferences.Conference{ID: 1, Name: "Echo", ReadSec: 10, WriteSec: 10, Echo: true})
	u := &users.User{Name: "Alice", SecurityLevel: 20, PageLength: 24, XferProtocol: "Z", ANSI: true}
	_ = userStore.Create(u, "password123")

	_ = msgStore.PostWithNumber(&messages.Message{
		ConferenceID: 0, MsgNumber: 1, FromName: "Bob", ToName: "Alice",
		Subject: "Netmail", Body: "private", Status: "A",
		FidoOrigin: "1:2/3 @ Somewhere",
	})
	_ = msgStore.PostWithNumber(&messages.Message{
		ConferenceID: 1, MsgNumber: 1, FromName: "Sysop", ToName: "All",
		Subject: "Echo", Body: "public echo", Status: "A", Echo: true,
	})

	addr := startTestServer(t, userStore, msgStore, confStore, fileStore)
	client := newTestClient(addr, "Alice", "password123")

	client.call(t, "messages.delete", map[string]any{
		"ConferenceID": VirtAndNetmailConferenceID,
		"MsgNumber":    1,
	})
	if _, err := msgStore.GetNetmail("Alice", false, 1); err == nil {
		t.Fatal("netmail should be deleted on server")
	}

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	req := map[string]any{
		"method": "messages.delete",
		"auth":   map[string]string{"username": "Alice", "password": "password123"},
		"params": map[string]any{"ConferenceID": 1, "MsgNumber": 1},
	}
	line, _ := json.Marshal(req)
	line = append(line, '\n')
	if _, err := conn.Write(line); err != nil {
		t.Fatalf("write: %v", err)
	}
	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		t.Fatalf("no response")
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("expected error deleting echomail from server")
	}
}

func startTestServer(t *testing.T, userStore *users.Store, msgStore *messages.Store,
	confStore *conferences.Store, fileStore *files.Store) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &Server{
		Addr: ln.Addr().String(),
		Deps: Deps{Users: userStore, Messages: msgStore, Conferences: confStore, Files: fileStore},
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.handle(c)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

type testClient struct {
	addr     string
	username string
	password string
}

func newTestClient(addr, username, password string) *testClient {
	return &testClient{addr: addr, username: username, password: password}
}

func (c *testClient) call(t *testing.T, method string, params any) map[string]any {
	t.Helper()
	conn, err := net.DialTimeout("tcp", c.addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	req := map[string]any{
		"method": method,
		"auth":   map[string]string{"username": c.username, "password": c.password},
	}
	if params != nil {
		req["params"] = params
	}
	line, _ := json.Marshal(req)
	line = append(line, '\n')
	if _, err := conn.Write(line); err != nil {
		t.Fatalf("write: %v", err)
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	if !sc.Scan() {
		t.Fatalf("no response")
	}
	var resp struct {
		Result map[string]any `json:"result"`
		Error  string         `json:"error"`
	}
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("%s: %s", method, resp.Error)
	}
	return resp.Result
}
