// Package fx implements the ambient dust and the confetti celebration.
package fx

import (
	"image/color"
	"math"
	"math/rand"

	"tuick-tack/internal/tui/canvas"
)

type mote struct {
	x, y   float64
	vx, vy float64
	phase  float64
	glyph  rune
}

// Dust is a slow field of ~30 ambient motes.
type Dust struct {
	W, H  int
	rng   *rand.Rand
	motes []mote
	t     float64
}

// NewDust creates a deterministic dust field (seed is explicit for tests).
func NewDust(seed int64, w, h int) *Dust {
	d := &Dust{W: w, H: h, rng: rand.New(rand.NewSource(seed))}
	d.reset()
	return d
}

func (d *Dust) reset() {
	const count = 30
	d.motes = make([]mote, count)
	glyphs := []rune{'·'}
	for i := range d.motes {
		d.motes[i] = mote{
			x:     d.rng.Float64() * float64(max(d.W, 1)),
			y:     d.rng.Float64() * float64(max(d.H, 1)),
			vx:    (d.rng.Float64()*2 - 1) * 0.35,
			vy:    (d.rng.Float64()*2 - 1) * 0.16,
			phase: d.rng.Float64() * math.Pi * 2,
			glyph: glyphs[d.rng.Intn(len(glyphs))],
		}
	}
}

// Resize updates the bounds and wraps motes into them.
func (d *Dust) Resize(w, h int) {
	d.W, d.H = w, h
	for i := range d.motes {
		d.motes[i].x = wrap(d.motes[i].x, float64(max(w, 1)))
		d.motes[i].y = wrap(d.motes[i].y, float64(max(h, 1)))
	}
}

// Update advances the field. dt is clamped to 200 ms.
func (d *Dust) Update(dt float64) {
	if dt < 0 {
		return
	}
	if dt > 0.2 {
		dt = 0.2
	}
	d.t += dt
	if d.W <= 0 || d.H <= 0 {
		return
	}
	for i := range d.motes {
		m := &d.motes[i]
		m.x += (m.vx + math.Sin(d.t*0.35+m.phase)*0.09) * dt
		m.y += (m.vy + math.Cos(d.t*0.28+m.phase)*0.05) * dt
		m.x = wrap(m.x, float64(d.W))
		m.y = wrap(m.y, float64(d.H))
	}
}

// Draw paints the motes. col should already be a low-contrast color.
func (d *Dust) Draw(c *canvas.Canvas, col color.RGBA) {
	if c == nil {
		return
	}
	st := canvas.Fg(col)
	for _, m := range d.motes {
		c.Set(int(m.x), int(m.y), m.glyph, st)
	}
}

// DrawWithin paints only the motes inside the rect, used for the gentler dust
// that lives inside the glass panel.
func (d *Dust) DrawWithin(c *canvas.Canvas, col color.RGBA, x, y, w, h int) {
	if c == nil || w <= 0 || h <= 0 {
		return
	}
	st := canvas.Fg(col)
	for _, m := range d.motes {
		mx, my := int(m.x), int(m.y)
		if mx < x || my < y || mx >= x+w || my >= y+h {
			continue
		}
		c.Set(mx, my, m.glyph, st)
	}
}

func wrap(v, size float64) float64 {
	if size <= 0 {
		return 0
	}
	v = math.Mod(v, size)
	if v < 0 {
		v += size
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
