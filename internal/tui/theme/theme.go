// Package theme holds the three palettes and color helpers shared by the TUI.
package theme

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

// Theme is one complete palette.
type Theme struct {
	Name     string
	BG       color.RGBA
	BG2      color.RGBA
	Panel    color.RGBA
	Border   color.RGBA
	Text     color.RGBA
	Muted    color.RGBA
	Accent   color.RGBA
	Accent2  color.RGBA
	Warn     color.RGBA
	Danger   color.RGBA
	Confetti []color.RGBA
}

// Names lists themes in cycle order.
var Names = []string{"midnight", "ember", "mono"}

// Get returns the named theme, falling back to midnight.
func Get(name string) Theme {
	switch name {
	case "ember":
		return Theme{
			Name:    "ember",
			BG:      rgb(0x120C0A),
			BG2:     rgb(0x1E100C),
			Panel:   rgb(0x1A1210),
			Border:  rgb(0x3A2520),
			Text:    rgb(0xF7EDE8),
			Muted:   rgb(0x8C7A72),
			Accent:  rgb(0xFFB86B),
			Accent2: rgb(0xFF7A59),
			Warn:    rgb(0xFBBF24),
			Danger:  rgb(0xF87171),
			Confetti: []color.RGBA{
				rgb(0xFFB86B), rgb(0xFF7A59), rgb(0xFBBF24),
				rgb(0xF7EDE8), rgb(0xE85D3D), rgb(0xFFD8A8),
			},
		}
	case "mono":
		return Theme{
			Name:    "mono",
			BG:      rgb(0x0A0A0C),
			BG2:     rgb(0x111114),
			Panel:   rgb(0x121216),
			Border:  rgb(0x2A2A33),
			Text:    rgb(0xF4F4F5),
			Muted:   rgb(0x71717A),
			Accent:  rgb(0xE5E7EB),
			Accent2: rgb(0x9CA3AF),
			Warn:    rgb(0xD4D4D8),
			Danger:  rgb(0xA1A1AA),
			Confetti: []color.RGBA{
				rgb(0xFFFFFF), rgb(0xD4D4D8), rgb(0xA1A1AA),
				rgb(0x71717A), rgb(0xE5E7EB), rgb(0xF4F4F5),
			},
		}
	default:
		return Theme{
			Name:    "midnight",
			BG:      rgb(0x070B14),
			BG2:     rgb(0x0B1524),
			Panel:   rgb(0x0C1524),
			Border:  rgb(0x1E3150),
			Text:    rgb(0xE7ECF5),
			Muted:   rgb(0x6C7A96),
			Accent:  rgb(0x6EE7B7),
			Accent2: rgb(0x60A5FA),
			Warn:    rgb(0xFBBF24),
			Danger:  rgb(0xF87171),
			Confetti: []color.RGBA{
				rgb(0x6EE7B7), rgb(0x60A5FA), rgb(0xA78BFA),
				rgb(0xF472B6), rgb(0xFBBF24), rgb(0xF8FAFC),
			},
		}
	}
}

// Next returns the next theme in cycle order.
func Next(name string) string {
	for i, n := range Names {
		if n == name {
			return Names[(i+1)%len(Names)]
		}
	}
	return Names[0]
}

func rgb(v uint32) color.RGBA {
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

// Hex renders a color as #RRGGBB for Lipgloss and Bubble Tea.
func Hex(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// RGBA parses #RRGGBB. Invalid input returns midnight's background.
func RGBA(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return rgb(0x070B14)
	}
	var v uint32
	if _, err := fmt.Sscanf(hex, "%06x", &v); err != nil {
		return rgb(0x070B14)
	}
	return rgb(v)
}

// Lerp linearly interpolates between a and b. t is clamped to 0..1.
func Lerp(a, b color.RGBA, t float64) color.RGBA {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	return color.RGBA{
		R: uint8(math.Round(float64(a.R) + (float64(b.R)-float64(a.R))*t)),
		G: uint8(math.Round(float64(a.G) + (float64(b.G)-float64(a.G))*t)),
		B: uint8(math.Round(float64(a.B) + (float64(b.B)-float64(a.B))*t)),
		A: 0xFF,
	}
}

// Scale mixes c toward black by factor (0 = black, 1 = unchanged).
func Scale(c color.RGBA, factor float64) color.RGBA {
	return Lerp(color.RGBA{A: 0xFF}, c, factor)
}

// Pulse blends a and b with a sine wave. phase is in radians.
func Pulse(a, b color.RGBA, phase float64) color.RGBA {
	t := (math.Sin(phase) + 1) / 2
	return Lerp(a, b, t)
}

// Luminance returns the WCAG relative luminance of c (0 is black, 1 is white).
func Luminance(c color.RGBA) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

// Contrast returns the WCAG contrast ratio between two colors (1..21).
func Contrast(a, b color.RGBA) float64 {
	la, lb := Luminance(a), Luminance(b)
	if lb > la {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Ink keeps the hue of c but mixes it toward black until it reaches at least
// minContrast against bg. Light stage text uses it so a transparent background
// stays readable over a light terminal. Colors that already meet the target
// are returned unchanged; if even black cannot reach it, black is returned.
func Ink(c, bg color.RGBA, minContrast float64) color.RGBA {
	if minContrast <= 1 || Contrast(c, bg) >= minContrast {
		return c
	}
	black := color.RGBA{A: 0xFF}
	if Contrast(black, bg) < minContrast {
		return black
	}
	// Contrast grows monotonically as the mix approaches black, so a binary
	// search finds the lightest variant that still meets the target.
	lo, hi := 0.0, 1.0
	for i := 0; i < 18; i++ {
		mid := (lo + hi) / 2
		if Contrast(Lerp(black, c, mid), bg) >= minContrast {
			lo = mid
		} else {
			hi = mid
		}
	}
	return Lerp(black, c, lo)
}

// HSL converts a hue (0..1), saturation (0..1) and lightness (0..1) to an
// opaque color. Hue wraps around; saturation and lightness are clamped.
func HSL(h, s, l float64) color.RGBA {
	h = math.Mod(h, 1)
	if h < 0 {
		h++
	}
	s = clamp01(s)
	l = clamp01(l)
	c := (1 - math.Abs(2*l-1)) * s
	hp := h * 6
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g, b = c, x, 0
	case hp < 2:
		r, g, b = x, c, 0
	case hp < 3:
		r, g, b = 0, c, x
	case hp < 4:
		r, g, b = 0, x, c
	case hp < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	m := l - c/2
	return color.RGBA{
		R: channel(r + m),
		G: channel(g + m),
		B: channel(b + m),
		A: 0xFF,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func channel(v float64) uint8 {
	return uint8(math.Round(clamp01(v) * 255))
}
