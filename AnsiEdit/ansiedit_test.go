package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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
	c := ConvertImage(img, ImportASCII, 10, 10)
	if c.Cols > 10 || c.Rows > 10 {
		t.Fatalf("exceeded fit box: %dx%d", c.Cols, c.Rows)
	}
	if c.Cols < 1 || c.Rows < 1 {
		t.Fatal("empty canvas")
	}
}

func TestConvertANSI(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{255, uint8(x * 6), uint8(y * 6), 255})
		}
	}
	c := ConvertImage(img, ImportANSI, 10, 10)
	if c.Cols > 10 || c.Rows > 10 {
		t.Fatalf("exceeded fit box: %dx%d", c.Cols, c.Rows)
	}
	cell := c.Get(0, 0)
	if !cell.FG.True || !cell.BG.True {
		t.Errorf("expected truecolor cell %+v", cell)
	}
}

func TestConvertScalesFullImage(t *testing.T) {
	const W, H = 200, 100
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			img.Set(x, y, color.RGBA{40, 40, 40, 255})
		}
	}
	paint := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, c)
			}
		}
	}
	paint(0, 0, 50, 25, color.RGBA{255, 0, 0, 255})
	paint(W-50, 0, W, 25, color.RGBA{0, 0, 255, 255})
	paint(0, H-25, 50, H, color.RGBA{0, 255, 0, 255})
	paint(W-50, H-25, W, H, color.RGBA{255, 255, 0, 255})

	c := ConvertImage(img, ImportANSI, 40, 25)
	if c.Cols > 40 || c.Rows > 25 {
		t.Fatalf("fit exceeded: %dx%d", c.Cols, c.Rows)
	}
	dominant := func(cell Cell) (r, g, b uint8) {
		fr, fg, fb := cell.FG.RGB()
		br, bg, bb := cell.BG.RGB()
		// Prefer the brighter / more saturated channel pair contribution
		if fr+fg+fb >= br+bg+bb {
			return fr, fg, fb
		}
		return br, bg, bb
	}
	near := func(gotR, gotG, gotB uint8, wantR, wantG, wantB uint8) bool {
		dr := int(gotR) - int(wantR)
		dg := int(gotG) - int(wantG)
		db := int(gotB) - int(wantB)
		return dr*dr+dg*dg+db*db < 80*80
	}
	tlr, tlg, tlb := dominant(c.Get(0, 0))
	trr, trg, trb := dominant(c.Get(c.Cols-1, 0))
	blr, blg, blb := dominant(c.Get(0, c.Rows-1))
	brr, brg, brb := dominant(c.Get(c.Cols-1, c.Rows-1))
	if !near(tlr, tlg, tlb, 255, 0, 0) {
		t.Errorf("TL want red got %d,%d,%d", tlr, tlg, tlb)
	}
	if !near(trr, trg, trb, 0, 0, 255) {
		t.Errorf("TR want blue got %d,%d,%d", trr, trg, trb)
	}
	if !near(blr, blg, blb, 0, 255, 0) {
		t.Errorf("BL want green got %d,%d,%d", blr, blg, blb)
	}
	if !near(brr, brg, brb, 255, 255, 0) {
		t.Errorf("BR want yellow got %d,%d,%d", brr, brg, brb)
	}
}

func TestFitCellSize(t *testing.T) {
	c, r := fitCellSize(800, 600, 80, 25)
	if c > 80 || r > 25 {
		t.Fatalf("800x600 → %dx%d exceeds box", c, r)
	}
	// Wide image should use full width more often than height
	c2, r2 := fitCellSize(1600, 400, 80, 25)
	if c2 > 80 || r2 > 25 {
		t.Fatalf("wide → %dx%d", c2, r2)
	}
	_ = c
	_ = r
	_ = c2
	_ = r2
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
	c := ConvertImage(loaded, ImportASCII, 8, 8)
	if c.Cols > 8 || c.Rows > 8 {
		t.Fatalf("size %dx%d", c.Cols, c.Rows)
	}
	var sauce Sauce
	PrefillSauceForImport(&sauce, path, c, ImportASCII)
	if !sauce.Present || sauce.Title != "dot" {
		t.Errorf("sauce=%+v", sauce)
	}
}

