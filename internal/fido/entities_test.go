package fido

import "testing"

func TestDecodeMessageEntities_legacyDecimal(t *testing.T) {
	got := DecodeMessageEntities(`&34;Happiness depends upon ourselves.&34;`)
	want := `"Happiness depends upon ourselves."`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDecodeMessageEntities_htmlDecimal(t *testing.T) {
	got := DecodeMessageEntities(`&#34;Hello&#34;`)
	if got != `"Hello"` {
		t.Fatalf("got %q", got)
	}
}

func TestDecodeMessageEntities_named(t *testing.T) {
	got := DecodeMessageEntities(`AT&amp;T`)
	if got != `AT&T` {
		t.Fatalf("got %q", got)
	}
}

func TestDecodeMessageEntities_plainAmpersand(t *testing.T) {
	got := DecodeMessageEntities(`100% & done`)
	if got != `100% & done` {
		t.Fatalf("got %q", got)
	}
}
