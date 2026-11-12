// Package canvas is the only place the TUI turns cells into ANSI. It uses
// truecolor SGR sequences and never emits double-width runes.
package canvas

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
)

// Style describes how a cell is painted. Unset foreground/background colors
// keep the color already present in the cell, which lets text sit over a
// gradient without repeating it.
type Style struct {
	FG    color.RGBA
	BG    color.RGBA
	HasFG bool
	HasBG bool
	Bold  bool
}

// Fg returns a foreground-only style.
func Fg(c color.RGBA) Style { return Style{FG: c, HasFG: true} }

// Bg returns a background-only style.
func Bg(c color.RGBA) Style { return Style{BG: c, HasBG: true} }

// Full returns a style with both colors.
func Full(fg, bg color.RGBA) Style {
	return Style{FG: fg, BG: bg, HasFG: true, HasBG: true}
}

// Cell is one terminal cell.
type Cell struct {
	Ch    rune
	FG    color.RGBA
	BG    color.RGBA
	HasFG bool
	HasBG bool
	Bold  bool
}

// Canvas is a fixed grid of cells.
type Canvas struct {
	W, H  int
	cells []Cell
}

// New creates a canvas filled with spaces.
func New(w, h int) *Canvas {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	c := &Canvas{W: w, H: h, cells: make([]Cell, w*h)}
	for i := range c.cells {
		c.cells[i] = Cell{Ch: ' '}
	}
	return c
}

// At returns the cell at x,y. Out-of-bounds coordinates return an empty cell.
func (c *Canvas) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return Cell{Ch: ' '}
	}
	return c.cells[y*c.W+x]
}

// Resize changes the canvas dimensions, preserving the top-left content.
func (c *Canvas) Resize(w, h int) {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	if w == c.W && h == c.H {
		return
	}
	next := make([]Cell, w*h)
	for i := range next {
		next[i] = Cell{Ch: ' '}
	}
	for y := 0; y < h && y < c.H; y++ {
		for x := 0; x < w && x < c.W; x++ {
			next[y*w+x] = c.cells[y*c.W+x]
		}
	}
	c.W, c.H, c.cells = w, h, next
}

// Clear paints every cell with bg.
func (c *Canvas) Clear(bg color.RGBA) {
	for i := range c.cells {
		c.cells[i] = Cell{Ch: ' ', BG: bg, HasBG: true}
	}
}

// Set paints one cell. Out-of-bounds and non-single-cell runes are ignored or
// replaced with '?' so the renderer can never shift the grid.
func (c *Canvas) Set(x, y int, ch rune, st Style) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	if ch == 0 || ch == '\n' || ch == '\r' || ch == '\t' || unicode.IsControl(ch) {
		ch = ' '
	}
	if runewidth.RuneWidth(ch) != 1 {
		ch = '?'
	}
	cell := &c.cells[y*c.W+x]
	cell.Ch = ch
	if st.HasFG {
		cell.FG, cell.HasFG = st.FG, true
	}
	if st.HasBG {
		cell.BG, cell.HasBG = st.BG, true
	}
	if st.Bold {
		cell.Bold = true
	}
}

// Dim moves every cell toward black by t (0 keeps the canvas, 1 leaves it
// completely black). It is used to push the stage behind a modal overlay.
func (c *Canvas) Dim(t float64) {
	if t <= 0 {
		return
	}
	if t > 1 {
		t = 1
	}
	black := color.RGBA{A: 0xFF}
	for i := range c.cells {
		cell := &c.cells[i]
		cell.FG = lerp(cell.FG, black, t)
		cell.BG = lerp(cell.BG, black, t)
	}
}

// Fill paints a rectangle with spaces and the style's background.
func (c *Canvas) Fill(x, y, w, h int, st Style) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			c.Set(xx, yy, ' ', st)
		}
	}
}

// GradientRect fills a rectangle with a vertical background gradient.
func (c *Canvas) GradientRect(x, y, w, h int, top, bottom color.RGBA) {
	if h <= 0 {
		return
	}
	for yy := 0; yy < h; yy++ {
		t := 0.0
		if h > 1 {
			t = float64(yy) / float64(h-1)
		}
		bg := lerp(top, bottom, t)
		c.Fill(x, y+yy, w, 1, Bg(bg))
	}
}

