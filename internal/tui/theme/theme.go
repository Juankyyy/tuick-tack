// Package theme holds the palette library and color helpers shared by the TUI.
package theme

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"tuick-tack/internal/config"
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

// palettes is the theme library. The IDs and their order live in config; this
// table only maps them to colors. Backgrounds, panels and accents follow the
// upstream palette; muted, accent2 and danger tones are lightened only where
// the TUI needs a readable contrast on a panel.
var palettes = map[string]Theme{
	// Catppuccin Mocha, from Omarchy's Catppuccin palette.
	config.ThemeCatppuccin: {
		Name:    config.ThemeCatppuccin,
		BG:      rgb(0x1E1E2E),
		BG2:     rgb(0x181825),
		Panel:   rgb(0x313244),
		Border:  rgb(0x45475A),
		Text:    rgb(0xCDD6F4),
		Muted:   rgb(0xA6ADC8),
		Accent:  rgb(0x89B4FA),
		Accent2: rgb(0xCBA6F7),
		Warn:    rgb(0xF9E2AF),
		Danger:  rgb(0xF38BA8),
		Confetti: []color.RGBA{
			rgb(0x89B4FA), rgb(0xCBA6F7), rgb(0xA6E3A1),
			rgb(0xF9E2AF), rgb(0xF5C2E7), rgb(0x94E2D5),
		},
	},
	// Tokyo Night, from Omarchy's Tokyo Night palette.
	config.ThemeTokyoNight: {
		Name:    config.ThemeTokyoNight,
		BG:      rgb(0x1A1B26),
		BG2:     rgb(0x16161E),
		Panel:   rgb(0x24283B),
		Border:  rgb(0x292E42),
		Text:    rgb(0xC0CAF5),
		Muted:   rgb(0x9AA5CE),
		Accent:  rgb(0x7AA2F7),
		Accent2: rgb(0xBB9AF7),
		Warn:    rgb(0xE0AF68),
		Danger:  rgb(0xF7768E),
		Confetti: []color.RGBA{
			rgb(0x7AA2F7), rgb(0xBB9AF7), rgb(0x9ECE6A),
			rgb(0xE0AF68), rgb(0xF7768E), rgb(0x7DCFFF),
		},
	},
	// Dracula, from the official Dracula palette.
	config.ThemeDracula: {
		Name:    config.ThemeDracula,
		BG:      rgb(0x282A36),
		BG2:     rgb(0x21222C),
		Panel:   rgb(0x343746),
		Border:  rgb(0x44475A),
		Text:    rgb(0xF8F8F2),
		Muted:   rgb(0x93A0DD),
		Accent:  rgb(0xBD93F9),
		Accent2: rgb(0xFF79C6),
		Warn:    rgb(0xF1FA8C),
		Danger:  rgb(0xFF6B7A),
		Confetti: []color.RGBA{
			rgb(0xBD93F9), rgb(0xFF79C6), rgb(0x50FA7B),
			rgb(0x8BE9FD), rgb(0xF1FA8C), rgb(0xFFB86C),
		},
	},
	// Material Palenight, from the Material Theme palette.
	config.ThemeMaterialPalenight: {
		Name:    config.ThemeMaterialPalenight,
		BG:      rgb(0x292D3E),
		BG2:     rgb(0x1E2132),
		Panel:   rgb(0x333747),
		Border:  rgb(0x444267),
		Text:    rgb(0xA6ACCD),
		Muted:   rgb(0x96A0C8),
		Accent:  rgb(0x82AAFF),
		Accent2: rgb(0xC792EA),
		Warn:    rgb(0xFFCB6B),
		Danger:  rgb(0xF28B90),
		Confetti: []color.RGBA{
			rgb(0x82AAFF), rgb(0xC792EA), rgb(0xC3E88D),
			rgb(0xFFCB6B), rgb(0x89DDFF), rgb(0xF28B90),
		},
	},
	// Gruvbox Dark, from Omarchy's Gruvbox palette.
	config.ThemeGruvbox: {
		Name:    config.ThemeGruvbox,
		BG:      rgb(0x282828),
		BG2:     rgb(0x1E1E1E),
		Panel:   rgb(0x3C3836),
		Border:  rgb(0x504945),
		Text:    rgb(0xEBDBB2),
		Muted:   rgb(0xBDAE93),
		Accent:  rgb(0x7DAEA3),
		Accent2: rgb(0xD8A657),
		Warn:    rgb(0xD8A657),
		Danger:  rgb(0xEF7E75),
		Confetti: []color.RGBA{
			rgb(0x7DAEA3), rgb(0xA9B665), rgb(0xD8A657),
			rgb(0xD3869B), rgb(0xEF7E75), rgb(0xEBDBB2),
		},
	},
	// Nord, from Omarchy's Nord palette.
	config.ThemeNord: {
		Name:    config.ThemeNord,
		BG:      rgb(0x2E3440),
		BG2:     rgb(0x272C36),
		Panel:   rgb(0x3B4252),
		Border:  rgb(0x434C5E),
		Text:    rgb(0xD8DEE9),
		Muted:   rgb(0xA3B0C6),
		Accent:  rgb(0x88C0D0),
		Accent2: rgb(0xC39BD3),
		Warn:    rgb(0xEBCB8B),
		Danger:  rgb(0xE3A1A8),
		Confetti: []color.RGBA{
			rgb(0x88C0D0), rgb(0x81A1C1), rgb(0xA3BE8C),
			rgb(0xEBCB8B), rgb(0xC39BD3), rgb(0xE5E9F0),
		},
	},
	// Everforest Dark, from Omarchy's Everforest palette.
	config.ThemeEverforest: {
		Name:    config.ThemeEverforest,
		BG:      rgb(0x2D353B),
		BG2:     rgb(0x232A2E),
		Panel:   rgb(0x343F44),
		Border:  rgb(0x475258),
		Text:    rgb(0xD3C6AA),
		Muted:   rgb(0xA9B3A9),
		Accent:  rgb(0x7FBBB3),
		Accent2: rgb(0xA7C080),
		Warn:    rgb(0xDBBC7F),
		Danger:  rgb(0xEC8F91),
		Confetti: []color.RGBA{
			rgb(0x7FBBB3), rgb(0xA7C080), rgb(0xDBBC7F),
			rgb(0xD699B6), rgb(0xEC8F91), rgb(0xD3C6AA),
		},
	},
	// Kanagawa Wave, from Omarchy's Kanagawa palette.
	config.ThemeKanagawa: {
		Name:    config.ThemeKanagawa,
		BG:      rgb(0x1F1F28),
		BG2:     rgb(0x181820),
		Panel:   rgb(0x2A2A37),
		Border:  rgb(0x363646),
		Text:    rgb(0xDCD7BA),
		Muted:   rgb(0x9A9580),
		Accent:  rgb(0x7E9CD8),
		Accent2: rgb(0xA792C9),
		Warn:    rgb(0xE6C384),
		Danger:  rgb(0xE87A85),
		Confetti: []color.RGBA{
			rgb(0x7E9CD8), rgb(0xA792C9), rgb(0x98BB6C),
			rgb(0xE6C384), rgb(0xE87A85), rgb(0xDCD7BA),
		},
	},
	// Rose Pine Main, the dark variant of the Rose Pine palette.
	config.ThemeRosePine: {
		Name:    config.ThemeRosePine,
		BG:      rgb(0x191724),
		BG2:     rgb(0x1F1D2E),
		Panel:   rgb(0x26233A),
		Border:  rgb(0x403D52),
		Text:    rgb(0xE0DEF4),
		Muted:   rgb(0x908CAA),
		Accent:  rgb(0xC4A7E7),
		Accent2: rgb(0xEB6F92),
		Warn:    rgb(0xF6C177),
		Danger:  rgb(0xEB6F92),
		Confetti: []color.RGBA{
			rgb(0xC4A7E7), rgb(0xEB6F92), rgb(0x31748F),
			rgb(0xF6C177), rgb(0x9CCFD8), rgb(0xE0DEF4),
		},
	},
	// Osaka Jade, from Omarchy's older Osaka Jade palette.
	config.ThemeOsakaJade: {
		Name:    config.ThemeOsakaJade,
		BG:      rgb(0x111C18),
		BG2:     rgb(0x0D1512),
		Panel:   rgb(0x23372B),
		Border:  rgb(0x2B5147),
		Text:    rgb(0xC1C497),
		Muted:   rgb(0x8CA894),
		Accent:  rgb(0x6FB89A),
		Accent2: rgb(0x2DD5B7),
		Warn:    rgb(0xE5C736),
		Danger:  rgb(0xFF6B5F),
		Confetti: []color.RGBA{
			rgb(0x6FB89A), rgb(0x2DD5B7), rgb(0x549E6A),
			rgb(0xE5C736), rgb(0xD2689C), rgb(0xC1C497),
		},
	},
	// Ristretto, from Omarchy's Ristretto palette.
	config.ThemeRistretto: {
		Name:    config.ThemeRistretto,
		BG:      rgb(0x2C2525),
		BG2:     rgb(0x211B1B),
		Panel:   rgb(0x3D2F2A),
		Border:  rgb(0x403E41),
		Text:    rgb(0xE6D9DB),
		Muted:   rgb(0xB0A0A2),
		Accent:  rgb(0xF38D70),
		Accent2: rgb(0xA8A9EB),
		Warn:    rgb(0xF9CC6C),
		Danger:  rgb(0xFD6883),
		Confetti: []color.RGBA{
			rgb(0xF38D70), rgb(0xA8A9EB), rgb(0xADDA78),
			rgb(0xF9CC6C), rgb(0xFD6883), rgb(0xE6D9DB),
		},
	},
	// Matte Black, from Omarchy's Matte Black palette.
	config.ThemeMatteBlack: {
		Name:    config.ThemeMatteBlack,
		BG:      rgb(0x121212),
		BG2:     rgb(0x0D0D0D),
		Panel:   rgb(0x1E1E1E),
		Border:  rgb(0x2A2A2A),
		Text:    rgb(0xBEBEBE),
		Muted:   rgb(0x8A8A8D),
		Accent:  rgb(0xE68E0D),
		Accent2: rgb(0xF59E0B),
		Warn:    rgb(0xFFC107),
		Danger:  rgb(0xE27A7A),
		Confetti: []color.RGBA{
			rgb(0xE68E0D), rgb(0xF59E0B), rgb(0xFFC107),
			rgb(0xBEBEBE), rgb(0xE27A7A), rgb(0x8A8A8D),
		},
	},
	// Vantablack, from Omarchy's Vantablack palette.
	config.ThemeVantablack: {
		Name:    config.ThemeVantablack,
		BG:      rgb(0x000000),
		BG2:     rgb(0x090909),
		Panel:   rgb(0x1A1A1A),
		Border:  rgb(0x2A2A2A),
		Text:    rgb(0xFFFFFF),
		Muted:   rgb(0xA4A4A4),
		Accent:  rgb(0xCECECE),
		Accent2: rgb(0xB0B0B0),
		Warn:    rgb(0xCECECE),
		Danger:  rgb(0xA4A4A4),
		Confetti: []color.RGBA{
			rgb(0xFFFFFF), rgb(0xCECECE), rgb(0xB0B0B0),
			rgb(0xA4A4A4), rgb(0x8D8D8D), rgb(0xF5F5F5),
		},
	},
}

// Get returns the named theme, falling back to the default theme.
func Get(name string) Theme {
	if t, ok := palettes[name]; ok {
		return t
	}
	return palettes[config.DefaultTheme]
}

func rgb(v uint32) color.RGBA {
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

// RGBA parses #RRGGBB. Invalid input returns the default theme's background.
func RGBA(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return Get(config.DefaultTheme).BG
	}
	var v uint32
	if _, err := fmt.Sscanf(hex, "%06x", &v); err != nil {
		return Get(config.DefaultTheme).BG
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
