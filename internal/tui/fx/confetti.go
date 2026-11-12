package fx

import (
	"image/color"
	"math"
	"math/rand"

	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

type burstKind uint8

const (
	burstCenter burstKind = iota
	burstLeft
	burstRight
	burstRain
	burstDrizzle
)

type burst struct {
	delay float64
	paper int
	spark int
	kind  burstKind
}

// glyphFamily is one kind of paper: the glyph shown face-on, its back and its
// edge-on profile. The renderer picks between them from the tumble angle.
type glyphFamily struct {
	front rune
	back  rune
	edge  rune
}

var paperFamilies = []glyphFamily{
	{'█', '▒', '▌'},
	{'▀', '▄', '─'},
	{'▐', '▌', '│'},
	{'◆', '◇', '▸'},
	{'▓', '░', '▏'},
}

var sparkFamily = glyphFamily{'✦', '✧', '·'}

var sparkWhite = color.RGBA{R: 255, G: 255, B: 255, A: 255}

const (
	trailGlyph = '·'

	maxParticles     = 360
	baseArea         = 80 * 24
	minParticleScale = 0.55
	maxParticleScale = 1.45
)

type particle struct {
	X, Y     float64
	VX, VY   float64
	Gravity  float64
	DragX    float64
	Delay    float64
	Age      float64
	Life     float64
	Phase    float64
	Spin     float64
	Angle    float64
	Sway     float64
	SwayFreq float64
	Bob      float64
	BobFreq  float64
	Color    color.RGBA
	Family   glyphFamily
	Kind     burstKind
	Spark    bool
	Placed   bool
	Active   bool
}

// Confetti owns the celebration particles.
type Confetti struct {
	W, H      int
	rng       *rand.Rand
	palette   []color.RGBA
	bg        color.RGBA
	particles []particle
	elapsed   float64
	fadeStart float64
	fadeEnd   float64
	short     bool
}

// NewCelebration creates the celebration bursts. short fires a compact burst
// for a TUI that connects with the timer already finished.
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
	c.fadeStart, c.fadeEnd = 5.0, 5.7

	bursts := []burst{
		{delay: 0.00, paper: 48, spark: 12, kind: burstCenter},
		{delay: 0.12, paper: 30, spark: 5, kind: burstLeft},
		{delay: 0.26, paper: 30, spark: 5, kind: burstRight},
		{delay: 0.55, paper: 55, spark: 15, kind: burstRain},
		{delay: 2.40, paper: 25, spark: 5, kind: burstDrizzle},
	}
	if short {
		bursts = []burst{
			{delay: 0.00, paper: 50, spark: 15, kind: burstCenter},
			{delay: 0.15, paper: 20, spark: 5, kind: burstRain},
		}
		c.fadeStart, c.fadeEnd = 2.6, 3.2
	}

	scale := particleScale(w, h)
	for _, b := range bursts {
		c.appendBurst(b, scale)
	}
	return c
}

// particleScale grows the number of pieces with the terminal area, with a
// floor for tiny windows and a ceiling so a large screen does not drown the
// panel in confetti.
func particleScale(w, h int) float64 {
	area := float64(max(w, 1) * max(h, 1))
	s := math.Pow(area/float64(baseArea), 0.55)
	if s < minParticleScale {
		s = minParticleScale
	}
	if s > maxParticleScale {
		s = maxParticleScale
	}
	return s
}

func (c *Confetti) appendBurst(b burst, scale float64) {
	scaled := func(v int) int {
		if v <= 0 {
			return 0
		}
		n := int(math.Round(float64(v) * scale))
		if n < 1 {
			n = 1
		}
		return n
	}
	for i := 0; i < scaled(b.paper); i++ {
		if len(c.particles) >= maxParticles {
			return
		}
		c.particles = append(c.particles, c.newParticle(b, false))
	}
	for i := 0; i < scaled(b.spark); i++ {
		if len(c.particles) >= maxParticles {
			return
		}
		c.particles = append(c.particles, c.newParticle(b, true))
	}
}

func (c *Confetti) newParticle(b burst, spark bool) particle {
	p := particle{
		Delay:  b.delay + c.rng.Float64()*burstJitter(b.kind),
		Phase:  c.rng.Float64() * math.Pi * 2,
		Kind:   b.kind,
		Spark:  spark,
		Color:  c.palette[c.rng.Intn(len(c.palette))],
		Family: paperFamilies[c.rng.Intn(len(paperFamilies))],
	}
	if spark {
		p.Family = sparkFamily
		p.Color = theme.Lerp(p.Color, sparkWhite, 0.3)
		p.Life = c.sparkLife(b.kind)
	} else {
		p.Life = c.paperLife(b.kind)
	}
	sign := 1.0
	if c.rng.Intn(2) == 0 {
		sign = -1
	}
	if spark {
		p.Spin = (1.4 + c.rng.Float64()*2.0) * sign
	} else {
		p.Spin = (2.5 + c.rng.Float64()*3.5) * sign
	}
	return p
}

func burstJitter(k burstKind) float64 {
	if k == burstRain || k == burstDrizzle {
		return 0.35
	}
	return 0.10
}

