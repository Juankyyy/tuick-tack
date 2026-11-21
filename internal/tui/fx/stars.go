package fx

import (
	"math"
	"math/rand"
	"sort"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

const (
	// starBase is the number of stars visible at 0% progress. The rest of the
	// pool fades in as the timer advances, up to starPool at 100%.
	starBase = 80
	starPool = 320
	starFade = 0.04

	// The warp is a simple perspective projection. A star falls towards the
	// camera along z with a constant speed; the apparent velocity and trail
	// length grow near the camera without moving the head's speed in z.
	warpZNear = 0.55
	warpZFar  = 12.5
	warpSpeed = 2.0

	warpTrailBase   = 0.12
	warpTrailGain   = 0.09
	warpTrailUrgent = 0.035
)

type star struct {
	wx, wy  float64
	z0      float64
	phase   float64
	twinkle float64
	accent  bool
	appear  float64
}

type shootingStar struct {
	period float64
	offset float64
	x, y   float64
	dx, dy float64
	length float64
}

// Stars is a warp field: stars stream out from a central vanishing point with
// dot trails that lengthen as the timer advances. The field starts with a
// sparse pool and reveals more stars as the progress reaches 100%, matching
// the denser look of the final minutes.
type Stars struct {
	w, h  int
	stars []star
	shoot shootingStar
}

func newStars(seed int64, w, h int) *Stars {
	rng := rand.New(rand.NewSource(seed ^ 0x51A7E))
	s := &Stars{w: w, h: h, stars: make([]star, 0, starPool)}
	for i := 0; i < starPool; i++ {
		appear := -1.0
		if i >= starBase {
			appear = rng.Float64() * 0.96
		}
		s.stars = append(s.stars, star{
			wx:      rng.Float64()*2 - 1,
			wy:      rng.Float64()*2 - 1,
			z0:      rng.Float64(),
			phase:   rng.Float64() * math.Pi * 2,
			twinkle: 0.6 + rng.Float64(),
			accent:  rng.Float64() < 0.3,
			appear:  appear,
		})
	}
	s.shoot = shootingStar{
		period: 12 + rng.Float64()*6,
		offset: rng.Float64() * 12,
		x:      0.15 + rng.Float64()*0.7,
		y:      0.12 + rng.Float64()*0.25,
		dx:     (rng.Float64()*2 - 1) * 0.55,
		dy:     0.18 + rng.Float64()*0.12,
		length: 8 + rng.Float64()*6,
	}
	return s
}

func (s *Stars) Name() string { return config.BackgroundStars }

// Resize only records the new bounds: star positions are normalized in world
// space, so the same star field projects correctly into any terminal size.
func (s *Stars) Resize(w, h int) { s.w, s.h = w, h }

func (s *Stars) Draw(c *canvas.Canvas, pal theme.Theme, t float64, st State) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	fillBase(c, pal)

	bright := st.brightness()
	prog := clamp01(st.Progress)
	trail := warpTrailBase + warpTrailGain*prog
	if st.Urgent {
		trail += warpTrailUrgent
	}
	zRange := warpZFar - warpZNear
	focal := float64(max(c.W, c.H*2)) / 2
	cx, cy := float64(c.W)/2, float64(c.H)/2

	type item struct {
		star star
		z    float64
	}
	items := make([]item, 0, len(s.stars))
	for _, star := range s.stars {
		z := warpZNear + wrap(star.z0*zRange-warpSpeed*t, zRange)
		items = append(items, item{star: star, z: z})
	}
	// Paint from the farthest to the nearest so a close head always wins over
	// a distant trail, the same depth order a real perspective field has.
	sort.Slice(items, func(i, j int) bool { return items[i].z > items[j].z })

	for _, it := range items {
		star, z := it.star, it.z
		vis := clamp01((prog - star.appear) / starFade)
		if vis <= 0 {
			continue
		}
		zTail := math.Min(z+warpSpeed*trail, warpZFar-0.001)
		hx := cx + star.wx/z*focal
		hy := cy + star.wy/z*focal*0.5
		tx := cx + star.wx/zTail*focal
		ty := cy + star.wy/zTail*focal*0.5
		if (hx < -2 && tx < -2) || (hx > float64(c.W)+2 && tx > float64(c.W)+2) ||
			(hy < -2 && ty < -2) || (hy > float64(c.H)+2 && ty > float64(c.H)+2) {
			continue
		}

		near := (warpZFar - z) / zRange
		alpha := (0.15 + 0.42*near) * bright * (0.95 + 0.10*prog) * vis
		alpha *= 0.9 + 0.1*math.Sin(t*star.twinkle*2+star.phase)
		if z > warpZFar*0.72 {
			alpha *= math.Max(0, (warpZFar-z)/(warpZFar*0.28))
		}
		if alpha <= 0.01 {
			continue
		}

		dx, dy := hx-tx, hy-ty
		steps := int(math.Ceil(math.Max(math.Abs(dx), math.Abs(dy))))
		if steps < 1 {
			steps = 1
		} else if steps > 64 {
			steps = 64
		}
		tint := pal.Muted
		if star.accent {
			tint = pal.Accent2
		}
		for i := 0; i <= steps; i++ {
			f := float64(i) / float64(steps)
			x := int(math.Round(tx + dx*f))
			y := int(math.Round(ty + dy*f))
			if x < 0 || y < 0 || x >= c.W || y >= c.H {
				continue
			}
			head := i == steps
			a := alpha
			if !head {
				a *= math.Pow(f, 1.6)
			}
			if a < 0.015 {
				continue
			}
			ch := '·'
			col := theme.Lerp(pal.BG, tint, a)
			if head {
				switch {
				case z < 1.2:
					ch = '*'
				case z < 3.0:
					ch = '+'
				}
				col = theme.Lerp(pal.BG, pal.Text, a)
			}
			c.Set(x, y, ch, canvas.Fg(col))
		}
	}
	s.drawShootingStar(c, pal, t, bright)
}

func (s *Stars) drawShootingStar(c *canvas.Canvas, pal theme.Theme, t, bright float64) {
	phase := math.Mod(t+s.shoot.offset, s.shoot.period)
	if phase < 0 || phase >= 0.9 {
		return
	}
	k := phase / 0.9
	sx := s.shoot.x*float64(c.W) + s.shoot.dx*float64(c.W)*k
	sy := s.shoot.y*float64(c.H) + s.shoot.dy*float64(c.H)*k
	vx := s.shoot.dx * float64(c.W)
	vy := s.shoot.dy * float64(c.H)
	mag := math.Hypot(vx, vy)
	if mag == 0 {
		return
	}
	ux, uy := vx/mag, vy/mag
	for i := 0; i < int(s.shoot.length); i++ {
		x := int(sx - ux*float64(i))
		y := int(sy - uy*float64(i))
		alpha := (1 - float64(i)/s.shoot.length) * 0.32 * bright
		c.Set(x, y, '·', canvas.Fg(theme.Lerp(pal.BG, pal.Text, alpha)))
	}
}
