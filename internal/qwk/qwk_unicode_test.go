package qwk

import (
	"testing"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/users"
)

func TestBuildPacketUnicodeNames(t *testing.T) {
	userStore, msgStore, confStore := openTestStores(t)

	if err := confStore.Create(&conferences.Conference{ID: 1, Name: "Chat", ReadSec: 10, WriteSec: 10}); err != nil {
		t.Fatalf("create conference: %v", err)
	}
	u := &users.User{Name: "PointUser", SecurityLevel: 20, PageLength: 24, XferProtocol: "Z", ANSI: true}
	if err := userStore.Create(u, "pw123456"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	m := &messages.Message{ConferenceID: 1, FromName: "日本語", ToName: "All", Subject: "Hello", Body: "body"}
	if err := msgStore.Post(m); err != nil {
		t.Fatalf("post: %v", err)
	}

	meta := PacketMeta{BBSName: "VirtBBS Test", SysopName: "Sysop", BBSID: "VBBS"}
	if _, err := BuildPacket(meta, userStore, msgStore, confStore, u.ID, []int{1}); err != nil {
		t.Fatalf("BuildPacket: %v", err)
	}
}
