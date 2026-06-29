package fido

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestBuildNodelistZipContainsDiz(t *testing.T) {
	when := time.Date(2026, 6, 28, 14, 30, 0, 0, time.UTC)
	diz := NodelistFileIDDiz("VirtNet", "MyBBS", "300:1/1", "NODELIST.Z79", false, when)
	if !strings.Contains(diz, "Network VirtNet") || !strings.Contains(diz, "week 26") || !strings.Contains(diz, "day 179") {
		t.Fatalf("diz = %q", diz)
	}

	zipData, err := BuildNodelistZip("NODELIST.Z79", []byte("Zone,300,Test\n"), diz)
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatal(err)
	}
	var hasPayload, hasDiz bool
	for _, f := range r.File {
		switch f.Name {
		case "NODELIST.Z79":
			hasPayload = true
		case "FILE_ID.DIZ":
			hasDiz = true
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				rc.Close()
				t.Fatal(err)
			}
			rc.Close()
			if buf.String() != diz {
				t.Fatalf("FILE_ID.DIZ = %q, want %q", buf.String(), diz)
			}
		}
	}
	if !hasPayload || !hasDiz {
		t.Fatalf("zip contents: payload=%v diz=%v", hasPayload, hasDiz)
	}
}
