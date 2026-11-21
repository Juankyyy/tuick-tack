package fx

import (
	"math"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

// Sunset is a retro sunset: a low horizon, a warm half-sun with scanline cuts,
// a soft halo, a reflection and a perspective grid. The config ID stays "grid"
// so existing configs keep working; the UI calls it Sunset.
type Sunset struct {
	w, h int
}

func newSunset(w, h int) *Sunset { return &Sunset{w: w, h: h} }

func (s *Sunset) Name() string { return config.BackgroundGrid }

func (s *Sunset) Resize(w, h int) { s.w, s.h = w, h }

func (s *Sunset) Draw(c *canvas.Canvas, pal theme.Theme, t float64, st State) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	fillBase(c, pal)
	if c.H < 8 {
		return
	}
	bright := st.brightness()
	pulse := 1 + 0.15*st.urgentPulse(t)
	hy := int(math.Round(float64(c.H) * 0.72))
	if hy < 2 {
		hy = 2
	}
	if hy > c.H-2 {
		hy = c.H - 2
	}
	cx := float64(c.W) / 2
	r := float64(c.H) * 0.22
	if r < 4.5 {
		r = 4.5
	}
	if r > 11 {
		r = 11
	}

	// Warm halo around the sun, above the horizon.
	haloR := r + 3
	for y := hy - int(math.Ceil(haloR)); y <= hy; y++ {
		if y < 0 || y >= c.H {
			continue
		}
		for x := 0; x < c.W; x++ {
			d := math.Hypot(float64(x)-cx, float64(y-hy))
			if d >= haloR {
				continue
			}
			alpha := (1 - d/haloR) * (1 - d/haloR) * 0.08 * bright * pulse
			base := c.At(x, y).BG
			c.Set(x, y, ' ', canvas.Bg(theme.Lerp(base, pal.Warn, alpha)))
		}
	}

	// The half-sun. The lower part gets the classic scanline cuts; the horizon
	// line is painted over the base of the disc afterwards.
	top := hy - int(math.Round(r))
	for y := top; y <= hy; y++ {
		dy := float64(y - hy)
		half := math.Sqrt(math.Max(0, r*r-dy*dy))
		if y == top && half < 0.5 {
			// Give the apex one or two cells so the top reads as a curve, not
			// as a flat mushroom cap.
			half = 0.5
		}
		if half < 0.5 {
			continue
		}
		if y > top+int(r*0.5) && (hy-y)%2 == 0 {
			continue
		}
		rowT := float64(y-top) / float64(max(hy-top, 1))
		alpha := (0.20 + 0.10*rowT) * bright
		for x := int(math.Round(cx - half)); x <= int(math.Round(cx+half)); x++ {
			if x < 0 || x >= c.W {
				continue
			}
			dx := (float64(x) - cx) / math.Max(half, 1)
			d := math.Hypot(float64(x)-cx, dy) / math.Max(r, 1)
			cellAlpha := alpha * (1 - 0.15*dx*dx)
			if d > 0.82 {
				// A thin, bright rim makes the semicircle read as a sun instead
				// of a striped mound. It is a small number of cells.
				cellAlpha = 0.80 * bright
			}
			c.Set(x, y, ' ', canvas.Bg(theme.Lerp(pal.BG2, pal.Warn, cellAlpha)))
		}
	}

	// A faint reflector under the sun, broken into rows like water.
	refW := int(math.Round(r * 0.75))
	for y := hy + 1; y < c.H; y++ {
		if (y-hy)%2 == 0 {
			continue
		}
		fade := 1 - float64(y-hy)/float64(c.H-hy)
		if fade <= 0 {
			continue
		}
		for x := int(cx) - refW; x <= int(cx)+refW; x++ {
			if x < 0 || x >= c.W {
				continue
			}
			edge := 1 - math.Abs(float64(x)-cx)/float64(refW+1)
			alpha := 0.05 * fade * edge * bright
			if (y-hy)%3 == 0 {
				alpha *= 1.4
			}
			base := c.At(x, y).BG
			c.Set(x, y, ' ', canvas.Bg(theme.Lerp(base, pal.Warn, alpha)))
		}
	}

	// Perspective floor: horizontal lines first, then the converging verticals.
	for i := 0; i < 9; i++ {
		phase := math.Mod(t*0.10+float64(i)/9, 1)
		y := hy + int(float64(c.H-1-hy)*phase*phase)
		if y <= hy || y >= c.H {
			continue
		}
		alpha := (0.05 + 0.12*phase) * bright
		for x := 0; x < c.W; x++ {
			base := c.At(x, y).BG
			c.Set(x, y, ' ', canvas.Bg(theme.Lerp(base, pal.Accent2, alpha)))
		}
	}
	for i := -4; i <= 4; i++ {
		bottomX := cx + float64(i)*float64(c.W)/10
		topX := cx + float64(i)*float64(c.W)/80
		for y := hy + 1; y < c.H; y++ {
			p := float64(y-hy) / float64(max(c.H-1-hy, 1))
			x := int(math.Round(topX + (bottomX-topX)*p))
			if x < 0 || x >= c.W {
				continue
			}
			alpha := (0.06 + 0.14*p) * bright
			base := c.At(x, y).BG
			c.Set(x, y, ' ', canvas.Bg(theme.Lerp(base, pal.Accent2, alpha)))
		}
	}

	// The horizon line sits on top so the sun reads as setting behind it.
	// (It is drawn once, after the reflection and the grid.)
	for y := hy - 1; y <= hy; y++ {
		if y < 0 || y >= c.H {
			continue
		}
		for x := 0; x < c.W; x++ {
			alpha := 0.10 * bright * pulse
			if math.Abs(float64(x)-cx) <= r {
				alpha += 0.03 * bright
			}
			c.Set(x, y, ' ', canvas.Bg(theme.Lerp(pal.BG2, pal.Warn, alpha)))
		}
	}
}
