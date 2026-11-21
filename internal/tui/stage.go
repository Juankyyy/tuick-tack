package tui

import (
	"image/color"

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

// lightStage reports whether the transparent "none" background sits on a
// light terminal, so the content painted directly on it must use dark ink.
func (m *Model) lightStage() bool {
	return m.noneBackground() && m.termBGKnown && m.termLight
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
