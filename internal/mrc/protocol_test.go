package mrc

import (
	"strings"
	"testing"
)

func TestParsePacket(t *testing.T) {
	p, ok := ParsePacket("alice~MyBBS~lobby~~~lobby~hello world~\n")
	if !ok {
		t.Fatal("parse failed")
	}
	if p.FromUser != "alice" || p.FromSite != "MyBBS" || p.Body != "hello world" {
		t.Fatalf("unexpected packet: %+v", p)
	}
}

func TestParsePacketNoTrailing(t *testing.T) {
	p, ok := ParsePacket("alice~MyBBS~lobby~~~lobby~hi")
	if !ok {
		t.Fatal("parse failed")
	}
	if p.Body != "hi" {
		t.Fatalf("body=%q", p.Body)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	orig := Packet{FromUser: "bob", FromSite: "VB", FromRoom: "lobby", ToRoom: "lobby", Body: "yo"}
	p, ok := ParsePacket(orig.Encode())
	if !ok {
		t.Fatal("parse failed")
	}
	if p.FromUser != "bob" || p.Body != "yo" {
		t.Fatalf("%+v", p)
	}
}

func TestSanitizeName(t *testing.T) {
	if got := SanitizeName("A Net Online"); got != "A_Net_Online" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(SanitizeName("hi~there"), "~") {
		t.Fatal("tilde not stripped")
	}
}

func TestSplitMessage(t *testing.T) {
	chunks := SplitMessage(strings.Repeat("a", 150), 140)
	if len(chunks) != 2 {
		t.Fatalf("len=%d", len(chunks))
	}
	if len(chunks[0]) > 140 || len(chunks[1]) > 140 {
		t.Fatal("chunk too long")
	}
}

func TestIsChatMessage(t *testing.T) {
	if (Packet{Body: "IAMHERE"}).IsChatMessage() {
		t.Fatal("IAMHERE should not be chat")
	}
	if !(Packet{Body: "hello"}).IsChatMessage() {
		t.Fatal("hello should be chat")
	}
}
