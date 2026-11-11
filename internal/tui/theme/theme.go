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
