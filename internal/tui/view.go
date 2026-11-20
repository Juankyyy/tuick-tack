package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/config"
	"tuick-tack/internal/duration"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/tui/bigfont"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/notes"
	"tuick-tack/internal/tui/theme"
)

type rect struct{ x, y, w, h int }

// button is one entry in a horizontal button row.
type button struct {
	label    string
	action   hitAction
	disabled bool
}

// The styled stage is supported from 48x22. Between 22 and 23 rows it drops
// the redundant meta line so the timer, the controls and the hints still fit;
// below the minimum the TUI switches to the plain-text compact view.
const (
	minCols   = 48
	minRows   = 22
	tightRows = 24
)

// viewLayout is the open-stage composition: a centred measure, a two-line
// masthead and the key hints tucked just under the presets/controls row. The
// whole block is centred, so the spare air falls below the hints instead of
// splitting the content. The timer block grows airier with the height of the
// terminal.
type viewLayout struct {
	x, w      int
	headerY   int
	heroY     int
	labelY    int
	barY      int
	metaY     int // -1 in the tight layout: the meta line is dropped
	controlsY int
	inputY    int
	chipsY    int
	hintY     int
	subHintY  int
	statusY   int
	barX      int
	barW      int
}

// View renders the whole TUI through the canvas package.
func (m *Model) View() tea.View {
	w, h := m.viewSize()
	if m.dust != nil {
		m.dust.Resize(w, h)
	}
	m.hits = m.hits[:0]
	if m.modal != modalNone || m.compact() {
		m.hoverUpdate = false
	}

	c := canvas.New(w, h)
	if m.compact() {
		m.drawCompact(c, w, h)
	} else {
		m.drawBackground(c, w, h)
		if m.dust != nil {
			m.dust.Draw(c, theme.Lerp(m.pal.BG, m.pal.Muted, 0.55))
		}
		if m.celebrating && m.confetti != nil {
			m.confetti.Draw(c)
		}
		m.drawContent(c, m.layout(w, h))
		if m.modal != modalNone {
			m.hits = m.hits[:0]
			c.Dim(0.70)
			m.drawModal(c, rect{x: 0, y: 0, w: w, h: h})
		}
	}

	v := tea.NewView(c.Render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	v.WindowTitle = buildinfo.Name
	v.BackgroundColor = lipgloss.Color(theme.Hex(m.pal.BG))
	v.ForegroundColor = lipgloss.Color(theme.Hex(m.pal.Text))
	return v
}

// compact reports whether the terminal is below the supported minimum and the
// plain-text fallback must take over.
func (m *Model) compact() bool {
	w, h := m.viewSize()
	return w < minCols || h < minRows
}

func (m *Model) layout(w, h int) viewLayout {
	sw := w - 8
	if sw > 100 {
		sw = 100
	}
	if sw < 10 {
		sw = w
	}
	sx := (w - sw) / 2

	headerY := 2
	gap := 1
	// The update hint and its blank row need a little more height before the
	// roomier spacing fits.
	if h >= 32 {
		gap = 2
	}
	// The block now ends with the key hints, one or two rows below the
	// presets/controls, so it is centred as a whole and the spare rows stay
	// under the hints rather than between the presets and the bottom edge.
	// From 22 rows the redundant meta line is dropped to keep that minimum
	// readable.
	tight := h < tightRows
	// From 24 rows there is room for a last, quieter hint under the main key
	// hints. The tight layout (22-23 rows) keeps only the essentials.
	subHint := !tight
	blockH := bigfont.GlyphH + 3*gap + 3
	groupH := blockH + 3*gap + 3
	if tight {
		groupH -= gap + 1
	} else if subHint {
		// The update hint has a blank row above it.
		groupH += 2
	}
	top := headerY + 3
	heroY := top
	if avail := h - top; avail > groupH {
		heroY += (avail - groupH) / 2
	}
	if heroY < top {
		heroY = top
	}
	labelY := heroY + bigfont.GlyphH + gap
	barGap := gap
	if subHint && h < 25 {
		// At the 24-row stage the space for the hint's blank row comes from
		// between the label and the progress bar.
		barGap = 0
	}
	barY := labelY + 1 + barGap
	metaY := -1
	controlsY := 0
	if tight {
		controlsY = barY + 2*gap + 1
	} else {
		metaY = barY + 1 + gap
		gapBeforeControls := 2*gap + 1
		if h < 26 {
			// Keep the meta but tighten the gap before the input at 24-25 rows,
			// where the update hint and its blank row need the space.
			gapBeforeControls = gap + 1
		}
		controlsY = metaY + gapBeforeControls
	}
	chipsY := controlsY + 1
	hintY := chipsY + 1 + gap
	if subHint {
		if hintY > h-3 {
			hintY = h - 3
		}
	} else if hintY > h-1 {
		hintY = h - 1
	}
	statusY := hintY - 1
	if statusY < 0 {
		statusY = 0
	}
	subHintY := -1
	if subHint {
		subHintY = hintY + 2
	}

	barX := sx + 4
	barW := sw - 8
	if barW < 10 {
		barW = sw
	}
	return viewLayout{
		x:         sx,
		w:         sw,
		headerY:   headerY,
		heroY:     heroY,
		labelY:    labelY,
		barY:      barY,
		metaY:     metaY,
		controlsY: controlsY,
		inputY:    controlsY - 1,
		chipsY:    chipsY,
		hintY:     hintY,
		subHintY:  subHintY,
		statusY:   statusY,
		barX:      barX,
		barW:      barW,
	}
}

func (m *Model) drawBackground(c *canvas.Canvas, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	now := m.now()
	flash := 0.0
	if m.celebrating {
		age := now.Sub(m.celebrationStart).Seconds()
		if age >= 0 && age < 0.12 {
			flash = 0.35 * (1 - age/0.12)
		}
	}
	r := float64(maxInt(w, h)) * 0.62
	cx1 := float64(w)*0.24 + math.Sin(m.animTime*0.07)*float64(w)*0.08
	cy1 := float64(h)*0.32 + math.Cos(m.animTime*0.05)*float64(h)*0.10
	cx2 := float64(w)*0.78 + math.Cos(m.animTime*0.06)*float64(w)*0.06
	cy2 := float64(h)*0.72 + math.Sin(m.animTime*0.04)*float64(h)*0.08
	for y := 0; y < h; y++ {
		t := 0.0
		if h > 1 {
			t = float64(y) / float64(h-1)
		}
		row := theme.Lerp(m.pal.BG, m.pal.BG2, t)
		for x := 0; x < w; x++ {
			base := row
			d1 := math.Hypot(float64(x)-cx1, float64(y)-cy1)
			if d1 < r {
				i := (1 - d1/r) * (1 - d1/r) * 0.16
				base = theme.Lerp(base, m.pal.Accent, i)
			}
			d2 := math.Hypot(float64(x)-cx2, float64(y)-cy2)
			if d2 < r {
				i := (1 - d2/r) * (1 - d2/r) * 0.13
				base = theme.Lerp(base, m.pal.Accent2, i)
			}
			if flash > 0 {
				base = theme.Lerp(base, m.pal.Text, flash)
			}
			c.Set(x, y, ' ', canvas.Bg(base))
		}
	}
}

func (m *Model) drawContent(c *canvas.Canvas, l viewLayout) {
	if l.w < 10 {
		return
	}
	m.drawHeader(c, l.x, l.headerY, l.w)
	m.drawHero(c, l.heroY, l.x, l.w)
	m.drawLabel(c, l.labelY, l.x, l.w)
	m.drawProgress(c, l.barY, l.barX, l.barW)
	if l.metaY >= 0 {
		m.drawMeta(c, l.metaY, l.x, l.w)
	}

	hintStyle := canvas.Fg(theme.Lerp(m.pal.BG, m.pal.Muted, 0.8))
	switch m.status() {
	case engine.Running, engine.Paused:
		if l.controlsY > l.metaY {
			m.drawTimerControls(c, l.controlsY, l.x, l.w)
		}
		m.drawCentered(c, l.hintY, copy.T(copy.ControlsRunning), hintStyle, l.x, l.w)
		if m.status() == engine.Paused {
			m.drawCentered(c, l.hintY, copy.T(copy.ControlsPaused), hintStyle, l.x, l.w)
		}
	case engine.Finished:
		if l.controlsY > l.metaY {
			m.drawFinishedControls(c, l.controlsY, l.x, l.w)
		}
		m.drawCentered(c, l.hintY, copy.T(copy.ControlsFinished), hintStyle, l.x, l.w)
	default:
		if l.inputY > l.metaY {
			m.drawInput(c, l.inputY, l.x, l.w)
		}
		if m.inputErr != "" {
			m.drawCentered(c, l.chipsY, m.inputErr, canvas.Fg(m.pal.Danger), l.x, l.w)
		} else if l.chipsY > l.labelY {
			m.drawChips(c, l.chipsY, l.x, l.w)
		}
		m.drawCentered(c, l.hintY, copy.T(copy.ControlsIdle), hintStyle, l.x, l.w)
	}

	// Secondary hint, quieter than the key line above it and below it on screen.
	if l.subHintY >= 0 {
		dim := theme.Lerp(m.pal.BG, m.pal.Muted, 0.5)
		hint := copy.T(copy.UpdateHint)
		if m.update != nil {
			hint = copy.T(copy.UpdateHintNew, m.update.Latest)
		}
		m.drawCentered(c, l.subHintY, hint, canvas.Fg(dim), l.x, l.w)
	}

	if !m.connected {
		msg := copy.T(copy.Connecting)
		col := m.pal.Muted
		if !m.connecting {
			msg = copy.T(copy.Disconnected)
			col = m.pal.Danger
		}
		m.drawCentered(c, l.statusY, msg, canvas.Fg(col), l.x, l.w)
	} else if m.checkingUpdate {
		m.drawCentered(c, l.statusY, copy.T(copy.UpdateChecking), canvas.Fg(m.pal.Muted), l.x, l.w)
	} else if m.toastActive() {
		m.drawCentered(c, l.statusY, m.toast, canvas.Fg(m.pal.Warn), l.x, l.w)
	}
}

// drawCompact is the plain-text fallback used below the supported minimum: a
// flat background and a few lines, most useful first, so the clock and the
// goal stay visible even in a tiny pane. It never draws hits or modals.
func (m *Model) drawCompact(c *canvas.Canvas, w, h int) {
	c.Fill(0, 0, w, h, canvas.Bg(m.pal.BG))
	lines := m.compactLines(w)
	if len(lines) > h {
		lines = lines[:h]
	}
	y := (h - len(lines)) / 2
	if y < 0 {
		y = 0
	}
	for _, line := range lines {
		m.drawCenteredIn(c, 0, y, w, line, canvas.Fg(m.pal.Text))
		y++
	}
}

// compactLines orders the information by importance: the clock first, then
// the goal, the state and the essential keys. A short terminal keeps a prefix
// of this list, so a one-row pane still shows the countdown.
func (m *Model) compactLines(w int) []string {
	var lines []string
	switch m.status() {
	case engine.Running, engine.Paused:
		lines = append(lines, duration.Format(m.remaining()))
		lines = append(lines, m.currentLabel())
		lines = append(lines, m.compactStatus())
		hint := copy.T(copy.CompactRunning)
		if m.status() == engine.Paused {
			hint = copy.T(copy.CompactPaused)
		}
		lines = append(lines, hint)
	case engine.Finished:
		word := m.finishWord
		if word == "" {
			word = copy.T(copy.HeroFinished)
		}
		lines = append(lines, word)
		lines = append(lines, copy.T(copy.FinishedTitle, m.finishLabel()))
		at := m.finishedAt
		if at.IsZero() {
			at = m.now()
		}
		lines = append(lines, copy.T(copy.TimeAt, at.Format("15:04")))
		lines = append(lines, copy.T(copy.CompactFinished))
	default:
		lines = append(lines, m.compactInput())
		if m.inputErr != "" {
			lines = append(lines, m.inputErr)
		}
		if presets := m.compactPresets(w); presets != "" {
			lines = append(lines, presets)
		}
		lines = append(lines, m.compactStatus())
		lines = append(lines, copy.T(copy.CompactIdle))
	}

	if m.modal == modalConfirmStop {
		// The confirmation hides the lower-priority lines but keeps the clock
		// on top whenever there is room for it.
		if len(lines) > 1 {
			lines = lines[:1]
		}
		lines = append(lines, copy.T(copy.StopTitle), copy.T(copy.CompactStopHint))
	}

	if m.update != nil {
		lines = append(lines, copy.T(copy.UpdateAvailable))
	}
	if m.checkingUpdate {
		lines = append(lines, copy.T(copy.UpdateChecking))
	}

	if !m.connected {
		msg := copy.T(copy.Connecting)
		if !m.connecting {
			msg = copy.T(copy.Disconnected)
		}
		lines = append(lines, msg)
	} else if m.toastActive() {
		lines = append(lines, m.toast)
	}
	return lines
}

func (m *Model) compactStatus() string {
	label, _ := m.statusInfo()
	label = strings.ToUpper(label)
	if sub := m.headerSubline(); sub != "" {
		return label + " · " + sub
	}
	return label
}

func (m *Model) compactInput() string {
	text := m.input
	cursor := "_"
	if text == "" {
		text = copy.T(copy.InputPlaceholder)
		cursor = " _"
	}
	return "› " + text + cursor
}

func (m *Model) compactPresets(w int) string {
	items := m.chips()
	if len(items) == 0 {
		return ""
	}
	selected := m.selected
	if selected < 0 || selected >= len(m.cfg.Presets) {
		selected = 0
	}
	start, end := windowFor(items, selected, w, 3)
	if start >= end {
		return ""
	}
	parts := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		label := items[i].label
		if items[i].index == selected {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " · ")
}

// drawHeader paints the two-line masthead: the wordmark on the left and the
// state on the right, with its detail (end time, hold, finish time) below it.
func (m *Model) drawHeader(c *canvas.Canvas, x, y, w int) {
	label, col := m.statusInfo()
	dot := m.headerDot(col)
	right := x + w - 1
	label = strings.ToUpper(label)
	lx := right - canvas.Width(label) + 1
	if lx < x+2 {
		lx = x + 2
	}
	dotX := lx - 2
	if dotX < x+2 {
		dotX = x + 2
	}

	brand := copy.T(copy.Brand)
	c.Set(x, y, '●', canvas.Fg(dot))
	c.Text(x+2, y, brand, canvas.Style{FG: m.pal.Text, HasFG: true, Bold: true})
	next := x + 2 + canvas.Width(brand)
	// The running version follows the wordmark as a quiet suffix. When an
	// update notice is showing, the version leaves room for its shortest form:
	// the notice is actionable and the version is also in the update dialog.
	if version := buildinfo.Version; version != "" {
		label := " " + version
		need := canvas.Width(label)
		if m.update != nil {
			short := "[" + copy.T(copy.UpdateShort) + " " + copy.T(copy.UpdateArrow) + "]"
			need += canvas.Width(short) + 2
		}
		if next+need < dotX {
			c.Text(next, y, label, canvas.Fg(headerVersionColor(m.pal)))
			next += canvas.Width(label)
		}
	}
	if m.update != nil {
		m.drawUpdateLink(c, next+2, y, dotX)
	}
	c.Set(dotX, y, '●', canvas.Fg(dot))
	c.Text(lx, y, label, canvas.Style{FG: col, HasFG: true, Bold: true})
	if sub := m.headerSubline(); sub != "" {
		c.Text(right-canvas.Width(sub)+1, y+1, sub, canvas.Fg(m.pal.Muted))
	}
}

// headerVersionColor is the quiet tone of the version suffix: the muted color
// at low opacity, so it reads as metadata next to the bold wordmark.
func headerVersionColor(pal theme.Theme) color.RGBA {
	return theme.Lerp(pal.BG, pal.Muted, 0.85)
}

// Update notice colors: a wide hue sweep at low saturation so the link reads as
// a rainbow without competing with the countdown. Mono keeps the palette's own
// grays.
const (
	updateHueSpan  = 0.8
	updateHueSpeed = 0.1
	updateSat      = 0.52
	updateLight    = 0.68
)

// drawUpdateLink paints the clickable notice after the wordmark. It falls back
// to a shorter label, and then to nothing, when the status would collide.
func (m *Model) drawUpdateLink(c *canvas.Canvas, x, y, limit int) {
	text := "[" + copy.T(copy.UpdateAvailable) + " " + copy.T(copy.UpdateArrow) + "]"
	if x+canvas.Width(text) >= limit {
		text = "[" + copy.T(copy.UpdateShort) + " " + copy.T(copy.UpdateArrow) + "]"
	}
	if x+canvas.Width(text) >= limit {
		return
	}
	colors := m.updateColors(len([]rune(text)))
	hoverBG := theme.Lerp(m.pal.BG, m.pal.Text, 0.12)
	cx := x
	for i, r := range []rune(text) {
		st := canvas.Style{FG: colors[i], HasFG: true, Bold: true}
		if m.hoverUpdate {
			st.BG = hoverBG
			st.HasBG = true
		}
		c.Set(cx, y, r, st)
		cx += canvas.Width(string(r))
	}
	m.addHit(x, y, canvas.Width(text), 1, hitUpdate, 0)
}

func (m *Model) updateColors(n int) []color.RGBA {
	if n <= 0 {
		return nil
	}
	colors := make([]color.RGBA, n)
	for i := range colors {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		if m.pal.Name == "mono" {
			colors[i] = theme.Lerp(m.pal.Muted, m.pal.Text, t)
			continue
		}
		colors[i] = theme.HSL(updateHueSpan*t+m.animTime*updateHueSpeed, updateSat, updateLight)
	}
	return colors
}

func (m *Model) headerDot(col color.RGBA) color.RGBA {
	phase := m.animTime * 2.4
	switch m.status() {
	case engine.Running:
		return theme.Pulse(m.pal.Accent, m.pal.Accent2, phase)
	case engine.Paused:
		return m.pal.Warn
	case engine.Finished:
		return theme.Pulse(m.pal.Warn, m.pal.Danger, phase)
	default:
		return col
	}
}

// headerSubline is the quiet second line under the state. It carries the end
// time while running and disappears when the daemon is offline.
func (m *Model) headerSubline() string {
	if !m.connected {
		return ""
	}
	switch m.status() {
	case engine.Running:
		return copy.T(copy.EndsAt, m.endClock())
	case engine.Paused:
		return copy.T(copy.HeaderOnHold)
	case engine.Finished:
		at := m.finishedAt
		if at.IsZero() {
			at = m.now()
		}
		return copy.T(copy.TimeAt, at.Format("15:04"))
	default:
		return copy.T(copy.HeaderNoTimer)
	}
}

func (m *Model) statusInfo() (string, color.RGBA) {
	if !m.connected {
		if m.connecting {
			return copy.T(copy.StatusConnecting), m.pal.Muted
		}
		return copy.T(copy.StatusOffline), m.pal.Danger
	}
	switch m.status() {
	case engine.Running:
		return copy.T(copy.StatusRunning), m.pal.Accent
	case engine.Paused:
		return copy.T(copy.StatusPaused), m.pal.Warn
	case engine.Finished:
		return copy.T(copy.StatusFinished), m.pal.Warn
	default:
		return copy.T(copy.StatusReady), m.pal.Accent
	}
}

// canvasColor is an alias to keep the color helpers readable.
type canvasColor = color.RGBA

func (m *Model) drawHero(c *canvas.Canvas, y, x, w int) {
	text := "00:00"
	ghost := false
	switch m.status() {
	case engine.Running, engine.Paused:
		text = duration.Format(m.remaining())
	case engine.Finished:
		text = m.finishWord
		if text == "" {
			text = copy.T(copy.HeroFinished)
		}
	default:
		ghost = true
		if strings.TrimSpace(m.input) != "" {
			if d, err := duration.Parse(m.input); err == nil {
				text = duration.Format(d)
				ghost = false
			}
		}
	}

	scale := 1
	proportional := false
	if m.status() == engine.Finished {
		// The celebration word keeps the wide scale whenever it fits: short
		// words do it on their own, and proportional spacing trims the glyph
		// bearings so TIME'S UP! and FINISHED! do not fall back to thin 1x
		// glyphs. The hero block is always seven rows, so width decides.
		if m.width >= 100 && bigfont.WidthProportional(text, 2) <= w {
			scale = 2
			proportional = true
		}
	} else if m.width >= 100 && m.height >= 30 {
		scale = 2
	}
	bw := bigfont.Width(text, scale)
	if proportional {
		bw = bigfont.WidthProportional(text, scale)
	}
	if bw > w {
		scale = 1
		proportional = false
		bw = bigfont.Width(text, scale)
	}
	if bw > w {
		// Extremely narrow terminal: keep the clock rather than clipping the
		// celebration word.
		if m.status() == engine.Finished {
			text = "0:00"
			bw = bigfont.Width(text, 1)
		}
	}
	top, bottom := m.heroColors(ghost)
	hideColon := false
	if m.status() != engine.Paused && m.status() != engine.Finished {
		hideColon = (m.now().UnixMilli()/500)%2 == 1
	}
	bx := x + (w-bw)/2
	if bx < x {
		bx = x
	}
	bigfont.Draw(c, bx, y, text, bigfont.Options{
		Scale:        scale,
		Top:          top,
		Bottom:       bottom,
		Shadow:       theme.Scale(m.pal.BG, 0.5),
		HasShadow:    true,
		Bold:         true,
		HideColon:    hideColon,
		Proportional: proportional,
	})
}

func (m *Model) heroColors(ghost bool) (canvasColor, canvasColor) {
	if ghost {
		return m.pal.Muted, theme.Lerp(m.pal.BG, m.pal.Border, 0.9)
	}
	switch m.status() {
	case engine.Paused:
		return m.pal.Warn, theme.Lerp(m.pal.Warn, m.pal.Accent2, 0.45)
	case engine.Finished:
		return m.pal.Warn, m.pal.Danger
	default:
		if m.remaining() <= 10*time.Second && m.remaining() > 0 {
			phase := float64(m.now().UnixMilli()) / 220
			warn := theme.Pulse(m.pal.Warn, m.pal.Danger, phase)
			return warn, theme.Pulse(m.pal.Danger, m.pal.Accent2, phase+1.2)
		}
		return m.pal.Accent, m.pal.Accent2
	}
}

func (m *Model) drawLabel(c *canvas.Canvas, y, x, w int) {
	text := copy.T(copy.ReadyTitle)
	col := m.pal.Muted
	switch m.status() {
	case engine.Running, engine.Paused:
		text = m.currentLabel()
		col = m.pal.Text
	case engine.Finished:
		text = copy.T(copy.FinishedTitle, m.finishLabel())
		col = m.pal.Text
	}
	m.drawCentered(c, y, text, canvas.Style{FG: col, HasFG: true, Bold: true}, x, w)
}

// currentLabel is the goal shown under the countdown, with a neutral fallback
// for ad-hoc timers without a preset name.
func (m *Model) currentLabel() string {
	if label := strings.TrimSpace(m.state.Label); label != "" {
		return label
	}
	return copy.T(copy.LabelFocus)
}

func (m *Model) drawProgress(c *canvas.Canvas, y, x, w int) {
	if w < 10 {
		return
	}
	frac := m.progress()
	if m.status() == engine.Idle {
		frac = 0
	}
	if m.status() == engine.Finished {
		frac = 1
	}
	filled := int(math.Round(frac * float64(w)))
	if filled > w {
		filled = w
	}
	if frac > 0 && filled == 0 {
		filled = 1
	}
	trackBG := theme.Lerp(m.pal.Panel, m.pal.BG, 0.35)
	trackFG := theme.Lerp(m.pal.Border, m.pal.Panel, 0.25)
	c.Fill(x, y, w, 1, canvas.Full(trackFG, trackBG))
	for i := 0; i < w; i++ {
		c.Set(x+i, y, '░', canvas.Fg(trackFG))
	}
	pulse := m.remaining() <= 10*time.Second && m.status() == engine.Running
	paused := m.status() == engine.Paused
	for i := 0; i < filled; i++ {
		t := 0.0
		if w > 1 {
			t = float64(i) / float64(w-1)
		}
		var col canvasColor
		switch {
		case paused:
			col = theme.Lerp(m.pal.Warn, m.pal.BG, 0.30)
		case pulse:
			phase := float64(m.now().UnixMilli())/260 + t*2
			col = theme.Pulse(m.pal.Warn, m.pal.Danger, phase)
		default:
			col = theme.Lerp(m.pal.Accent, m.pal.Accent2, t)
		}
		c.Set(x+i, y, '█', canvas.Fg(col))
	}
	for _, mark := range []float64{0.25, 0.5, 0.75} {
		mx := x + int(mark*float64(w-1))
		c.Set(mx, y, '┆', canvas.Fg(theme.Lerp(trackFG, m.pal.Text, 0.55)))
	}
	if filled > 0 {
		head := m.pal.Text
		if paused {
			head = m.pal.Warn
		}
		c.Set(x+filled-1, y, '█', canvas.Style{FG: head, HasFG: true, Bold: true})
	}
}

func (m *Model) drawMeta(c *canvas.Canvas, y, x, w int) {
	text := copy.T(copy.ReadyMeta)
	col := m.pal.Muted
	switch m.status() {
	case engine.Running:
		text = copy.T(copy.RunningMeta, duration.Format(m.remaining()))
		col = m.pal.Muted
	case engine.Paused:
		text = copy.T(copy.PausedMeta, duration.Format(m.remaining()))
		col = m.pal.Warn
	case engine.Finished:
		text = copy.T(copy.FinishedMeta)
		col = m.pal.Muted
	}
	m.drawCentered(c, y, text, canvas.Fg(col), x, w)
}

func (m *Model) endClock() string {
	if m.state.EndAtUnixMS > 0 {
		return time.UnixMilli(m.state.EndAtUnixMS).Format("15:04")
	}
	return m.now().Add(m.remaining()).Format("15:04")
}

func (m *Model) drawInput(c *canvas.Canvas, y, x, w int) {
	text := m.input
	prompt := "› "
	col := m.pal.Text
	if text == "" {
		text = copy.T(copy.InputPlaceholder)
		col = m.pal.Muted
	}
	line := prompt + text
	lineW := canvas.Width(line)
	if text == m.input {
		line += "_"
		lineW++
	} else {
		line += " _"
		lineW += 2
	}
	sx := x + (w-lineW)/2
	if sx < x {
		sx = x
	}
	c.Text(sx, y, prompt, canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
	c.Text(sx+canvas.Width(prompt), y, text, canvas.Fg(col))
	if text == m.input {
		c.Set(sx+canvas.Width(prompt)+canvas.Width(text), y, '_', canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
	} else {
		c.Set(sx+canvas.Width(prompt)+canvas.Width(text)+1, y, '_', canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
	}
}

type chip struct {
	label string
	index int
	width int
}

func (m *Model) chips() []chip {
	out := make([]chip, 0, len(m.cfg.Presets))
	for i, p := range m.cfg.Presets {
		label := p.Label
		if label == "" {
			label = duration.Short(time.Duration(p.Seconds) * time.Second)
		}
		label = fitText(label, 12)
		out = append(out, chip{label: label, index: i, width: canvas.Width(label) + 4})
	}
	return out
}

func (m *Model) drawChips(c *canvas.Canvas, y, x, w int) {
	items := m.chips()
	plusW := 5
	budget := w
	if len(items) > 0 {
		budget -= plusW + 2
	}
	if budget < 0 {
		budget = 0
	}
	start, end := windowFor(items, m.selected, budget, 2)
	total := 0
	for i := start; i < end; i++ {
		total += items[i].width
	}
	visible := end - start
	if visible > 0 {
		total += (visible - 1) * 2
	}
	if len(items) > 0 {
		total += 2
	}
	total += plusW
	sx := x + (w-total)/2
	if sx < x {
		sx = x
	}
	for i := start; i < end; i++ {
		it := items[i]
		selected := it.index == m.selected
		bg := theme.Lerp(m.pal.Panel, m.pal.Text, 0.06)
		fg := m.pal.Text
		if selected {
			bg = m.pal.Accent
			fg = m.pal.BG
		}
		c.Fill(sx, y, it.width, 1, canvas.Full(fg, bg))
		c.Set(sx, y, '[', canvas.Fg(m.pal.Muted))
		c.Set(sx+it.width-1, y, ']', canvas.Fg(m.pal.Muted))
		body := " " + it.label + " "
		c.Text(sx+1, y, body, canvas.Style{FG: fg, HasFG: true, Bold: selected})
		m.addHit(sx, y, it.width, 1, hitChip, it.index)
		sx += it.width + 2
	}
	// The add chip is always reachable, even with 16 presets on an 80-col
	// terminal.
	bg := theme.Lerp(m.pal.Panel, m.pal.Accent, 0.12)
	c.Fill(sx, y, plusW, 1, canvas.Full(m.pal.Accent, bg))
	c.Set(sx, y, '[', canvas.Fg(m.pal.Muted))
	c.Set(sx+plusW-1, y, ']', canvas.Fg(m.pal.Muted))
	c.Text(sx+1, y, " + ", canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
	m.addHit(sx, y, plusW, 1, hitPlus, -1)
}

func windowFor(items []chip, selected, avail, gap int) (int, int) {
	if len(items) == 0 {
		return 0, 0
	}
	start, end := 0, len(items)
	for start < end {
		width := 0
		for i := start; i < end; i++ {
			width += items[i].width
		}
		width += maxInt(0, end-start-1) * gap
		if width <= avail {
			break
		}
		if selected < start {
			start = selected
			continue
		}
		if selected >= end-1 {
			start++
		} else if end > selected+2 {
			end--
		} else {
			start++
		}
		if start > selected {
			start = selected
			break
		}
	}
	if start < 0 {
		start = 0
	}
	if end > len(items) {
		end = len(items)
	}
	if start >= end {
		start = maxInt(0, minInt(selected, len(items)-1))
		end = minInt(start+1, len(items))
	}
	return start, end
}

func (m *Model) drawTimerControls(c *canvas.Canvas, y, x, w int) {
	pauseLabel := copy.T(copy.Pause)
	if m.status() == engine.Paused {
		pauseLabel = copy.T(copy.Resume)
	}
	items := []button{
		{pauseLabel, hitPause, false},
		{copy.T(copy.PlusMin), hitAdjustPlus, false},
		{copy.T(copy.MinusMin), hitAdjustMinus, false},
		{copy.T(copy.Cancel), hitCancel, false},
	}
	m.drawButtons(c, y, x, w, items, hitNone)
}

func (m *Model) drawFinishedControls(c *canvas.Canvas, y, x, w int) {
	m.drawButtons(c, y, x, w, []button{
		{copy.T(copy.RepeatHint), hitRepeat, false},
		{copy.T(copy.CloseHint), hitDismiss, false},
	}, hitNone)
}

func (m *Model) drawButtons(c *canvas.Canvas, y, x, w int, items []button, focus hitAction) {
	widths := make([]int, len(items))
	total := 0
	for i, it := range items {
		widths[i] = canvas.Width(it.label) + 4
		total += widths[i]
	}
	total += maxInt(0, len(items)-1) * 2
	sx := x + (w-total)/2
	if sx < x {
		sx = x
	}
	for i, it := range items {
		bw := widths[i]
		bg := theme.Lerp(m.pal.Panel, m.pal.Text, 0.08)
		fg := m.pal.Text
		bracket := m.pal.Muted
		bold := true
		switch {
		case it.disabled:
			bg = theme.Lerp(m.pal.Panel, m.pal.Text, 0.03)
			fg = theme.Lerp(m.pal.Panel, m.pal.Muted, 0.7)
			bold = false
		case focus != hitNone && it.action == focus:
			bg = m.pal.Accent
			fg = m.pal.BG
			bracket = m.pal.BG
		case it.action == hitCancel:
			bg = theme.Lerp(m.pal.Panel, m.pal.Danger, 0.16)
			fg = m.pal.Danger
		case it.action == hitPause:
			bg = theme.Lerp(m.pal.Panel, m.pal.Accent, 0.18)
			fg = m.pal.Accent
		}
		c.Fill(sx, y, bw, 1, canvas.Full(fg, bg))
		c.Set(sx, y, '[', canvas.Fg(bracket))
		c.Set(sx+bw-1, y, ']', canvas.Fg(bracket))
		c.Text(sx+2, y, it.label, canvas.Style{FG: fg, HasFG: true, Bold: bold})
		m.addHit(sx, y, bw, 1, it.action, 0)
		sx += bw + 2
	}
}

func (m *Model) drawModal(c *canvas.Canvas, base rect) {
	switch m.modal {
	case modalHelp:
		m.drawHelp(c, base)
	case modalConfirmStop:
		m.drawConfirm(c, base, copy.T(copy.StopTitle), hitConfirmYes)
	case modalConfirmDelete:
		m.drawConfirm(c, base, copy.T(copy.PresetDeleteTitle, m.deleteTarget.Label), hitConfirmYes)
	case modalPresets:
		m.drawPresets(c, base)
	case modalPresetForm:
		m.drawForm(c, base)
	case modalSound:
		m.drawSound(c, base)
	case modalUpdate:
		m.drawUpdate(c, base)
	case modalStats:
		m.drawStats(c, base)
	}
}

func (m *Model) modalRect(base rect, w, h int) rect {
	tw := minInt(w, base.w-4)
	th := minInt(h, base.h-2)
	if tw < 20 {
		tw = minInt(w, base.w)
	}
	if th < 6 {
		th = minInt(h, base.h)
	}
	return rect{x: (m.width - tw) / 2, y: (m.height - th) / 2, w: tw, h: th}
}

func (m *Model) drawOverlay(c *canvas.Canvas, r rect, title string) {
	c.Fill(r.x, r.y, r.w, r.h, canvas.Full(m.pal.Panel, m.pal.Panel))
	c.Set(r.x+2, r.y+1, '●', canvas.Fg(m.pal.Accent))
	c.Text(r.x+4, r.y+1, title, canvas.Style{FG: m.pal.Text, HasFG: true, Bold: true})
}

func (m *Model) drawHelp(c *canvas.Canvas, base rect) {
	r := m.modalRect(base, 70, 17)
	m.drawOverlay(c, r, copy.T(copy.HelpTitle))
	lines := []struct {
		text string
		col  canvasColor
	}{
		{copy.T(copy.HelpIdle), m.pal.Text},
		{copy.T(copy.HelpTimer), m.pal.Text},
		{copy.T(copy.HelpModal), m.pal.Text},
		{copy.T(copy.HelpSound), m.pal.Muted},
		{copy.T(copy.HelpPresets), m.pal.Muted},
		{"", m.pal.Muted},
		{copy.T(copy.HelpFormats), m.pal.Muted},
		{copy.T(copy.HelpFinished), m.pal.Muted},
		{copy.T(copy.HelpUpdate), m.pal.Muted},
		{copy.T(copy.HelpQuit), m.pal.Muted},
	}
	y := r.y + 3
	for _, line := range lines {
		if line.text != "" {
			m.drawCenteredIn(c, r.x+2, y, r.w-4, line.text, canvas.Fg(line.col))
		}
		y++
	}
	bw := canvas.Width(copy.T(copy.Close)) + 4
	bx := r.x + (r.w-bw)/2
	by := r.y + r.h - 3
	c.Fill(bx, by, bw, 1, canvas.Full(m.pal.Accent, theme.Lerp(m.pal.Panel, m.pal.Accent, 0.18)))
	c.Set(bx, by, '[', canvas.Fg(m.pal.Muted))
	c.Set(bx+bw-1, by, ']', canvas.Fg(m.pal.Muted))
	c.Text(bx+2, by, copy.T(copy.Close), canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
	m.addHit(bx, by, bw, 1, hitClose, 0)
}

func (m *Model) drawConfirm(c *canvas.Canvas, base rect, title string, yes hitAction) {
	r := m.modalRect(base, 52, 8)
	m.drawOverlay(c, r, title)
	y := r.y + 4
	yesLabel := "enter " + copy.T(copy.Yes)
	noLabel := "esc " + copy.T(copy.No)
	items := []button{
		{yesLabel, yes, false},
		{noLabel, hitConfirmNo, false},
	}
	m.drawButtons(c, y, r.x+2, r.w-4, items, hitNone)
}

func (m *Model) drawPresets(c *canvas.Canvas, base rect) {
	wantH := 8 + len(m.cfg.Presets)
	if wantH > 20 {
		wantH = 20
	}
	if wantH < 10 {
		wantH = 10
	}
	r := m.modalRect(base, 76, wantH)
	m.drawOverlay(c, r, copy.T(copy.PresetsTitle))
	listX := r.x + 2
	listY := r.y + 3
	listW := r.w - 4
	listH := r.h - 6
	if listH < 1 {
		listH = 1
	}
	m.clampPresetCursor()
	m.clampPresetFocus()
	if m.presetCursor < m.presetScroll {
		m.presetScroll = m.presetCursor
	}
	if m.presetCursor >= m.presetScroll+listH {
		m.presetScroll = m.presetCursor - listH + 1
	}
	if m.presetScroll < 0 {
		m.presetScroll = 0
	}

	if len(m.cfg.Presets) == 0 {
		m.drawCenteredIn(c, listX, listY, listW, copy.T(copy.PresetsEmpty), canvas.Fg(m.pal.Muted))
	} else {
		for row := 0; row < listH; row++ {
			idx := m.presetScroll + row
			if idx >= len(m.cfg.Presets) {
				break
			}
			p := m.cfg.Presets[idx]
			y := listY + row
			selected := idx == m.presetCursor
			if selected {
				c.Fill(listX, y, listW, 1, canvas.Full(m.pal.Text, theme.Lerp(m.pal.Panel, m.pal.Accent, 0.22)))
				c.Set(listX, y, '›', canvas.Fg(m.pal.Accent))
			}
			label := fitText(p.Label, listW-14)
			c.Text(listX+2, y, label, canvas.Style{FG: m.pal.Text, HasFG: true, Bold: selected})
			right := duration.Short(time.Duration(p.Seconds) * time.Second)
			rx := listX + listW - canvas.Width(right) - 1
			c.Text(rx, y, right, canvas.Fg(m.pal.Muted))
			m.addHit(listX, y, listW, 1, hitPresetRow, idx)
		}
	}

	by := r.y + r.h - 3
	items := make([]button, 0, len(presetActionDefs))
	for i, def := range presetActionDefs {
		items = append(items, button{
			label:    copy.T(def.label),
			action:   def.action,
			disabled: !m.presetActionEnabled(i),
		})
	}
	focus := hitNone
	if m.presetFocus >= 0 && m.presetFocus < len(presetActionDefs) {
		focus = presetActionDefs[m.presetFocus].action
	}
	m.drawButtons(c, by, r.x+2, r.w-4, items, focus)

	hint := copy.T(copy.PresetsHint)
	if m.presetFocus != presetFocusList {
		hint = copy.T(copy.PresetsActionHint)
	}
	m.drawCenteredIn(c, r.x+2, r.y+r.h-2, r.w-4, hint, canvas.Fg(m.pal.Muted))
}

func (m *Model) drawSound(c *canvas.Canvas, base rect) {
	r := m.modalRect(base, 62, 16)
	m.drawOverlay(c, r, copy.T(copy.SoundTitle))
	leftX := r.x + 3
	width := r.w - 6

	// Enabled row. The whole row is clickable, like the mute button below.
	y := r.y + 2
	mark := "[ ]"
	status := copy.T(copy.SoundOff)
	col := m.pal.Muted
	if m.cfg.Sound {
		mark = "[x]"
		status = copy.T(copy.SoundOn)
		col = m.pal.Accent
	}
	c.Text(leftX, y, mark, canvas.Style{FG: col, HasFG: true, Bold: true})
	c.Text(leftX+4, y, status, canvas.Style{FG: col, HasFG: true, Bold: m.cfg.Sound})
	m.addHit(leftX, y, width, 1, hitSoundMute, 0)

	// Sound list.
	for i, name := range config.Sounds {
		row := r.y + 4 + i
		selected := name == m.cfg.SoundName
		if selected {
			c.Fill(leftX, row, width, 1,
				canvas.Full(m.pal.Text, theme.Lerp(m.pal.Panel, m.pal.Accent, 0.22)))
			c.Set(leftX, row, '›', canvas.Fg(m.pal.Accent))
		}
		dot := "○"
		dotCol := m.pal.Muted
		if selected {
			dot = "●"
			dotCol = m.pal.Accent
		}
		c.Text(leftX+2, row, dot, canvas.Style{FG: dotCol, HasFG: true, Bold: selected})
		c.Text(leftX+4, row, copy.SoundLabel(name),
			canvas.Style{FG: m.pal.Text, HasFG: true, Bold: selected})
		m.addHit(leftX, row, width, 1, hitSoundRow, i)
	}

	// Volume row: [-5] [20 cells] [+5] 80%.
	y = r.y + 9
	c.Text(leftX, y, copy.T(copy.SoundVolume),
		canvas.Style{FG: m.pal.Muted, HasFG: true, Bold: true})
	minusX := leftX + 9
	m.drawSoundStep(c, minusX, y, "-5", hitSoundVolumeDown)
	barX := minusX + 6
	barW := 20
	filled := (m.cfg.Volume + 2) / 5
	if filled > barW {
		filled = barW
	}
	for i := 0; i < barW; i++ {
		ch := '░'
		col := theme.Lerp(m.pal.Border, m.pal.Panel, 0.25)
		if i < filled {
			ch = '█'
			col = theme.Lerp(m.pal.Accent, m.pal.Accent2, float64(i)/float64(barW-1))
		}
		c.Set(barX+i, y, ch, canvas.Fg(col))
		m.addHit(barX+i, y, 1, 1, hitSoundBar, (i+1)*5)
	}
	plusX := barX + barW + 1
	m.drawSoundStep(c, plusX, y, "+5", hitSoundVolumeUp)
	pct := fmt.Sprintf("%d%%", m.cfg.Volume)
	c.Text(plusX+6, y, pct, canvas.Style{FG: m.pal.Text, HasFG: true, Bold: true})

	m.drawCenteredIn(c, r.x+2, r.y+11, r.w-4, copy.T(copy.SoundHint), canvas.Fg(m.pal.Muted))
	m.drawCenteredIn(c, r.x+2, r.y+12, r.w-4, copy.T(copy.SoundActionHint), canvas.Fg(m.pal.Muted))

	muteLabel := copy.T(copy.SoundUnmute)
	if m.cfg.Sound {
		muteLabel = copy.T(copy.SoundMute)
	}
	by := r.y + r.h - 3
	m.drawButtons(c, by, r.x+2, r.w-4, []button{
		{copy.T(copy.SoundPreview), hitSoundPreview, false},
		{muteLabel, hitSoundMute, false},
		{copy.T(copy.Close), hitSoundClose, false},
	}, hitNone)
}

func (m *Model) drawSoundStep(c *canvas.Canvas, x, y int, label string, action hitAction) {
	w := canvas.Width(label) + 2
	bg := theme.Lerp(m.pal.Panel, m.pal.Accent, 0.12)
	c.Fill(x, y, w, 1, canvas.Full(m.pal.Accent, bg))
	c.Set(x, y, '[', canvas.Fg(m.pal.Muted))
	c.Set(x+w-1, y, ']', canvas.Fg(m.pal.Muted))
	c.Text(x+1, y, label, canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
	m.addHit(x, y, w, 1, action, 0)
}

// drawUpdateTransition paints "current → target" in the quiet version tone.
// The chooser keeps it static; while the daemon downloads or installs, a soft
// highlight sweeps across the line so the dialog reads as live without
// competing with the progress bar.
func (m *Model) drawUpdateTransition(c *canvas.Canvas, x, y, w int, animate bool) {
	latest := ""
	if m.update != nil {
		latest = m.update.Latest
	}
	text := fitText(copy.T(copy.UpdateFromTo, buildinfo.Version, latest), w)
	if text == "" {
		return
	}
	sx := x + (w-canvas.Width(text))/2
	if sx < x {
		sx = x
	}
	if !animate {
		c.Text(sx, y, text, canvas.Fg(m.pal.Muted))
		return
	}

	// The band travels a little beyond both ends and then waits a beat off
	// screen, so the loop does not restart abruptly. The base color stays
	// muted; only the moving glow brightens.
	period := float64(canvas.Width(text) + 10)
	head := math.Mod(m.animTime*10, period) - 5
	cx := sx
	for _, r := range text {
		d := float64(cx-sx) - head
		glow := math.Exp(-(d * d) / 6)
		c.Set(cx, y, r, canvas.Fg(theme.Lerp(m.pal.Muted, m.pal.Text, 0.55*glow)))
		cx += canvas.Width(string(r))
	}
}

// drawUpdate paints the install/download chooser, the countdown warning, the
// progress view, the portable result and the failure view. It reuses the same
// overlay, button row and bar language as the timer.
func (m *Model) drawUpdate(c *canvas.Canvas, base rect) {
	height := 18
	if m.updateStep == updateStepChoose {
		height = m.updateChooserHeight()
	}
	switch m.updateStep {
	case updateStepWarn:
		height = 12
	case updateStepProgress:
		height = 13
	case updateStepPortableDone:
		height = 12
	case updateStepFailed:
		height = 11
	}
	r := m.modalRect(base, 66, height)
	title := copy.T(copy.UpdateAvailable)
	switch m.updateStep {
	case updateStepWarn:
		title = copy.T(copy.UpdateTimerTitle)
	case updateStepProgress:
		if m.updateKind == "portable" {
			title = copy.T(copy.UpdatePortableTitle)
		} else {
			title = copy.T(copy.UpdateInProgress)
		}
	case updateStepPortableDone:
		title = copy.T(copy.UpdatePortableTitle)
	case updateStepFailed:
		title = copy.T(copy.UpdateFailed)
	}
	m.drawOverlay(c, r, title)

	x, w := r.x+3, r.w-6
	y := r.y + 3

	switch m.updateStep {
	case updateStepChoose:
		m.ensureUpdateType()
		m.ensureUpdateFocus()
		layout := m.updateChooserLayout()
		m.drawUpdateTransition(c, x, y, w, false)
		badge := copy.T(copy.UpdatePortableBadge)
		if m.installInfo.Installed {
			badge = copy.T(copy.UpdateInstalledBadge, m.installInfo.Dir)
		}
		m.drawCenteredIn(c, x, y+1, w, fitText(badge, w), canvas.Fg(m.pal.Accent))

		// Type chips stay horizontal when they fit; at narrow widths they
		// stack. The notes pane takes the rows that are left above the buttons.
		m.drawUpdateTypes(c, x, layout.typeY, w, layout.typeRows)
		m.drawUpdateTypeNote(c, x, layout.noteY, w, layout.noteRows)
		if layout.hasNotes {
			m.drawUpdateNotesPanel(c, x, layout)
		}

		items := make([]button, 0, 3)
		if m.updateTypeAvailable(m.updateType) {
			if m.updateType == updateTypeSetup {
				items = append(items, button{copy.T(copy.UpdateInstallShort), hitUpdateInstall, false})
			} else {
				items = append(items, button{copy.T(copy.UpdateDownloadShort), hitUpdatePortable, false})
			}
		}
		items = append(items,
			button{copy.T(copy.UpdateReleaseShort), hitUpdateRelease, false},
			button{copy.T(copy.UpdateLater), hitUpdateLater, false},
		)
		by := r.y + r.h - 3
		m.drawButtons(c, by, r.x+2, r.w-4, items, m.updateFocus)
		m.drawCenteredIn(c, x, r.y+r.h-2, w, m.updateChooserHint(layout), canvas.Fg(m.pal.Muted))

	case updateStepWarn:
		m.drawCenteredIn(c, x, y, w, copy.T(copy.UpdateTimerBody), canvas.Style{FG: m.pal.Text, HasFG: true, Bold: true})
		m.drawCenteredIn(c, x, y+1, w, copy.T(copy.UpdateTimerWarn, duration.Format(m.remaining())), canvas.Fg(m.pal.Warn))
		m.drawCenteredIn(c, x, y+3, w, copy.T(copy.UpdateTimerPortable), canvas.Fg(m.pal.Muted))
		by := r.y + r.h - 3
		m.drawButtons(c, by, r.x+2, r.w-4, []button{
			{copy.T(copy.UpdateContinue), hitUpdateContinue, false},
			{copy.T(copy.UpdateBack), hitUpdateBack, false},
		}, hitUpdateContinue)
		m.drawCenteredIn(c, x, r.y+r.h-2, w, copy.T(copy.UpdateHintWarn), canvas.Fg(m.pal.Muted))

	case updateStepProgress:
		m.drawUpdateTransition(c, x, y, w, true)
		phase := copy.T(copy.UpdatePreparing)
		switch m.updatePhase {
		case "downloading":
			phase = copy.T(copy.UpdateDownloading)
		case "verifying":
			phase = copy.T(copy.UpdateVerifying)
		case "installing":
			phase = copy.T(copy.UpdateInstalling)
		}
		m.drawCenteredIn(c, x, y+1, w, phase, canvas.Fg(m.pal.Muted))
		frac := 0.0
		if m.updateTotal > 0 {
			frac = float64(m.updateReceived) / float64(m.updateTotal)
			if frac > 1 {
				frac = 1
			}
			m.drawCenteredIn(c, x, y+2, w, fmt.Sprintf("%d%%", int(frac*100)), canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
		}
		m.drawModalBar(c, y+4, x, w, frac)
		if m.updateTotal > 0 {
			m.drawCenteredIn(c, x, y+5, w, copy.T(copy.UpdateSize, humanBytes(m.updateReceived), humanBytes(m.updateTotal)), canvas.Fg(m.pal.Muted))
		}
		steps := copy.T(copy.UpdateStepsSetup)
		if m.updateKind == "portable" {
			steps = copy.T(copy.UpdateStepsPortable)
		}
		m.drawCenteredIn(c, x, y+7, w, fitText(steps, w), canvas.Fg(m.pal.Muted))
		m.drawCenteredIn(c, x, y+8, w, copy.T(copy.UpdateWait), canvas.Fg(m.pal.Muted))

	case updateStepPortableDone:
		m.drawCenteredIn(c, x, y, w, copy.T(copy.UpdatePortableSaved), canvas.Style{FG: m.pal.Text, HasFG: true, Bold: true})
		path := m.updatePath
		if w < 55 {
			if i := strings.LastIndexAny(path, `\/`); i >= 0 {
				path = path[i+1:]
			}
		}
		m.drawCenteredIn(c, x, y+2, w, fitText(path, w), canvas.Fg(m.pal.Accent))
		m.drawCenteredIn(c, x, y+4, w, copy.T(copy.UpdatePortableDone), canvas.Fg(m.pal.Muted))
		by := r.y + r.h - 3
		m.drawButtons(c, by, r.x+2, r.w-4, []button{
			{copy.T(copy.UpdateOpenFolder), hitUpdateOpenFolder, false},
			{copy.T(copy.Close), hitUpdateLater, false},
		}, hitUpdateOpenFolder)
		m.drawCenteredIn(c, x, r.y+r.h-2, w, copy.T(copy.UpdateHintPortable), canvas.Fg(m.pal.Muted))

	case updateStepFailed:
		m.drawCenteredIn(c, x, y, w, copy.T(copy.UpdateFailed), canvas.Style{FG: m.pal.Text, HasFG: true, Bold: true})
		m.drawCenteredIn(c, x, y+1, w, fitText(m.updateErr, w), canvas.Fg(m.pal.Danger))
		by := r.y + r.h - 3
		m.drawButtons(c, by, r.x+2, r.w-4, []button{
			{copy.T(copy.UpdateRetry), hitUpdateRetry, false},
			{copy.T(copy.UpdateReleaseShort), hitUpdateRelease, false},
			{copy.T(copy.Close), hitUpdateLater, false},
		}, hitUpdateRetry)
		m.drawCenteredIn(c, x, r.y+r.h-2, w, copy.T(copy.UpdateHintFailed), canvas.Fg(m.pal.Muted))
	}
}

// drawUpdateTypes paints the Setup/Portable chips: one centered row when both
// fit, and the stacked full-width rows used before the notes pane existed when
// they do not. The selection follows m.updateType, not the copy in use.
func (m *Model) drawUpdateTypes(c *canvas.Canvas, x, y, w, rows int) {
	current := m.currentUpdateType()
	other := m.otherUpdateType(current)
	if rows == 1 {
		type chip struct {
			kind      string
			label     string
			status    string
			text      string
			available bool
			selected  bool
		}
		chips := []chip{{kind: current}, {kind: other}}
		for i := range chips {
			chips[i].label = copy.T(copy.UpdateTypePortable)
			if chips[i].kind == updateTypeSetup {
				chips[i].label = copy.T(copy.UpdateTypeSetup)
			}
			chips[i].status = m.updateTypeStatus(chips[i].kind)
			chips[i].available = m.updateTypeAvailable(chips[i].kind)
			chips[i].selected = chips[i].available && chips[i].kind == m.updateType
			chips[i].text = chips[i].label + " · " + chips[i].status
			if chips[i].selected {
				chips[i].text = "› " + chips[i].text
			}
		}
		total := canvas.Width(chips[0].text) + 4 + canvas.Width(chips[1].text)
		sx := x + (w-total)/2
		if sx < x {
			sx = x
		}
		for _, chip := range chips {
			if chip.selected {
				c.Fill(sx, y, canvas.Width(chip.text), 1, canvas.Full(m.pal.Accent, theme.Lerp(m.pal.Panel, m.pal.Accent, 0.18)))
				c.Text(sx, y, chip.text, canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
			} else {
				labelStyle := canvas.Fg(m.pal.Text)
				statusStyle := canvas.Fg(m.pal.Muted)
				if !chip.available {
					labelStyle = canvas.Fg(theme.Lerp(m.pal.Panel, m.pal.Muted, 0.6))
					statusStyle = labelStyle
				}
				c.Text(sx, y, chip.label, labelStyle)
				c.Text(sx+canvas.Width(chip.label), y, " · ", statusStyle)
				c.Text(sx+canvas.Width(chip.label)+3, y, chip.status, statusStyle)
			}
			sx += canvas.Width(chip.text) + 4
		}
		return
	}
	m.drawUpdateTypeRow(c, x, y, w, current)
	m.drawUpdateTypeRow(c, x, y+1, w, other)
}

// drawUpdateTypeRow paints one row of the stacked Setup/Portable selector.
// The current copy is the first row; the other copy follows. Only available
// types can be selected, and the row uses a soft selection instead of the
// accent fill used by the horizontal action buttons.
func (m *Model) drawUpdateTypeRow(c *canvas.Canvas, x, y, w int, updateType string) {
	label := copy.T(copy.UpdateTypePortable)
	if updateType == updateTypeSetup {
		label = copy.T(copy.UpdateTypeSetup)
	}
	status := m.updateTypeStatus(updateType)
	available := m.updateTypeAvailable(updateType)
	selected := available && updateType == m.updateType
	labelColor := m.pal.Text
	statusColor := m.pal.Muted
	if !available {
		labelColor = theme.Lerp(m.pal.Panel, m.pal.Muted, 0.6)
		statusColor = labelColor
	}
	if selected {
		background := theme.Lerp(m.pal.Panel, m.pal.Accent, 0.18)
		c.Fill(x, y, w, 1, canvas.Full(m.pal.Accent, background))
		c.Set(x, y, '›', canvas.Fg(m.pal.Accent))
		labelColor = m.pal.Accent
	}
	c.Text(x+2, y, label, canvas.Style{FG: labelColor, HasFG: true, Bold: selected})
	sx := x + w - canvas.Width(status) - 1
	if sx < x+2 {
		sx = x + 2
	}
	c.Text(sx, y, status, canvas.Fg(statusColor))
}

// drawUpdateTypeNote paints the one-line explanation under the type chips.
// rows is the height the layout reserved for it; long copy wraps to at most
// three centered lines.
func (m *Model) drawUpdateTypeNote(c *canvas.Canvas, x, y, w, rows int) {
	lines := m.updateTypeNoteLines(w)
	if len(lines) > rows {
		lines = lines[:rows]
	}
	_, warn := m.updateTypeNoteText()
	style := canvas.Fg(m.pal.Muted)
	if warn {
		style = canvas.Fg(m.pal.Warn)
	}
	for i, line := range lines {
		m.drawCenteredIn(c, x, y+i, w, line, style)
	}
}

// drawUpdateNotesPanel paints the release notes well, its scrollbar and the
// header that names the target version.
func (m *Model) drawUpdateNotesPanel(c *canvas.Canvas, x int, layout updateChooserLayout) {
	if m.update == nil || layout.bodyH <= 0 {
		return
	}
	title := copy.T(copy.UpdateNotesTitle, m.update.Latest)
	c.Text(x, layout.headerY, fitText(title, layout.innerW),
		canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
	well := theme.Lerp(m.pal.Panel, m.pal.BG, 0.55)
	c.Fill(x, layout.bodyTop, layout.innerW, layout.bodyH, canvas.Bg(well))
	textX := x + 1
	for row := 0; row < layout.bodyH; row++ {
		idx := m.notesScroll + row
		if idx >= len(m.updateNotes) {
			break
		}
		line := m.updateNotes[idx]
		if line.Text == "" {
			continue
		}
		y := layout.bodyTop + row
		switch line.Kind {
		case notes.Heading:
			c.Text(textX, y, line.Text, canvas.Style{FG: m.pal.Accent, HasFG: true, Bold: true})
		case notes.Quote:
			c.Text(textX, y, line.Text, canvas.Fg(m.pal.Muted))
		case notes.Bullet:
			if strings.HasPrefix(line.Text, notes.BulletMarker) {
				c.Set(textX, y, '·', canvas.Fg(m.pal.Accent))
				c.Text(textX+2, y, strings.TrimPrefix(line.Text, notes.BulletMarker), canvas.Fg(m.pal.Text))
			} else {
				c.Text(textX, y, line.Text, canvas.Fg(m.pal.Text))
			}
		default:
			c.Text(textX, y, line.Text, canvas.Fg(m.pal.Text))
		}
	}
	m.drawUpdateNotesBar(c, x, layout)
}

// drawUpdateNotesBar paints the vertical scrollbar at the right edge of the
// well. The thumb is proportional and stays at least one cell tall.
func (m *Model) drawUpdateNotesBar(c *canvas.Canvas, x int, layout updateChooserLayout) {
	total := len(m.updateNotes)
	if layout.bodyH <= 0 || total <= layout.bodyH {
		return
	}
	barX := x + layout.innerW - 2
	track := canvas.Fg(theme.Lerp(m.pal.Panel, m.pal.Border, 0.9))
	for row := 0; row < layout.bodyH; row++ {
		c.Set(barX, layout.bodyTop+row, '│', track)
	}
	thumb := layout.bodyH * layout.bodyH / total
	if thumb < 1 {
		thumb = 1
	}
	if thumb > layout.bodyH {
		thumb = layout.bodyH
	}
	max := total - layout.bodyH
	pos := 0
	if max > 0 {
		pos = m.notesScroll * (layout.bodyH - thumb) / max
	}
	for i := 0; i < thumb; i++ {
		c.Set(barX, layout.bodyTop+pos+i, '█', canvas.Fg(m.pal.Accent))
	}
}

func (m *Model) updateTypeStatus(updateType string) string {
	if !m.updateTypeAvailable(updateType) {
		return copy.T(copy.UpdateStatusNotInRelease)
	}
	if updateType == updateTypeSetup {
		if m.currentUpdateType() == updateTypeSetup {
			return copy.T(copy.UpdateStatusInstalled)
		}
		return copy.T(copy.UpdateStatusNotInstalled)
	}
	if m.currentUpdateType() == updateTypePortable {
		return copy.T(copy.UpdateStatusInUse)
	}
	return copy.T(copy.UpdateStatusNotInUse)
}

func (m *Model) drawModalBar(c *canvas.Canvas, y, x, w int, frac float64) {
	if w <= 0 {
		return
	}
	c.Fill(x, y, w, 1, canvas.Bg(theme.Lerp(m.pal.Panel, m.pal.BG, 0.55)))
	filled := int(float64(w) * frac)
	if filled > w {
		filled = w
	}
	for i := 0; i < filled; i++ {
		t := 0.0
		if w > 1 {
			t = float64(i) / float64(w-1)
		}
		c.Set(x+i, y, '█', canvas.Fg(theme.Lerp(m.pal.Accent, m.pal.Accent2, t)))
	}
}

func humanBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	if n >= 1<<20 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	if n >= 1<<10 {
		return fmt.Sprintf("%d KB", n/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func (m *Model) drawForm(c *canvas.Canvas, base rect) {
	title := copy.T(copy.PresetNewTitle)
	if m.form.mode == formEdit {
		title = copy.T(copy.PresetEditTitle)
	}
	r := m.modalRect(base, 64, 11)
	m.drawOverlay(c, r, title)
	m.drawCenteredIn(c, r.x+2, r.y+2, r.w-4, copy.T(copy.PresetFormHint), canvas.Fg(m.pal.Muted))

	fieldX := r.x + 12
	fieldW := r.w - 16
	m.drawField(c, r.x+2, r.y+4, fieldX, fieldW, copy.T(copy.PresetName), m.form.label, m.form.focus == 0, hitFormName)
	m.drawField(c, r.x+2, r.y+5, fieldX, fieldW, copy.T(copy.PresetDuration), m.form.duration, m.form.focus == 1, hitFormDuration)
	if m.form.err != "" {
		m.drawCenteredIn(c, r.x+2, r.y+7, r.w-4, m.form.err, canvas.Fg(m.pal.Danger))
	}
	by := r.y + r.h - 3
	m.drawButtons(c, by, r.x+2, r.w-4, []button{
		{copy.T(copy.Save), hitFormSave, false},
		{copy.T(copy.Cancel), hitFormCancel, false},
	}, hitNone)
}

func (m *Model) drawField(c *canvas.Canvas, labelX, y, fieldX, fieldW int, label, value string, focused bool, action hitAction) {
	col := m.pal.Muted
	if focused {
		col = m.pal.Accent
	}
	c.Text(labelX, y, label, canvas.Style{FG: col, HasFG: true, Bold: focused})
	bg := theme.Lerp(m.pal.Panel, m.pal.BG, 0.25)
	border := m.pal.Border
	if focused {
		border = m.pal.Accent
	}
	c.Fill(fieldX, y, fieldW, 1, canvas.Full(m.pal.Text, bg))
	shown := value
	if focused {
		shown += "_"
	}
	c.Text(fieldX+1, y, fitText(shown, fieldW-2), canvas.Fg(m.pal.Text))
	c.Set(fieldX, y, '│', canvas.Fg(border))
	c.Set(fieldX+fieldW-1, y, '│', canvas.Fg(border))
	m.addHit(fieldX, y, fieldW, 1, action, 0)
}

func (m *Model) addHit(x, y, w, h int, action hitAction, index int) {
	if w <= 0 || h <= 0 {
		return
	}
	m.hits = append(m.hits, hitbox{x: x, y: y, w: w, h: h, action: action, index: index})
}

func (m *Model) drawCentered(c *canvas.Canvas, y int, s string, st canvas.Style, x, w int) {
	m.drawCenteredIn(c, x, y, w, s, st)
}

func (m *Model) drawCenteredIn(c *canvas.Canvas, x, y, w int, s string, st canvas.Style) {
	s = fitText(s, w)
	sx := x + (w-canvas.Width(s))/2
	if sx < x {
		sx = x
	}
	c.Text(sx, y, s, st)
}

func fitText(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if canvas.Width(s) <= max {
		return s
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := canvas.Width(string(r))
		if rw <= 0 {
			continue
		}
		if width+rw > max-1 {
			break
		}
		b.WriteRune(r)
		width += rw
	}
	return b.String() + "…"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
