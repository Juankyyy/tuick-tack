// Package fx implements the ambient dust and the confetti celebration.
package fx

import (
	"image/color"
	"math"
	"math/rand"

	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
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

// Particle is one confetti piece.
type Particle struct {
	X, Y    float64
	VX, VY  float64
	Gravity float64
	Delay   float64
	Age     float64
	Life    float64
	Phase   float64
	Spin    float64
	Glyph   rune
	Glyph2  rune
	Color   color.RGBA
	Streak  bool
	Active  bool
}

type burst struct {
	delay float64
	count int
	kind  int
}

// Confetti owns the celebration particles.
type Confetti struct {
	W, H      int
	rng       *rand.Rand
	palette   []color.RGBA
	bg        color.RGBA
	particles []Particle
	bursts    []burst
	elapsed   float64
	maxDelay  float64
	maxLife   float64
	short     bool
}

// NewCelebration creates the four bursts from SPEC §11. short fires a single
// small burst for a TUI that connects with the timer already finished.
func NewCelebration(seed int64, w, h int, palette []color.RGBA, bg color.RGBA, short bool) *Confetti {
	c := &Confetti{
		W: w, H: h,
		rng:     rand.New(rand.NewSource(seed)),
		palette: append([]color.RGBA(nil), palette...),
		bg:      bg,
		short:   short,
	}
	if len(c.palette) == 0 {
		c.palette = []color.RGBA{{R: 255, G: 255, B: 255, A: 255}}
	}
	if short {
		c.bursts = []burst{{delay: 0, count: 60, kind: 0}}
	} else {
		c.bursts = []burst{
			{delay: 0.0, count: 70, kind: 0},
			{delay: 0.2, count: 55, kind: 1},
			{delay: 0.4, count: 55, kind: 2},
			{delay: 0.9, count: 40, kind: 3},
		}
	}
	for _, b := range c.bursts {
		if b.delay > c.maxDelay {
			c.maxDelay = b.delay
		}
		for i := 0; i < b.count && len(c.particles) < 260; i++ {
			c.particles = append(c.particles, c.spawn(b))
		}
	}
	if c.maxLife == 0 {
		c.maxLife = 3
	}
	return c
}

func (c *Confetti) spawn(b burst) Particle {
	glyphs := []rune{'✦', '✧', '◆', '◇', '•', '▸'}
	p := Particle{
		Glyph:  glyphs[c.rng.Intn(len(glyphs))],
		Glyph2: glyphs[c.rng.Intn(len(glyphs))],
		Color:  c.palette[c.rng.Intn(len(c.palette))],
		Delay:  b.delay + c.rng.Float64()*0.08,
		Life:   2.2 + c.rng.Float64()*1.4,
		Phase:  c.rng.Float64() * math.Pi * 2,
		Spin:   2 + c.rng.Float64()*5,
		Streak: c.rng.Float64() < 0.35,
		Active: false,
	}
	w, h := float64(max(c.W, 1)), float64(max(c.H, 1))
	center := w / 2
	switch b.kind {
	case 0: // center-bottom cannon
		p.X = center + (c.rng.Float64()*2-1)*w*0.18
		p.Y = h - 2
		p.VX = (c.rng.Float64()*2 - 1) * 6
		p.VY = -12 - c.rng.Float64()*14
	case 1: // left cannon
		p.X = 1 + c.rng.Float64()*4
		p.Y = h*0.72 + c.rng.Float64()*3
		p.VX = 10 + c.rng.Float64()*10
		p.VY = -10 - c.rng.Float64()*12
	case 2: // right cannon
		p.X = w - 1 - c.rng.Float64()*4
		p.Y = h*0.72 + c.rng.Float64()*3
		p.VX = -10 - c.rng.Float64()*10
		p.VY = -10 - c.rng.Float64()*12
	default: // top rain
		p.X = c.rng.Float64() * w
		p.Y = -1 - c.rng.Float64()*2
		p.VX = (c.rng.Float64()*2 - 1) * 3
		p.VY = 1 + c.rng.Float64()*3
	}
	p.Gravity = 180 + c.rng.Float64()*120
	if p.Life > c.maxLife {
		c.maxLife = p.Life
	}
	return p
}

// Resize updates the bounds; existing particles are clamped so a resize cannot
// leave them outside the terminal.
func (c *Confetti) Resize(w, h int) {
	c.W, c.H = w, h
	for i := range c.particles {
		p := &c.particles[i]
		if w > 0 && p.X > float64(w)+2 {
			p.X = float64(w) + 2
		}
		if h > 0 && p.Y > float64(h)+2 {
			p.Y = float64(h) + 2
		}
		if p.X < -2 {
			p.X = -2
		}
		if p.Y < -2 {
			p.Y = -2
		}
	}
}

// Update advances the simulation. dt is capped at 50 ms per SPEC §11.
func (c *Confetti) Update(dt float64) {
	if dt < 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}
	if dt > 0.05 {
		dt = 0.05
	}
	c.elapsed += dt
	for i := range c.particles {
		p := &c.particles[i]
		if c.elapsed < p.Delay {
			continue
		}
		p.Active = true
		if p.Age >= p.Life {
			continue
		}
		p.Age += dt
		// SPEC §11 quotes gravity in 180-300 px/s². A terminal row is about
		// eight pixels tall, so divide by eight to keep the same trajectory on
		// a cell grid instead of dropping every piece out of frame in 0.2 s.
		p.VY += (p.Gravity / 8) * dt
		// Exponential drag, applied per 60 Hz frame.
		drag := math.Pow(0.985, dt*60)
		p.VX *= drag
		p.VY *= drag
		p.X += (p.VX + math.Sin(p.Age*3+p.Phase)*2.4) * dt
		p.Y += p.VY * dt
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			p.X, p.Y = float64(c.W)/2, float64(c.H)/2
		}
	}
}

// Draw paints the particles, fading the last 30% of each life into bg.
func (c *Confetti) Draw(cv *canvas.Canvas) {
	if cv == nil {
		return
	}
	for _, p := range c.particles {
		if !p.Active || p.Age >= p.Life {
			continue
		}
		t := p.Age / p.Life
		col := p.Color
		if t > 0.7 {
			col = theme.Lerp(p.Color, c.bg, (t-0.7)/0.3)
		}
		glyph := p.Glyph
		if int(p.Age*p.Spin)%2 == 1 {
			glyph = p.Glyph2
		}
		x, y := int(math.Round(p.X)), int(math.Round(p.Y))
		if p.Streak && t < 0.4 {
			trail := theme.Lerp(col, c.bg, 0.55)
			cv.Set(x-intSign(p.VX), y-intSign(p.VY), '·', canvas.Fg(trail))
		}
		cv.Set(x, y, glyph, canvas.Fg(col))
	}
}

func intSign(v float64) int {
	switch {
	case v > 0.5:
		return 1
	case v < -0.5:
		return -1
	default:
		return 0
	}
}

// Done reports that every burst has fired and every particle has expired.
func (c *Confetti) Done() bool {
	if c.elapsed < c.maxDelay+c.maxLife {
		return false
	}
	for _, p := range c.particles {
		if p.Active && p.Age < p.Life {
			return false
		}
	}
	return true
}

// Count returns the number of particles (including waiting ones).
func (c *Confetti) Count() int { return len(c.particles) }

// Elapsed returns the simulated time in seconds.
func (c *Confetti) Elapsed() float64 { return c.elapsed }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
