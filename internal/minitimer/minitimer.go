// Package minitimer draws a small always-on-top countdown over the Windows
// taskbar. Windows has no supported way to embed a third-party widget in the
// taskbar, so the widget is a separate no-activate popup placed over the free
// left area. On macOS the package is a no-op: the menu bar already shows the
// remaining time next to its icon.
package minitimer

import (
	"image/color"
	"math"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/duration"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/tui/bigfont"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/theme"
)

// Grid and pixel layout. A cell is 3 px at 100% DPI; the 42x9 glyph grid plus
// the preferred two cells of padding and the 1 px borders is 140x41 px.
const (
	Cols     = 42
	Rows     = 9
	BarCells = 36
	barRow   = 8

	baseCell = 3
	minCell  = 3
	maxCell  = 6

	// padCells is the padding around the grid, in cells. LayoutForTray drops
	// it to one cell on short taskbars so the widget still fits.
	padCells = 2
)

// FinishedHold is how long the finished screen stays on screen after the
// countdown ends. It matches the TUI's ten-second auto-close of the
// celebration, so the overlay dismisses itself with or without the TUI open.
const FinishedHold = 10 * time.Second

// finishWindow tracks the finished screen's ten-second window. The engine
// keeps the finished state until someone dismisses it, so the widget needs its
// own deadline to hide the overlay on its own.
type finishWindow struct {
	seq int64
	at  time.Time
}

// observe starts the window the first time the widget sees the current finish.
// A new FinishSeq restarts it and leaving the finished state clears it.
func (f *finishWindow) observe(st engine.State, now time.Time) {
	if st.Status != engine.Finished {
		f.seq = 0
		f.at = time.Time{}
		return
	}
	if f.at.IsZero() || f.seq != st.FinishSeq {
		f.seq = st.FinishSeq
		f.at = now
	}
}

// closed reports whether the finished screen has outlived FinishedHold.
func (f *finishWindow) closed(now time.Time) bool {
	return !f.at.IsZero() && now.Sub(f.at) >= FinishedHold
}

// elapsed is the time since the current finish began, reused as the finish
// burst clock so the widget has a single source of truth for the event.
func (f *finishWindow) elapsed(now time.Time) (time.Duration, bool) {
	if f.at.IsZero() {
		return 0, false
	}
	return now.Sub(f.at), true
}

// burstActive reports whether a finish age is inside the confetti window.
func burstActive(age time.Duration, ok bool) bool {
	return ok && age >= 0 && age < BurstDuration
}

// BurstDuration is how long the finish confetti is painted. The fixed table
// empties the grid in about 1.4 s; the tail covers the slowest piece of the
// second wave.
const BurstDuration = 1600 * time.Millisecond

// barFlash lifts the completed bar for the first moments of the burst.
const barFlash = 160 * time.Millisecond

// burstGlowHold and burstGlowEnd shape the whole-widget glow: full strength
// for the hold, then a linear fade.
const (
	burstGlowHold = 140 * time.Millisecond
	burstGlowEnd  = 260 * time.Millisecond
)

// burstOriginY is the bar row the pieces start from, in canvas rows.
const burstOriginY = 8.2

// burstGravity pulls the pieces back down, in cells per second squared. It is
// tuned so no piece leaves the nine-row grid through the top.
const burstGravity = 36.0

// burstGlow is the finish flash envelope. The widget mixes its panel and
// border with it; the canvas itself has no panel.
func burstGlow(age time.Duration) float64 {
	if age < 0 || age >= burstGlowEnd {
		return 0
	}
	if age <= burstGlowHold {
		return 1
	}
	return 1 - float64(age-burstGlowHold)/float64(burstGlowEnd-burstGlowHold)
}

// burstPiece is one confetti piece: an offset from the canvas centre, a
// velocity in cells per second, an optional delay in seconds, a palette index,
// a size in cells and whether it is brightened towards the text color.
type burstPiece struct {
	dx, vx, vy, delay float64
	color             int
	w, h              int
	bright            bool
}

