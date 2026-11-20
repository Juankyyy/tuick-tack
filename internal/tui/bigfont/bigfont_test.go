package bigfont

import (
	"regexp"
	"strings"
	"testing"

	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestWidth(t *testing.T) {
	tests := []struct {
		text  string
		scale int
		want  int
	}{
		{"00:00", 1, 29},
		{"00:00", 2, 58},
		{"TIME'S UP!", 1, 59},
		{"", 1, 0},
	}
	for _, tt := range tests {
		if got := Width(tt.text, tt.scale); got != tt.want {
			t.Errorf("Width(%q, %d) = %d, want %d", tt.text, tt.scale, got, tt.want)
		}
	}
}

func TestWidthProportional(t *testing.T) {
	tests := []struct {
		text  string
		scale int
		want  int
	}{
		{"TIME'S UP!", 2, 92},
		{"FINISHED!", 2, 90},
		{"TIME OUT!", 2, 88},
		{"THE END!", 2, 80},
		{"BINGO!", 2, 58},
		{"DONE!", 2, 50},
		{"I", 2, 6},
		{" ", 2, 4},
		{"", 1, 0},
	}
	for _, tt := range tests {
		if got := WidthProportional(tt.text, tt.scale); got != tt.want {
			t.Errorf("WidthProportional(%q, %d) = %d, want %d", tt.text, tt.scale, got, tt.want)
		}
	}
}

func TestDrawProportionalTrimsBearings(t *testing.T) {
	th := theme.Get("midnight")
	c := canvas.New(100, 10)
	w := Draw(c, 3, 1, "TIME'S UP!", Options{
		Scale: 2, Proportional: true, Top: th.Warn, Bottom: th.Danger,
		Shadow: th.BG, HasShadow: true, Bold: true,
	})
	if w != 92 {
		t.Fatalf("width = %d, want 92", w)
	}
	minX, maxX := c.W, -1
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y).Ch == '█' {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
		}
	}
	if minX != 3 || maxX != 94 {
		t.Fatalf("proportional word spans %d..%d, want 3..94", minX, maxX)
	}
}

func TestDrawProportionalNarrowGlyph(t *testing.T) {
	c := canvas.New(10, 8)
	if w := Draw(c, 0, 0, "I", Options{Scale: 1, Proportional: true}); w != 3 {
		t.Fatalf("width = %d, want 3", w)
	}
	for x := 0; x < c.W; x++ {
		if c.At(x, 0).Ch == '█' && x > 2 {
			t.Fatalf("I ink at column %d, want it trimmed to 0..2", x)
		}
	}
}

func TestDrawFitsAndUsesGradient(t *testing.T) {
	th := theme.Get("midnight")
	c := canvas.New(40, 10)
	w := Draw(c, 2, 1, "04:32", Options{
		Scale: 1, Top: th.Accent, Bottom: th.Accent2,
		Shadow: th.BG, HasShadow: true, Bold: true,
	})
	if w != 29 {
		t.Fatalf("width = %d, want 29", w)
	}
	plain := ansi.ReplaceAllString(c.Render(), "")
	lines := strings.Split(plain, "\n")
	if !strings.Contains(lines[3], "█") {
		t.Fatalf("expected block glyphs, got %q", lines[3])
	}
}

func TestDrawClips(t *testing.T) {
	c := canvas.New(5, 3)
	Draw(c, 4, 2, "12345", Options{Scale: 1})
	// Must not panic and must keep every line 5 cells.
	for _, line := range strings.Split(c.Render(), "\n") {
		if got := canvas.Width(ansi.ReplaceAllString(line, "")); got != 5 {
			t.Fatalf("line width = %d", got)
		}
	}
}

func TestHideColon(t *testing.T) {
	c := canvas.New(20, 8)
	Draw(c, 0, 0, ":", Options{Scale: 1, HideColon: true})
	filled := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y).Ch == '█' {
				filled++
			}
		}
	}
	if filled != 0 {
		t.Fatalf("hidden colon drew %d cells", filled)
	}
}

func TestUnknownRune(t *testing.T) {
	c := canvas.New(10, 8)
	Draw(c, 0, 0, "@", Options{Scale: 1})
	filled := false
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y).Ch == '█' {
				filled = true
			}
		}
	}
	if !filled {
		t.Fatal("unknown rune should fall back to a visible glyph")
	}
}

func TestGlyph(t *testing.T) {
	for _, r := range "TUICK-TACK" {
		g, ok := Glyph(r)
		if !ok {
			t.Fatalf("Glyph(%q) not found", r)
		}
		if len(g) != GlyphH {
			t.Fatalf("Glyph(%q) has %d rows, want %d", r, len(g), GlyphH)
		}
		for row, line := range g {
			if got := len([]rune(line)); got != GlyphW {
				t.Fatalf("Glyph(%q) row %d has width %d, want %d", r, row, got, GlyphW)
			}
		}
	}
	if _, ok := Glyph('@'); ok {
		t.Fatal("Glyph('@') should report a missing glyph")
	}
}
