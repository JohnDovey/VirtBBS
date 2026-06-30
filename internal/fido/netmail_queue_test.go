package fido

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestEnqueue_pendingPreservesReplyMsgID(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store, err := messages.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	_ = store

	ndb := OpenNetmailDB(db)
	replyID := "1:234/1 PARENTMSG"
	_, err = ndb.Enqueue(&NetmailMsg{
		Network:    "TestNet",
		FromName:   "Alice",
		FromAddr:   "1:234/1",
		ToName:     "Bob",
		ToAddr:     "1:234/3",
		Subject:    "Re: Hello",
		Body:       "reply body",
		ReplyMsgID: replyID,
	})
	if err != nil {
		t.Fatal(err)
	}

	msgs, _, err := ndb.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("pending len = %d, want 1", len(msgs))
	}
	if msgs[0].ReplyMsgID != replyID {
		t.Fatalf("ReplyMsgID = %q, want %q", msgs[0].ReplyMsgID, replyID)
	}
	if msgs[0].MsgID == "" {
		t.Fatal("expected MsgID to be assigned on enqueue")
	}
}

func TestRecordSentNetmail_postsOutboundCopy(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store, err := messages.Open(db)
	if err != nil {
		t.Fatal(err)
	}

	nd := &NetworkDef{Name: "TestNet", Address: "1:234/1"}
	m := &NetmailMsg{
		FromName:   "Alice",
		FromAddr:   "1:234/1",
		ToName:     "Bob",
		ToAddr:     "1:234/3",
		Subject:    "Hello",
		Body:       "body",
		ReplyMsgID: "1:234/3 PARENT",
	}
	if err := RecordSentNetmail(store, nd, m); err != nil {
		t.Fatal(err)
	}
	if m.MsgID == "" {
		t.Fatal("MsgID should be set")
	}

	posted, err := store.Get(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !posted.NetmailOutbound {
		t.Fatal("expected NetmailOutbound=true")
	}
	if posted.FidoMsgID != m.MsgID {
		t.Fatalf("FidoMsgID = %q, want %q", posted.FidoMsgID, m.MsgID)
	}
	if posted.FidoReply != m.ReplyMsgID {
		t.Fatalf("FidoReply = %q, want %q", posted.FidoReply, m.ReplyMsgID)
	}
	if posted.FidoOrigin != "1:234/1" {
		t.Fatalf("FidoOrigin = %q", posted.FidoOrigin)
	}
}

func TestScanNetmailQueue_writesPKTAndMarksSent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store, err := messages.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	_ = store

	outDir := t.TempDir()
	nd := &NetworkDef{
		Name:        "TestNet",
		Address:     "1:234/1",
		Uplink:      "1:234/2",
		OutboundDir: outDir,
	}

	ndb := OpenNetmailDB(db)
	id, err := ndb.Enqueue(&NetmailMsg{
		Network:  "TestNet",
		FromName: "Alice",
		FromAddr: nd.Address,
		ToName:   "Bob",
		ToAddr:   "1:234/3",
		Subject:  "Hello",
		Body:     "Test body",
	})
	if err != nil {
		t.Fatal(err)
	}

	result := ScanNetmailQueue(nil, nd, db, "")
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.Exported != 1 {
		t.Fatalf("exported %d, want 1", result.Exported)
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || filepath.Ext(entries[0].Name()) != ".pkt" {
		t.Fatalf("expected one .pkt in outbound, got %v", entries)
	}

	msgs, ids, err := ndb.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 || len(ids) != 0 {
		t.Fatalf("queue still has pending rows after scan")
	}

	// Idempotent: nothing left to export.
	again := ScanNetmailQueue(nil, nd, db, "")
	if again.Exported != 0 {
		t.Fatalf("second scan exported %d, want 0", again.Exported)
	}
	_ = id
}
