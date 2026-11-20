// Package bigfont draws the 7x5 block digits used by the countdown hero.
package bigfont

import (
	"image/color"
	"strings"

	"tuick-tack/internal/tui/canvas"
)

// Glyph dimensions.
const (
	GlyphW = 5
	GlyphH = 7
)

// Options controls scale, colors and effects.
type Options struct {
	Scale     int
	Top       color.RGBA
	Bottom    color.RGBA
	Shadow    color.RGBA
	HasShadow bool
	Bold      bool
	HideColon bool
	// Proportional trims each glyph to its ink columns and lets a word space
	// take two columns. The clock stays monospaced so digits do not jitter;
	// celebration words use it to keep the wide scale without the 5-column
	// side bearings.
	Proportional bool
}

var glyphs = map[rune][GlyphH]string{
	'0':  {" ### ", "#   #", "#  ##", "# # #", "##  #", "#   #", " ### "},
	'1':  {"  #  ", " ##  ", "  #  ", "  #  ", "  #  ", "  #  ", " ### "},
	'2':  {" ### ", "#   #", "    #", "   # ", "  #  ", " #   ", "#####"},
	'3':  {"#### ", "    #", "    #", " ### ", "    #", "    #", "#### "},
	'4':  {"   # ", "  ## ", " # # ", "#  # ", "#####", "   # ", "   # "},
	'5':  {"#####", "#    ", "#### ", "    #", "    #", "#   #", " ### "},
	'6':  {"  ## ", " #   ", "#    ", "#### ", "#   #", "#   #", " ### "},
	'7':  {"#####", "    #", "   # ", "  #  ", " #   ", " #   ", " #   "},
	'8':  {" ### ", "#   #", "#   #", " ### ", "#   #", "#   #", " ### "},
	'9':  {" ### ", "#   #", "#   #", " ####", "    #", "   # ", " ##  "},
	'A':  {" ### ", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'B':  {"#### ", "#   #", "#   #", "#### ", "#   #", "#   #", "#### "},
	'C':  {" ### ", "#   #", "#    ", "#    ", "#    ", "#   #", " ### "},
	'D':  {"#### ", "#   #", "#   #", "#   #", "#   #", "#   #", "#### "},
	'E':  {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#####"},
	'F':  {"#####", "#    ", "#    ", "#### ", "#    ", "#    ", "#    "},
	'G':  {" ### ", "#   #", "#    ", "# ###", "#   #", "#   #", " ### "},
	'H':  {"#   #", "#   #", "#   #", "#####", "#   #", "#   #", "#   #"},
	'I':  {" ### ", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", " ### "},
	'J':  {"   ##", "    #", "    #", "    #", "    #", "#   #", " ### "},
	'K':  {"#   #", "#  # ", "# #  ", "##   ", "# #  ", "#  # ", "#   #"},
	'L':  {"#    ", "#    ", "#    ", "#    ", "#    ", "#    ", "#####"},
	'M':  {"#   #", "## ##", "# # #", "# # #", "#   #", "#   #", "#   #"},
	'N':  {"#   #", "##  #", "# # #", "#  ##", "#   #", "#   #", "#   #"},
	'O':  {" ### ", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'P':  {"#### ", "#   #", "#   #", "#### ", "#    ", "#    ", "#    "},
	'Q':  {" ### ", "#   #", "#   #", "#   #", "# # #", "#  # ", " ## #"},
	'R':  {"#### ", "#   #", "#   #", "#### ", "# #  ", "#  # ", "#   #"},
	'S':  {" ####", "#    ", "#    ", " ### ", "    #", "    #", "#### "},
	'T':  {"#####", "  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "  #  "},
	'U':  {"#   #", "#   #", "#   #", "#   #", "#   #", "#   #", " ### "},
	'V':  {"#   #", "#   #", "#   #", "#   #", "#   #", " # # ", "  #  "},
	'W':  {"#   #", "#   #", "#   #", "# # #", "# # #", "## ##", "#   #"},
	'X':  {"#   #", "#   #", " # # ", "  #  ", " # # ", "#   #", "#   #"},
	'Y':  {"#   #", "#   #", " # # ", "  #  ", "  #  ", "  #  ", "  #  "},
	'Z':  {"#####", "    #", "   # ", "  #  ", " #   ", "#    ", "#####"},
	':':  {"     ", "  #  ", "  #  ", "     ", "  #  ", "  #  ", "     "},
	'!':  {"  #  ", "  #  ", "  #  ", "  #  ", "  #  ", "     ", "  #  "},
	'\'': {"  #  ", "  #  ", "  #  ", "     ", "     ", "     ", "     "},
	'.':  {"     ", "     ", "     ", "     ", "     ", "  #  ", "  #  "},
	'-':  {"     ", "     ", "     ", " ### ", "     ", "     ", "     "},
	'?':  {" ### ", "#   #", "    #", "   # ", "  #  ", "     ", "  #  "},
	' ':  {"     ", "     ", "     ", "     ", "     ", "     ", "     "},
}

// Width returns the display width of text at the given horizontal scale.
func Width(text string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	n := len([]rune(text))
	if n == 0 {
		return 0
	}
	return n*GlyphW*scale + (n-1)*scale
}

// WidthProportional returns the display width of text when every glyph is
// trimmed to its ink columns. The gap between glyphs is still one scale step
// and a space takes two columns, so a long word fits where the full 5-column
// boxes would not.
func WidthProportional(text string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	text = strings.ToUpper(text)
	total, count := 0, 0
	for _, r := range text {
		if r == '\n' {
			continue
		}
		count++
		if r == ' ' {
			total += 2 * scale
			continue
		}
		lo, hi := inkBounds(glyphFor(r))
		if hi >= lo {
			total += (hi - lo + 1) * scale
		}
	}
	if count > 1 {
		total += (count - 1) * scale
	}
	return total
}

// Draw renders text and returns its width. scale repeats each glyph column and
// is how the wide layout gets its chunky digits.
func Draw(c *canvas.Canvas, x, y int, text string, opt Options) int {
	if c == nil {
		return 0
	}
	scale := opt.Scale
	if scale < 1 {
		scale = 1
	}
	text = strings.ToUpper(text)
	runes := []rune(text)
	if opt.HideColon {
		for i, r := range runes {
			if r == ':' {
				runes[i] = ' '
			}
		}
	}
	total := Width(string(runes), scale)
	if opt.Proportional {
		total = WidthProportional(string(runes), scale)
	}

	if opt.HasShadow {
		shadow := canvas.Bg(opt.Shadow)
		gx := x
		for _, r := range runes {
			if r == '\n' {
				continue
			}
			g := glyphFor(r)
			lo, hi := inkColumns(r, g, opt.Proportional)
			for row := 0; row < GlyphH; row++ {
				for col := lo; col <= hi; col++ {
					if g[row][col] != '#' {
						continue
					}
					for sx := 0; sx < scale; sx++ {
						c.Set(gx+(col-lo)*scale+sx+1, y+row+1, ' ', shadow)
					}
				}
			}
			gx += columnAdvance(r, lo, hi, scale, opt.Proportional) + scale
		}
	}

	gx := x
	for _, r := range runes {
		if r == '\n' {
			continue
		}
		g := glyphFor(r)
		lo, hi := inkColumns(r, g, opt.Proportional)
		for row := 0; row < GlyphH; row++ {
			t := 0.0
			if GlyphH > 1 {
				t = float64(row) / float64(GlyphH-1)
			}
			col := lerp(opt.Top, opt.Bottom, t)
			st := canvas.Fg(col)
			st.Bold = opt.Bold
			for cx := lo; cx <= hi; cx++ {
				if g[row][cx] != '#' {
					continue
				}
				for sx := 0; sx < scale; sx++ {
					c.Set(gx+(cx-lo)*scale+sx, y+row, '█', st)
				}
			}
		}
		gx += columnAdvance(r, lo, hi, scale, opt.Proportional) + scale
	}
	return total
}

// inkColumns returns the glyph columns to paint. Monospaced text keeps the
// full 5-column box so the clock width does not change between digits;
// proportional text trims the blank side bearings.
func inkColumns(r rune, g [GlyphH]string, proportional bool) (int, int) {
	if !proportional {
		return 0, GlyphW - 1
	}
	if r == ' ' {
		return 0, -1
	}
	lo, hi := inkBounds(g)
	if hi < lo {
		return 0, -1
	}
	return lo, hi
}

// columnAdvance is how many cells a glyph occupies before the inter-glyph gap.
func columnAdvance(r rune, lo, hi, scale int, proportional bool) int {
	if !proportional {
		return GlyphW * scale
	}
	if r == ' ' {
		return 2 * scale
	}
	if hi < lo {
		return 0
	}
	return (hi - lo + 1) * scale
}

// inkBounds returns the first and last columns with ink. A blank glyph reports
// hi < lo.
func inkBounds(g [GlyphH]string) (int, int) {
	lo, hi := GlyphW, -1
	for row := 0; row < GlyphH; row++ {
		for col := 0; col < GlyphW; col++ {
			if g[row][col] != '#' {
				continue
			}
			if col < lo {
				lo = col
			}
			if col > hi {
				hi = col
			}
		}
	}
	return lo, hi
}

func glyphFor(r rune) [GlyphH]string {
	if g, ok := glyphs[r]; ok {
		return g
	}
	return glyphs['?']
}

// Glyph returns the 5x7 cells of r and reports whether the font has it. The
// banner generator uses this so the branding wordmark and the countdown share
// a single font.
func Glyph(r rune) ([GlyphH]string, bool) {
	g, ok := glyphs[r]
	return g, ok
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t)
	}
	return color.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 0xFF}
}
