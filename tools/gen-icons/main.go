// Command gen-icons draws the pixel-art tray and executable icons into assets/.
//
// The tray icons live on a 7x7 grid with one cell of margin: at 16 px every
// cell is exactly 2 px, and at 24/32/48 px it is 3/4/6 px. Each state gets an
// ICO for Windows and a 32 px PNG for the macOS menu bar, where the image is
// shown at 16 pt and the 2x bitmap keeps it sharp on Retina. The executable
// icon is a 16x16 grid on a navy tile. Every ICO entry is rendered at its own
// size without antialiasing; the small entries are never downsampled from a
// 256 px master. The ICO carries the sizes Windows asks for, and winres embeds
// the same file in the executable; assets/icon.iconset is the same artwork at
// the macOS sizes, ready for iconutil.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

var (
	// traySizes covers the notification-area sizes: 16 px at 100%, 20 at 125%,
	// 24 at 150%, 28 at 175% and 32 at 200%, plus the 2x HICON sizes systray
	// loads (32/40/48/56/64).
	traySizes = []int{16, 20, 24, 32, 40, 48, 56, 64}
	// appSizes covers Explorer and the Start menu. 256 is the PNG master too.
	appSizes = []int{16, 24, 32, 40, 48, 64, 96, 128, 256}
	// appIconSet is the macOS iconset iconutil turns into icon.icns. Every entry
	// is rendered from the 16x16 grid at an exact integer cell size, so the
	// pixel art never blurs on any display.
	appIconSet = []struct {
		name string
		size int
	}{
		{"icon_16x16.png", 16},
		{"icon_16x16@2x.png", 32},
		{"icon_32x32.png", 32},
		{"icon_32x32@2x.png", 64},
		{"icon_128x128.png", 128},
		{"icon_128x128@2x.png", 256},
		{"icon_256x256.png", 256},
		{"icon_256x256@2x.png", 512},
		{"icon_512x512.png", 512},
		{"icon_512x512@2x.png", 1024},
	}

	idleColor     = rgba(0x10B981) // emerald 500
	runningColor  = rgba(0x3B82F6) // blue 500
	pausedColor   = rgba(0xF59E0B) // amber 500
	finishedColor = rgba(0xEF4444) // red 500

	tileColor = rgba(0x0B1524) // midnight panel
	mintColor = rgba(0x6EE7B7) // midnight accent
	handColor = rgba(0xF8FAFC)
)

// sprite is a square pixel grid. Only cells whose rune is in colors are painted.
type sprite struct {
	rows   []string
	colors map[rune]color.NRGBA
}

func solidSprite(rows []string, c color.NRGBA) sprite {
	return sprite{rows: rows, colors: map[rune]color.NRGBA{'#': c}}
}

// render paints the grid at size with hard-edged cells, centered on the canvas.
func (s sprite) render(size int) *image.NRGBA {
	grid := len(s.rows)
	cell := size / grid
	if cell < 1 {
		cell = 1
	}
	art := grid * cell
	off := (size - art) / 2
	// systray loads the 2x HICON and the shell halves it, so keep the offset even.
	if size >= 32 && off%2 == 1 {
		off--
	}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y, row := range s.rows {
		for x, ch := range row {
			c, ok := s.colors[ch]
			if !ok {
				continue
			}
			for py := 0; py < cell; py++ {
				for px := 0; px < cell; px++ {
					img.SetNRGBA(off+x*cell+px, off+y*cell+py, c)
				}
			}
		}
	}
	return img
}

// Tray states. The ring is shared; the state glyph lives in the 5x5 interior.
var (
	// Idle: a wide play with a two-cell base and a one-cell gap around it.
	idleRows = []string{
		"..###..",
		".#...#.",
		"#.##..#",
		"#.###.#",
		"#.##..#",
		".#...#.",
		"..###..",
	}
	// Running: clock hands at 12:15. The tips meet the ring; that is the
	// version chosen over the play stopgap.
	runningRows = []string{
		"..###..",
		".#.#.#.",
		"#..#..#",
		"#..####",
		"#.....#",
		".#...#.",
		"..###..",
	}
	// Paused: two bars.
	pausedRows = []string{
		"..###..",
		".#...#.",
		"#.#.#.#",
		"#.#.#.#",
		"#.#.#.#",
		".#...#.",
		"..###..",
	}
	// Finished: a zero for 00:00.
	finishedRows = []string{
		"..###..",
		".#...#.",
		"#.###.#",
		"#.#.#.#",
		"#.###.#",
		".#...#.",
		"..###..",
	}
)

