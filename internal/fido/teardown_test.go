package fido

import (
	"strings"
	"testing"
)

func TestParseTearLine(t *testing.T) {
	sw, ver := ParseTearLine("--- VirtBBS 1.7.5")
	if sw != "VirtBBS" || ver != "1.7.5" {
		t.Fatalf("got %q %q", sw, ver)
	}
	sw, ver = ParseTearLine("--- hpt/lnx 1.9 2024-03-02")
	if sw != "hpt/lnx" || ver != "1.9" {
		t.Fatalf("got %q %q", sw, ver)
	}
}

func TestParseEchoFooters_taglineAndTear(t *testing.T) {
	body := "Hello world\r\n\r\n\"The only way to do great work is to love what you do.\"\r\n--- VirtBBS 1.7.5\r\n * Origin: Test (1:234/1)\r\n"
	tags, sw, ver := ParseEchoFooters(body)
	if len(tags) != 1 || !strings.Contains(tags[0], "great work") {
		t.Fatalf("tags = %v", tags)
	}
	if sw != "VirtBBS" || ver != "1.7.5" {
		t.Fatalf("tear = %q %q", sw, ver)
	}
}
