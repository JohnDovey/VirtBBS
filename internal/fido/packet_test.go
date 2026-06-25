package fido

import (
	"bytes"
	"os"
	"testing"
)

// TestWritePacketReadPacketRoundTrip verifies that ReadPacket can correctly
// parse a .PKT file written by WritePacket for both an echomail-style
// message (no attrib flags, AREA: line in body) and a netmail-style message
// (Private+Crash attrib flags set, no AREA: line), confirming both message
// kinds share one on-disk record layout.
func TestWritePacketReadPacketRoundTrip(t *testing.T) {
	orig := Addr{Zone: 1, Net: 234, Node: 1}
	dest := Addr{Zone: 1, Net: 234, Node: 2}

	echo := &Message{
		OrigAddr: Addr{Zone: 1, Net: 234, Node: 1},
		DestAddr: Addr{Zone: 1, Net: 234, Node: 2},
		DateTime: "24 Jun 26  12:00:00",
		ToName:   "All",
		FromName: "John Dovey",
		Subject:  "Re: Test",
		Body:     "AREA:GENERAL\r\nHello echomail world\r\n",
	}

	netmail := &Message{
		OrigAddr: Addr{Zone: 1, Net: 234, Node: 1},
		DestAddr: Addr{Zone: 1, Net: 234, Node: 2},
		DateTime: "24 Jun 26  12:00:05",
		ToName:   "Jane Doe",
		FromName: "John Dovey",
		Subject:  "Hello",
		Body:     "\x01MSGID: 1:234/1 ABCD1234\r\nHi Jane\r\n",
		Attrib:   AttribPrivate | AttribCrash,
	}

	var buf bytes.Buffer
	if err := WritePacket(&buf, orig, dest, "", []*Message{echo, netmail}); err != nil {
		t.Fatalf("WritePacket: %v", err)
	}

	got, err := ReadPacket(&buf)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}

	gotEcho, gotNetmail := got[0], got[1]

	for _, tc := range []struct {
		name string
		want *Message
		got  *Message
	}{
		{"echo", echo, gotEcho},
		{"netmail", netmail, gotNetmail},
	} {
		if tc.got.OrigAddr != tc.want.OrigAddr {
			t.Errorf("%s: OrigAddr = %v, want %v", tc.name, tc.got.OrigAddr, tc.want.OrigAddr)
		}
		if tc.got.DestAddr != tc.want.DestAddr {
			t.Errorf("%s: DestAddr = %v, want %v", tc.name, tc.got.DestAddr, tc.want.DestAddr)
		}
		if tc.got.DateTime != tc.want.DateTime {
			t.Errorf("%s: DateTime = %q, want %q", tc.name, tc.got.DateTime, tc.want.DateTime)
		}
		if tc.got.ToName != tc.want.ToName {
			t.Errorf("%s: ToName = %q, want %q", tc.name, tc.got.ToName, tc.want.ToName)
		}
		if tc.got.FromName != tc.want.FromName {
			t.Errorf("%s: FromName = %q, want %q", tc.name, tc.got.FromName, tc.want.FromName)
		}
		if tc.got.Subject != tc.want.Subject {
			t.Errorf("%s: Subject = %q, want %q", tc.name, tc.got.Subject, tc.want.Subject)
		}
		if tc.got.Body != tc.want.Body {
			t.Errorf("%s: Body = %q, want %q", tc.name, tc.got.Body, tc.want.Body)
		}
		if tc.got.Attrib != tc.want.Attrib {
			t.Errorf("%s: Attrib = %#x, want %#x", tc.name, tc.got.Attrib, tc.want.Attrib)
		}
	}

	if !gotEcho.IsEcho {
		t.Error("echo: IsEcho = false, want true")
	}
	if gotEcho.Area != "GENERAL" {
		t.Errorf("echo: Area = %q, want %q", gotEcho.Area, "GENERAL")
	}
	if gotNetmail.IsEcho {
		t.Error("netmail: IsEcho = true, want false")
	}
}

// TestWritePKTReadableByReadPacket verifies that WritePKT (netmail.go) and
// WritePacket (packet.go) share the same on-disk record layout: a file
// written by WritePKT must be parseable by ReadPacket.
func TestWritePKTReadableByReadPacket(t *testing.T) {
	origAddr := Addr{Zone: 1, Net: 234, Node: 1}
	destAddr := Addr{Zone: 1, Net: 234, Node: 2}

	tmpDir := t.TempDir()
	msgs := []*NetmailMsg{
		{
			FromName: "John Dovey",
			FromAddr: "1:234/1",
			ToName:   "Jane Doe",
			ToAddr:   "1:234/2",
			Subject:  "Hello",
			Body:     "Hi Jane\r\n",
			Crash:    true,
		},
	}

	path, err := WritePKT(origAddr, destAddr, "", tmpDir, msgs)
	if err != nil {
		t.Fatalf("WritePKT: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	got, err := ReadPacket(f)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d messages, want 1", len(got))
	}

	m := got[0]
	if m.ToName != "Jane Doe" {
		t.Errorf("ToName = %q, want %q", m.ToName, "Jane Doe")
	}
	if m.FromName != "John Dovey" {
		t.Errorf("FromName = %q, want %q", m.FromName, "John Dovey")
	}
	if m.Subject != "Hello" {
		t.Errorf("Subject = %q, want %q", m.Subject, "Hello")
	}
	if m.Attrib&AttribPrivate == 0 {
		t.Error("Attrib missing AttribPrivate bit")
	}
	if m.Attrib&AttribCrash == 0 {
		t.Error("Attrib missing AttribCrash bit")
	}
}