var burstPieces = [...]burstPiece{
	// First wave: it leaves the bar as soon as the countdown crosses zero.
	{dx: 15.2, vx: 0.94, vy: -16.06, color: 0, w: 1, h: 1},
	{dx: -15.98, vx: 1.5, vy: -11.01, color: 1, w: 2, h: 1},
	{dx: 0.61, vx: 1.2, vy: -12.3, color: 2, w: 2, h: 1},
	{dx: -3.66, vx: -3.11, vy: -14.22, color: 3, w: 1, h: 1},
	{dx: -1.29, vx: 3.17, vy: -16.28, color: 4, w: 1, h: 1},
	{dx: 1.21, vx: 0.43, vy: -12.47, color: 5, w: 2, h: 1, bright: true},
	{dx: 0.16, vx: 1.6, vy: -20.24, color: 0, w: 1, h: 1},
	{dx: 6.02, vx: 0.1, vy: -19.71, color: 1, w: 2, h: 1},
	{dx: 1.62, vx: 0.07, vy: -13.58, color: 2, w: 2, h: 2},
	{dx: 0.06, vx: 3.57, vy: -17.93, color: 3, w: 2, h: 1, bright: true},
	{dx: -13.29, vx: -1.16, vy: -12.65, color: 4, w: 2, h: 1},
	{dx: -14.96, vx: -0.3, vy: -16.16, color: 5, w: 1, h: 1, bright: true},
	{dx: -9.87, vx: -3.05, vy: -12.87, color: 0, w: 2, h: 2, bright: true},
	{dx: 10.91, vx: 3.09, vy: -14.66, color: 1, w: 2, h: 2, bright: true},
	{dx: -1.82, vx: 1.93, vy: -18.46, color: 2, w: 2, h: 2},
	{dx: 3.52, vx: 2.38, vy: -15.65, color: 3, w: 2, h: 1, bright: true},
	{dx: 7.68, vx: -2.39, vy: -12.75, color: 4, w: 1, h: 1},
	{dx: 4.1, vx: -0.56, vy: -17.3, color: 5, w: 2, h: 2},
	// Second wave: a short beat later, so the motion reads for longer.
	{dx: 7.41, vx: 2.89, vy: -17.79, delay: 0.32, color: 2, w: 1, h: 1, bright: true},
	{dx: -13.45, vx: 2.14, vy: -16.47, delay: 0.31, color: 3, w: 1, h: 1},
	{dx: -0.24, vx: 0.01, vy: -9.15, delay: 0.32, color: 4, w: 1, h: 1},
	{dx: 9.84, vx: 1.75, vy: -12.37, delay: 0.35, color: 5, w: 1, h: 1, bright: true},
	{dx: -6.64, vx: -2.39, vy: -14.06, delay: 0.37, color: 0, w: 1, h: 1},
	{dx: -10.01, vx: 2.88, vy: -15.99, delay: 0.32, color: 1, w: 1, h: 1},
	{dx: -0.21, vx: -2.96, vy: -11.99, delay: 0.31, color: 2, w: 1, h: 1, bright: true},
	{dx: 11.82, vx: -0.71, vy: -13.69, delay: 0.31, color: 3, w: 1, h: 1},
	{dx: 12.85, vx: -2.99, vy: -13.76, delay: 0.32, color: 4, w: 1, h: 1},
	{dx: 13.55, vx: -1.59, vy: -13.47, delay: 0.34, color: 5, w: 1, h: 1, bright: true},
}

// Actions is the daemon API the widget uses.
type Actions interface {
	Snapshot() engine.State
	Config() config.Config
	OpenTUI()
	TogglePause()
	Stop()
	SetMiniTimer(bool)
}

// Environment is the non-engine part of the visibility decision. It is split
// from the Win32 probes so the matrix is pure and testable.
type Environment struct {
	TaskbarVisible    bool
	TaskbarHorizontal bool
	Fullscreen        bool
	TUIFocused        bool
	ShellOverlay      bool
}

// Layout is the pixel geometry for one DPI and time width.
type Layout struct {
	Cell   int
	Cells  int
	Pad    int
	Width  int
	Height int
}

// WidthCells returns the glyph columns needed by the current text. The base
// grid fits MM:SS, one-digit hours and the finish word; two-digit hours need 47
// columns and grow the window to 155 px at 100 %.
func WidthCells(st engine.State) int {
	n := textWidth(st)
	if n < Cols {
		return Cols
	}
	return n
}

// textWidth is the drawn width of Text. The clock stays monospaced so the
// digits do not jitter; the finish word trims its side bearings like the TUI
// hero.
func textWidth(st engine.State) int {
	if st.Status == engine.Finished {
		return bigfont.WidthProportional(Text(st), 1)
	}
	return bigfont.Width(Text(st), 1)
}

