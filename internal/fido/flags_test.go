package fido

import (
	"strings"
	"testing"
)

func TestFlagsKludgeLine_and_Parse(t *testing.T) {
	line := FlagsKludgeLine("PVT")
	if !strings.Contains(line, "PVT") {
		t.Fatalf("line = %q", line)
	}
	got := ParseFlagsFromKludges("\x01TZUTC: +0000\r" + strings.TrimRight(line, "\r"))
	if got != "PVT" {
		t.Fatalf("flags = %q, want PVT", got)
	}
}

func TestIntlKludgeLine_and_Parse(t *testing.T) {
	from := Addr{Zone: 227, Net: 1, Node: 200}
	to := Addr{Zone: 227, Net: 1, Node: 17}
	line := IntlKludgeLine(from, to)
	if !strings.Contains(line, "227:1/17") || !strings.Contains(line, "227:1/200") {
		t.Fatalf("intl line = %q", line)
	}
	dest, orig := ParseIntlFromKludges(strings.TrimRight(line, "\r"))
	if dest != "227:1/17" || orig != "227:1/200" {
		t.Fatalf("dest=%q orig=%q", dest, orig)
	}
}

func TestBuildBody_intlAndFlags(t *testing.T) {
	body := buildBody(&NetmailMsg{Body: "Hello\r\n", AuthorLang: "en"},
		Addr{Zone: 227, Net: 1, Node: 17},
		Addr{Zone: 227, Net: 1, Node: 1})
	if !strings.Contains(body, "\x01INTL 227:1/1 227:1/17") {
		t.Fatalf("missing INTL in body: %q", body)
	}
	if !strings.Contains(body, "\x01FLAGS PVT") {
		t.Fatalf("missing FLAGS PVT in body: %q", body)
	}
}

func TestHasPrivateFlag(t *testing.T) {
	if !HasPrivateFlag("\x01FLAGS PVT\r", 0) {
		t.Fatal("expected PVT from kludge")
	}
	if !HasPrivateFlag("", AttribPrivate) {
		t.Fatal("expected private from attrib")
	}
}