func (c *Confetti) paperLife(k burstKind) float64 {
	if c.short {
		return 2.2 + c.rng.Float64()*0.9
	}
	switch k {
	case burstDrizzle:
		return 2.8 + c.rng.Float64()*0.8
	case burstRain:
		return 3.8 + c.rng.Float64()*1.4
	default:
		return 3.2 + c.rng.Float64()*1.2
	}
}

func (c *Confetti) sparkLife(k burstKind) float64 {
	if c.short {
		return 2.8 + c.rng.Float64()*0.9
	}
	switch k {
	case burstDrizzle:
		return 2.8 + c.rng.Float64()*0.8
	case burstRain:
		return 4.2 + c.rng.Float64()*1.3
	default:
		return 4.0 + c.rng.Float64()*1.4
	}
}

// place materializes a waiting particle using the current terminal size. Doing
// this at activation, instead of at construction, keeps bursts anchored to the
// screen after a resize before they fire.
func (c *Confetti) place(p *particle) {
	w, h := float64(max(c.W, 1)), float64(max(c.H, 1))
	sx, sy := w/80, h/24
	speedMul := 1.0
	if p.Spark {
		speedMul = 0.65
	}

	switch p.Kind {
	case burstCenter:
		p.X = w/2 + (c.rng.Float64()*2-1)*w*0.10
		p.Y = h * 0.80
		angle := (c.rng.Float64()*2 - 1) * 0.42
		speed := (2.0 + c.rng.Float64()*0.7) * speedMul
		p.VX = math.Sin(angle) * speed * w
		p.VY = -math.Cos(angle) * speed * h
	case burstLeft:
		p.X = 1 + c.rng.Float64()*2
		p.Y = h * (0.68 + c.rng.Float64()*0.10)
		angle := 0.35 + c.rng.Float64()*0.35
		speed := (1.4 + c.rng.Float64()*0.6) * speedMul
		p.VX = math.Cos(angle) * speed * w
		p.VY = -math.Sin(angle) * speed * h
	case burstRight:
		p.X = w - 2 - c.rng.Float64()*2
		p.Y = h * (0.68 + c.rng.Float64()*0.10)
		angle := 0.35 + c.rng.Float64()*0.35
		speed := (1.4 + c.rng.Float64()*0.6) * speedMul
		p.VX = -math.Cos(angle) * speed * w
		p.VY = -math.Sin(angle) * speed * h
	default: // rain and drizzle
		p.X = c.rng.Float64() * w
		p.Y = -1 - c.rng.Float64()*2
		p.VX = (c.rng.Float64()*2 - 1) * 2.0 * sx
		p.VY = (0.8 + c.rng.Float64()*1.6) * sy
		if p.Spark {
			p.VX *= 0.7
			p.VY *= 0.5
		}
	}

	if p.Spark {
		p.Gravity = (4.0 + c.rng.Float64()*2.0) * sy
		p.DragX = 1.4
		p.Sway = (1.7 + c.rng.Float64()*1.5) * sx
		p.Bob = (0.55 + c.rng.Float64()*0.7) * sy
	} else {
		p.Gravity = (11.0 + c.rng.Float64()*5.0) * sy
		p.DragX = 2.0
		p.Sway = (1.2 + c.rng.Float64()*1.2) * sx
		p.Bob = (0.35 + c.rng.Float64()*0.55) * sy
	}
	p.SwayFreq = 1.8 + c.rng.Float64()*2.2
	p.BobFreq = 2.2 + c.rng.Float64()*2.0
}

// Resize remaps active pieces proportionally so shrinking the terminal does
// not pile confetti against the new edges.
func (c *Confetti) Resize(w, h int) {
	oldW, oldH := c.W, c.H
	c.W, c.H = w, h
	if oldW <= 0 || oldH <= 0 || w <= 0 || h <= 0 {
		return
	}
	rx := float64(w) / float64(oldW)
	ry := float64(h) / float64(oldH)
	for i := range c.particles {
		p := &c.particles[i]
		if !p.Placed {
			continue
		}
		p.X *= rx
		p.Y *= ry
		p.VX *= rx
		p.VY *= ry
		p.Gravity *= ry
		p.Sway *= rx
		p.Bob *= ry
		if p.X < -3 {
			p.X = -3
		}
		if p.X > float64(w)+3 {
			p.X = float64(w) + 3
		}
		if p.Y < -3 {
			p.Y = -3
		}
		if p.Y > float64(h)+3 {
			p.Y = float64(h) + 3
		}
	}
}

