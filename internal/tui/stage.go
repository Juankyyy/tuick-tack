package tui

import (
	"image/color"

	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

// Contrast targets for the transparent stage over a light terminal. Body text
// aims for AAA, secondary copy for AA and large or decorative pieces for the
// 3:1 large-text floor.
const (
	stageTextContrast   = 7.0
	stageMutedContrast  = 4.5
	stageAccentContrast = 4.5
	stageDimContrast    = 3.0
)

// readableBandBG is the solid backing used by the readable-text bands. It is
// deliberately pure black so the bands work on any terminal background.
var readableBandBG = color.RGBA{A: 0xFF}

// lightStage reports whether the transparent "none" background sits on a
// light terminal, so the content painted directly on it must use dark ink.
func (m *Model) lightStage() bool {
	return m.noneBackground() && m.termBGKnown && m.termLight
}

// readableBands reports whether stage text should sit on a black backing.
// Bands only exist with "none"; the user can turn them off in the TUI.
func (m *Model) readableBands() bool {
	return m.noneBackground() && m.cfg.ReadableText
}

// stageColor picks the color for text painted directly on the stage. With
// readable bands, the caller's normal light color sits on black; without them,
// the color is adapted to a light terminal background.
func (m *Model) stageColor(adaptive, onBand color.RGBA, minContrast float64) color.RGBA {
	if m.readableBands() {
		return onBand
	}
	return m.stageInk(adaptive, minContrast)
}

// band paints a one-row black backing behind a text run, with one cell of
// breathing room on each side. It is a no-op when bands are disabled or the
// run is empty.
func (m *Model) band(c *canvas.Canvas, x, y, w int) {
	if !m.readableBands() || w <= 0 {
		return
	}
	x0 := x - 1
	if x0 < 0 {
		x0 = 0
	}
	x1 := x + w + 1
	if x1 > c.W {
		x1 = c.W
	}
	if x1 <= x0 {
		return
	}
	c.Fill(x0, y, x1-x0, 1, canvas.Bg(readableBandBG))
}

// drawCenteredBand is drawCenteredIn plus the readable backing. It behaves
// exactly like drawCenteredIn when bands are disabled.
func (m *Model) drawCenteredBand(c *canvas.Canvas, y int, s string, st canvas.Style, x, w int) {
	s = fitText(s, w)
	sx := x + (w-canvas.Width(s))/2
	if sx < x {
		sx = x
	}
	m.band(c, sx, y, canvas.Width(s))
	c.Text(sx, y, s, st)
}

// lightBackground reports whether dark ink reads better than light ink on bg.
// It follows the WCAG contrast ratio instead of HSL lightness, so saturated
// colors such as pure blue or red are classified by readability rather than by
// a brightness threshold that would send them down the dark path.
func lightBackground(bg color.RGBA) bool {
	black := color.RGBA{A: 0xFF}
	white := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	return theme.Contrast(black, bg) >= theme.Contrast(white, bg)
}

// stageInk adapts a color that is painted directly on the terminal background.
// It is a no-op except with "none" over a light terminal, where the color is
// darkened just enough to reach minContrast against the real background.
func (m *Model) stageInk(c color.RGBA, minContrast float64) color.RGBA {
	if !m.lightStage() {
		return c
	}
	return theme.Ink(c, m.termBG, minContrast)
}
