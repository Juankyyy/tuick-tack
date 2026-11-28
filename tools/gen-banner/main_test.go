package main

import (
	"image/color"
	"strings"
	"testing"

	"tuick-tack/internal/tui/bigfont"
)

func TestComposeShape(t *testing.T) {
	cells, err := compose(bannerText)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != bigfont.GlyphH {
		t.Fatalf("rows = %d, want %d", len(cells), bigfont.GlyphH)
	}
	if got := len(cells[0]); got != 59 {
		t.Fatalf("cols = %d, want 59", got)
	}
	if c := cells[0][0]; !c.filled || !c.tui {
		t.Fatalf("T top-left = %+v, want filled TUI", c)
	}
	if c := cells[0][17]; c.filled {
		t.Fatal("column 17 should be the gap between I and C")
	}
	if c := cells[0][19]; !c.filled || c.tui {
		t.Fatalf("C top-left = %+v, want filled non-TUI", c)
	}
}

func TestComposeRejectsMissingGlyph(t *testing.T) {
	if _, err := compose("TUICK-TACK@"); err == nil {
		t.Fatal("expected an error for a glyph missing from bigfont")
	}
}

func TestASCII(t *testing.T) {
	cells, err := compose(bannerText)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		scale int
		width int
	}{
		{"wide", asciiScale, 118},
		{"compact", 1, 59},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			art := renderASCII(cells, tt.scale)
			if !strings.HasSuffix(art, "\n") {
				t.Fatal("ASCII should end with a newline")
			}
			lines := strings.Split(strings.TrimSuffix(art, "\n"), "\n")
			if len(lines) != bigfont.GlyphH {
				t.Fatalf("lines = %d, want %d", len(lines), bigfont.GlyphH)
			}
			if !strings.HasPrefix(lines[0], strings.Repeat("█", 5*tt.scale)) {
				t.Fatalf("first line = %q, want a solid TUI T", lines[0])
			}
			if !strings.Contains(lines[0], strings.Repeat("▓", tt.scale)) {
				t.Fatalf("first line = %q, want a shaded rest", lines[0])
			}
			for i, line := range lines {
				if got := len([]rune(line)); got > tt.width {
					t.Fatalf("line %d has width %d, want <= %d", i, got, tt.width)
				}
				if strings.HasSuffix(line, " ") {
					t.Fatalf("line %d has trailing spaces", i)
				}
			}
		})
	}
}

func TestPNG(t *testing.T) {
	cells, err := compose(bannerText)
	if err != nil {
		t.Fatal(err)
	}
	img := renderPNG(cells)
	bounds := img.Bounds()
	if bounds.Dx() != 1476 || bounds.Dy() != 240 {
		t.Fatalf("PNG size = %dx%d, want 1476x240", bounds.Dx(), bounds.Dy())
	}
	if c := img.NRGBAAt(0, 0); c.A != 0 {
		t.Fatalf("top-left alpha = %d, want 0", c.A)
	}
	if c := img.NRGBAAt(bounds.Dx()-1, bounds.Dy()-1); c.A != 0 {
		t.Fatalf("bottom-right alpha = %d, want 0", c.A)
	}

	ember := brandEmber()
	tuiTop := img.NRGBAAt(padX+12, padY+12)
	wantTUI := color.NRGBA{
		R: ember.Accent.R, G: ember.Accent.G, B: ember.Accent.B, A: ember.Accent.A,
	}
	if tuiTop != wantTUI {
		t.Fatalf("TUI top = %+v, want %+v", tuiTop, wantTUI)
	}

	midnight := brandMidnight()
	restTop := img.NRGBAAt(padX+19*cellW+12, padY+12)
	wantRest := color.NRGBA{
		R: midnight.Accent.R, G: midnight.Accent.G, B: midnight.Accent.B, A: midnight.Accent.A,
	}
	if restTop != wantRest {
		t.Fatalf("rest top = %+v, want %+v", restTop, wantRest)
	}

	shadow := img.NRGBAAt(padX+shadowDX, padY+shadowDY)
	if shadow.A != shadowAlpha || shadow.R != 0 || shadow.G != 0 || shadow.B != 0 {
		t.Fatalf("shadow = %+v, want black with alpha %d", shadow, shadowAlpha)
	}
}