// appRows is the 16x16 executable icon: a navy rounded tile, a mint ring and
// white clock hands. The orange 12 o'clock tick was dropped on purpose.
var appRows = []string{
	"..nnnnnnnnnnnn..",
	".nnnnnnnnnnnnnn.",
	"nnnnnnmmmmnnnnnn",
	"nnnnmmmmmmmmnnnn",
	"nnnmmmnwwnmmmnnn",
	"nnnmmnnwwnnmmnnn",
	"nnmmnnnwwnnnmmnn",
	"nnmmnnnwwwwwmmnn",
	"nnmmnnnwwwwwmmnn",
	"nnmmnnnnnnnnmmnn",
	"nnnmmnnnnnnmmnnn",
	"nnnmmmnnnnmmmnnn",
	"nnnnmmmmmmmmnnnn",
	"nnnnnnmmmmnnnnnn",
	".nnnnnnnnnnnnnn.",
	"..nnnnnnnnnnnn..",
}

var appSprite = sprite{
	rows: appRows,
	colors: map[rune]color.NRGBA{
		'n': tileColor,
		'm': mintColor,
		'w': handColor,
	},
}

func main() {
	root := repoRoot()
	iconsDir := filepath.Join(root, "assets", "icons")
	if err := os.MkdirAll(iconsDir, 0o755); err != nil {
		fail(err)
	}

	states := []struct {
		name string
		art  sprite
	}{
		{"idle", solidSprite(idleRows, idleColor)},
		{"running", solidSprite(runningRows, runningColor)},
		{"paused", solidSprite(pausedRows, pausedColor)},
		{"finished", solidSprite(finishedRows, finishedColor)},
	}
	for _, st := range states {
		art := st.art
		path := filepath.Join(iconsDir, st.name+".ico")
		if err := writeICO(path, traySizes, func(size int) image.Image { return art.render(size) }); err != nil {
			fail(err)
		}
		fmt.Printf("gen-icons: wrote assets/icons/%s.ico\n", st.name)

		// macOS shows the menu-bar image at 16 pt; a 32 px PNG is the Retina
		// bitmap for the same spot, so it stays sharp on every Mac display.
		if err := writePNG(filepath.Join(iconsDir, st.name+".png"), art.render(32)); err != nil {
			fail(err)
		}
		fmt.Printf("gen-icons: wrote assets/icons/%s.png\n", st.name)
	}

	if err := writeICO(filepath.Join(root, "assets", "icon.ico"), appSizes, func(size int) image.Image {
		return appSprite.render(size)
	}); err != nil {
		fail(err)
	}
	if err := writePNG(filepath.Join(root, "assets", "icon.png"), appSprite.render(256)); err != nil {
		fail(err)
	}
	iconsetDir := filepath.Join(root, "assets", "icon.iconset")
	for _, entry := range appIconSet {
		if err := writePNG(filepath.Join(iconsetDir, entry.name), appSprite.render(entry.size)); err != nil {
			fail(err)
		}
	}
	fmt.Println("gen-icons: wrote assets/icon.ico, assets/icon.png and assets/icon.iconset")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-icons:", err)
	os.Exit(1)
}

func rgba(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

func writeICO(path string, sizes []int, render func(int) image.Image) error {
	payloads := make([][]byte, len(sizes))
	for i, size := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, render(size)); err != nil {
			return fmt.Errorf("%s %d px: %w", filepath.Base(path), size, err)
		}
		payloads[i] = buf.Bytes()
	}

	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1)) // icon
	binary.Write(&out, binary.LittleEndian, uint16(len(sizes)))

	offset := 6 + 16*len(sizes)
	for i, size := range sizes {
		b := byte(size)
		if size >= 256 {
			b = 0 // 256 is encoded as 0 in the directory
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

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
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
