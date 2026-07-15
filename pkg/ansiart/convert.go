package ansiart

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Mode selects the conversion style.
type Mode string

const (
	ModeANSI  Mode = "ansi"
	ModeASCII Mode = "ascii"
)

// Options controls conversion.
type Options struct {
	Mode   Mode
	Width  int // character columns (default 80)
	Title  string
	Author string
	Source string // original filename for SAUCE comment
	Version string
}

type rgb8 struct{ r, g, b uint8 }

var semigraphics = []string{
	" ",      // 0000
	"\u2598", // 0001 ▘
	"\u259d", // 0010 ▝
	"\u2580", // 0011 ▀
	"\u2596", // 0100 ▖
	"\u258c", // 0101 ▌
	"\u259e", // 0110 ▞
	"\u259b", // 0111 ▛
	"\u2597", // 1000 ▗
	"\u259a", // 1001 ▚
	"\u2590", // 1010 ▐
	"\u259c", // 1011 ▜
	"\u2584", // 1100 ▄
	"\u2599", // 1101 ▙
	"\u259f", // 1110 ▟
	"\u2588", // 1111 █
}

const asciiRamp = " .:-=+*#%@"

// ConvertFile loads an image from path and returns art bytes (with SAUCE) plus dimensions.
func ConvertFile(path string, opt Options) (art []byte, width, height int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}
	return Convert(img, opt)
}

// Convert turns an image into ANSI or ASCII art with SAUCE trailer.
func Convert(img image.Image, opt Options) (art []byte, width, height int, err error) {
	if opt.Width <= 0 {
		opt.Width = 80
	}
	if opt.Mode == "" {
		opt.Mode = ModeANSI
	}
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw < 1 || ih < 1 {
		return nil, 0, 0, fmt.Errorf("empty image")
	}

	// Character cells are ~2× taller than wide; for ANSI each cell is 2×2 pixels.
	cols := opt.Width
	aspect := float64(ih) / float64(iw)
	rows := int(math.Round(aspect * float64(cols) / 2.0))
	if rows < 1 {
		rows = 1
	}

	var body string
	switch opt.Mode {
	case ModeASCII:
		body, width, height = convertASCII(img, cols, rows)
	default:
		body, width, height = convertANSI(img, cols, rows)
	}

	artBytes := []byte(strings.ReplaceAll(body, "\n", "\r\n"))
	sauce := SauceMeta{
		Title:    opt.Title,
		Author:   opt.Author,
		Group:    "VirtBBS AnsiArt",
		Width:    width,
		Height:   height,
		ANSI:     opt.Mode == ModeANSI,
		Comment:  fmt.Sprintf("%s from %s (AnsiArt %s)", strings.ToUpper(string(opt.Mode)), opt.Source, opt.Version),
	}
	if sauce.Title == "" {
		sauce.Title = opt.Source
	}
	return AppendSAUCE(artBytes, sauce), width, height, nil
}

func convertANSI(img image.Image, cols, rows int) (string, int, int) {
	// Work bitmap: 2*cols by 2*rows RGB pixels
	pw, ph := cols*2, rows*2
	resized := resizeRGBA(img, pw, ph)
	var b strings.Builder
	for y := 0; y < ph; y += 2 {
		for x := 0; x < pw; x += 2 {
			pat, fg, bg := optimizeAt(resized, x, y)
			b.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm%s",
				fg.r, fg.g, fg.b, bg.r, bg.g, bg.b, semigraphics[pat]))
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String(), cols, rows
}

func convertASCII(img image.Image, cols, rows int) (string, int, int) {
	resized := resizeRGBA(img, cols, rows)
	ramp := []rune(asciiRamp)
	n := len(ramp)
	var b strings.Builder
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			c := resized.RGBAAt(x, y)
			// luminance
			lum := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255.0
			idx := int(lum * float64(n-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= n {
				idx = n - 1
			}
			b.WriteRune(ramp[idx])
		}
		b.WriteByte('\n')
	}
	return b.String(), cols, rows
}

func resizeRGBA(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func optimizeAt(img *image.RGBA, x, y int) (pattern int, fore, back rgb8) {
	offsets := [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
	pix := [4]rgb8{}
	for i, o := range offsets {
		c := img.RGBAAt(x+o[0], y+o[1])
		pix[i] = rgb8{c.R, c.G, c.B}
	}
	bestErr := int64(math.MaxInt64)
	bestPat := 0
	var bestF, bestB rgb8
	for p := 0; p < 8; p++ {
		var sum [2][3]int
		var cnt [2]int
		for b := 0; b < 4; b++ {
			k := (p >> b) & 1
			cnt[k]++
			sum[k][0] += int(pix[b].r)
			sum[k][1] += int(pix[b].g)
			sum[k][2] += int(pix[b].b)
		}
		var avg [2]rgb8
		for k := 0; k < 2; k++ {
			if cnt[k] == 0 {
				continue
			}
			avg[k] = rgb8{
				uint8(sum[k][0] / cnt[k]),
				uint8(sum[k][1] / cnt[k]),
				uint8(sum[k][2] / cnt[k]),
			}
		}
		var err int64
		for b := 0; b < 4; b++ {
			k := (p >> b) & 1
			err += colorDiff(pix[b], avg[k])
		}
		if err < bestErr {
			bestErr = err
			bestPat = p
			bestB = avg[0]
			bestF = avg[1]
		}
	}
	return bestPat, bestF, bestB
}

func colorDiff(a, b rgb8) int64 {
	dr := int64(a.r) - int64(b.r)
	dg := int64(a.g) - int64(b.g)
	db := int64(a.b) - int64(b.b)
	return dr*dr + dg*dg + db*db
}

// StripSAUCEForDisplay returns art bytes without the SAUCE trailer for terminal preview.
func StripSAUCEForDisplay(data []byte) []byte {
	if Meta, ok := ReadSAUCE(data); ok {
		_ = Meta
		// art ends at first 0x1A before SAUCE
		if i := findEOFBeforeSauce(data); i >= 0 {
			return data[:i]
		}
	}
	return data
}

func findEOFBeforeSauce(data []byte) int {
	if len(data) < 128 {
		return -1
	}
	rec := data[len(data)-128:]
	if string(rec[0:7]) != "SAUCE00" {
		return -1
	}
	// search back for 0x1A
	for i := len(data) - 129; i >= 0; i-- {
		if data[i] == 0x1a {
			return i
		}
	}
	return -1
}
