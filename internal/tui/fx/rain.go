package fx

import (
	"math/rand"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

const rainGlyphs = ":/\\|<>=+-"

type rainColumn struct {
	x, speed, length, offset float64
	glyphs                   []rune
}

// Rain is a sparse field of slow, falling glyph columns. It is deliberately
// slower and dimmer than the usual "matrix" effect so the countdown stays
// readable.
type Rain struct {
	w, h int
	cols []rainColumn
}

func newRain(seed int64, w, h int) *Rain {
	r := &Rain{w: w, h: h}
	if w <= 0 || h <= 0 {
		return r
	}
	rng := rand.New(rand.NewSource(seed ^ 0x2A1D))
	count := w / 6
	if count < 10 {
		count = 10
	}
	if count > 24 {
		count = 24
	}
	chars := []rune(rainGlyphs)
	r.cols = make([]rainColumn, 0, count)
	for i := 0; i < count; i++ {
		length := 6 + rng.Float64()*8
		glyphs := make([]rune, 16)
		for j := range glyphs {
			glyphs[j] = chars[rng.Intn(len(chars))]
		}
		x := (float64(i)+0.5)*float64(w)/float64(count) +
			(rng.Float64()*2-1)*1.5
		if x < 0 {
			x = 0
		} else if x >= float64(w) {
			x = float64(w - 1)
		}
		r.cols = append(r.cols, rainColumn{
			x:      x,
			speed:  2.5 + rng.Float64()*2,
			length: length,
			offset: rng.Float64() * (float64(h) + length),
			glyphs: glyphs,
		})
	}
	return r
}

func (r *Rain) Name() string { return config.BackgroundRain }

func (r *Rain) Resize(w, h int) {
	if r.w > 0 && w > 0 && w != r.w {
		sx := float64(w) / float64(r.w)
		for i := range r.cols {
			r.cols[i].x *= sx
		}
	}
	if r.h > 0 && h > 0 && h != r.h {
		sy := float64(h) / float64(r.h)
		for i := range r.cols {
			r.cols[i].offset *= sy
		}
	}
	r.w, r.h = w, h
}

func (r *Rain) Draw(c *canvas.Canvas, pal theme.Theme, t float64, st State) {
	if c == nil || c.W <= 0 || c.H <= 0 {
		return
	}
	fillBase(c, pal)
	bright := st.brightness()
	for _, col := range r.cols {
		head := wrap(col.offset+t*col.speed,
			float64(c.H)+col.length) - col.length
		for j := 0; j < int(col.length); j++ {
			y := head - float64(j)
			if y < 0 || y >= float64(c.H) {
				continue
			}
			glyph := col.glyphs[j%len(col.glyphs)]
			alpha := (0.05 + 0.25*(1-float64(j)/col.length)) * bright
			colr := pal.Accent2
			if j == 0 {
				alpha = 0.4 * bright
				colr = pal.Text
			}
			c.Set(int(col.x), int(y), glyph,
				canvas.Fg(theme.Lerp(pal.BG, colr, alpha)))
		}
	}
}
