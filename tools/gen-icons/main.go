// Command gen-icons draws the tray and executable icons into assets/.
//
// The icons are original clock glyphs rendered with supersampling so they stay
// readable at 16 px. Each .ico carries PNG images for 16, 32 and 48 px (the
// executable icon also includes 256 px).
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type state struct {
	name  string
	ring  color.NRGBA
	badge string
}

func main() {
	root := repoRoot()
	states := []state{
		{name: "idle", ring: color.NRGBA{R: 0x6E, G: 0xE7, B: 0xB7, A: 0xFF}, badge: "dot"},
		{name: "running", ring: color.NRGBA{R: 0x60, G: 0xA5, B: 0xFA, A: 0xFF}, badge: "hand"},
		{name: "paused", ring: color.NRGBA{R: 0xFB, G: 0xBF, B: 0x24, A: 0xFF}, badge: "pause"},
		{name: "finished", ring: color.NRGBA{R: 0xF8, G: 0x71, B: 0x71, A: 0xFF}, badge: "bang"},
	}

	if err := os.MkdirAll(filepath.Join(root, "assets", "icons"), 0o755); err != nil {
		fail(err)
	}
	for _, s := range states {
		img := draw(s, 256)
		if err := writeICO(filepath.Join(root, "assets", "icons", s.name+".ico"), img, []int{16, 32, 48}); err != nil {
			fail(err)
		}
		fmt.Printf("gen-icons: wrote assets/icons/%s.ico\n", s.name)
	}
	// The executable icon uses the idle artwork and adds a 256 px image.
	if err := writeICO(filepath.Join(root, "assets", "icon.ico"), draw(states[0], 256), []int{16, 32, 48, 256}); err != nil {
		fail(err)
	}
	fmt.Println("gen-icons: wrote assets/icon.ico")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-icons:", err)
	os.Exit(1)
}

// draw renders the icon at the requested size with 4x supersampling.
func draw(s state, size int) image.Image {
	const ss = 4
	big := size * ss
	// Accumulate coverage and color at high resolution, then downsample.
	accR := make([]float64, size*size)
	accG := make([]float64, size*size)
	accB := make([]float64, size*size)
	accA := make([]float64, size*size)

	ring := float64(big) * 0.36
	stroke := float64(big) * 0.095
	cx := float64(big) * 0.5
	cy := float64(big) * 0.5
	white := color.NRGBA{R: 0xF4, G: 0xF7, B: 0xFB, A: 0xFF}

	for y := 0; y < big; y++ {
		for x := 0; x < big; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			dx := px - cx
			dy := py - cy
			dist := math.Hypot(dx, dy)
			var c color.NRGBA
			alpha := 0.0

			// Antialiased ring.
			if ringCoverage(dist, ring, stroke/2) > 0 {
				alpha = ringCoverage(dist, ring, stroke/2)
				c = s.ring
			}

			// State glyph inside the ring.
			ga, gc := glyph(s.badge, dx, dy, float64(big), white, s.ring)
			if ga > alpha {
				alpha = ga
				c = gc
			}

			si := (y/ss)*size + (x / ss)
			accR[si] += float64(c.R) / 255 * alpha
			accG[si] += float64(c.G) / 255 * alpha
			accB[si] += float64(c.B) / 255 * alpha
			accA[si] += alpha
		}
	}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	inv := 1.0 / float64(ss*ss)
	for i := 0; i < size*size; i++ {
		a := accA[i] * inv
		if a <= 0 {
			continue
		}
		r := accR[i] / accA[i]
		g := accG[i] / accA[i]
		b := accB[i] / accA[i]
		img.SetNRGBA(i%size, i/size, color.NRGBA{
			R: clamp8(r),
			G: clamp8(g),
			B: clamp8(b),
			A: clamp8(a),
		})
	}
	return img
}

func ringCoverage(dist, radius, halfStroke float64) float64 {
	d := math.Abs(dist - radius)
	if d >= halfStroke+0.75 {
		return 0
	}
	if d <= halfStroke-0.75 {
		return 1
	}
	return math.Max(0, math.Min(1, halfStroke+0.75-d))
}

func glyph(kind string, dx, dy, size float64, white, ring color.NRGBA) (float64, color.NRGBA) {
	unit := size / 100
	switch kind {
	case "dot":
		d := math.Hypot(dx, dy)
		if d < 9*unit {
			return 1, white
		}
	case "hand":
		// Minute hand up, hour hand right.
		if math.Abs(dx) < 4.5*unit && dy < 2*unit && dy > -26*unit {
			return 1, white
		}
		if math.Abs(dy) < 4.5*unit && dx > 2*unit && dx < 18*unit {
			return 1, white
		}
		if math.Hypot(dx, dy) < 7*unit {
			return 1, white
		}
	case "pause":
		if math.Abs(dx+8*unit) < 5*unit && math.Abs(dy) < 18*unit {
			return 1, white
		}
		if math.Abs(dx-8*unit) < 5*unit && math.Abs(dy) < 18*unit {
			return 1, white
		}
	case "bang":
		if math.Abs(dx) < 5*unit && dy > -23*unit && dy < 8*unit {
			return 1, white
		}
		if math.Hypot(dx, dy-18*unit) < 5*unit {
			return 1, white
		}
	}
	return 0, ring
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(math.Round(v * 255))
}

func writeICO(path string, src image.Image, sizes []int) error {
	base := src.Bounds()
	var payloads [][]byte
	for _, size := range sizes {
		img := scaleImage(src, size, base)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		payloads = append(payloads, buf.Bytes())
	}

	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1)) // icon
	binary.Write(&out, binary.LittleEndian, uint16(len(sizes)))

	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		b := byte(size)
		if size >= 256 {
			b = 0
		}
		out.WriteByte(b) // width
		out.WriteByte(b) // height
		out.WriteByte(0) // palette
		out.WriteByte(0) // reserved
		binary.Write(&out, binary.LittleEndian, uint16(1))
		binary.Write(&out, binary.LittleEndian, uint16(32))
		binary.Write(&out, binary.LittleEndian, uint32(len(payloads[i])))
		binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(payloads[i])
	}
	for _, p := range payloads {
		out.Write(p)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

func scaleImage(src image.Image, size int, bounds image.Rectangle) image.Image {
	if bounds.Dx() == size && bounds.Dy() == size {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/size
			sy := bounds.Min.Y + y*bounds.Dy()/size
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

// repoRoot walks up from the working directory until it finds go.mod.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
