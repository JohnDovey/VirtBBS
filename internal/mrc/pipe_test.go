package mrc

import (
	"strings"
	"testing"
)

func TestPipeToANSI(t *testing.T) {
	in := "|15Hello|07 world"
	out := PipeToANSI(in)
	if out == in {
		t.Fatal("expected conversion")
	}
	if StripPipe(in) != "Hello world" {
		t.Fatalf("strip=%q", StripPipe(in))
	}
}

func TestPipeToHTML(t *testing.T) {
	out := PipeToHTML("|15Hi|07 <world>")
	if !strings.Contains(out, "<span") || !strings.Contains(out, "&lt;world&gt;") {
		t.Fatalf("unexpected html: %q", out)
	}
}
