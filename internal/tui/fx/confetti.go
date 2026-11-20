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

// tint is one flat tone of a confetti tile. The cartoon look uses a fixed
// three-tone ramp (highlight, body, shadow) plus a darker drop shadow; no
// color is blended into the background while the piece is alive.
type tint uint8

const (
	tintBody tint = iota
	tintHighlight
	tintShadow
	tintDrop
	tintWhite
)

// spriteCell is one terminal cell of a tile, in cell units from the tile's
// top-left anchor. UseBG paints the cell background with the body color, so
// the glyph can draw a flat highlight or shadow over a solid tile.
type spriteCell struct {
	DX, DY int
	Glyph  rune
	Tint   tint
	UseBG  bool
	Rank   int
}

// spriteFrame is one frame of the tumble. Width is the frame's width in cells.
type spriteFrame struct {
	Cells []spriteCell
	Width int
}

// paperFrames is the 2x1 cartoon tile: a solid body with a top-left highlight
// and a bottom-right shadow. The tumble moves through a full tile, a top
// sliver, an edge-on strip and a bottom sliver. Only block glyphs are used.
var paperFrames = [4]spriteFrame{
	{
		Width: 2,
		Cells: []spriteCell{
			{DX: 0, DY: 0, Glyph: '▘', Tint: tintHighlight, UseBG: true, Rank: 1},
			{DX: 1, DY: 0, Glyph: '▄', Tint: tintShadow, UseBG: true, Rank: 0},
		},
	},
	{
		Width: 2,
		Cells: []spriteCell{
			{DX: 0, DY: 0, Glyph: '▀', Tint: tintHighlight, Rank: 1},
			{DX: 1, DY: 0, Glyph: '▀', Tint: tintBody, Rank: 0},
		},
	},
	{
		Width: 1,
		Cells: []spriteCell{
			{DX: 0, DY: 0, Glyph: '▌', Tint: tintBody, Rank: 1},
		},
	},
	{
		Width: 2,
		Cells: []spriteCell{
			{DX: 0, DY: 0, Glyph: '▄', Tint: tintBody, Rank: 1},
			{DX: 1, DY: 0, Glyph: '▄', Tint: tintShadow, Rank: 0},
		},
	},
}

// paperDrop is the thin shadow cast one cell down and right of the tile. It is
// a top-half block, so it reads as a soft strip rather than a second piece.
var paperDrop = spriteCell{DX: 1, DY: 1, Glyph: '▀', Tint: tintDrop}

// sparkFrame is a chunky pixel star: four arms and a white center.
var sparkFrame = []spriteCell{
	{DX: -1, DY: 0, Glyph: '█', Tint: tintBody, Rank: 0},
	{DX: 1, DY: 0, Glyph: '█', Tint: tintBody, Rank: 0},
	{DX: 0, DY: -1, Glyph: '█', Tint: tintBody, Rank: 0},
	{DX: 0, DY: 1, Glyph: '█', Tint: tintBody, Rank: 0},
	{DX: 0, DY: 0, Glyph: '█', Tint: tintWhite, Rank: 1},
}

var sparkWhite = color.RGBA{R: 255, G: 255, B: 255, A: 255}

const (
	maxParticles     = 240
	baseArea         = 80 * 24
	minParticleScale = 0.55
	maxParticleScale = 1.45

	// Alpha thresholds of the pixel dissolve. The drop shadow and the right
	// cell disappear first, then the left cell. Quantised steps keep the fade
	// crisp instead of blending the palette into the background.
	keepBothAlpha = 0.55
	keepOneAlpha  = 0.16
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
	particles []particle
	elapsed   float64
	fadeStart float64
	fadeEnd   float64
	short     bool
}

