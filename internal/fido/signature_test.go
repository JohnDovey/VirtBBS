package fido

import (
	"strings"
	"testing"

	"github.com/virtbbs/virtbbs/internal/version"
)

func TestOutboundSignatureLines(t *testing.T) {
	orig := Addr{Zone: 1, Net: 234, Node: 1}
	sig := OutboundSignatureLines("My BBS", orig)
	if !strings.Contains(sig, "VirtBBS "+version.Version) {
		t.Fatalf("missing version in %q", sig)
	}
	if !strings.Contains(sig, "My BBS") || !strings.Contains(sig, "1:234/1") {
		t.Fatalf("missing BBS/origin in %q", sig)
	}
}

func TestAppendOutboundSignature_idempotent(t *testing.T) {
	orig := Addr{Zone: 1, Net: 1, Node: 1}
	body := "Hello\r\n"
	once := AppendOutboundSignature(body, "Test", orig)
	twice := AppendOutboundSignature(once, "Test", orig)
	if once != twice {
		t.Fatalf("signature duplicated:\n%q\n%q", once, twice)
	}
	if !strings.Contains(once, OutboundTearLine()) {
		t.Fatalf("missing tear: %q", once)
	}
}

func TestBuildBody_includesOutboundSignature(t *testing.T) {
	SetOutboundBBSName("VirtBBS Test")
	t.Cleanup(func() { SetOutboundBBSName("") })

	body := buildBody(&NetmailMsg{Body: "Hello\r\n", AuthorLang: "en"},
		Addr{Zone: 227, Net: 1, Node: 17},
		Addr{Zone: 227, Net: 1, Node: 1})
	if !strings.Contains(body, OutboundTearLine()) {
		t.Fatalf("missing tear in body: %q", body)
	}
	if !strings.Contains(body, "VirtBBS Test") {
		t.Fatalf("missing BBS name in body: %q", body)
	}
}
