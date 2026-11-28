package tui

import (
	"charm.land/bubbletea/v2"

	"tuick-tack/internal/config"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/theme"
)

// themePage is how many themes PgUp and PgDn move through the picker.
const themePage = 5

// openThemes opens the picker on the persisted theme without changing it. The
// palette follows the cursor, but m.cfg.Theme is only written by applyTheme.
func (m *Model) openThemes() {
	m.modal = modalTheme
	m.themeOriginal = m.cfg.Theme
	m.themeCursor = themeIndex(m.cfg.Theme)
	m.themeScroll = 0
	m.previewTheme()
}

// closeThemes cancels the preview and returns to the persisted theme.
func (m *Model) closeThemes() {
	m.modal = modalNone
	name := m.themeOriginal
	if !config.ValidTheme(name) {
		name = m.cfg.Theme
	}
	m.pal = theme.Get(name)
}

// moveTheme moves the cursor with wrap-around and previews the new palette.
func (m *Model) moveTheme(delta int) {
	n := len(config.Themes)
	if n == 0 {
		return
	}
	m.themeCursor = (m.themeCursor + delta) % n
	if m.themeCursor < 0 {
		m.themeCursor += n
	}
	m.previewTheme()
}

// jumpTheme moves by delta but clamps at the ends, for PgUp and PgDn. Arrow
// keys keep the wrap-around in moveTheme.
func (m *Model) jumpTheme(delta int) {
	n := len(config.Themes)
	if n == 0 {
		return
	}
	m.themeCursor += delta
	if m.themeCursor < 0 {
		m.themeCursor = 0
	}
	if m.themeCursor >= n {
		m.themeCursor = n - 1
	}
	m.previewTheme()
}

// previewTheme paints the theme under the cursor without persisting it.
func (m *Model) previewTheme() {
	if m.themeCursor < 0 || m.themeCursor >= len(config.Themes) {
		return
	}
	m.pal = theme.Get(config.Themes[m.themeCursor])
}

// applyTheme persists the selected theme through the daemon. The local palette
// already shows the preview, so only the config and the protocol message move.
func (m *Model) applyTheme() tea.Cmd {
	if m.themeCursor < 0 || m.themeCursor >= len(config.Themes) {
		return nil
	}
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	name := config.Themes[m.themeCursor]
	m.modal = modalNone
	m.pal = theme.Get(name)
	if name == m.cfg.Theme {
		return nil
	}
	m.cfg.Theme = name
	m.showToast(copy.T(copy.ToastTheme, copy.ThemeLabel(name)))
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetTheme, Name: name})
}

// handleThemeKey drives the keyboard side of the picker. Esc restores the
// persisted theme; Enter applies the preview.
func (m *Model) handleThemeKey(key tea.Key) tea.Cmd {
	switch key.Code {
	case tea.KeyEscape:
		m.closeThemes()
	case tea.KeyEnter:
		return m.applyTheme()
	case tea.KeyUp:
		m.moveTheme(-1)
	case tea.KeyDown:
		m.moveTheme(1)
	case tea.KeyPgUp:
		m.jumpTheme(-themePage)
	case tea.KeyPgDown:
		m.jumpTheme(themePage)
	case tea.KeyHome:
		m.themeCursor = 0
		m.previewTheme()
	case tea.KeyEnd:
		if n := len(config.Themes); n > 0 {
			m.themeCursor = n - 1
			m.previewTheme()
		}
	}
	return nil
}

// drawThemes paints the picker with one row per theme. The row under the
// cursor gets a filled background; the dot marks the persisted theme.
func (m *Model) drawThemes(c *canvas.Canvas, base rect) {
	wantH := len(config.Themes) + 7
	if wantH > 20 {
		wantH = 20
	}
	if wantH < 12 {
		wantH = 12
	}
	r := m.modalRect(base, 64, wantH)
	m.drawOverlay(c, r, copy.T(copy.ThemeTitle))

	listX := r.x + 2
	listY := r.y + 3
	listW := r.w - 4
	listH := r.h - 6
	if listH < 1 {
		listH = 1
	}
	if n := len(config.Themes); n > 0 {
		if m.themeCursor < 0 {
			m.themeCursor = 0
		}
		if m.themeCursor >= n {
			m.themeCursor = n - 1
		}
		if m.themeCursor < m.themeScroll {
			m.themeScroll = m.themeCursor
		}
		if m.themeCursor >= m.themeScroll+listH {
			m.themeScroll = m.themeCursor - listH + 1
		}
		maxScroll := n - listH
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.themeScroll > maxScroll {
			m.themeScroll = maxScroll
		}
	}
	if m.themeScroll < 0 {
		m.themeScroll = 0
	}

	for row := 0; row < listH; row++ {
		idx := m.themeScroll + row
		if idx >= len(config.Themes) {
			break
		}
		name := config.Themes[idx]
		y := listY + row
		selected := idx == m.themeCursor
		applied := name == m.cfg.Theme
		if selected {
			c.Fill(listX, y, listW, 1, canvas.Full(m.pal.Text, theme.Lerp(m.pal.Panel, m.pal.Accent, 0.14)))
			c.Set(listX, y, '›', canvas.Fg(m.pal.Accent))
		}
		dot := "○"
		dotCol := m.pal.Muted
		if applied {
			dot = "●"
			dotCol = m.pal.Accent
		}
		c.Text(listX+2, y, dot, canvas.Style{FG: dotCol, HasFG: true, Bold: applied})
		rightX := listX + listW - 3
		label := fitText(copy.ThemeLabel(name), rightX-listX-5)
		c.Text(listX+4, y, label, canvas.Style{FG: m.pal.Text, HasFG: true, Bold: selected || applied})
		c.Set(rightX, y, '█', canvas.Fg(m.pal.Accent))
		c.Set(rightX+1, y, '█', canvas.Fg(m.pal.Accent2))
		c.Set(rightX+2, y, '█', canvas.Fg(m.pal.Warn))
		m.addHit(listX, y, listW, 1, hitThemeRow, idx)
	}

	by := r.y + r.h - 3
	m.drawButtons(c, by, r.x+2, r.w-4, []button{
		{copy.T(copy.Apply), hitThemeApply, false},
		{copy.T(copy.Cancel), hitThemeClose, false},
	}, hitNone)
	m.drawCenteredIn(c, r.x+2, r.y+r.h-2, r.w-4, copy.T(copy.ThemeHint), canvas.Fg(m.pal.Muted))
}

// themeIndex returns the position of name in the picker order, or 0 when the
// name is unknown.
func themeIndex(name string) int {
	for i, candidate := range config.Themes {
		if candidate == name {
			return i
		}
	}
	return 0
}
