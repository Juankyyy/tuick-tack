// Command gen-installer draws the branded images used by the Inno Setup
// wizard. The countdown uses the same 5x7 glyphs as the TUI, so the installer
// and the app share one visual language.
//
//go:generate go run . -root ../..
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"tuick-tack/internal/tui/bigfont"
	"tuick-tack/internal/tui/theme"
)

const (
	largeW = 164
	largeH = 314
	smallW = 55
	smallH = 55
)

func main() {
	root := flag.String("root", "../..", "repository root")
	flag.Parse()

	pal := brandPalette()
	icon, err := loadIcon(filepath.Join(*root, "assets", "icon.png"))
	if err != nil {
		fail(err)
	}

	large := image.NewRGBA(image.Rect(0, 0, largeW, largeH))
	drawBackground(large, pal)
	drawDots(large, pal)
	placeIcon(large, icon, 34, 34, 96)
	drawClock(large, 24, 158, "12:30", 4, pal)
	drawWordmark(large, 22, 222, "TUICK-TACK", 2, pal)
	drawAccent(large, 22, 258, 120, pal.Accent)

	if err := writePNG(filepath.Join(*root, "installer", "wizard-large.png"), large); err != nil {
		fail(err)
	}

	small := image.NewRGBA(image.Rect(0, 0, smallW, smallH))
	drawBackground(small, pal)
	placeIcon(small, icon, 7, 7, 41)
	if err := writePNG(filepath.Join(*root, "installer", "wizard-small.png"), small); err != nil {
		fail(err)
	}
	fmt.Println("gen-installer: wrote installer/wizard-large.png and installer/wizard-small.png")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-installer:", err)
	os.Exit(1)
}

// brandPalette is the original Midnight branding used by the installer art.
// It is intentionally fixed: the committed wizard images and the app icon keep
// the original look even though the default theme is now Catppuccin.
func brandPalette() theme.Theme {
	return theme.Theme{
		BG:      theme.RGBA("#070B14"),
		BG2:     theme.RGBA("#0B1524"),
		Panel:   theme.RGBA("#0C1524"),
		Border:  theme.RGBA("#1E3150"),
		Text:    theme.RGBA("#E7ECF5"),
		Muted:   theme.RGBA("#6C7A96"),
		Accent:  theme.RGBA("#6EE7B7"),
		Accent2: theme.RGBA("#60A5FA"),
	}
}

func drawBackground(img *image.RGBA, pal theme.Theme) {
	top := pal.BG2
	bottom := theme.Scale(pal.BG, 0.7)
	for y := 0; y < img.Bounds().Dy(); y++ {
		t := 0.0
		if img.Bounds().Dy() > 1 {
			t = float64(y) / float64(img.Bounds().Dy()-1)
		}
		row := theme.Lerp(top, bottom, t)
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, row)
		}
	}
}

func drawDots(img *image.RGBA, pal theme.Theme) {
	dot := theme.Lerp(pal.BG, pal.Text, 0.11)
	for y := 12; y < largeH; y += 22 {
		for x := 12; x < largeW; x += 22 {
			if (x+y)%3 == 0 {
				img.SetRGBA(x, y, dot)
			}
		}
	}
}

func drawAccent(img *image.RGBA, x, y, w int, c color.RGBA) {
	for dx := 0; dx < w; dx++ {
		img.SetRGBA(x+dx, y, c)
		img.SetRGBA(x+dx, y+1, theme.Scale(c, 0.55))
	}
}

func drawClock(img *image.RGBA, x, y int, text string, scale int, pal theme.Theme) {
	drawBigText(img, x, y, text, scale, pal.Accent, pal.Accent2, true, pal)
}

func drawWordmark(img *image.RGBA, x, y int, text string, scale int, pal theme.Theme) {
	drawBigText(img, x, y, text, scale, pal.Text, theme.Lerp(pal.Text, pal.Muted, 0.45), false, pal)
}

func drawBigText(img *image.RGBA, x, y int, text string, scale int, top, bottom color.RGBA, shadow bool, pal theme.Theme) {
	step := bigfont.GlyphW*scale + scale
	cursor := x
	for _, r := range text {
		glyph, ok := bigfont.Glyph(r)
		if !ok {
			glyph, _ = bigfont.Glyph('?')
		}
		for row := 0; row < bigfont.GlyphH; row++ {
			t := 0.0
			if bigfont.GlyphH > 1 {
				t = float64(row) / float64(bigfont.GlyphH-1)
			}
			col := theme.Lerp(top, bottom, t)
			for colIdx := 0; colIdx < bigfont.GlyphW; colIdx++ {
				if glyph[row][colIdx] != '#' {
					continue
				}
				if shadow {
					fillRect(img, cursor+colIdx*scale+scale, y+row*scale+scale, scale, scale, theme.Scale(pal.BG, 0.25))
				}
				fillRect(img, cursor+colIdx*scale, y+row*scale, scale, scale, col)
			}
		}
		cursor += step
	}
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			img.SetRGBA(x+dx, y+dy, c)
		}
	}
}

func loadIcon(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func placeIcon(dst *image.RGBA, src image.Image, x, y, size int) {
	sb := src.Bounds()
	for dy := 0; dy < size; dy++ {
		sy := sb.Min.Y + dy*sb.Dy()/size
		for dx := 0; dx < size; dx++ {
			sx := sb.Min.X + dx*sb.Dx()/size
			dst.Set(x+dx, y+dy, src.At(sx, sy))
		}
	}
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