func TestUndoStack(t *testing.T) {
	e := NewEditor(nil, NewCanvas(10, 5), "", Sauce{})
	e.cx, e.cy = 2, 1
	e.fg = classicFG(9)
	e.paint('A')
	e.paint('B')
	if e.canvas.Get(2, 1).Ch != 'A' || e.canvas.Get(3, 1).Ch != 'B' {
		t.Fatalf("paint failed")
	}
	e.undoLast()
	if e.canvas.Get(3, 1).Ch != ' ' || e.cx != 3 || e.cy != 1 {
		t.Fatalf("undo B: cell=%q pos=%d,%d", string(e.canvas.Get(3, 1).Ch), e.cx, e.cy)
	}
	e.undoLast()
	if e.canvas.Get(2, 1).Ch != ' ' {
		t.Fatalf("undo A: %q", string(e.canvas.Get(2, 1).Ch))
	}
	// Cap at undoLimit
	e.cx, e.cy = 0, 0
	for i := 0; i < undoLimit+10; i++ {
		e.cx = i % e.canvas.Cols
		e.cy = (i / e.canvas.Cols) % e.canvas.Rows
		e.paint('X')
	}
	if len(e.undo) != undoLimit {
		t.Fatalf("undo depth=%d want %d", len(e.undo), undoLimit)
	}
}

func TestRenderTextAndStamp(t *testing.T) {
	grid := fontBlock.RenderText("Hi", 1)
	if len(grid) != 5 {
		t.Fatalf("rows=%d", len(grid))
	}
	hasInk := false
	for _, row := range grid {
		for _, ch := range row {
			if ch != ' ' {
				hasInk = true
			}
		}
	}
	if !hasInk {
		t.Fatal("empty render")
	}
	big := fontBlock.RenderText("A", 2)
	if len(big) != 10 {
		t.Fatalf("size2 height=%d", len(big))
	}
	e := NewEditor(nil, NewCanvas(40, 20), "", Sauce{})
	e.cx, e.cy = 1, 1
	e.fg = classicFG(14)
	n := e.stampTextGrid(fontMini.RenderText("OK", 1), true)
	if n < 2 {
		t.Fatalf("stamped cells=%d", n)
	}
	e.undoLast()
	if len(e.undo) != 0 {
		t.Fatalf("batch undo left %d", len(e.undo))
	}
}

func TestTrimPathSpaces(t *testing.T) {
	s := strings.TrimSpace("  /tmp/foo.png  ")
	if s != "/tmp/foo.png" {
		t.Fatal(s)
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
	// Ambiguous as UTF-8 (0xDB 0xB0 = U+06F0) but must decode as CP437 █░ for .ANS.
	raw := []byte{0xDB, 0xB0, 'A'}
	s := decodeANSBytes(raw)
	runes := []rune(s)
	if len(runes) != 3 || runes[0] != '█' || runes[1] != '░' || runes[2] != 'A' {
		t.Fatalf("decoded=%q runes=%v", s, runes)
	}
	// Incomplete UTF-8 lead byte alone → CP437.
	raw2 := []byte{0xDB, 'A', 0xB1}
	s2 := decodeANSBytes(raw2)
	r2 := []rune(s2)
	if len(r2) != 3 || r2[0] != '█' || r2[1] != 'A' || r2[2] != '▒' {
		t.Fatalf("decoded2=%q runes=%v", s2, r2)
	}
	// Real UTF-8 box-drawing (3-byte sequences) must pass through.
	utf := "╔═╗"
	if decodeANSBytes([]byte(utf)) != utf {
		t.Fatalf("utf8 passthrough failed")
	}
}
