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
	step := GlyphW*scale + scale
	total := Width(string(runes), scale)

	if opt.HasShadow {
		shadow := canvas.Bg(opt.Shadow)
		for i, r := range runes {
			g := glyphFor(r)
			gx := x + i*step
			for row := 0; row < GlyphH; row++ {
				for col := 0; col < GlyphW; col++ {
					if g[row][col] != '#' {
						continue
					}
					for sx := 0; sx < scale; sx++ {
						c.Set(gx+col*scale+sx+1, y+row+1, ' ', shadow)
					}
				}
			}
		}
	}

	for i, r := range runes {
		if r == '\n' {
			continue
		}
		g := glyphFor(r)
		gx := x + i*step
		for row := 0; row < GlyphH; row++ {
			t := 0.0
			if GlyphH > 1 {
				t = float64(row) / float64(GlyphH-1)
			}
			col := lerp(opt.Top, opt.Bottom, t)
			st := canvas.Fg(col)
			st.Bold = opt.Bold
			for cx := 0; cx < GlyphW; cx++ {
				if g[row][cx] != '#' {
					continue
				}
				for sx := 0; sx < scale; sx++ {
					c.Set(gx+cx*scale+sx, y+row, '█', st)
				}
			}
		}
	}
	return total
}

func glyphFor(r rune) [GlyphH]string {
	if g, ok := glyphs[r]; ok {
		return g
	}
	return glyphs['?']
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t)
	}
	return color.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 0xFF}
}
