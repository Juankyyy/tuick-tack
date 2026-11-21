package fx

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/theme"
)

func backdropHash(c *canvas.Canvas) string {
	sum := sha256.New()
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			cell := c.At(x, y)
			sum.Write([]byte{cell.BG.R, cell.BG.G, cell.BG.B})
		}
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// TestAuroraMatchesOriginal compares the extracted aurora with the background
// that view.go painted before the fx.Backdrop layer existed. The hashes were
// captured from that implementation at fixed sizes and times, with the
// celebration flash disabled.
func TestAuroraMatchesOriginal(t *testing.T) {
	cases := []struct {
		theme string
		w, h  int
		t     float64
		want  string
	}{
		{"midnight", 80, 24, 0, "d7f467d6a0a59a81754f794b78e691dc6645af2a98b55488a51bff687ce3661e"},
		{"midnight", 80, 24, 12.34, "5820dd866e3fb3c25bf04284d2c6dab611271f30ebdb3b1898f2c657e6ffbc96"},
		{"midnight", 120, 40, 12.34, "ea1d48801dd6fd61027ffd042e75cfefd92cb9fc6a978c683985ba6222409250"},
		{"midnight", 120, 40, 47.6, "cf153efbcf5dbe61f393ae1937c6f0ebb048e2fbdf1770d628f4dd8b922e8b0d"},
		{"ember", 80, 24, 12.34, "f2b37806f8894034923cfe48c5702778e81f3e5a736b79a2578a297fe92ec0a8"},
	}
	for _, tc := range cases {
		b := NewBackdrop(config.BackgroundAurora, 1, tc.w, tc.h)
		c := canvas.New(tc.w, tc.h)
		b.Draw(c, theme.Get(tc.theme), tc.t, State{})
		if got := backdropHash(c); got != tc.want {
			t.Errorf("%s %dx%d t=%.2f: hash %s, want %s",
				tc.theme, tc.w, tc.h, tc.t, got, tc.want)
		}
	}
}

func TestBackdropCatalog(t *testing.T) {
	pal := theme.Get("midnight")
	sizes := [][2]int{{0, 0}, {48, 22}, {80, 24}, {120, 40}}
	states := []State{
		{},
		{Paused: true},
		{Urgent: true},
		{Celebrating: true},
		{Progress: 1, Urgent: true},
	}
	for _, name := range config.Backgrounds {
		t.Run(name, func(t *testing.T) {
			b := NewBackdrop(name, 1234, 80, 24)
			if got := b.Name(); got != name {
				t.Fatalf("Name() = %q, want %q", got, name)
			}
			for _, size := range sizes {
				b.Resize(size[0], size[1])
				for _, st := range states {
					c := canvas.New(size[0], size[1])
					b.Draw(c, pal, 12.34, st)
					for y := 0; y < c.H; y++ {
						for x := 0; x < c.W; x++ {
							cell := c.At(x, y)
							if cell.Ch == '?' || !canvas.SingleCell(cell.Ch) {
								t.Fatalf("%dx%d state %+v: cell %d,%d has rune %q",
									size[0], size[1], st, x, y, cell.Ch)
							}
							if name == config.BackgroundNone {
								if cell.HasFG || cell.HasBG {
									t.Fatalf("%dx%d state %+v: none painted cell %d,%d: %+v",
										size[0], size[1], st, x, y, cell)
								}
								continue
							}
							if !cell.HasBG {
								t.Fatalf("%dx%d state %+v: cell %d,%d has no background",
									size[0], size[1], st, x, y)
							}
						}
					}
				}
			}
		})
	}
}

func TestBackdropDeterministic(t *testing.T) {
	pal := theme.Get("ember")
	for _, name := range config.Backgrounds {
		t.Run(name, func(t *testing.T) {
			a := canvas.New(120, 40)
			b := canvas.New(120, 40)
			NewBackdrop(name, 77, 120, 40).Draw(a, pal, 12.34, State{Progress: 0.4})
			NewBackdrop(name, 77, 120, 40).Draw(b, pal, 12.34, State{Progress: 0.4})
			if got, want := backdropHash(a), backdropHash(b); got != want {
				t.Fatalf("same seed and time drew different frames: %s vs %s", got, want)
			}
		})
	}
}

func TestBackdropResize(t *testing.T) {
	pal := theme.Get("mono")
	for _, name := range config.Backgrounds {
		t.Run(name, func(t *testing.T) {
			b := NewBackdrop(name, 5, 120, 40)
			for _, size := range [][2]int{{120, 40}, {48, 22}, {0, 0}, {120, 40}} {
				b.Resize(size[0], size[1])
				c := canvas.New(size[0], size[1])
				b.Draw(c, pal, 3.5, State{Progress: 0.7})
				if c.W != size[0] || c.H != size[1] {
					t.Fatalf("canvas dimensions %dx%d, want %dx%d",
						c.W, c.H, size[0], size[1])
				}
			}
		})
	}
}

func TestBackdropUnknownFallsBackToAurora(t *testing.T) {
	for _, name := range []string{"", "neon", "AURORA"} {
		b := NewBackdrop(name, 1, 80, 24)
		if b.Name() != config.BackgroundAurora {
			t.Fatalf("%q fell back to %q, want aurora", name, b.Name())
		}
	}
}

func TestNoneIsTransparent(t *testing.T) {
	for _, name := range []string{"midnight", "ember", "mono"} {
		pal := theme.Get(name)
		c := canvas.New(80, 24)
		NewBackdrop(config.BackgroundNone, 1, 80, 24).Draw(c, pal, 9, State{})
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				cell := c.At(x, y)
				if cell.Ch != ' ' || cell.HasFG || cell.HasBG || cell.Bold {
					t.Fatalf("%s: cell %d,%d is not transparent: %+v",
						name, x, y, cell)
				}
			}
		}
	}
}

// TestBackdropContrast keeps every background well below the text so the
// countdown remains the brightest thing on screen.
func TestBackdropContrast(t *testing.T) {
	const maxLuminance = 0.45
	for _, themeName := range theme.Names {
		pal := theme.Get(themeName)
		for _, name := range config.Backgrounds {
			t.Run(themeName+"/"+name, func(t *testing.T) {
				limit := maxLuminance
				if name == config.BackgroundGrid {
					// The sun rim is intentionally allowed to be the one bright
					// accent inside the backdrop; its area is checked separately in
					// TestSunsetHasVisibleSun.
					limit = 0.70
				}
				for _, st := range []State{{Progress: 0.8}, {Progress: 1, Urgent: true}} {
					c := canvas.New(120, 40)
					NewBackdrop(name, 42, 120, 40).Draw(c, pal, 12.34, st)
					worst := 0.0
					for y := 0; y < c.H; y++ {
						for x := 0; x < c.W; x++ {
							bg := c.At(x, y).BG
							lum := (0.2126*float64(bg.R) +
								0.7152*float64(bg.G) +
								0.0722*float64(bg.B)) / 255
							if lum > worst {
								worst = lum
							}
						}
					}
					if worst > limit {
						t.Fatalf("state %+v: background luminance %.3f exceeds %.2f",
							st, worst, limit)
					}
				}
			})
		}
	}
}

func BenchmarkBackdrop(b *testing.B) {
	pal := theme.Get("midnight")
	for _, name := range config.Backgrounds {
		b.Run(name, func(b *testing.B) {
			backdrop := NewBackdrop(name, 3, 120, 40)
			c := canvas.New(120, 40)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				backdrop.Draw(c, pal, 12.34, State{Progress: 0.5})
			}
		})
	}
}
