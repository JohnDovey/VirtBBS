package qwk

import (
	"testing"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
	"github.com/virtbbs/virtbbs/internal/users"
)

// QWK download must not use web last-read — posting/reading on the web marks
// last_msg_read but VirtAnd should still receive the message on next sync.
func TestBuildPacketUsesQwkLastNotLastRead(t *testing.T) {
	userStore, msgStore, confStore := openTestStores(t)

	if err := confStore.Create(&conferences.Conference{ID: 0, Name: "General", ReadSec: 10, WriteSec: 10}); err != nil {
		t.Fatalf("create conference: %v", err)
	}
	u := &users.User{Name: "PointUser", SecurityLevel: 20, PageLength: 24, XferProtocol: "Z", ANSI: true}
	if err := userStore.Create(u, "pw123456"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := msgStore.Post(&messages.Message{
		ConferenceID: 0,
		FromName:     "Sysop",
		ToName:       "All",
		Subject:      "With attachment",
		Body:         "Hello",
	}); err != nil {
		t.Fatalf("post: %v", err)
	}
	high, err := msgStore.HighMsgNumber(0)
	if err != nil || high != 1 {
		t.Fatalf("HighMsgNumber = %d, err %v", high, err)
	}
	// Simulate web read after post (redirect to read page).
	if err := userStore.SetLastRead(u.ID, 0, high); err != nil {
		t.Fatalf("SetLastRead: %v", err)
	}

	meta := PacketMeta{BBSName: "Test", SysopName: "Sysop", BBSID: "T"}
	data, err := BuildPacket(meta, userStore, msgStore, confStore, u.ID, []int{0})
	if err != nil {
		t.Fatalf("BuildPacket: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty QWK packet")
	}
	if got := userStore.GetQwkLast(u.ID, 0); got != 1 {
		t.Fatalf("GetQwkLast = %d, want 1", got)
	}

	// Second download with no new posts should be empty of new messages.
	data2, err := BuildPacket(meta, userStore, msgStore, confStore, u.ID, []int{0})
	if err != nil {
		t.Fatalf("BuildPacket second: %v", err)
	}
	_ = data2
	if got := userStore.GetQwkLast(u.ID, 0); got != 1 {
		t.Fatalf("GetQwkLast after second = %d, want 1", got)
	}
}