// Update advances the simulation. dt is capped at 50 ms.
func (c *Confetti) Update(dt float64) {
	if dt < 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		return
	}
	if dt > 0.05 {
		dt = 0.05
	}
	c.elapsed += dt
	if c.elapsed >= c.fadeEnd {
		for i := range c.particles {
			c.particles[i].Active = false
		}
		return
	}

	w, h := float64(max(c.W, 1)), float64(max(c.H, 1))
	for i := range c.particles {
		p := &c.particles[i]
		if !p.Placed {
			if c.elapsed < p.Delay {
				continue
			}
			c.place(p)
			p.Placed = true
			p.Active = true
		}
		if !p.Active {
			continue
		}
		if p.Age >= p.Life {
			p.Active = false
			continue
		}
		p.Age += dt
		p.Angle += p.Spin * dt

		// Paper falls slowly when flat and fast when edge-on; the varying
		// drag against gravity is what makes the tumble read as fluttering.
		face := math.Abs(math.Cos(p.Angle))
		drag := 1.8 + 2.0*face
		if p.Spark {
			drag = 1.5 + 1.1*face
		}
		p.VY += p.Gravity * dt
		p.VX *= math.Exp(-p.DragX * dt)
		p.VY *= math.Exp(-drag * dt)
		p.X += (p.VX + p.Sway*math.Sin(p.Age*p.SwayFreq+p.Phase)) * dt
		p.Y += (p.VY + p.Bob*math.Sin(p.Age*p.BobFreq+p.Phase*1.7)) * dt

		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			p.X, p.Y = w/2, h/2
			p.VX, p.VY = 0, 0
		}
		if p.X < -3 || p.X > w+3 || p.Y > h+2 {
			p.Active = false
		}
	}
}

// Draw paints the particles. It fades into the cell's own background so pieces
// over the panel dissolve into the panel, not into the outer aurora.
func (c *Confetti) Draw(cv *canvas.Canvas) {
	if cv == nil {
		return
	}
	global := c.fade()
	if global <= 0 {
		return
	}
	for i := range c.particles {
		p := &c.particles[i]
		if !p.Placed || !p.Active || p.Age >= p.Life {
			continue
		}
		x, y := int(math.Round(p.X)), int(math.Round(p.Y))
		if x < 0 || y < 0 || x >= cv.W || y >= cv.H {
			continue
		}
		alpha := global
		if p.Age > p.Life*0.85 {
			alpha *= 1 - (p.Age-p.Life*0.85)/(p.Life*0.15)
		}
		if alpha <= 0 {
			continue
		}
		c.drawTrail(cv, p, x, y, alpha)
		glyph, shade := p.surface()
		bg := c.localBG(cv, x, y)
		col := theme.Lerp(p.Color, bg, 1-alpha*shade)
		cv.Set(x, y, glyph, canvas.Fg(col))
	}
}

func (p *particle) surface() (rune, float64) {
	if p.Spark {
		glyph := p.Family.front
		if math.Sin(p.Age*5+p.Phase) < -0.25 {
			glyph = p.Family.back
		}
		return glyph, 0.78 + 0.22*math.Sin(p.Age*6.5+p.Phase)
	}
	face := math.Cos(p.Angle)
	switch {
	case face > 0.35:
		return p.Family.front, 1.0
	case face < -0.35:
		return p.Family.back, 0.55
	default:
		return p.Family.edge, 0.78
	}
}

// drawTrail leaves two dim ghosts behind a piece while it is still moving fast
// enough for motion to blur. Slow, fluttering pieces stay clean.
func (c *Confetti) drawTrail(cv *canvas.Canvas, p *particle, x, y int, alpha float64) {
	if p.Spark {
		return
	}
	speed := math.Hypot(p.VX, p.VY)
	unit := float64(max(c.H, 1)) / 24
	if speed < 14*unit {
		return
	}
	for k := 1; k <= 2; k++ {
		tx := int(math.Round(p.X - p.VX*0.03*float64(k)))
		ty := int(math.Round(p.Y - p.VY*0.03*float64(k)))
		if (tx == x && ty == y) || tx < 0 || ty < 0 || tx >= cv.W || ty >= cv.H {
			continue
		}
		cell := cv.At(tx, ty)
		if cell.Ch != ' ' && cell.Ch != trailGlyph {
			continue
		}
		bg := c.bg
		if cell.HasBG {
			bg = cell.BG
		}
		col := theme.Lerp(p.Color, bg, 0.62+0.10*float64(k))
		col = theme.Lerp(col, bg, 1-alpha)
		cv.Set(tx, ty, trailGlyph, canvas.Fg(col))
	}
}

func (c *Confetti) localBG(cv *canvas.Canvas, x, y int) color.RGBA {
	if cell := cv.At(x, y); cell.HasBG {
		return cell.BG
	}
	return c.bg
}

func (c *Confetti) fade() float64 {
	if c.elapsed <= c.fadeStart {
		return 1
	}
	if c.elapsed >= c.fadeEnd {
		return 0
	}
	return 1 - (c.elapsed-c.fadeStart)/(c.fadeEnd-c.fadeStart)
}

// Done reports that the celebration has finished fading out or that every
// piece has either fired and left the screen or expired.
func (c *Confetti) Done() bool {
	if c.elapsed >= c.fadeEnd {
		return true
	}
	for i := range c.particles {
		p := &c.particles[i]
		if !p.Placed || p.Active {
			return false
		}
	}
	return true
}

// Count returns the number of particles (including waiting ones).
func (c *Confetti) Count() int { return len(c.particles) }

// Elapsed returns the simulated time in seconds.
func (c *Confetti) Elapsed() float64 { return c.elapsed }
