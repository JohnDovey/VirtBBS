package fido

import "testing"

func TestValidateNodeFlags(t *testing.T) {
	flags, err := ValidateNodeFlags([]string{"ibn", "ITN", "beer", "UNKNOWN"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	flags, err = ValidateNodeFlags([]string{"ibn", "ITN", "beer", "PING", "ITN"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"IBN", "ITN", "BEER", "PING"}
	if len(flags) != len(want) {
		t.Fatalf("got %v, want %v", flags, want)
	}
	for i := range want {
		if flags[i] != want[i] {
			t.Fatalf("got %v, want %v", flags, want)
		}
	}
}

func TestBuildNodelistFlags(t *testing.T) {
	got := BuildNodelistFlags([]string{"IBN", "ITN", "BEER", "TRACE", "PING"},
		"bbs.example.com", 24554, 23)
	if got != "IBN:bbs.example.com:24554,ITN:23,BEER,TRACE,PING" {
		t.Fatalf("BuildNodelistFlags = %q", got)
	}
	got = BuildNodelistFlags([]string{"INA"}, "host.example.net", 0, 0)
	if got != "INA:host.example.net" {
		t.Fatalf("BuildNodelistFlags INA = %q", got)
	}
}