// CellForDPI maps a Windows DPI to the pixel size of one font cell. Whole
// blocks only: 3 px at 96 DPI, 4 px at 120 and 5 px at 144.
func CellForDPI(dpi int) int {
	if dpi <= 0 {
		dpi = 96
	}
	return clampCell(int(math.Round(float64(dpi) * baseCell / 96)))
}

// LayoutFor returns the base window geometry for a cell size.
func LayoutFor(cell int) Layout {
	return layoutForCells(cell, Cols, padCells)
}

// LayoutForState returns the geometry needed by the current countdown.
func LayoutForState(cell int, st engine.State) Layout {
	return layoutForCells(cell, WidthCells(st), padCells)
}

// LayoutForTray is LayoutForState with the padding cut to what fits in a
// taskbar of the given height. A zero height means the taskbar is unknown and
// the preferred padding is kept.
func LayoutForTray(cell int, st engine.State, trayHeight int) Layout {
	return layoutForCells(cell, WidthCells(st), padForTray(cell, trayHeight))
}

// padForTray keeps the preferred padding when the window fits with two pixels
// of slack and falls back to one cell otherwise (a 40 px taskbar at 100 %, or
// 50 px at 125 %).
func padForTray(cell, trayHeight int) int {
	if trayHeight <= 0 {
		return padCells
	}
	cell = clampCell(cell)
	for pad := padCells; pad > 1; pad-- {
		if (Rows+2*pad)*cell+2 <= trayHeight-2 {
			return pad
		}
	}
	return 1
}

func clampCell(cell int) int {
	if cell < minCell {
		return minCell
	}
	if cell > maxCell {
		return maxCell
	}
	return cell
}

// fadeDuration is how long the overlay takes to appear or disappear. It is
// short enough to stay out of the way when the TUI takes focus.
const fadeDuration = 160 * time.Millisecond

// riseCells is how far below its final position the overlay starts, in cells.
// The actual travel is clamped to the free space inside the taskbar.
const riseCells = 2

// fadeState eases a 0..1 value towards a target. The widget drives both the
// window alpha and the vertical rise from it, so one cannot drift from the
// other.
type fadeState struct {
	value  float64
	from   float64
	to     float64
	start  time.Time
	active bool
}

// toward starts a fade to target from the current value. Calling it again with
// the same target keeps the original deadline, and reversing mid-fade keeps the
// current opacity and position instead of jumping.
func (f *fadeState) toward(target float64, now time.Time) {
	target = clamp01(target)
	if f.active && f.to == target {
		return
	}
	if !f.active && f.value == target {
		f.to = target
		return
	}
	f.from = f.value
	f.to = target
	f.start = now
	f.active = f.from != f.to
}

// step advances the fade to now and returns the eased value. active becomes
// false on the step that reaches the target, so the caller can do the final
// placement or hide the window exactly once.
func (f *fadeState) step(now time.Time) float64 {
	if !f.active {
		return f.value
	}
	t := now.Sub(f.start).Seconds() / fadeDuration.Seconds()
	if t >= 1 {
		f.value = f.to
		f.active = false
		return f.value
	}
	if t <= 0 {
		f.value = f.from
		return f.value
	}
	f.value = f.from + (f.to-f.from)*smoothstep(t)
	return f.value
}

