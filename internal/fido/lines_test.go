package fido

import "testing"

func TestParse_lfLineEndings(t *testing.T) {
	body := "\x01MSGID: 227:1/1 6a439a9c\n" +
		"\x01REPLY: 227:1/17 1AABCBD0\n" +
		"\x01INTL 227:1/17 227:1/1\n" +
		"\x01FLAGS NPD\n" +
		"FileFix help text line 1\n" +
		"line 2\n"

	pb := (&Message{Body: body}).Parse()
	if pb.MSGID != "227:1/1 6a439a9c" {
		t.Fatalf("MSGID = %q", pb.MSGID)
	}
	if pb.REPLY != "227:1/17 1AABCBD0" {
		t.Fatalf("REPLY = %q", pb.REPLY)
	}
	if pb.Text == "" || pb.Text == body {
		t.Fatalf("Text should be cleaned, got %q", pb.Text)
	}
	if containsSubstr(pb.Text, "\x01MSGID") {
		t.Fatalf("kludges leaked into Text: %q", pb.Text)
	}
}

func TestParse_inlineKludges(t *testing.T) {
	body := "\x01MSGID: 1:2/3 ABCD\x01REPLY: 1:2/4 EFGH\r\nHello\r\n"
	pb := (&Message{Body: body}).Parse()
	if pb.MSGID != "1:2/3 ABCD" {
		t.Fatalf("MSGID = %q", pb.MSGID)
	}
	if pb.REPLY != "1:2/4 EFGH" {
		t.Fatalf("REPLY = %q", pb.REPLY)
	}
}

func TestNormalizeDisplayEOL(t *testing.T) {
	got := NormalizeDisplayEOL("a\rb\nc\r\n")
	if got != "a\r\nb\r\nc\r\n" {
		t.Fatalf("got %q", got)
	}
}
