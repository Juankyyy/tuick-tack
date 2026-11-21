package fx

import (
	"math"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

// State is the small, UI-neutral slice of the timer that a backdrop may read.
// Drawing stays a pure function of the clock, the palette and this state, so a
// paused or resized screen can be repainted without advancing any simulation.
type State struct {
	Progress    float64
	Paused      bool
	Urgent      bool
	Celebrating bool
}

// Backdrop paints the ambient layer behind the stage. Most implementations
// cover every cell with a base gradient, but the "none" background deliberately
// leaves the canvas untouched so the terminal's own background shows through.
// Implementations must never emit a double-width rune and must be safe to call
// with an empty canvas.
type Backdrop interface {
	Name() string
	Resize(w, h int)
	Draw(c *canvas.Canvas, pal theme.Theme, t float64, st State)
}

// NewBackdrop returns the requested background. An unknown or empty name falls
// back to the original aurora so an old or corrupt config never leaves the TUI
// without a background.
func NewBackdrop(name string, seed int64, w, h int) Backdrop {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	switch name {
	case config.BackgroundStars:
		return newStars(seed, w, h)
	case config.BackgroundFireflies:
		return newFireflies(seed, w, h)
	case config.BackgroundRain:
		return newRain(seed, w, h)
	case config.BackgroundWaves:
		return newWaves(w, h)
	case config.BackgroundGrid:
		return newSunset(w, h)
	case config.BackgroundNone:
		return &flat{w: w, h: h}
	default:
		return newAurora(w, h)
	}
}

// fillBase paints the vertical background gradient shared by the opaque
// effects. The "none" background does not call it.
func fillBase(c *canvas.Canvas, pal theme.Theme) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	for y := 0; y < c.H; y++ {
		t := 0.0
		if c.H > 1 {
			t = float64(y) / float64(c.H-1)
		}
		c.Fill(0, y, c.W, 1, canvas.Bg(theme.Lerp(pal.BG, pal.BG2, t)))
	}
}

// brightness dims the decorations while the timer is paused or the confetti
// celebration is taking over the screen.
func (s State) brightness() float64 {
	switch {
	case s.Celebrating:
		return 0.5
	case s.Paused:
		return 0.7
	default:
		return 1
	}
}

// urgentPulse is 0 when the timer is calm and a slow 0..1 pulse in the last
// ten seconds.
func (s State) urgentPulse(t float64) float64 {
	if !s.Urgent {
		return 0
	}
	return 0.5 + 0.5*math.Sin(t*math.Pi*2)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
