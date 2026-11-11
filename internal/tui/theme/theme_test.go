package theme

import (
	"image/color"
	"testing"
)

func TestGetAndNext(t *testing.T) {
	for _, name := range Names {
		if got := Get(name); got.Name != name {
			t.Errorf("Get(%q).Name = %q", name, got.Name)
		}
	}
	if Get("nope").Name != "midnight" {
		t.Error("unknown theme should fall back to midnight")
	}
	if Next("midnight") != "ember" || Next("ember") != "mono" || Next("mono") != "midnight" {
		t.Error("theme cycle is wrong")
	}
	if Next("nope") != "midnight" {
		t.Error("unknown theme should cycle to midnight")
	}
}

func TestLerpAndScale(t *testing.T) {
	a := color.RGBA{R: 0, G: 100, B: 200, A: 255}
	b := color.RGBA{R: 100, G: 200, B: 250, A: 255}
	if got := Lerp(a, b, 0); got != a {
		t.Fatalf("Lerp(0) = %+v", got)
	}
	if got := Lerp(a, b, 1); got != b {
		t.Fatalf("Lerp(1) = %+v", got)
	}
	if got := Lerp(a, b, 0.5); got.R != 50 || got.G != 150 || got.B != 225 {
		t.Fatalf("Lerp(0.5) = %+v", got)
	}
	if got := Scale(a, 0); got.R != 0 || got.G != 0 || got.B != 0 {
		t.Fatalf("Scale(0) = %+v", got)
	}
	if got := Scale(a, 1); got != a {
		t.Fatalf("Scale(1) = %+v", got)
	}
}

func TestHexRoundTrip(t *testing.T) {
	c := color.RGBA{R: 0x6E, G: 0xE7, B: 0xB7, A: 0xFF}
	if got := Hex(c); got != "#6EE7B7" {
		t.Fatalf("Hex = %q", got)
	}
	if got := RGBA("#6EE7B7"); got != c {
		t.Fatalf("RGBA = %+v", got)
	}
	if got := RGBA("bad"); got != Get("midnight").BG {
		t.Fatalf("bad hex should fall back, got %+v", got)
	}
}

func TestPulseStaysInRange(t *testing.T) {
	a := Get("midnight").Accent
	b := Get("midnight").Danger
	for i := 0.0; i < 10; i += 0.37 {
		c := Pulse(a, b, i)
		if c.A != 0xFF {
			t.Fatalf("alpha = %d", c.A)
		}
	}
}
