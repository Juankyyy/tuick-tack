package fx

import (
	"math"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

// Waves paints three slow sine bands rising from the bottom of the screen.
type Waves struct {
	w, h int
}

func newWaves(w, h int) *Waves { return &Waves{w: w, h: h} }

func (w *Waves) Name() string { return config.BackgroundWaves }

func (w *Waves) Resize(width, height int) { w.w, w.h = width, height }

func (w *Waves) Draw(c *canvas.Canvas, pal theme.Theme, t float64, st State) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	fillBase(c, pal)
	bright := st.brightness()
	ampScale := 0.65 + 0.55*clamp01(st.Progress)
	if st.Urgent {
		ampScale *= 1 + 0.25*st.urgentPulse(t)
	}
	for k := 0; k < 3; k++ {
		baseY := float64(c.H) * (0.58 + 0.10*float64(k))
		amp := float64(c.H) * 0.045 * ampScale
		if amp < 1 {
			amp = 1
		}
		freq := 0.055 + 0.012*float64(k)
		speed := 0.30 - 0.05*float64(k)
		phase := t*speed + float64(k)*2.1
		col := pal.Accent
		if k%2 == 1 {
			col = pal.Accent2
		}
		alpha := (0.05 + 0.015*float64(k)) * bright
		if st.Urgent {
			alpha *= 1 + 0.2*st.urgentPulse(t)
		}
		for x := 0; x < c.W; x++ {
			xf := float64(x)
			yf := baseY +
				math.Sin(xf*freq+phase)*amp +
				math.Sin(xf*freq*2.3-phase*1.4)*amp*0.35
			y := int(math.Round(yf))
			if y < 0 {
				y = 0
			}
			if y >= c.H {
				continue
			}
			for yy := y; yy < c.H; yy++ {
				c.Set(x, yy, ' ', canvas.Bg(theme.Lerp(pal.BG2, col, alpha)))
			}
			c.Set(x, y, ' ', canvas.Bg(theme.Lerp(pal.BG2, col, alpha*2.4)))
		}
	}
}
