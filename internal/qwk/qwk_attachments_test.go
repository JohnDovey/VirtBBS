package qwk

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/virtbbs/virtbbs/internal/config"
	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/db"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/users"
)

func openAttachmentTestStores(t *testing.T) (*users.Store, *messages.Store, *conferences.Store, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	if _, err := config.Load(filepath.Join(dir, "virtbbs.dat")); err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg := config.Get()
	cfg.Paths.DB = dbPath
	if err := config.Save(cfg); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	userStore, err := users.Open(sqlDB)
	if err != nil {
		t.Fatalf("users.Open: %v", err)
	}
	msgStore, err := messages.Open(sqlDB)
	if err != nil {
		t.Fatalf("messages.Open: %v", err)
	}
	confStore, err := conferences.Open(sqlDB)
	if err != nil {
		t.Fatalf("conferences.Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return userStore, msgStore, confStore, config.Get().AttachmentsDir()
}

func TestEncodeBodyNormalizesCarriageReturn(t *testing.T) {
	body := "line one\rline two\r\nline three"
	encoded := encodeBody(body)
	decoded := strings.TrimSuffix(decodeBody(encoded), "\r\n")
	want := "line one\r\nline two\r\nline three"
	if decoded != want {
		t.Fatalf("decode = %q, want %q", decoded, want)
	}
	if strings.Count(string(encoded), string([]byte{softCR})) != 3 {
		t.Fatalf("expected 3 soft-CR markers, got %q", encoded)
	}
}

func TestBuildPacketSmallAttachmentInline(t *testing.T) {
	userStore, msgStore, confStore, attachRoot := openAttachmentTestStores(t)
	if err := confStore.Create(&conferences.Conference{ID: 1, Name: "Chat", ReadSec: 10, WriteSec: 10}); err != nil {
		t.Fatalf("create conference: %v", err)
	}
	u := &users.User{Name: "PointUser", SecurityLevel: 20, PageLength: 24, XferProtocol: "Z", ANSI: true}
	if err := userStore.Create(u, "pw123456"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	m := &messages.Message{ConferenceID: 1, FromName: "Sysop", ToName: "All", Subject: "File", Body: "See attached."}
	if err := msgStore.Post(m); err != nil {
		t.Fatalf("post: %v", err)
	}
	fileData := []byte("hello attachment")
	if err := msgStore.SaveAttachments(attachRoot, m.ID, []messages.AttachmentInput{
		{Filename: "test.txt", Data: fileData},
	}, messages.DefaultAttachmentLimitBytes); err != nil {
		t.Fatalf("SaveAttachments: %v", err)
	}

	data, err := BuildPacket(PacketMeta{BBSName: "Test"}, userStore, msgStore, confStore, u.ID, []int{1})
	if err != nil {
		t.Fatalf("BuildPacket: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, "ATTACH.IDX") {
			t.Fatal("small attachment should be inline, not in ATTACH.IDX")
		}
	}

	msgsDat := zipFile(t, zr, "MESSAGES.DAT")
	header, err := decodeHeader(msgsDat[blockSize : blockSize*2])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	bodyStart := blockSize * 2
	bodyEnd := bodyStart + (header.NumBlocks-1)*blockSize
	body := decodeBody(msgsDat[bodyStart:bodyEnd])
	if !strings.Contains(body, "begin 644 test.txt") {
		t.Fatalf("inline body missing uuencode header: %q", body)
	}
	if !strings.Contains(body, "See attached.\r\n\r\nbegin 644 test.txt") {
		t.Fatalf("inline body missing blank line before uuencode: %q", body)
	}
}

func TestBuildPacketLargeAttachmentSidecar(t *testing.T) {
	userStore, msgStore, confStore, attachRoot := openAttachmentTestStores(t)
	if err := confStore.Create(&conferences.Conference{ID: 1, Name: "Chat", ReadSec: 10, WriteSec: 10}); err != nil {
		t.Fatalf("create conference: %v", err)
	}
	u := &users.User{Name: "PointUser", SecurityLevel: 20, PageLength: 24, XferProtocol: "Z", ANSI: true}
	if err := userStore.Create(u, "pw123456"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	m := &messages.Message{ConferenceID: 1, FromName: "Sysop", ToName: "All", Subject: "Big file", Body: "Large attachment follows in sidecar."}
	if err := msgStore.Post(m); err != nil {
		t.Fatalf("post: %v", err)
	}
	fileData := bytes.Repeat([]byte("X"), 15*1024)
	if err := msgStore.SaveAttachments(attachRoot, m.ID, []messages.AttachmentInput{
		{Filename: "big.bin", Data: fileData},
	}, messages.DefaultAttachmentLimitBytes); err != nil {
		t.Fatalf("SaveAttachments: %v", err)
	}

	data, err := BuildPacket(PacketMeta{BBSName: "Test"}, userStore, msgStore, confStore, u.ID, []int{1})
	if err != nil {
		t.Fatalf("BuildPacket: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}

	idx := string(zipFile(t, zr, "ATTACH.IDX"))
	if !strings.Contains(idx, "big.bin") {
		t.Fatalf("ATTACH.IDX missing file entry: %q", idx)
	}
	uue := string(zipFile(t, zr, "ATTACH/001/0000001_0.UUE"))
	if !strings.HasPrefix(uue, "begin 644 big.bin") {
		t.Fatalf("sidecar UUE missing header: %.80q...", uue)
	}
	if len(uue) < 10000 {
		t.Fatalf("sidecar UUE looks truncated: len=%d", len(uue))
	}

	msgsDat := zipFile(t, zr, "MESSAGES.DAT")
	header, err := decodeHeader(msgsDat[blockSize : blockSize*2])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if header.NumBlocks > 99 {
		t.Fatalf("NumBlocks = %d, exceeds QWK 2-digit limit", header.NumBlocks)
	}
	body := decodeBody(msgsDat[blockSize*2 : blockSize*2+(header.NumBlocks-1)*blockSize])
	if strings.Contains(body, "begin 644") {
		t.Fatalf("inline body should not contain uuencode for large attachment: %.80q...", body)
	}
	if !strings.Contains(body, "Large attachment follows in sidecar.") {
		t.Fatalf("inline body missing message text: %q", body)
	}
}