// Text writes a single-cell string starting at x,y and returns the next
// column. A newline stops the write.
func (c *Canvas) Text(x, y int, s string, st Style) int {
	for _, r := range s {
		if r == '\n' {
			break
		}
		c.Set(x, y, r, st)
		if runewidth.RuneWidth(r) == 1 {
			x++
		}
	}
	return x
}

// Center writes s horizontally centered on row y and returns the start column.
func (c *Canvas) Center(y int, s string, st Style) int {
	x := (c.W - runewidth.StringWidth(s)) / 2
	if x < 0 {
		x = 0
	}
	return c.Text(x, y, s, st)
}

// Frame draws a rounded panel: filled interior and a border gradient. A width
// or height below 2 is ignored.
func (c *Canvas) Frame(x, y, w, h int, top, bottom color.RGBA, fill color.RGBA) {
	if w < 2 || h < 2 {
		return
	}
	c.Fill(x, y, w, h, Full(fill, fill))
	for yy := 0; yy < h; yy++ {
		t := 0.0
		if h > 1 {
			t = float64(yy) / float64(h-1)
		}
		border := lerp(top, bottom, t)
		st := Fg(border)
		left, right := '│', '│'
		switch yy {
		case 0:
			left, right = '╭', '╮'
		case h - 1:
			left, right = '╰', '╯'
		}
		c.Set(x, y+yy, left, st)
		c.Set(x+w-1, y+yy, right, st)
		if yy == 0 || yy == h-1 {
			for xx := x + 1; xx < x+w-1; xx++ {
				c.Set(xx, y+yy, '─', st)
			}
		}
	}
}

// Box is Frame with a solid border color.
func (c *Canvas) Box(x, y, w, h int, border, fill color.RGBA) {
	c.Frame(x, y, w, h, border, border, fill)
}

// Blit copies src onto c at x,y, clipping to both canvases.
func (c *Canvas) Blit(src *Canvas, x, y int) {
	if src == nil {
		return
	}
	for sy := 0; sy < src.H; sy++ {
		for sx := 0; sx < src.W; sx++ {
			cell := src.cells[sy*src.W+sx]
			if cell.Ch == ' ' && !cell.HasFG && !cell.HasBG {
				continue
			}
			st := Style{FG: cell.FG, BG: cell.BG, HasFG: cell.HasFG, HasBG: cell.HasBG, Bold: cell.Bold}
			c.Set(x+sx, y+sy, cell.Ch, st)
		}
	}
}

// Render serializes the canvas as truecolor ANSI. Each row is W cells and the
// result has no trailing newline.
func (c *Canvas) Render() string {
	if c.W == 0 || c.H == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(c.W * c.H * 12)
	for y := 0; y < c.H; y++ {
		var cur Cell
		first := true
		for x := 0; x < c.W; x++ {
			cell := c.cells[y*c.W+x]
			if first || cell.FG != cur.FG || cell.BG != cur.BG || cell.Bold != cur.Bold {
				b.WriteString("\x1b[")
				fmt.Fprintf(&b, "38;2;%d;%d;%d;48;2;%d;%d;%d",
					cell.FG.R, cell.FG.G, cell.FG.B, cell.BG.R, cell.BG.G, cell.BG.B)
				if cell.Bold {
					b.WriteString(";1")
				} else {
					b.WriteString(";22")
				}
				b.WriteByte('m')
				cur = cell
				first = false
			}
			b.WriteRune(cell.Ch)
		}
		b.WriteString("\x1b[0m")
		if y != c.H-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// Width returns the display width of a string in cells.
func Width(s string) int { return runewidth.StringWidth(s) }

// SingleCell reports whether r occupies exactly one terminal cell.
func SingleCell(r rune) bool { return runewidth.RuneWidth(r) == 1 }

func lerp(a, b color.RGBA, t float64) color.RGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	mix := func(x, y uint8) uint8 {
		return uint8(float64(x) + (float64(y)-float64(x))*t)
	}
	return color.RGBA{R: mix(a.R, b.R), G: mix(a.G, b.G), B: mix(a.B, b.B), A: 0xFF}
}
