package uuencode

import (
	"bytes"
	"testing"
)

func TestEncodeDecode_roundTrip(t *testing.T) {
	data := []byte("Hello, FidoNet attachment test!\x00\x01\x02")
	enc := Encode(data, "test.bin")
	files, clean, err := Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files=%d", len(files))
	}
	if files[0].Filename != "test.bin" || !bytes.Equal(files[0].Data, data) {
		t.Fatalf("got %+v", files[0])
	}
	if stringsTrim(clean) != "" {
		t.Fatalf("clean=%q", clean)
	}
}

func TestDecode_embeddedInMessage(t *testing.T) {
	body := "See attached file.\r\r" + Encode([]byte("ABC"), "foo.txt")
	files, clean, err := Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || string(files[0].Data) != "ABC" {
		t.Fatalf("files=%+v", files)
	}
	if !stringsHasPrefix(clean, "See attached") {
		t.Fatalf("clean=%q", clean)
	}
}

func stringsTrim(s string) string {
	return bytesTrimSpace([]byte(s))
}

func stringsHasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

func bytesTrimSpace(b []byte) string {
	return string(bytes.TrimSpace(b))
}
