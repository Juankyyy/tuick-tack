package canvas

import (
	"regexp"
	"strings"
	"testing"

	"tuick-tack/internal/tui/theme"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestCanvasDimensionsAndText(t *testing.T) {
	c := New(20, 5)
	bg := theme.Get("midnight").BG
	c.Clear(bg)
	c.Text(2, 1, "hello", Fg(theme.Get("midnight").Text))
	if got := c.cells[1*20+2].Ch; got != 'h' {
		t.Fatalf("cell = %q", got)
	}
	c.Text(18, 1, "overflow", Fg(theme.Get("midnight").Text))
	c.Set(-1, 0, 'x', Fg(theme.Get("midnight").Text))
	lines := strings.Split(c.Render(), "\n")
	if len(lines) != 5 {
		t.Fatalf("lines = %d, want 5", len(lines))
	}
	for i, line := range lines {
		plain := ansi.ReplaceAllString(line, "")
		if Width(plain) != 20 {
			t.Fatalf("line %d width = %d (%q)", i, Width(plain), plain)
		}
	}
}

func TestCanvasWideRuneReplaced(t *testing.T) {
	c := New(4, 1)
	c.Set(0, 0, '中', Fg(theme.Get("midnight").Text))
	if got := c.At(0, 0).Ch; got != '?' {
		t.Fatalf("wide rune = %q, want ?", got)
	}
}

func TestGradientRect(t *testing.T) {
	th := theme.Get("midnight")
	c := New(4, 3)
	c.GradientRect(0, 0, 4, 3, th.BG, th.BG2)
	if c.cells[0].BG == c.cells[2*4].BG {
		t.Fatal("gradient rows should differ")
	}
	if !c.cells[0].HasBG {
		t.Fatal("gradient must set the background")
	}
}

func TestFrame(t *testing.T) {
	th := theme.Get("ember")
	c := New(10, 5)
	c.Frame(0, 0, 10, 5, th.Border, th.Accent2, th.Panel)
	if c.cells[0].Ch != '╭' || c.cells[9].Ch != '╮' {
		t.Fatalf("corners = %q %q", c.cells[0].Ch, c.cells[9].Ch)
	}
	if c.cells[4*10].Ch != '╰' || c.cells[4*10+9].Ch != '╯' {
		t.Fatalf("bottom corners wrong")
	}
	if c.cells[2*10+2].BG != th.Panel {
		t.Fatal("interior not filled with panel")
	}
}

func TestDim(t *testing.T) {
	th := theme.Get("midnight")
	c := New(2, 1)
	c.Clear(th.BG)
	c.Text(0, 0, "a", Full(th.Text, th.Panel))
	c.Dim(0.5)
	got := c.At(0, 0)
	if got.FG.R >= th.Text.R || got.BG.R >= th.Panel.R {
		t.Fatalf("dim did not darken cell: %+v", got)
	}
	if got.Ch != 'a' {
		t.Fatalf("dim changed the glyph: %q", got.Ch)
	}
}

func TestResize(t *testing.T) {
	c := New(2, 2)
	c.Set(0, 0, 'a', Fg(theme.Get("midnight").Text))
	c.Resize(4, 3)
	if c.W != 4 || c.H != 3 || c.cells[0].Ch != 'a' {
		t.Fatalf("resize lost content: %+v", c.cells[0])
	}
}

func TestSingleCell(t *testing.T) {
	if !SingleCell('a') || !SingleCell('·') || !SingleCell('█') {
		t.Fatal("expected single-cell runes")
	}
	if SingleCell('中') {
		t.Fatal("CJK rune should not be single-cell")
	}
}

func TestRenderNoTrailingNewline(t *testing.T) {
	c := New(3, 2)
	c.Clear(theme.Get("midnight").BG)
	s := c.Render()
	if strings.HasSuffix(s, "\n") {
		t.Fatal("render should not end with a newline")
	}
	if !strings.Contains(s, "\x1b[38;2;") {
		t.Fatal("render should use truecolor")
	}
}
