package fx

import (
	"math"
	"math/rand"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

type firefly struct {
	x, y         float64
	swayX, swayY float64
	phase        float64
	warm         bool
}

// Fireflies is a small swarm of warm, glowing motes that hover in the lower
// half of the screen. It is intentionally different from the star field: fewer
// points, a visible halo, a strong slow pulse and a looping hover.
type Fireflies struct {
	w, h  int
	flies []firefly
}

func newFireflies(seed int64, w, h int) *Fireflies {
	rng := rand.New(rand.NewSource(seed ^ 0xF17E))
	count := 14 + (w*h)/1200
	if count < 14 {
		count = 14
	}
	if count > 18 {
		count = 18
	}
	f := &Fireflies{w: w, h: h, flies: make([]firefly, 0, count)}
	top := float64(h) * 0.38
	bottom := float64(max(h-1, 1))
	if bottom <= top {
		top = 0
	}
	span := bottom - top
	if span < 2 {
		span = 2
	}
	for i := 0; i < count; i++ {
		swayX := 2 + rng.Float64()*2.5
		swayY := 0.8 + rng.Float64()*0.9
		baseX := 5 + rng.Float64()*float64(max(w-10, 1))
		if w < 12 {
			baseX = rng.Float64() * float64(max(w, 1))
		}
		baseY := top + 1 + rng.Float64()*(span-2)
		f.flies = append(f.flies, firefly{
			x:     baseX,
			y:     baseY,
			swayX: swayX,
			swayY: swayY,
			phase: rng.Float64() * math.Pi * 2,
			warm:  rng.Float64() < 0.66,
		})
	}
	return f
}

func (f *Fireflies) Name() string { return config.BackgroundFireflies }

func (f *Fireflies) Resize(w, h int) {
	if f.w > 0 && f.h > 0 && w > 0 && h > 0 &&
		(w != f.w || h != f.h) {
		sx := float64(w) / float64(f.w)
		sy := float64(h) / float64(f.h)
		for i := range f.flies {
			f.flies[i].x *= sx
			f.flies[i].y *= sy
		}
	}
	f.w, f.h = w, h
}

// position returns the hovering position of one firefly at time t. The motion
// is a slow Lissajous loop: no straight rise, no wrap from the bottom to the
// top, so it never reads like a star.
func (f *Fireflies) position(fly firefly, t float64, w, h int) (float64, float64) {
	x := fly.x + math.Sin(t*0.23+fly.phase)*fly.swayX
	y := fly.y + math.Sin(t*0.17+fly.phase*1.7)*fly.swayY
	if w > 0 {
		if x < 0 {
			x = 0
		} else if x >= float64(w) {
			x = float64(w - 1)
		}
	}
	if h > 0 {
		if y < 0 {
			y = 0
		} else if y >= float64(h) {
			y = float64(h - 1)
		}
	}
	return x, y
}

func (f *Fireflies) Draw(c *canvas.Canvas, pal theme.Theme, t float64, st State) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	fillBase(c, pal)
	bright := st.brightness()
	urgent := st.urgentPulse(t)

	// Trails and halos first, so a neighbour's halo never covers a center.
	for _, fly := range f.flies {
		col := pal.Warn
		if !fly.warm {
			col = pal.Accent
		}
		pulse := 0.5 + 0.5*math.Sin(t*1.1+fly.phase)
		halo := (0.08 + 0.12*pulse) * bright
		if st.Urgent {
			halo += 0.02 * urgent
		}
		for k := 1; k <= 2; k++ {
			tx, ty := f.position(fly, t-0.7*float64(k), c.W, c.H)
			alpha := halo * 0.5 / float64(k)
			c.Set(int(tx), int(ty), '·',
				canvas.Fg(theme.Lerp(pal.BG, col, alpha)))
		}
		x, y := f.position(fly, t, c.W, c.H)
		px, py := int(x), int(y)
		c.Set(px-1, py, '·', canvas.Fg(theme.Lerp(pal.BG, col, halo)))
		c.Set(px+1, py, '·', canvas.Fg(theme.Lerp(pal.BG, col, halo)))
		c.Set(px, py-1, '·', canvas.Fg(theme.Lerp(pal.BG, col, halo)))
		c.Set(px, py+1, '·', canvas.Fg(theme.Lerp(pal.BG, col, halo)))
		diag := halo * 0.55
		c.Set(px-1, py-1, '·', canvas.Fg(theme.Lerp(pal.BG, col, diag)))
		c.Set(px+1, py-1, '·', canvas.Fg(theme.Lerp(pal.BG, col, diag)))
		c.Set(px-1, py+1, '·', canvas.Fg(theme.Lerp(pal.BG, col, diag)))
		c.Set(px+1, py+1, '·', canvas.Fg(theme.Lerp(pal.BG, col, diag)))
	}

	for _, fly := range f.flies {
		col := pal.Warn
		if !fly.warm {
			col = pal.Accent
		}
		pulse := 0.5 + 0.5*math.Sin(t*1.1+fly.phase)
		alpha := (0.24 + 0.24*pulse) * bright
		if st.Urgent {
			alpha += 0.04 * urgent
		}
		x, y := f.position(fly, t, c.W, c.H)
		c.Set(int(x), int(y), '✦',
			canvas.Fg(theme.Lerp(pal.BG, col, alpha)))
	}
}