// snap jumps to target without animating, for the reduced-motion path.
func (f *fadeState) snap(target float64) {
	f.value = clamp01(target)
	f.from, f.to = f.value, f.value
	f.start = time.Time{}
	f.active = false
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

func smoothstep(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

// riseOffset is how many pixels below its final position the overlay sits for a
// shown fraction in 0..1. The travel is riseCells, reduced to the free space
// below the centred window so a short taskbar cannot push it out of the bar.
func riseOffset(shown float64, layout Layout, trayHeight int) int {
	if layout.Height <= 0 || trayHeight <= 0 {
		return 0
	}
	y := (trayHeight - layout.Height) / 2
	if y < 0 {
		y = 0
	}
	space := trayHeight - (y + layout.Height)
	if space < 0 {
		space = 0
	}
	travel := riseCells * layout.Cell
	if space < travel {
		travel = space
	}
	return int(math.Round((1 - clamp01(shown)) * float64(travel)))
}

// cornerRadius is the window corner radius in pixels. One cell keeps the
// rounding subtle and scales with the DPI.
func cornerRadius(cell int) int {
	return clampCell(cell)
}

func layoutForCells(cell, cells, pad int) Layout {
	cell = clampCell(cell)
	if cells < Cols {
		cells = Cols
	}
	if pad < 1 {
		pad = 1
	}
	if pad > padCells {
		pad = padCells
	}
	return Layout{
		Cell:   cell,
		Cells:  cells,
		Pad:    pad,
		Width:  (cells+2*pad)*cell + 2, // 1 px border on each side
		Height: (Rows+2*pad)*cell + 2,  // 1 px border on the top and bottom
	}
}

// ContentX is the left edge of the glyph grid inside the window.
func (l Layout) ContentX() int { return 1 + l.Pad*l.Cell }

// ContentY is the top edge of the glyph grid inside the window.
func (l Layout) ContentY() int { return 1 + l.Pad*l.Cell }

// ShouldShow is the visibility matrix: an enabled widget only appears over a
// horizontal taskbar while a countdown is active, the desktop is available and
// the TUI is not focused.
func ShouldShow(enabled bool, st engine.State, env Environment) bool {
	if !enabled {
		return false
	}
	switch st.Status {
	case engine.Running, engine.Paused, engine.Finished:
	default:
		return false
	}
	if !env.TaskbarVisible || !env.TaskbarHorizontal {
		return false
	}
	if env.Fullscreen || env.TUIFocused || env.ShellOverlay {
		return false
	}
	return true
}

// Text is what the widget shows in the glyph row: the remaining time, or the
// finish word when the countdown has ended. The word comes from the same copy
// package as the TUI celebration and must fit the 5x7 font and the 42-column
// base grid; TestFinishedWordFits guards both.
func Text(st engine.State) string {
	switch st.Status {
	case engine.Running, engine.Paused:
		return duration.Format(st.Remaining)
	case engine.Finished:
		return copy.T(copy.HeroDone)
	default:
		return ""
	}
}

// Progress is the elapsed fraction used by the bar. It is clamped to 0..1 and
// is full at the finished screen.
func Progress(st engine.State) float64 {
	switch st.Status {
	case engine.Finished:
		return 1
	case engine.Running, engine.Paused:
		if st.Duration <= 0 {
			return 0
		}
		p := 1 - float64(st.Remaining)/float64(st.Duration)
		if p < 0 {
			return 0
		}
		if p > 1 {
			return 1
		}
		return p
	default:
		return 0
	}
}

// Palette is the widget's color set for one state.
type Palette struct {
	Top    color.RGBA
	Bottom color.RGBA
	Shadow color.RGBA
	Track  color.RGBA
	Head   color.RGBA
}

// PaletteFor maps the active TUI theme to the widget colors.
func PaletteFor(pal theme.Theme, st engine.State) Palette {
	out := Palette{
		Top:    pal.Accent,
		Bottom: pal.Accent2,
		Shadow: theme.Scale(pal.BG, 0.5),
		Track:  theme.Lerp(pal.Panel, pal.BG, 0.35),
		Head:   pal.Text,
	}
	switch st.Status {
	case engine.Paused:
		out.Top = pal.Warn
		out.Bottom = pal.Warn
		out.Head = pal.Warn
	case engine.Finished:
		out.Top = pal.Warn
		out.Bottom = pal.Danger
	}
	return out
}

// hoverPanelMix and hoverBorderMix lift the frame when the pointer is over the
// widget. The border carries most of the signal; the panel gets a slight
// accent wash.
const (
	hoverPanelMix  = 0.06
	hoverBorderMix = 0.70
)

// hoverFrame returns the panel and border for the current hover amount. The
// amount is 0 for the normal frame and 1 for the full hover tint; intermediate
// values are used by the entry and exit transition. It is pure so the palette
// choice is testable without a window.
func hoverFrame(panel, border, accent color.RGBA, amount float64) (color.RGBA, color.RGBA) {
	amount = clamp01(amount)
	if amount == 0 {
		return panel, border
	}
	return theme.Lerp(panel, accent, hoverPanelMix*amount),
		theme.Lerp(border, accent, hoverBorderMix*amount)
}

// Render builds the small canvas: the remaining time (or the finish word) in
// the 5x7 block font and the progress bar. phase drives the optional
// last-ten-seconds pulse; callers pass a monotonically increasing value (for
// example Unix milliseconds / 260).
func Render(st engine.State, pal theme.Theme, phase float64) *canvas.Canvas {
	return RenderAt(st, pal, phase, WidthCells(st))
}

// RenderAt is Render with an explicit canvas width in cells. The widget uses
// the width it placed so a paint cannot outgrow the window while the state is
// changing.
func RenderAt(st engine.State, pal theme.Theme, phase float64, width int) *canvas.Canvas {
	if width < Cols {
		width = Cols
	}
	c := canvas.New(width, Rows)
	p := PaletteFor(pal, st)

	urgent := st.Status == engine.Running && st.Remaining > 0 && st.Remaining <= 10*time.Second
	if urgent {
		p.Top = theme.Pulse(pal.Warn, pal.Danger, phase)
		p.Bottom = theme.Pulse(pal.Danger, pal.Warn, phase)
	}

	if text := Text(st); text != "" {
		x := (width - textWidth(st)) / 2
		if x < 0 {
			x = 0
		}
		bigfont.Draw(c, x, 0, text, bigfont.Options{
			Scale:        1,
			Top:          p.Top,
			Bottom:       p.Bottom,
			Shadow:       p.Shadow,
			HasShadow:    true,
			Proportional: st.Status == engine.Finished,
		})
	}

	x := (width - BarCells) / 2
	c.Fill(x, barRow, BarCells, 1, canvas.Bg(p.Track))
	filled := int(math.Round(Progress(st) * float64(BarCells)))
	if filled > BarCells {
		filled = BarCells
	}
	if filled == 0 && Progress(st) > 0 {
		filled = 1
	}
	for i := 0; i < filled; i++ {
		t := 0.0
		if BarCells > 1 {
			t = float64(i) / float64(BarCells-1)
		}
		col := theme.Lerp(p.Top, p.Bottom, t)
		switch {
		case st.Status == engine.Paused:
			col = p.Top
		case urgent:
			col = theme.Pulse(pal.Warn, pal.Danger, phase+t*2)
		}
		c.Set(x+i, barRow, ' ', canvas.Bg(col))
	}
	if filled > 0 {
		c.Set(x+filled-1, barRow, ' ', canvas.Bg(p.Head))
	}
	return c
}

// RenderBurstAt is RenderAt plus the finish confetti at age, the time since
// the countdown crossed zero. Negative ages, states other than finished and
// ages past BurstDuration render the calm screen.
func RenderBurstAt(st engine.State, pal theme.Theme, phase float64, width int, age time.Duration) *canvas.Canvas {
	c := RenderAt(st, pal, phase, width)
	if st.Status != engine.Finished || age < 0 || age >= BurstDuration {
		return c
	}
	if width < Cols {
		width = Cols
	}
	drawBurst(c, pal, width, age)
	return c
}

// drawBurst paints the bar flash and the confetti pieces. Cells are solid
// blocks of color: pieces do not blend per cell with the digit ink (the whole
// window fades at the compositor level during the open/close animation).
func drawBurst(c *canvas.Canvas, pal theme.Theme, width int, age time.Duration) {
	p := PaletteFor(pal, engine.State{Status: engine.Finished})
	x := (width - BarCells) / 2
	if age < barFlash {
		for i := 0; i < BarCells; i++ {
			t := 0.0
			if BarCells > 1 {
				t = float64(i) / float64(BarCells-1)
			}
			col := theme.Lerp(theme.Lerp(p.Top, p.Bottom, t), p.Head, 0.45)
			c.Set(x+i, barRow, ' ', canvas.Bg(col))
		}
		c.Set(x+BarCells-1, barRow, ' ', canvas.Bg(p.Head))
	}

	colors := pal.Confetti
	if len(colors) == 0 {
		colors = []color.RGBA{pal.Warn, pal.Danger, pal.Text}
	}
	ageSeconds := age.Seconds()
	center := float64(width) / 2
	for _, piece := range burstPieces {
		t := ageSeconds - piece.delay
		if t < 0 {
			continue
		}
		col := colors[piece.color%len(colors)]
		if piece.bright {
			col = theme.Lerp(col, pal.Text, 0.4)
		}
		px := center + piece.dx + piece.vx*t
		py := burstOriginY + piece.vy*t + 0.5*burstGravity*t*t
		cellX, cellY := int(math.Round(px)), int(math.Round(py))
		for dy := 0; dy < piece.h; dy++ {
			for dx := 0; dx < piece.w; dx++ {
				// Both colors: the widget painter draws BG and then FG, so a
				// background-only piece would disappear under a digit's ink.
				c.Set(cellX+dx, cellY+dy, ' ', canvas.Full(col, col))
			}
		}
	}
}
