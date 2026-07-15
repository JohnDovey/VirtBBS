package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ImportMode selects conversion algorithm.
type ImportMode int

const (
	ImportANSI ImportMode = iota
	ImportASCII
)

var semigraphics = []rune{
	' ',      // 0000
	'\u2598', // 0001 ▘
	'\u259d', // 0010 ▝
	'\u2580', // 0011 ▀
	'\u2596', // 0100 ▖
	'\u258c', // 0101 ▌
	'\u259e', // 0110 ▞
	'\u259b', // 0111 ▛
	'\u2597', // 1000 ▗
	'\u259a', // 1001 ▚
	'\u2590', // 1010 ▐
	'\u259c', // 1011 ▜
	'\u2584', // 1100 ▄
	'\u2599', // 1101 ▙
	'\u259f', // 1110 ▟
	'\u2588', // 1111 █
}

var asciiRamp = []rune(" .:-=+*#%@")

type rgb8 struct{ R, G, B uint8 }

func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// ConvertImage builds a canvas from an image.
// widthCells is the target character width; height follows aspect (~2:1 cells).
func ConvertImage(img image.Image, mode ImportMode, widthCells int) *Canvas {
	if widthCells < 8 {
		widthCells = 8
	}
	if widthCells > maxCols {
		widthCells = maxCols
	}
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw < 1 || ih < 1 {
		return NewCanvas(widthCells, 1)
	}

	switch mode {
	case ImportASCII:
		// One sample per cell; aspect: char ~2 high → sample height = widthCells * (ih/iw) / 2
		hCells := int(math.Round(float64(ih) / float64(iw) * float64(widthCells) / 2.0))
		if hCells < 1 {
			hCells = 1
		}
		if hCells > maxRows {
			hCells = maxRows
		}
		resized := resizeImage(img, widthCells, hCells)
		c := NewCanvas(widthCells, hCells)
		for y := 0; y < hCells; y++ {
			for x := 0; x < widthCells; x++ {
				rr, gg, bb, _ := resized.At(x, y).RGBA()
				r, g, b := uint8(rr>>8), uint8(gg>>8), uint8(bb>>8)
				lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
				idx := int(lum / 255.0 * float64(len(asciiRamp)-1))
				if idx < 0 {
					idx = 0
				}
				if idx >= len(asciiRamp) {
					idx = len(asciiRamp) - 1
				}
				c.Set(x, y, Cell{Ch: asciiRamp[idx], FG: classicFG(7), BG: classicBG(0)})
			}
		}
		return c

	default: // ImportANSI HBFS
		// Sample 2×2 pixels per cell → resize to widthCells*2 by heightCells*2
		hCells := int(math.Round(float64(ih) / float64(iw) * float64(widthCells)))
		if hCells < 1 {
			hCells = 1
		}
		if hCells > maxRows {
			hCells = maxRows
		}
		pw, ph := widthCells*2, hCells*2
		resized := resizeImage(img, pw, ph)
		c := NewCanvas(widthCells, hCells)
		for cy := 0; cy < hCells; cy++ {
			for cx := 0; cx < widthCells; cx++ {
				px := cx * 2
				py := cy * 2
				pix := [4]rgb8{
					pixelRGB(resized, px, py),
					pixelRGB(resized, px+1, py),
					pixelRGB(resized, px, py+1),
					pixelRGB(resized, px+1, py+1),
				}
				pat, fore, back := optimizeTile(pix)
				c.Set(cx, cy, Cell{
					Ch: semigraphics[pat],
					FG: rgbColor(fore.R, fore.G, fore.B),
					BG: rgbColor(back.R, back.G, back.B),
				})
			}
		}
		return c
	}
}

func pixelRGB(img image.Image, x, y int) rgb8 {
	b := img.Bounds()
	if x < b.Min.X {
		x = b.Min.X
	}
	if y < b.Min.Y {
		y = b.Min.Y
	}
	if x >= b.Max.X {
		x = b.Max.X - 1
	}
	if y >= b.Max.Y {
		y = b.Max.Y - 1
	}
	r, g, b32, _ := img.At(x, y).RGBA()
	return rgb8{uint8(r >> 8), uint8(g >> 8), uint8(b32 >> 8)}
}

func resizeImage(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func optimizeTile(pix [4]rgb8) (pattern int, fore, back rgb8) {
	bestErr := int64(math.MaxInt64)
	bestP := 0
	var bestF, bestB rgb8
	for p := 0; p < 8; p++ {
		var cnt [2]int
		var sum [2][3]int
		for b := 0; b < 4; b++ {
			k := (p >> b) & 1
			cnt[k]++
			sum[k][0] += int(pix[b].R)
			sum[k][1] += int(pix[b].G)
			sum[k][2] += int(pix[b].B)
		}
		var avg [2]rgb8
		for k := 0; k < 2; k++ {
			if cnt[k] > 0 {
				avg[k] = rgb8{
					uint8(sum[k][0] / cnt[k]),
					uint8(sum[k][1] / cnt[k]),
					uint8(sum[k][2] / cnt[k]),
				}
			}
		}
		var err int64
		for b := 0; b < 4; b++ {
			k := (p >> b) & 1
			err += colorDiff(pix[b], avg[k])
		}
		if err < bestErr {
			bestErr = err
			bestP = p
			bestB = avg[0]
			bestF = avg[1]
		}
	}
	return bestP, bestF, bestB
}

func colorDiff(a, b rgb8) int64 {
	dr := int64(a.R) - int64(b.R)
	dg := int64(a.G) - int64(b.G)
	db := int64(a.B) - int64(b.B)
	return dr*dr + dg*dg + db*db
}

// PrefillSauceForImport updates sauce metadata after an image import.
func PrefillSauceForImport(s *Sauce, imgPath string, c *Canvas, mode ImportMode) {
	if !s.Present {
		*s = NewSauce()
	}
	base := filepath.Base(imgPath)
	title := strings.TrimSuffix(base, filepath.Ext(base))
	if len(title) > 35 {
		title = title[:35]
	}
	s.Title = title
	s.TInfo1 = uint16(c.Cols)
	s.TInfo2 = uint16(c.Rows)
	s.DataType = 1
	if mode == ImportASCII {
		s.FileType = 0
	} else {
		s.FileType = 1
	}
	modeName := "ANSI truecolor"
	if mode == ImportASCII {
		modeName = "ASCII"
	}
	line := fmt.Sprintf("Imported from %s; AnsiEdit %s; mode=%s", base, Version, modeName)
	if len(line) > comntLineLen {
		line = line[:comntLineLen]
	}
	s.CommentLines = []string{line}
	s.Comments = 1
	s.Date = formatSauceDate()
}

func formatSauceDate() string {
	return time.Now().Format("20060102")
}