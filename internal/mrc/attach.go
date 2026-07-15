package mrc

import (
	"sync"
	"time"
)

// Event is a line delivered to an attached terminal session.
type Event struct {
	Kind    EventKind
	From    string
	Site    string
	Room    string
	Body    string
	Raw     Packet
	Local   bool   // generated locally (notice)
	PipeRaw string // original body with pipe codes
}

// EventKind classifies inbound traffic for the UI.
type EventKind int

const (
	EventChat EventKind = iota
	EventPrivate
	EventSystem
	EventNotice
	EventTopic
	EventUserList
)

// Attachment is one local user connected to the hub.
type Attachment struct {
	ID     string
	UserID int64
	Handle string
	Room   string
	Inbox  chan Event

	mu     sync.Mutex
	nicks  map[string]struct{}
	topic  string
	closed bool
}

func newAttachment(id string, userID int64, handle, room string) *Attachment {
	return &Attachment{
		ID:     id,
		UserID: userID,
		Handle: handle,
		Room:   room,
		Inbox:  make(chan Event, 256),
		nicks:  make(map[string]struct{}),
	}
}

// Deliver pushes an event without blocking forever.
func (a *Attachment) Deliver(ev Event) {
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return
	}
	select {
	case a.Inbox <- ev:
	default:
		// Drop if client is too slow
	}
}

// SetRoom updates the attachment room.
func (a *Attachment) SetRoom(room string) {
	a.mu.Lock()
	a.Room = SanitizeName(room)
	a.mu.Unlock()
}

// CurrentRoom returns the room under lock.
func (a *Attachment) CurrentRoom() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Room
}

// SetTopic stores room topic text.
func (a *Attachment) SetTopic(t string) {
	a.mu.Lock()
	a.topic = t
	a.mu.Unlock()
}

// Topic returns the last known topic.
func (a *Attachment) Topic() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.topic
}

// SetNicks replaces the known nick list for tab-complete.
func (a *Attachment) SetNicks(names []string) {
	a.mu.Lock()
	a.nicks = make(map[string]struct{}, len(names))
	for _, n := range names {
		n = SanitizeName(n)
		if n != "" {
			a.nicks[n] = struct{}{}
		}
	}
	a.mu.Unlock()
}

// NoteNick remembers a nick seen in chat.
func (a *Attachment) NoteNick(name string) {
	name = SanitizeName(name)
	if name == "" {
		return
	}
	a.mu.Lock()
	a.nicks[name] = struct{}{}
	a.mu.Unlock()
}

// CompleteNick returns the next tab-complete candidate for prefix.
func (a *Attachment) CompleteNick(prefix string, index int) (string, int) {
	prefix = stringsEqualFoldPrefix(prefix)
	a.mu.Lock()
	defer a.mu.Unlock()
	var matches []string
	for n := range a.nicks {
		if len(prefix) == 0 || hasPrefixFold(n, prefix) {
			matches = append(matches, n)
		}
	}
	if len(matches) == 0 {
		return "", 0
	}
	// stable-ish order
	sortStrings(matches)
	if index < 0 || index >= len(matches) {
		index = 0
	}
	return matches[index], (index + 1) % len(matches)
}

// Nicks returns a snapshot of known nicknames.
func (a *Attachment) Nicks() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.nicks))
	for n := range a.nicks {
		out = append(out, n)
	}
	sortStrings(out)
	return out
}

func (a *Attachment) closeInbox() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	a.closed = true
	close(a.Inbox)
}

func hasPrefixFold(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c1, c2 := s[i], prefix[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

func stringsEqualFoldPrefix(s string) string {
	return SanitizeName(s)
}

func sortStrings(a []string) {
	// tiny insertion sort — nick lists are small
	for i := 1; i < len(a); i++ {
		j := i
		for j > 0 && a[j-1] > a[j] {
			a[j-1], a[j] = a[j], a[j-1]
			j--
		}
	}
}

// Status is a snapshot for admin UI.
type Status struct {
	Enabled      bool      `json:"enabled"`
	Connected    bool      `json:"connected"`
	Reconnecting bool      `json:"reconnecting"`
	Addr         string    `json:"addr"`
	BBSName      string    `json:"bbs_name"`
	Attached     int       `json:"attached"`
	LastError    string    `json:"last_error"`
	LastConnect  time.Time `json:"last_connect,omitempty"`
	UseTLS       bool      `json:"use_tls"`
}
