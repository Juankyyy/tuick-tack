package fx

import (
	"math"
	"testing"

	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

func TestDustDeterministic(t *testing.T) {
	a := NewDust(42, 80, 24)
	b := NewDust(42, 80, 24)
	for i := range a.motes {
		if a.motes[i] != b.motes[i] {
			t.Fatalf("mote %d differs: %+v vs %+v", i, a.motes[i], b.motes[i])
		}
	}
}

func TestDustUpdateBounds(t *testing.T) {
	d := NewDust(7, 20, 10)
	for i := 0; i < 500; i++ {
		d.Update(0.1)
	}
	for i, m := range d.motes {
		if math.IsNaN(m.x) || math.IsNaN(m.y) {
			t.Fatalf("mote %d is NaN", i)
		}
		if m.x < 0 || m.x >= 20 || m.y < 0 || m.y >= 10 {
			t.Fatalf("mote %d out of bounds: %+v", i, m)
		}
	}
}

func TestConfettiBursts(t *testing.T) {
	th := theme.Get("midnight")
	full := NewCelebration(1, 80, 24, th.Confetti, th.BG, false)
	if full.Count() != 230 {
		t.Fatalf("particles = %d, want 230", full.Count())
	}
	if full.Count() > maxParticles {
		t.Fatalf("particle cap exceeded: %d", full.Count())
	}

	short := NewCelebration(1, 80, 24, th.Confetti, th.BG, true)
	if short.Count() != 90 {
		t.Fatalf("short burst = %d, want 90", short.Count())
	}

	big := NewCelebration(1, 120, 40, th.Confetti, th.BG, false)
	if big.Count() <= full.Count() {
		t.Fatalf("120x40 count = %d, should exceed 80x24 count %d", big.Count(), full.Count())
	}
	if big.Count() > maxParticles {
		t.Fatalf("120x40 cap exceeded: %d", big.Count())
	}

	tiny := NewCelebration(1, 40, 12, th.Confetti, th.BG, false)
	if tiny.Count() <= 0 || tiny.Count() >= full.Count() {
		t.Fatalf("40x12 count = %d, want between 1 and %d", tiny.Count(), full.Count())
	}
}

func TestConfettiExpires(t *testing.T) {
	th := theme.Get("midnight")
	c := NewCelebration(2, 80, 24, th.Confetti, th.BG, false)
	if c.Done() {
		t.Fatal("confetti reported done before the bursts fired")
	}
	for i := 0; i < 300; i++ {
		c.Update(0.05)
	}
	if !c.Done() {
		t.Fatalf("confetti still active after %.1fs", c.Elapsed())
	}
	for i, p := range c.particles {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			t.Fatalf("particle %d has invalid position: %+v", i, p)
		}
	}
}

func TestConfettiResize(t *testing.T) {
	th := theme.Get("mono")
	c := NewCelebration(3, 120, 40, th.Confetti, th.BG, false)
	for i := 0; i < 10; i++ {
		c.Update(0.05)
	}
	c.Resize(20, 10)
	placed := 0
	for i, p := range c.particles {
		if !p.Placed {
			continue
		}
		placed++
		if p.X < -3 || p.X > 23 || p.Y < -3 || p.Y > 13 {
			t.Fatalf("particle %d not remapped: %+v", i, p)
		}
	}
	if placed == 0 {
		t.Fatal("no particles were active before the resize")
	}
}

func TestConfettiResizeBeforeBurst(t *testing.T) {
	th := theme.Get("midnight")
	c := NewCelebration(4, 120, 40, th.Confetti, th.BG, false)
	c.Resize(80, 24)
	c.Update(0.05)
	placed := 0
	for i, p := range c.particles {
		if !p.Placed {
			continue
		}
		placed++
		if p.X < -3 || p.X > 83 || p.Y < -3 || p.Y > 27 {
			t.Fatalf("particle %d placed outside the resized screen: %+v", i, p)
		}
	}
	if placed == 0 {
		t.Fatal("no particle fired after the resize")
	}
}

func TestConfettiDeterministic(t *testing.T) {
	th := theme.Get("midnight")
	a := NewCelebration(42, 80, 24, th.Confetti, th.BG, false)
	b := NewCelebration(42, 80, 24, th.Confetti, th.BG, false)
	for i := 0; i < 100; i++ {
		a.Update(0.033)
		b.Update(0.033)
	}
	if len(a.particles) != len(b.particles) {
		t.Fatalf("particle counts differ: %d vs %d", len(a.particles), len(b.particles))
	}
	for i := range a.particles {
		pa, pb := &a.particles[i], &b.particles[i]
		if pa.X != pb.X || pa.Y != pb.Y || pa.Age != pb.Age || pa.Active != pb.Active {
			t.Fatalf("particle %d diverged: %+v vs %+v", i, *pa, *pb)
		}
	}
}

func TestConfettiFlutterSpeed(t *testing.T) {
	th := theme.Get("midnight")
	c := NewCelebration(5, 80, 24, th.Confetti, th.BG, false)
	maxFall := 0.0
	for i := 0; i < 200; i++ {
		c.Update(0.05)
		for j := range c.particles {
			p := &c.particles[j]
			if p.Active && p.VY > maxFall {
				maxFall = p.VY
			}
		}
	}
	if maxFall > 12 {
		t.Fatalf("confetti falls too fast: %.1f rows/s", maxFall)
	}
}

func TestConfettiGlyphsSingleCell(t *testing.T) {
	for i, f := range paperFamilies {
		for _, g := range []rune{f.front, f.back, f.edge} {
			if !canvas.SingleCell(g) {
				t.Fatalf("paper family %d has a non-single-cell rune %q", i, g)
			}
		}
	}
	for _, g := range []rune{sparkFamily.front, sparkFamily.back, sparkFamily.edge, trailGlyph} {
		if !canvas.SingleCell(g) {
			t.Fatalf("spark/trail has a non-single-cell rune %q", g)
		}
	}
}

func TestConfettiDraw(t *testing.T) {
	th := theme.Get("midnight")
	c := NewCelebration(6, 80, 24, th.Confetti, th.BG, false)
	cv := canvas.New(80, 24)
	cv.Clear(th.BG)
	for i := 0; i < 30; i++ {
		c.Update(0.033)
	}
	c.Draw(cv)
	c.Draw(nil) // must not panic
	painted := 0
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			if cv.At(x, y).Ch != ' ' {
				painted++
			}
		}
	}
	if painted == 0 {
		t.Fatal("confetti drew nothing")
	}
}
