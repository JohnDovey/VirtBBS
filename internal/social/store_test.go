package social_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/virtbbs/virtbbs/internal/social"
)

func TestSocialShoutboxAndPolls(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Exec(`PRAGMA foreign_keys = ON`)

	store, err := social.Open(db)
	if err != nil {
		t.Fatal(err)
	}

	sh, err := store.PostShout(1, "alice", "hello")
	if err != nil || sh.Body != "hello" {
		t.Fatalf("PostShout: %v %#v", err, sh)
	}
	shouts, err := store.ListShouts(10)
	if err != nil || len(shouts) != 1 {
		t.Fatalf("ListShouts: %v len=%d", err, len(shouts))
	}

	rooms, err := store.ListRooms()
	if err != nil || len(rooms) == 0 {
		t.Fatalf("seed room: %v len=%d", err, len(rooms))
	}
	if _, err := store.PostMessage(rooms[0].ID, 1, "alice", "hi room"); err != nil {
		t.Fatal(err)
	}

	pollID, err := store.CreatePoll("Favorite?", []string{"A", "B"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Vote(pollID, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.ClosePoll(pollID); err != nil {
		t.Fatal(err)
	}
	opts, err := store.ListPollOptions(pollID)
	if err != nil || len(opts) != 2 || opts[0].Votes != 1 {
		t.Fatalf("options: %v %#v", err, opts)
	}
}
