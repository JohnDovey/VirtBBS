package messages

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestFindThread_followsReplyChain(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}

	post := func(msgID, reply string) {
		t.Helper()
		if err := store.Post(&Message{
			ConferenceID: 1,
			FromName:     "User",
			ToName:       "ALL",
			Subject:      "Test",
			DatePosted:   time.Now(),
			Status:       "A",
			Body:         "body",
			FidoMsgID:    msgID,
			FidoReply:    reply,
		}); err != nil {
			t.Fatal(err)
		}
	}

	post("1:234/1 AAAAAAAA", "")
	post("1:234/1 BBBBBBBB", "1:234/1 AAAAAAAA")
	post("1:234/1 CCCCCCCC", "1:234/1 BBBBBBBB")

	thread, err := store.FindThread(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(thread) != 3 {
		t.Fatalf("thread len = %d, want 3", len(thread))
	}
	if thread[0].FidoMsgID != "1:234/1 AAAAAAAA" {
		t.Errorf("root = %q", thread[0].FidoMsgID)
	}
	if thread[2].FidoMsgID != "1:234/1 CCCCCCCC" {
		t.Errorf("last = %q", thread[2].FidoMsgID)
	}
}

func TestFindNetmailThread_linksInboundAndOutbound(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}

	inboundID := "1:234/567 INBOUND01"
	if err := store.Post(&Message{
		ConferenceID: 0,
		FromName:     "Remote",
		ToName:       "Alice",
		Subject:      "Question",
		DatePosted:   time.Now(),
		Status:       "A",
		Body:         "inbound",
		FidoMsgID:    inboundID,
		FidoOrigin:   "1:234/567",
	}); err != nil {
		t.Fatal(err)
	}

	outboundID := "1:234/1 OUTBOUND1"
	if err := store.Post(&Message{
		ConferenceID:    0,
		FromName:        "Alice",
		ToName:          "Remote",
		Subject:         "Re: Question",
		DatePosted:      time.Now(),
		Status:          "A",
		Body:            "reply",
		FidoMsgID:       outboundID,
		FidoReply:       inboundID,
		FidoOrigin:      "1:234/1",
		FidoNetwork:     "FidoNet",
		NetmailOutbound: true,
	}); err != nil {
		t.Fatal(err)
	}

	thread, err := store.FindNetmailThread("Alice", false, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(thread) != 2 {
		t.Fatalf("thread len = %d, want 2", len(thread))
	}
	if thread[0].FidoMsgID != inboundID {
		t.Errorf("root = %q", thread[0].FidoMsgID)
	}
	if thread[1].FidoMsgID != outboundID {
		t.Errorf("reply = %q", thread[1].FidoMsgID)
	}

	// Bob cannot see Alice's outbound copy.
	if _, err := store.FindNetmailThread("Bob", false, 2); err == nil {
		t.Fatal("Bob should not access Alice's outbound netmail thread start")
	}
}

func TestCountReplies(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := store.Post(&Message{
			ConferenceID: 1,
			FromName:     "User",
			ToName:       "ALL",
			Subject:      "Re",
			DatePosted:   time.Now(),
			Status:       "A",
			Body:         "reply",
			FidoMsgID:    "1:234/1 CHILD" + string(rune('A'+i)),
			FidoReply:    "1:234/1 PARENT",
		}); err != nil {
			t.Fatal(err)
		}
	}

	n, err := store.CountReplies(1, "1:234/1 PARENT")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("CountReplies = %d, want 2", n)
	}
}
