package fx

import (
	"image/color"
	"math"
	"testing"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

func testLuminance(c color.RGBA) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

func TestFirefliesAreLowAndGlow(t *testing.T) {
	pal := theme.Get("midnight")
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		f := newFireflies(9, size[0], size[1])
		if n := len(f.flies); n < 14 || n > 18 {
			t.Fatalf("%dx%d: count = %d, want 14..18", size[0], size[1], n)
		}
		for _, tm := range []float64{0, 12.34, 47.6} {
			c := canvas.New(size[0], size[1])
			f.Draw(c, pal, tm, State{})
			centers, dots, low := 0, 0, 0
			top := int(float64(size[1])*0.38) - 1
			for y := 0; y < c.H; y++ {
				for x := 0; x < c.W; x++ {
					cell := c.At(x, y)
					switch cell.Ch {
					case '✦':
						centers++
						if y < top {
							low++
						}
					case '·':
						dots++
					}
				}
			}
			if low > 0 {
				t.Fatalf("%dx%d t=%.2f: %d firefly centers above the lower band",
					size[0], size[1], tm, low)
			}
			if centers < len(f.flies)/2 {
				t.Fatalf("%dx%d t=%.2f: only %d centers for %d fireflies",
					size[0], size[1], tm, centers, len(f.flies))
			}
			if dots < len(f.flies)*3 {
				t.Fatalf("%dx%d t=%.2f: only %d halo/trail cells for %d fireflies",
					size[0], size[1], tm, dots, len(f.flies))
			}
		}
	}
}

func TestFirefliesLookDifferentFromStars(t *testing.T) {
	pal := theme.Get("midnight")
	fireflies := canvas.New(80, 24)
	NewBackdrop(config.BackgroundFireflies, 9, 80, 24).Draw(fireflies, pal, 12.34, State{})
	stars := canvas.New(80, 24)
	NewBackdrop(config.BackgroundStars, 9, 80, 24).Draw(stars, pal, 12.34, State{})
	fireflyCenters, starCenters := 0, 0
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			if fireflies.At(x, y).Ch == '✦' {
				fireflyCenters++
			}
			if stars.At(x, y).Ch == '✦' {
				starCenters++
			}
		}
	}
	if fireflyCenters == 0 {
		t.Fatal("fireflies should draw a distinct ✦ center")
	}
	if starCenters != 0 {
		t.Fatalf("stars should not use the firefly center glyph (%d found)", starCenters)
	}
}

func TestStarsGrowWithProgress(t *testing.T) {
	s := newStars(9, 80, 24)
	if got := len(s.stars); got != starPool {
		t.Fatalf("star pool = %d, want %d", got, starPool)
	}
	if got := visibleStars(s, 0); got != starBase {
		t.Fatalf("stars at 0%% = %d, want %d", got, starBase)
	}
	if got := visibleStars(s, 1); got != starPool {
		t.Fatalf("stars at 100%% = %d, want %d", got, starPool)
	}
	prev := 0
	for _, p := range []float64{0, 0.25, 0.5, 0.75, 1} {
		got := visibleStars(s, p)
		if got < prev {
			t.Fatalf("visible stars fell at %.0f%%: %d after %d", p*100, got, prev)
		}
		prev = got
	}
}

// visibleStars mirrors the fade window used by Draw. A star is visible once
// its threshold is below the current progress.
func visibleStars(s *Stars, progress float64) int {
	n := 0
	for _, star := range s.stars {
		if (progress-star.appear)/starFade > 0 {
			n++
		}
	}
	return n
}

func TestStarsWarpOutward(t *testing.T) {
	s := newStars(9, 80, 24)
	var sample star
	found := false
	for _, candidate := range s.stars {
		if candidate.z0 > 0.2 {
			sample = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no star with enough depth for the sample")
	}
	zAt := func(tm float64) float64 {
		return warpZNear + wrap(sample.z0*(warpZFar-warpZNear)-warpSpeed*tm, warpZFar-warpZNear)
	}
	z0, z1 := zAt(0), zAt(0.25)
	if z1 >= z0 {
		t.Fatalf("z did not decrease: %.3f -> %.3f", z0, z1)
	}
	focal := float64(max(80, 24*2)) / 2
	radius := func(z float64) float64 {
		return math.Hypot(sample.wx/z*focal, sample.wy/z*focal*0.5)
	}
	if radius(z1) <= radius(z0) {
		t.Fatalf("star did not move outward: radius %.3f -> %.3f", radius(z0), radius(z1))
	}
}

func starGlyphs(c *canvas.Canvas) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			switch c.At(x, y).Ch {
			case '·', '+', '*':
				n++
			}
		}
	}
	return n
}

func TestStarsDrawMoreWithProgress(t *testing.T) {
	pal := theme.Get("midnight")
	low := canvas.New(120, 40)
	newStars(9, 120, 40).Draw(low, pal, 12.34, State{Progress: 0})
	high := canvas.New(120, 40)
	newStars(9, 120, 40).Draw(high, pal, 12.34, State{Progress: 1})
	if got, want := starGlyphs(high), starGlyphs(low); got <= want {
		t.Fatalf("100%% drew %d star cells, 0%% drew %d", got, want)
	}
}

func TestSunsetHasVisibleSun(t *testing.T) {
	pal := theme.Get("midnight")
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		c := canvas.New(size[0], size[1])
		NewBackdrop(config.BackgroundGrid, 9, size[0], size[1]).
			Draw(c, pal, 12.34, State{})
		hy := int(math.Round(float64(size[1]) * 0.72))
		warm := 0
		worst := 0.0
		bright := 0
		worstX, worstY := 0, 0
		for y := 0; y < hy; y++ {
			for x := 0; x < c.W; x++ {
				bg := c.At(x, y).BG
				lum := testLuminance(bg)
				if lum > worst {
					worst = lum
					worstX, worstY = x, y
				}
				if lum > 0.45 {
					bright++
				}
				if bg.R > bg.B+8 && lum > 0.06 {
					warm++
				}
			}
		}
		if warm < c.W/4 {
			t.Fatalf("%dx%d: only %d warm sun cells above the horizon", size[0], size[1], warm)
		}
		if worst > 0.70 {
			t.Fatalf("%dx%d: sun luminance %.3f at %d,%d (%v) exceeds 0.70",
				size[0], size[1], worst, worstX, worstY, c.At(worstX, worstY).BG)
		}
		if bright > c.W/3 {
			t.Fatalf("%dx%d: %d bright cells above the horizon, want a thin rim", size[0], size[1], bright)
		}
		center := testLuminance(c.At(c.W/2, hy-3).BG)
		edge := testLuminance(c.At(2, hy-3).BG)
		if center <= edge {
			t.Fatalf("%dx%d: sun center %.3f is not brighter than the sky %.3f",
				size[0], size[1], center, edge)
		}
	}
}
