package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSauceRoundTrip(t *testing.T) {
	art := []byte("\x1b[1;31mHello\r\n\x1b[0m")
	s := NewSauce()
	s.Title = "Test Title"
	s.Author = "Tester"
	s.Group = "VirtBBS"
	s.TInfo1 = 80
	s.TInfo2 = 25
	s.CommentLines = []string{"line one", "line two padded"}
	out := AppendSauce(art, s)
	if len(out) < sauceLen {
		t.Fatalf("output too short: %d", len(out))
	}
	rec := out[len(out)-sauceLen:]
	if string(rec[0:7]) != sauceID {
		t.Fatalf("missing SAUCE00 id: %q", rec[0:7])
	}
	gotArt, got := SplitSauce(out)
	if !bytes.Equal(gotArt, art) {
		t.Fatalf("art mismatch:\n%x\n%x", gotArt, art)
	}
	if !got.Present {
		t.Fatal("sauce not present")
	}
	if got.Title != "Test Title" {
		t.Errorf("title=%q", got.Title)
	}
	if got.Author != "Tester" {
		t.Errorf("author=%q", got.Author)
	}
	if got.FileSize != uint32(len(art)) {
		t.Errorf("filesize=%d want %d", got.FileSize, len(art))
	}
	if len(got.CommentLines) != 2 {
		t.Fatalf("comments=%d", len(got.CommentLines))
	}
	if got.CommentLines[0] != "line one" {
		t.Errorf("comnt0=%q", got.CommentLines[0])
	}
	if got.Comments != 2 {
		t.Errorf("Comments field=%d", got.Comments)
	}
}

func TestANSIRoundTripClassic(t *testing.T) {
	c := NewCanvas(10, 3)
	c.Set(0, 0, Cell{Ch: 'A', FG: classicFG(9), BG: classicBG(0)})
	c.Set(1, 0, Cell{Ch: 'B', FG: classicFG(10), BG: classicBG(4)})
	c.Set(0, 1, Cell{Ch: '█', FG: classicFG(15), BG: classicBG(0)})
	raw := WriteANSI(c, false)
	parsed := ParseANSI(string(raw), 10, 3)
	if parsed.Get(0, 0).Ch != 'A' {
		t.Errorf("0,0 ch=%q", parsed.Get(0, 0).Ch)
	}
	if parsed.Get(1, 0).Ch != 'B' {
		t.Errorf("1,0 ch=%q", parsed.Get(1, 0).Ch)
	}
}

func TestANSITruecolorParse(t *testing.T) {
	text := "\x1b[38;2;10;20;30;48;2;1;2;3mX\x1b[0m\r\n"
	c := ParseANSI(text, 8, 2)
	cell := c.Get(0, 0)
	if cell.Ch != 'X' {
		t.Fatalf("ch=%q", cell.Ch)
	}
	if !cell.FG.True || cell.FG.R != 10 || cell.FG.G != 20 || cell.FG.B != 30 {
		t.Errorf("fg=%+v", cell.FG)
	}
	if !cell.BG.True || cell.BG.R != 1 || cell.BG.G != 2 || cell.BG.B != 3 {
		t.Errorf("bg=%+v", cell.BG)
	}
}

func TestLoadSaveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ans")
	c := NewCanvas(40, 5)
	c.Set(2, 1, Cell{Ch: 'Z', FG: classicFG(14), BG: classicBG(1)})
	s := NewSauce()
	s.Title = "RoundTrip"
	s.CommentLines = []string{"hello sauce"}
	if err := SaveFile(path, c, s, false); err != nil {
		t.Fatal(err)
	}
	got, gs, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Get(2, 1).Ch != 'Z' {
		t.Errorf("cell=%q", got.Get(2, 1).Ch)
	}
	if !gs.Present || gs.Title != "RoundTrip" {
		t.Errorf("sauce=%+v", gs)
	}
	if len(gs.CommentLines) != 1 || gs.CommentLines[0] != "hello sauce" {
		t.Errorf("comments=%v", gs.CommentLines)
	}
}

func TestConvertASCII(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 12), uint8(y * 12), 128, 255})
		}
	}
	c := ConvertImage(img, ImportASCII, 10)
	if c.Cols != 10 {
		t.Fatalf("cols=%d", c.Cols)
	}
	if c.Rows < 1 {
		t.Fatal("no rows")
	}
}

func TestConvertANSI(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{255, uint8(x * 6), uint8(y * 6), 255})
		}
	}
	c := ConvertImage(img, ImportANSI, 10)
	if c.Cols != 10 {
		t.Fatalf("cols=%d", c.Cols)
	}
	cell := c.Get(0, 0)
	if !cell.FG.True || !cell.BG.True {
		t.Errorf("expected truecolor cell %+v", cell)
	}
}

func TestImportPNGFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dot.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{0, 200, 0, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	loaded, err := LoadImage(path)
	if err != nil {
		t.Fatal(err)
	}
	c := ConvertImage(loaded, ImportASCII, 8)
	if c.Cols != 8 {
		t.Fatalf("cols=%d", c.Cols)
	}
	var sauce Sauce
	PrefillSauceForImport(&sauce, path, c, ImportASCII)
	if !sauce.Present || sauce.Title != "dot" {
		t.Errorf("sauce=%+v", sauce)
	}
}

func TestPCBAnsiExpand(t *testing.T) {
	s := expandPCBAnsi("[1;36mHi[0m")
	if !bytes.Contains([]byte(s), []byte{0x1b, '['}) {
		t.Fatalf("expand failed: %q", s)
	}
	c := ParseANSI(s, 10, 2)
	if c.Get(0, 0).Ch != 'H' {
		t.Errorf("ch=%q", c.Get(0, 0).Ch)
	}
}

func TestLoadLogonANS(t *testing.T) {
	path := "../display/LOGON.ANS"
	if _, err := os.Stat(path); err != nil {
		t.Skip("LOGON.ANS not present")
	}
	c, _, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Cols < 40 || c.Rows < 1 {
		t.Fatalf("unexpected size %dx%d", c.Cols, c.Rows)
	}
}

func TestCP437Bridge(t *testing.T) {
	// 0xDB alone is not valid UTF-8 (incomplete sequence) → CP437 path.
	raw := []byte{0xDB, 'A', 0xB1}
	s := decodeANSBytes(raw)
	runes := []rune(s)
	if len(runes) != 3 || runes[0] != '█' || runes[1] != 'A' || runes[2] != '▒' {
		t.Fatalf("decoded=%q runes=%v", s, runes)
	}
	// Real UTF-8 box-drawing must pass through.
	utf := "╔═╗"
	if decodeANSBytes([]byte(utf)) != utf {
		t.Fatalf("utf8 passthrough failed")
	}
}
