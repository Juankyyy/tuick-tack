package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func trayArt() []struct {
	name string
	art  sprite
} {
	return []struct {
		name string
		art  sprite
	}{
		{"idle", solidSprite(idleRows, idleColor)},
		{"running", solidSprite(runningRows, runningColor)},
		{"paused", solidSprite(pausedRows, pausedColor)},
		{"finished", solidSprite(finishedRows, finishedColor)},
	}
}

func TestSpriteMaps(t *testing.T) {
	tray := trayArt()
	for _, tc := range tray {
		checkMap(t, tc.name, tc.art, 7)
	}
	checkMap(t, "app", appSprite, 16)
}

func checkMap(t *testing.T, name string, s sprite, grid int) {
	t.Helper()
	if len(s.rows) != grid {
		t.Fatalf("%s: %d rows, want %d", name, len(s.rows), grid)
	}
	for y, row := range s.rows {
		if len(row) != grid {
			t.Errorf("%s row %d: %d cells, want %d", name, y, len(row), grid)
		}
		for x, ch := range row {
			if ch == '.' {
				continue
			}
			if _, ok := s.colors[ch]; !ok {
				t.Errorf("%s cell (%d,%d): %q has no color", name, x, y, ch)
			}
		}
	}
}

func TestRenderSizes(t *testing.T) {
	for _, s := range []sprite{
		solidSprite(idleRows, idleColor),
		solidSprite(runningRows, runningColor),
		solidSprite(pausedRows, pausedColor),
		solidSprite(finishedRows, finishedColor),
		appSprite,
	} {
		for _, size := range []int{16, 24, 32, 48, 64, 128, 256} {
			img := s.render(size)
			if got := img.Bounds(); got != image.Rect(0, 0, size, size) {
				t.Fatalf("render(%d) bounds = %v", size, got)
			}
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					if a := img.NRGBAAt(x, y).A; a != 0 && a != 255 {
						t.Fatalf("render(%d) pixel (%d,%d) alpha = %d, want 0 or 255", size, x, y, a)
					}
				}
			}
		}
	}
}

func TestTrayGlyphs(t *testing.T) {
	// The 16 px tray art is 14 px wide with a 1 px margin and 2 px cells.
	const size = 16
	idle := solidSprite(idleRows, idleColor).render(size)
	if got := idle.NRGBAAt(0, 0); got.A != 0 {
		t.Errorf("idle corner alpha = %d, want 0", got.A)
	}
	if got := idle.NRGBAAt(1, 7); got != idleColor {
		t.Errorf("idle ring left = %v, want %v", got, idleColor)
	}
	if got := idle.NRGBAAt(8, 8); got.A != 255 {
		t.Errorf("idle center alpha = %d, want the wide play", got.A)
	}

	// Finished is a zero: the center of the glyph is empty, the ring is not.
	finished := solidSprite(finishedRows, finishedColor).render(size)
	if got := finished.NRGBAAt(8, 8); got.A != 0 {
		t.Errorf("finished center alpha = %d, want 0", got.A)
	}
	if got := finished.NRGBAAt(1, 7); got != finishedColor {
		t.Errorf("finished ring left = %v, want %v", got, finishedColor)
	}

	// Running has a filled center: the clock hands meet there.
	running := solidSprite(runningRows, runningColor).render(size)
	if got := running.NRGBAAt(8, 8); got.A != 255 {
		t.Errorf("running center alpha = %d, want 255", got.A)
	}
}

func TestTrayStatesDiffer(t *testing.T) {
	states := trayArt()
	for i := 0; i < len(states); i++ {
		for j := i + 1; j < len(states); j++ {
			var a, b bytes.Buffer
			if err := png.Encode(&a, states[i].art.render(32)); err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(&b, states[j].art.render(32)); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(a.Bytes(), b.Bytes()) {
				t.Errorf("%s and %s render identically", states[i].name, states[j].name)
			}
		}
	}
}

func TestWriteICO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.ico")
	sizes := []int{16, 32, 48, 256}
	if err := writeICO(path, sizes, func(size int) image.Image {
		return appSprite.render(size)
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint16(data[0:]); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}
	if got := binary.LittleEndian.Uint16(data[2:]); got != 1 {
		t.Errorf("type = %d, want 1", got)
	}
	if got := int(binary.LittleEndian.Uint16(data[4:])); got != len(sizes) {
		t.Fatalf("count = %d, want %d", got, len(sizes))
	}
	for i, want := range sizes {
		e := 6 + 16*i
		w := int(data[e])
		if w == 0 {
			w = 256
		}
		h := int(data[e+1])
		if h == 0 {
			h = 256
		}
		if w != want || h != want {
			t.Errorf("entry %d: %dx%d, want %dx%d", i, w, h, want, want)
		}
		if got := binary.LittleEndian.Uint16(data[e+4:]); got != 1 {
			t.Errorf("entry %d: planes = %d, want 1", i, got)
		}
		if got := binary.LittleEndian.Uint16(data[e+6:]); got != 32 {
			t.Errorf("entry %d: bit count = %d, want 32", i, got)
		}
		n := int(binary.LittleEndian.Uint32(data[e+8:]))
		off := int(binary.LittleEndian.Uint32(data[e+12:]))
		if off < 0 || n < 0 || off+n > len(data) {
			t.Fatalf("entry %d: payload %d..%d outside %d bytes", i, off, off+n, len(data))
		}
		img, err := png.Decode(bytes.NewReader(data[off : off+n]))
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		if got := img.Bounds(); got != image.Rect(0, 0, want, want) {
			t.Errorf("entry %d: decoded bounds = %v", i, got)
		}
	}
}

func TestDeterministic(t *testing.T) {
	s := solidSprite(finishedRows, finishedColor)
	var a, b bytes.Buffer
	if err := png.Encode(&a, s.render(48)); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&b, s.render(48)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("rendering the same sprite twice produced different PNGs")
	}
}
