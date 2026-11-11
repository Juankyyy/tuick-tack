package fx

import (
	"math"
	"testing"

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
	c := NewCelebration(1, 80, 24, th.Confetti, th.BG, false)
	if c.Count() != 220 {
		t.Fatalf("particles = %d, want 220", c.Count())
	}
	if c.Count() > 260 {
		t.Fatalf("particle cap exceeded: %d", c.Count())
	}
	short := NewCelebration(1, 80, 24, th.Confetti, th.BG, true)
	if short.Count() != 60 {
		t.Fatalf("short burst = %d, want 60", short.Count())
	}
}

func TestConfettiExpires(t *testing.T) {
	th := theme.Get("midnight")
	c := NewCelebration(2, 80, 24, th.Confetti, th.BG, false)
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
	for i, p := range c.particles {
		if p.X < -2 || p.X > 22 || p.Y < -2 || p.Y > 12 {
			t.Fatalf("particle %d not clamped: %+v", i, p)
		}
	}
}
