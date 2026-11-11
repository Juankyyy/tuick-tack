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
