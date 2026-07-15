package mrc

import "testing"

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
