package ansiart

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendAndReadSAUCE(t *testing.T) {
	art := []byte("hello ansi\r\n")
	out := AppendSAUCE(art, SauceMeta{
		Title: "TestTitle", Author: "Alice", Group: "VirtBBS AnsiArt",
		Width: 80, Height: 25, ANSI: true, Comment: "unit test",
	})
	if len(out) < 128+1+len(art) {
		t.Fatalf("too short: %d", len(out))
	}
	if out[len(art)] != 0x1a {
		t.Fatal("missing EOF")
	}
	rec := out[len(out)-128:]
	if string(rec[0:7]) != "SAUCE00" {
		t.Fatalf("bad id %q", rec[0:7])
	}
	info, ok := ReadSAUCE(out)
	if !ok {
		t.Fatal("ReadSAUCE failed")
	}
	if info.Title != "TestTitle" || info.Author != "Alice" {
		t.Fatalf("meta %+v", info)
	}
	if info.Width != 80 || info.Height != 25 || !info.ANSI {
		t.Fatalf("dims/type %+v", info)
	}
	if info.Comment == "" {
		t.Fatal("expected comment")
	}
	stripped := StripSAUCEForDisplay(out)
	if !bytes.Equal(stripped, art) {
		t.Fatalf("strip got %q want %q", stripped, art)
	}
}

func TestConvertANSIAndASCII(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 8), uint8(y * 8), 128, 255})
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "t.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	art, w, h, err := ConvertFile(path, Options{Mode: ModeANSI, Width: 16, Title: "t", Author: "Bob", Source: "t.png", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if w != 16 || h < 1 {
		t.Fatalf("size %dx%d", w, h)
	}
	if _, ok := ReadSAUCE(art); !ok {
		t.Fatal("ANSI missing SAUCE")
	}
	if !bytes.Contains(StripSAUCEForDisplay(art), []byte("\x1b[38;2;")) {
		t.Fatal("expected truecolor SGR")
	}

	art2, _, _, err := ConvertFile(path, Options{Mode: ModeASCII, Width: 16, Author: "Bob", Source: "t.png", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	info, ok := ReadSAUCE(art2)
	if !ok || info.ANSI {
		t.Fatalf("ASCII sauce %+v ok=%v", info, ok)
	}
}

func TestLibrarySave(t *testing.T) {
	root := t.TempDir()
	lib, err := NewLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "in.png")
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	f, _ := os.Create(src)
	_ = png.Encode(f, img)
	f.Close()
	art := AppendSAUCE([]byte("x\r\n"), SauceMeta{Title: "x", Author: "u", ANSI: true, Width: 1, Height: 1})
	e, err := lib.SaveConversion("User One", "x", src, art, ModeANSI, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(e.ResultPath()); err != nil {
		t.Fatal(err)
	}
	list, err := lib.ListRecent(5)
	if err != nil || len(list) != 1 {
		t.Fatalf("list %v %d", err, len(list))
	}
}
