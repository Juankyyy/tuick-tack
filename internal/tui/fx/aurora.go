package fx

import (
	"math"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

// Aurora is the original background: a vertical gradient with two soft radial
// halos. Its geometry and colors are intentionally the same as the background
// that lived in view.go before the backdrop layer existed; the golden test
// keeps it that way.
type Aurora struct {
	w, h int
}

func newAurora(w, h int) *Aurora { return &Aurora{w: w, h: h} }

func (a *Aurora) Name() string { return config.BackgroundAurora }

func (a *Aurora) Resize(w, h int) { a.w, a.h = w, h }

func (a *Aurora) Draw(c *canvas.Canvas, pal theme.Theme, t float64, st State) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	bright := st.brightness()
	r := float64(max(c.W, c.H)) * 0.62
	cx1 := float64(c.W)*0.24 + math.Sin(t*0.07)*float64(c.W)*0.08
	cy1 := float64(c.H)*0.32 + math.Cos(t*0.05)*float64(c.H)*0.10
	cx2 := float64(c.W)*0.78 + math.Cos(t*0.06)*float64(c.W)*0.06
	cy2 := float64(c.H)*0.72 + math.Sin(t*0.04)*float64(c.H)*0.08
	for y := 0; y < c.H; y++ {
		rowT := 0.0
		if c.H > 1 {
			rowT = float64(y) / float64(c.H-1)
		}
		row := theme.Lerp(pal.BG, pal.BG2, rowT)
		for x := 0; x < c.W; x++ {
			base := row
			if d := math.Hypot(float64(x)-cx1, float64(y)-cy1); d < r {
				i := (1 - d/r) * (1 - d/r) * 0.16 * bright
				base = theme.Lerp(base, pal.Accent, i)
			}
			if d := math.Hypot(float64(x)-cx2, float64(y)-cy2); d < r {
				i := (1 - d/r) * (1 - d/r) * 0.13 * bright
				base = theme.Lerp(base, pal.Accent2, i)
			}
			c.Set(x, y, ' ', canvas.Bg(base))
		}
	}
}
