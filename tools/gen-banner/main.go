// Command gen-banner draws the branding banner into assets/.
//
// The ASCII and the PNG come from the same 5x7 bigfont grid as the countdown,
// so the wordmark and the TUI cannot drift apart. The text file is plain
// Unicode block art (no ANSI); the PNG has a transparent background, uses the
// Ember palette for TUI and the Midnight palette for the rest, and carries the
// countdown's offset shadow.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"tuick-tack/internal/tui/bigfont"
	"tuick-tack/internal/tui/theme"
)

const (
	bannerText = "TUICK-TACK"
	tuiGlyphs  = 3

	// Square cells match the countdown's scale-2 rendering: two terminal
	// columns per glyph column, so the 5x7 glyph keeps its normal proportions.
	cellW = 24
	cellH = 24
	padX  = 24
	padY  = 24

	shadowDX    = 12 // half a cell right, like the scale-2 shadow
	shadowDY    = 24 // one row down
	shadowAlpha = 89 // 35% black

	// The wide ASCII doubles every cell horizontally. Terminal cells are
	// roughly twice as tall as wide, so this makes the text look square too.
	asciiScale = 2
)

type cell struct {
	filled bool
	tui    bool
}

func main() {
	root := repoRoot()
	cells, err := compose(bannerText)
	if err != nil {
		fail(err)
	}

	txtPath := filepath.Join(root, "assets", "banner.txt")
	if err := os.WriteFile(txtPath, []byte(renderASCII(cells, asciiScale)), 0o644); err != nil {
		fail(err)
	}
	compactPath := filepath.Join(root, "assets", "banner-compact.txt")
	if err := os.WriteFile(compactPath, []byte(renderASCII(cells, 1)), 0o644); err != nil {
		fail(err)
	}

	img := renderPNG(cells)
	pngPath := filepath.Join(root, "assets", "banner.png")
	if err := writePNG(pngPath, img); err != nil {
		fail(err)
	}
	fmt.Printf("gen-banner: wrote assets/banner.txt, assets/banner-compact.txt and assets/banner.png (%dx%d)\n",
		img.Bounds().Dx(), img.Bounds().Dy())
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-banner:", err)
	os.Exit(1)
}

// compose lays the text out on the bigfont grid and marks the first tuiGlyphs
// glyphs as the TUI part.
func compose(text string) ([][]cell, error) {
	runes := []rune(strings.ToUpper(text))
	if len(runes) == 0 {
		return nil, fmt.Errorf("empty banner text")
	}
	cols := len(runes)*bigfont.GlyphW + (len(runes) - 1)
	cells := make([][]cell, bigfont.GlyphH)
	for row := range cells {
		cells[row] = make([]cell, cols)
	}

	for i, r := range runes {
		g, ok := bigfont.Glyph(r)
		if !ok {
			return nil, fmt.Errorf("bigfont has no glyph for %q", r)
		}
		x := i * (bigfont.GlyphW + 1)
		for row := 0; row < bigfont.GlyphH; row++ {
			line := []rune(g[row])
			if len(line) != bigfont.GlyphW {
				return nil, fmt.Errorf("glyph %q row %d has width %d", r, row, len(line))
			}
			for col, ch := range line {
				if ch == '#' {
					cells[row][x+col] = cell{filled: true, tui: i < tuiGlyphs}
				}
			}
		}
	}
	return cells, nil
}

// renderASCII keeps the TUI solid and shades the rest so the acronym still
// reads without colour. scale repeats every cell horizontally; 1 is the raw
// 5x7 grid and 2 matches the countdown's wide layout.
func renderASCII(cells [][]cell, scale int) string {
	if scale < 1 {
		scale = 1
	}
	var b strings.Builder
	for _, row := range cells {
		line := make([]rune, 0, len(row)*scale)
		for _, c := range row {
			ch := ' '
			switch {
			case c.filled && c.tui:
				ch = '█'
			case c.filled:
				ch = '▓'
			}
			for i := 0; i < scale; i++ {
				line = append(line, ch)
			}
		}
		b.WriteString(strings.TrimRight(string(line), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderPNG paints one flat band per glyph row, mirroring bigfont.Draw. The
// shadow is a single translucent copy offset by one cell.
func renderPNG(cells [][]cell) *image.NRGBA {
	rows := len(cells)
	cols := 0
	if rows > 0 {
		cols = len(cells[0])
	}
	img := image.NewNRGBA(image.Rect(0, 0,
		padX*2+cols*cellW+shadowDX,
		padY*2+rows*cellH+shadowDY))

	shadow := color.NRGBA{A: shadowAlpha}
	for y, row := range cells {
		for x, c := range row {
			if !c.filled {
				continue
			}
			fillRect(img,
				padX+x*cellW+shadowDX, padY+y*cellH+shadowDY,
				cellW, cellH, shadow)
		}
	}

	ember := theme.Get("ember")
	midnight := theme.Get("midnight")
	for y, row := range cells {
		t := 0.0
		if rows > 1 {
			t = float64(y) / float64(rows-1)
		}
		tuiColor := toNRGBA(theme.Lerp(ember.Accent, ember.Accent2, t))
		restColor := toNRGBA(theme.Lerp(midnight.Accent, midnight.Accent2, t))
		for x, c := range row {
			if !c.filled {
				continue
			}
			col := restColor
			if c.tui {
				col = tuiColor
			}
			fillRect(img, padX+x*cellW, padY+y*cellH, cellW, cellH, col)
		}
	}
	return img
}

func fillRect(img *image.NRGBA, x, y, w, h int, c color.NRGBA) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			img.SetNRGBA(xx, yy, c)
		}
	}
}

func toNRGBA(c color.RGBA) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func writePNG(path string, img image.Image) error {
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

// repoRoot walks up from the working directory until it finds go.mod.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
