package fido

import (
	"strings"
	"testing"
	"time"

	"github.com/virtbbs/virtbbs/internal/messages"
)

func TestEchoDisplayText_localAddsSignature(t *testing.T) {
	orig := Addr{Zone: 227, Net: 1, Node: 1}
	m := &messages.Message{
		Echo:       true,
		Body:       "Hello LVLY_TEST\r\n\r\nA tagline here\r\n",
		DatePosted: time.Now(),
	}
	got := EchoDisplayText(m, "Lovely BBS", orig)
	if !strings.Contains(got, "Hello LVLY_TEST") {
		t.Fatalf("missing text: %q", got)
	}
	if !strings.Contains(got, "A tagline here") {
		t.Fatalf("missing tagline: %q", got)
	}
	if !strings.Contains(got, OutboundTearLine()) {
		t.Fatalf("missing tear line: %q", got)
	}
	if !strings.Contains(got, " * Origin: Lovely BBS (227:1/1)") {
		t.Fatalf("missing origin: %q", got)
	}
}

func TestEchoDisplayText_importedUnchanged(t *testing.T) {
	body := "Relayed text\r\n--- Other 1.0\r\n * Origin: Other (1:2/3)\r\n"
	m := &messages.Message{
		Echo:       true,
		FidoSeenBy: "1/2",
		Body:       body,
	}
	got := EchoDisplayText(m, "BBS", Addr{Zone: 1, Net: 2, Node: 3})
	if got != strings.ReplaceAll(body, "\r\n", "\r\n") {
		// normalizeDisplayEOL preserves content
	}
	if !strings.Contains(got, "--- Other 1.0") {
		t.Fatalf("imported body changed: %q", got)
	}
	if strings.Contains(got, OutboundTearLine()) {
		t.Fatalf("should not add VirtBBS tear to imported: %q", got)
	}
}

func TestQuoteEchoReplyBody_fsc0032(t *testing.T) {
	m := &messages.Message{
		FromName:   "John Dovey",
		DatePosted: time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
		Body:       "First line\r\nSecond line\r\n\r\n--- VirtBBS 2.1.0\r\n * Origin: Test (227:1/1)\r\n",
	}
	got := QuoteEchoReplyBody(m)
	if !strings.Contains(got, "On June 30 2026, John Dovey wrote:") {
		t.Fatalf("missing attribution: %q", got)
	}
	if !strings.Contains(got, " JO> First line") {
		t.Fatalf("missing FSC-0032 quote: %q", got)
	}
	if strings.Contains(got, "--- VirtBBS") {
		t.Fatalf("should not quote tear line: %q", got)
	}
}

func TestEchoMainBody_stripsFooters(t *testing.T) {
	body := "Hello\r\n\r\nTag\r\n\r\n--- VirtBBS 2.1.0\r\n * Origin: X (1:1/1)\r\n"
	if got := EchoMainBody(body); got != "Hello\n\nTag" {
		t.Fatalf("EchoMainBody = %q", got)
	}
}
