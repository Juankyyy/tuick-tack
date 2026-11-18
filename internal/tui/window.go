package tui

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/protocol"
)

// windowSaveDelay is how long the TUI waits after the last resize before it
// persists the new size. It keeps a window drag from writing the config on
// every cell.
const windowSaveDelay = 300 * time.Millisecond

type windowSaveMsg struct{ width, height int }

type windowSavedMsg struct {
	width, height int
	err           error
}

// handleWindowSize records a terminal resize and decides whether it is a real
// user change worth persisting. The size the terminal reports at startup, and
// the one produced by applying the stored preference, are not stored again.
func (m *Model) handleWindowSize(w, h int) tea.Cmd {
	prevW, prevH := m.width, m.height
	first := !m.sizeKnown
	m.width, m.height = w, h
	m.sizeKnown = true
	if m.dust != nil {
		m.dust.Resize(w, h)
	}
	if m.confetti != nil {
		m.confetti.Resize(w, h)
	}

	// If the handshake arrived before the terminal reported its startup size,
	// wait for that first report and then restore the stored size. Applying it
	// earlier would race the report and could persist the old size.
	if first && m.windowApplyPending {
		m.windowApplyPending = false
		if m.connected && m.windowSupported {
			m.windowArmed = true
			if w != m.windowSavedW || h != m.windowSavedH {
				m.applyWindow(m.windowSavedW, m.windowSavedH)
			}
		}
		return nil
	}
	if !m.connected {
		// A change before the handshake means the user resized while the app was
		// connecting. Honor it instead of restoring the stored size over it.
		if !first && (w != prevW || h != prevH) {
			m.windowChangedOffline = true
		}
		return nil
	}
	if !m.windowSupported {
		return nil
	}
	if !m.windowArmed {
		// The first size after the handshake is the startup size (or the result
		// of applying the stored one), not a user resize.
		m.windowArmed = true
		return nil
	}
	if w == m.windowSavedW && h == m.windowSavedH {
		m.windowDirty = false
		return nil
	}
	m.windowDirty = true
	return m.scheduleWindowSave(w, h)
}

// noteHelloWindow applies the stored size on the first handshake and refreshes
// the known value on reconnects. A reconnect never reapplies the size: the user
// may have resized the window while the daemon was away.
func (m *Model) noteHelloWindow(win *protocol.WindowSizePayload) tea.Cmd {
	if !m.windowConnected {
		m.windowConnected = true
		m.windowSupported = win != nil
		if win == nil {
			return nil
		}
		m.windowSavedW, m.windowSavedH = win.Width, win.Height
		if !m.windowChangedOffline && win.Width > 0 && win.Height > 0 {
			if m.sizeKnown {
				if m.width != win.Width || m.height != win.Height {
					m.applyWindow(win.Width, win.Height)
				}
			} else {
				m.windowApplyPending = true
			}
		}
		if m.sizeKnown {
			m.windowArmed = true
		}
		if m.windowChangedOffline && m.sizeKnown &&
			(m.width != win.Width || m.height != win.Height) {
			m.windowChangedOffline = false
			m.windowDirty = true
			return m.scheduleWindowSave(m.width, m.height)
		}
		m.windowChangedOffline = false
		return nil
	}

	m.windowApplyPending = false
	if win == nil {
		m.windowSupported = false
		return nil
	}
	m.windowSupported = true
	m.windowSavedW, m.windowSavedH = win.Width, win.Height
	if m.windowChangedOffline && m.sizeKnown &&
		(m.width != win.Width || m.height != win.Height) {
		m.windowDirty = true
	}
	m.windowChangedOffline = false
	if m.windowDirty && m.sizeKnown &&
		m.width == m.windowSavedW && m.height == m.windowSavedH {
		m.windowDirty = false
	}
	if m.windowDirty && m.sizeKnown {
		return m.scheduleWindowSave(m.width, m.height)
	}
	return nil
}

func (m *Model) scheduleWindowSave(w, h int) tea.Cmd {
	return tea.Tick(windowSaveDelay, func(time.Time) tea.Msg {
		return windowSaveMsg{width: w, height: h}
	})
}

// handleWindowSave persists the size once the debounce expires. A newer resize
// replaces the pending value, so stale timers do nothing.
func (m *Model) handleWindowSave(msg windowSaveMsg) tea.Cmd {
	if !m.connected || !m.windowSupported {
		return nil
	}
	if msg.width != m.width || msg.height != m.height {
		return nil
	}
	if msg.width == m.windowSavedW && msg.height == m.windowSavedH {
		m.windowDirty = false
		return nil
	}
	width, height := msg.width, msg.height
	send := m.sendWindow
	return func() tea.Msg {
		return windowSavedMsg{width: width, height: height, err: send(width, height)}
	}
}

func (m *Model) handleWindowSaved(msg windowSavedMsg) {
	if msg.err != nil {
		return
	}
	m.windowSavedW, m.windowSavedH = msg.width, msg.height
	if m.width == msg.width && m.height == msg.height {
		m.windowDirty = false
	}
}

// flushWindowSize makes a last synchronous attempt to keep the size when the
// TUI exits before the debounce fired.
func (m *Model) flushWindowSize() {
	if !m.connected || !m.windowSupported || !m.windowDirty || !m.sizeKnown {
		return
	}
	if m.width == m.windowSavedW && m.height == m.windowSavedH {
		m.windowDirty = false
		return
	}
	if err := m.sendWindow(m.width, m.height); err != nil {
		return
	}
	m.windowSavedW, m.windowSavedH = m.width, m.height
	m.windowDirty = false
}

func (m *Model) sendWindowSize(w, h int) error {
	if !m.connected || m.client == nil {
		return nil
	}
	return m.client.Send(protocol.ClientMessage{
		Type:   protocol.TypeSetWindowSize,
		Width:  w,
		Height: h,
	})
}

// resizeSequence is the xterm window manipulation sequence that asks the
// terminal to resize its text area to w columns by h rows. Conhost and modern
// Windows Terminal implement it; terminals that do not simply ignore it, so it
// is safer than SetConsoleWindowInfo, which leaves Windows Terminal reporting a
// size the real window does not have.
func resizeSequence(w, h int) string {
	return fmt.Sprintf("\x1b[8;%d;%dt", h, w)
}

func resizeTerminal(w, h int) {
	_, _ = fmt.Fprint(os.Stdout, resizeSequence(w, h))
}