// NewCelebration creates the celebration bursts. short fires a compact burst
// for a TUI that connects with the timer already finished.
func NewCelebration(seed int64, w, h int, palette []color.RGBA, short bool) *Confetti {
	c := &Confetti{
		W: w, H: h,
		rng:     rand.New(rand.NewSource(seed)),
		palette: append([]color.RGBA(nil), palette...),
		short:   short,
	}
	if len(c.palette) == 0 {
		c.palette = []color.RGBA{{R: 255, G: 255, B: 255, A: 255}}
	}
	c.fadeStart, c.fadeEnd = 5.0, 5.7

	bursts := []burst{
		{delay: 0.00, paper: 28, spark: 4, kind: burstCenter},
		{delay: 0.12, paper: 14, spark: 1, kind: burstLeft},
		{delay: 0.26, paper: 14, spark: 1, kind: burstRight},
		{delay: 0.55, paper: 32, spark: 3, kind: burstRain},
		{delay: 2.40, paper: 12, spark: 1, kind: burstDrizzle},
	}
	if short {
		bursts = []burst{
			{delay: 0.00, paper: 30, spark: 4, kind: burstCenter},
			{delay: 0.15, paper: 16, spark: 2, kind: burstRain},
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
		Delay: b.delay + c.rng.Float64()*burstJitter(b.kind),
		Phase: c.rng.Float64() * math.Pi * 2,
		Kind:  b.kind,
		Spark: spark,
		Color: c.palette[c.rng.Intn(len(c.palette))],
	}
	sign := 1.0
	if c.rng.Intn(2) == 0 {
		sign = -1
	}
	if spark {
		p.Color = theme.Lerp(p.Color, sparkWhite, 0.45)
		p.Life = c.sparkLife(b.kind)
		p.Spin = (1.4 + c.rng.Float64()*2.0) * sign
	} else {
		p.Life = c.paperLife(b.kind)
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

// Draw paints the particles. Flat palette tones and a pixel dissolve replace
// the old continuous fade, so pieces drop cells instead of blending into the
// background. It draws in three passes: drop shadows, tiles, then sparkles.
func (c *Confetti) Draw(cv *canvas.Canvas) {
	if cv == nil {
		return
	}
	global := c.fade()
	if global <= 0 {
		return
	}

	// Drop shadows first, so every tile sits over its own shadow and no tile
	// is painted by another piece's shadow.
	for i := range c.particles {
		p := &c.particles[i]
		if !p.Placed || !p.Active || p.Age >= p.Life || p.Spark {
			continue
		}
		alpha := pieceAlpha(p, global)
		if alpha <= keepBothAlpha {
			continue
		}
		frame := paperFrames[p.frame()]
		x, y := tileOrigin(p, frame.Width)
		drawCell(cv, x+paperDrop.DX, y+paperDrop.DY, paperDrop, p.Color)
	}

	// Tumbling tiles.
	for i := range c.particles {
		p := &c.particles[i]
		if !p.Placed || !p.Active || p.Age >= p.Life || p.Spark {
			continue
		}
		alpha := pieceAlpha(p, global)
		if alpha <= keepOneAlpha {
			continue
		}
		frame := paperFrames[p.frame()]
		x, y := tileOrigin(p, frame.Width)
		for _, cell := range frame.Cells {
			if cell.Rank == 0 && alpha <= keepBothAlpha {
				continue
			}
			drawCell(cv, x+cell.DX, y+cell.DY, cell, p.Color)
		}
	}

	// Sparkles over the paper.
	for i := range c.particles {
		p := &c.particles[i]
		if !p.Placed || !p.Active || p.Age >= p.Life || !p.Spark {
			continue
		}
		alpha := pieceAlpha(p, global)
		if alpha <= keepOneAlpha {
			continue
		}
		arms := math.Sin(p.Age*8+p.Phase) > -0.15
		x, y := tileOrigin(p, 1)
		for _, cell := range sparkFrame {
			if cell.Rank == 0 && (!arms || alpha <= keepBothAlpha) {
				continue
			}
			drawCell(cv, x+cell.DX, y+cell.DY, cell, p.Color)
		}
	}
}

// frame picks the tumble frame from the current rotation. A full turn visits
// the four block silhouettes in order.
func (p *particle) frame() int {
	q := int(math.Floor(p.Angle / (math.Pi / 2)))
	return ((q % 4) + 4) % 4
}

// tileOrigin centers a frame of the given width on the particle position.
func tileOrigin(p *particle, width int) (int, int) {
	return int(math.Round(p.X - float64(width-1)/2)), int(math.Round(p.Y))
}

// pieceAlpha is the piece opacity: the global fade multiplied by the short
// tail at the end of the piece's life.
func pieceAlpha(p *particle, global float64) float64 {
	if p.Life <= 0 {
		return 0
	}
	a := global
	if p.Age > p.Life*0.85 {
		a *= 1 - (p.Age-p.Life*0.85)/(p.Life*0.15)
	}
	if a < 0 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

func drawCell(cv *canvas.Canvas, x, y int, cell spriteCell, base color.RGBA) {
	col := tintColor(base, cell.Tint)
	st := canvas.Fg(col)
	if cell.UseBG {
		st = canvas.Full(col, base)
	}
	cv.Set(x, y, cell.Glyph, st)
}

// tintColor maps the flat ramp of a piece. The highlight and shadow are fixed
// tones derived from the palette color, not a blend with the background.
func tintColor(base color.RGBA, t tint) color.RGBA {
	switch t {
	case tintHighlight:
		return theme.Lerp(base, sparkWhite, 0.38)
	case tintShadow:
		return theme.Scale(base, 0.45)
	case tintDrop:
		return theme.Scale(base, 0.22)
	case tintWhite:
		return sparkWhite
	default:
		return base
	}
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
