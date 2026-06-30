package fido

import "testing"

func TestResolveFreqOutboundMode(t *testing.T) {
	nd := &NetworkDef{FreqOutbound: FreqOutboundFileRequest}
	if got := ResolveFreqOutboundMode(nd, ""); got != FreqOutboundFileRequest {
		t.Fatalf("network default = %q", got)
	}
	if got := ResolveFreqOutboundMode(nd, "classic"); got != FreqOutboundClassic {
		t.Fatalf("override = %q", got)
	}
	if got := NormalizeFreqOutboundMode("file-request"); got != FreqOutboundFileRequest {
		t.Fatalf("normalize = %q", got)
	}
}
