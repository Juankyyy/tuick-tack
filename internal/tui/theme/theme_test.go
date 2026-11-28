package theme

import (
	"image/color"
	"testing"

	"tuick-tack/internal/config"
)

func TestGetAndCatalog(t *testing.T) {
	for _, name := range config.Themes {
		got := Get(name)
		if got.Name != name {
			t.Errorf("Get(%q).Name = %q", name, got.Name)
		}
		if got.BG == got.Panel || got.Text == got.Muted || got.Accent == got.Accent2 {
			t.Errorf("%q has ambiguous palette fields: %+v", name, got)
		}
		if len(got.Confetti) < 6 {
			t.Errorf("%q has %d confetti colors, want at least 6", name, len(got.Confetti))
		}
	}
	for _, old := range []string{"midnight", "ember", "mono", "nope"} {
		if got := Get(old); got.Name != config.DefaultTheme {
			t.Errorf("Get(%q) = %q, want the default %q", old, got.Name, config.DefaultTheme)
		}
	}
}

func TestPaletteContrast(t *testing.T) {
	for _, name := range config.Themes {
		pal := Get(name)
		checks := []struct {
			label string
			fg    color.RGBA
			bg    color.RGBA
			min   float64
		}{
			{"text/bg", pal.Text, pal.BG, 4.5},
			{"text/panel", pal.Text, pal.Panel, 4.5},
			{"muted/bg", pal.Muted, pal.BG, 4.5},
			{"muted/panel", pal.Muted, pal.Panel, 4.0},
			{"accent/bg", pal.Accent, pal.BG, 4.5},
			{"accent/panel", pal.Accent, pal.Panel, 4.5},
			{"accent fill", pal.BG, pal.Accent, 4.5},
			{"accent2/bg", pal.Accent2, pal.BG, 4.5},
			{"accent2/panel", pal.Accent2, pal.Panel, 4.0},
			{"warn/bg", pal.Warn, pal.BG, 4.5},
			{"warn/panel", pal.Warn, pal.Panel, 4.0},
			{"danger/bg", pal.Danger, pal.BG, 4.5},
			{"danger/panel", pal.Danger, pal.Panel, 4.0},
		}
		for _, c := range checks {
			if got := Contrast(c.fg, c.bg); got < c.min {
				t.Errorf("%s: %s contrast = %.2f, want %.1f", name, c.label, got, c.min)
			}
		}
		for i, c := range pal.Confetti {
			if got := Contrast(c, pal.BG); got < 2.0 {
				t.Errorf("%s: confetti %d %+v contrast = %.2f, want 2.0", name, i, c, got)
			}
		}
	}
}

func TestLerpAndScale(t *testing.T) {
	a := color.RGBA{R: 0, G: 100, B: 200, A: 255}
	b := color.RGBA{R: 100, G: 200, B: 250, A: 255}
	if got := Lerp(a, b, 0); got != a {
		t.Fatalf("Lerp(0) = %+v", got)
	}
	if got := Lerp(a, b, 1); got != b {
		t.Fatalf("Lerp(1) = %+v", got)
	}
	if got := Lerp(a, b, 0.5); got.R != 50 || got.G != 150 || got.B != 225 {
		t.Fatalf("Lerp(0.5) = %+v", got)
	}
	if got := Scale(a, 0); got.R != 0 || got.G != 0 || got.B != 0 {
		t.Fatalf("Scale(0) = %+v", got)
	}
	if got := Scale(a, 1); got != a {
		t.Fatalf("Scale(1) = %+v", got)
	}
}

func TestRGBAParsing(t *testing.T) {
	c := color.RGBA{R: 0x6E, G: 0xE7, B: 0xB7, A: 0xFF}
	if got := RGBA("#6EE7B7"); got != c {
		t.Fatalf("RGBA = %+v", got)
	}
	if got := RGBA("bad"); got != Get(config.DefaultTheme).BG {
		t.Fatalf("bad hex should fall back, got %+v", got)
	}
}

func TestPulseStaysInRange(t *testing.T) {
	a := Get(config.DefaultTheme).Accent
	b := Get(config.DefaultTheme).Danger
	for i := 0.0; i < 10; i += 0.37 {
		c := Pulse(a, b, i)
		if c.A != 0xFF {
			t.Fatalf("alpha = %d", c.A)
		}
	}
}

func TestHSL(t *testing.T) {
	tests := []struct {
		h, s, l float64
		want    color.RGBA
	}{
		{0, 1, 0.5, color.RGBA{R: 0xFF, G: 0, B: 0, A: 0xFF}},
		{1.0 / 3, 1, 0.5, color.RGBA{R: 0, G: 0xFF, B: 0, A: 0xFF}},
		{2.0 / 3, 1, 0.5, color.RGBA{R: 0, G: 0, B: 0xFF, A: 0xFF}},
		{-0.5, 1, 0.5, color.RGBA{R: 0, G: 0xFF, B: 0xFF, A: 0xFF}},
		{0.5, 0, 0.5, color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}},
		{0.5, 1, 0, color.RGBA{A: 0xFF}},
	}
	for _, tt := range tests {
		if got := HSL(tt.h, tt.s, tt.l); got != tt.want {
			t.Errorf("HSL(%v, %v, %v) = %+v, want %+v", tt.h, tt.s, tt.l, got, tt.want)
		}
	}
}

func TestHSLPastelStaysMuted(t *testing.T) {
	for i := 0.0; i < 1; i += 0.05 {
		c := HSL(0.8*i, 0.52, 0.68)
		spread := maxChannel(c) - minChannel(c)
		if spread > 140 {
			t.Fatalf("hue %v is too saturated: %+v (spread %d)", i, c, spread)
		}
		if c.A != 0xFF {
			t.Fatalf("alpha = %d", c.A)
		}
	}
}

func TestContrastAndInk(t *testing.T) {
	white := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	black := color.RGBA{A: 0xFF}
	if got := Contrast(white, black); got < 20.9 || got > 21.1 {
		t.Fatalf("white/black contrast = %.2f, want 21", got)
	}
	if got := Contrast(white, white); got != 1 {
		t.Fatalf("white/white contrast = %.2f, want 1", got)
	}
	if Luminance(white) < 0.99 || Luminance(black) != 0 {
		t.Fatalf("luminance white=%.3f black=%.3f", Luminance(white), Luminance(black))
	}

	// Dark colors that already meet the target are untouched.
	dark := color.RGBA{R: 0x20, G: 0x30, B: 0x40, A: 0xFF}
	if got := Ink(dark, white, 4.5); got != dark {
		t.Fatalf("Ink changed a readable color: %+v", got)
	}
	// A pastel accent is darkened just enough to meet the target.
	accent := Get(config.DefaultTheme).Accent
	ink := Ink(accent, white, 4.5)
	if got := Contrast(ink, white); got < 4.5 {
		t.Fatalf("ink contrast = %.2f, want at least 4.5", got)
	}
	if ink == accent {
		t.Fatal("Ink returned the unreadable accent unchanged")
	}
	// A target even black cannot meet falls back to black.
	if got := Ink(color.RGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xFF}, black, 4.5); got != black {
		t.Fatalf("Ink on black = %+v, want black", got)
	}
}

func maxChannel(c color.RGBA) int {
	return int(maxInt(int(c.R), maxInt(int(c.G), int(c.B))))
}

func minChannel(c color.RGBA) int {
	return int(minInt(int(c.R), minInt(int(c.G), int(c.B))))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
