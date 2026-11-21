package tui

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/config"
	"tuick-tack/internal/protocol"
)

const (
	// windowSaveDelay is how long the TUI waits after the last resize or move
	// before it persists the new placement. It keeps a window drag from writing
	// the config on every cell.
	windowSaveDelay = 300 * time.Millisecond
	// windowPosSampleInterval is how often the window position is sampled. The
	// check is cheap but there is no move event to subscribe to.
	windowPosSampleInterval = 500 * time.Millisecond
	// windowPosApplyWindow bounds the startup retries while the terminal sets
	// the window title and the real host window can be located.
	windowPosApplyWindow = 2 * time.Second
	// windowPosSettle is how long a host may still reposition a freshly created
	// window after the stored placement succeeded. Moves in that window are
	// corrected instead of being saved as a user move.
	windowPosSettle = 2 * time.Second
)

type windowSaveMsg struct {
	width, height int
	x, y          int
	hasPos        bool
}

type windowSavedMsg struct {
	width, height int
	pos           *config.WindowPosition
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

	// Reassert the position once after the terminal applied the stored size:
	// some hosts move the window while resizing.
	if m.posReapply {
		m.posReapply = false
		m.tryPlaceWindow()
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
				m.windowClearPending = true
				m.posReapply = m.canPlaceWindow() && m.windowHasPos
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
	if m.windowMaximized() {
		// The maximized grid is not a placement: saving it would reopen a huge
		// normal window.
		if w == m.windowSavedW && h == m.windowSavedH {
			m.windowDirty = false
		}
		return nil
	}
	if w == m.windowSavedW && h == m.windowSavedH {
		m.windowDirty = false
		return nil
	}
	m.windowDirty = true
	return m.scheduleWindowSave()
}

// noteHelloWindow applies the stored placement on the first handshake and
// refreshes the known values on reconnects. A reconnect never reapplies the
// size or position: the user may have moved the window while the daemon was
// away.
func (m *Model) noteHelloWindow(win *protocol.WindowSizePayload) tea.Cmd {
	if !m.windowConnected {
		m.windowConnected = true
		m.windowSupported = win != nil
		if win == nil {
			return nil
		}
		m.windowSavedW, m.windowSavedH = win.Width, win.Height
		m.noteHelloPosition(win.Position, true)
		if !m.windowChangedOffline && win.Width > 0 && win.Height > 0 {
			if m.sizeKnown {
				if m.width != win.Width || m.height != win.Height {
					m.applyWindow(win.Width, win.Height)
					m.windowClearPending = true
					m.posReapply = m.canPlaceWindow() && m.windowHasPos
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
			return m.scheduleWindowSave()
		}
		m.windowChangedOffline = false
		return nil
	}

	m.windowApplyPending = false
	m.posReapply = false
	if win == nil {
		m.windowSupported = false
		return nil
	}
	m.windowSupported = true
	m.windowSavedW, m.windowSavedH = win.Width, win.Height
	m.noteHelloPosition(win.Position, false)
	if m.windowChangedOffline && m.sizeKnown &&
		(m.width != win.Width || m.height != win.Height) {
		m.windowDirty = true
	}
	m.windowChangedOffline = false
	if m.windowDirty && m.sizeKnown &&
		m.width == m.windowSavedW && m.height == m.windowSavedH {
		m.windowDirty = false
	}
	if m.posDirty && m.sizeKnown {
		return m.scheduleWindowSave()
	}
	if m.windowDirty && m.sizeKnown {
		return m.scheduleWindowSave()
	}
	return nil
}

// noteHelloPosition records the stored position. Only a first handshake applies
// it; a reconnect refreshes the value so a move made while the daemon was away
// can be persisted.
func (m *Model) noteHelloPosition(pos *config.WindowPosition, apply bool) {
	if !m.canPlaceWindow() || pos == nil {
		return
	}
	m.windowHasPos = true
	m.posKnown = true
	m.windowSavedX, m.windowSavedY = pos.X, pos.Y
	if !apply {
		if m.posArmed && (m.windowX != pos.X || m.windowY != pos.Y) {
			m.posDirty = true
		}
		return
	}
	m.posApplyPending = true
	m.posApplyUntil = m.now().Add(windowPosApplyWindow)
	if m.tryPlaceWindow() {
		m.posSettleUntil = m.now().Add(windowPosSettle)
	}
}

// tryPlaceWindow moves the window to the stored position. It returns whether
// the move succeeded; a failure leaves the retry pending until the startup
// window closes.
func (m *Model) tryPlaceWindow() bool {
	if !m.canPlaceWindow() || !m.windowHasPos || m.moveWindow == nil {
		m.posApplyPending = false
		return false
	}
	if m.windowMaximized() {
		return false
	}
	if !m.moveWindow(m.windowSavedX, m.windowSavedY) {
		return false
	}
	m.posApplyPending = false
	m.posArmed = true
	m.posValid = true
	m.windowX, m.windowY = m.windowSavedX, m.windowSavedY
	return true
}

// canPlaceWindow reports whether the TUI owns the whole terminal window. A TUI
// sharing a console with a shell must not move the user's terminal around.
func (m *Model) canPlaceWindow() bool {
	return m.windowOwned
}

func (m *Model) windowMaximized() bool {
	if !m.windowOwned || m.windowMaximizedFn == nil {
		return false
	}
	return m.windowMaximizedFn()
}

// sampleWindow records a user move after the debounce interval. It runs from
// the existing tick, so it adds no timer.
func (m *Model) sampleWindow() tea.Cmd {
	if !m.windowOwned || m.windowPositionFn == nil {
		return nil
	}
	now := m.now()
	if !m.lastPosSample.IsZero() && now.Sub(m.lastPosSample) < windowPosSampleInterval {
		return nil
	}
	m.lastPosSample = now

	if m.posApplyPending {
		if now.After(m.posApplyUntil) {
			// The stored position could not be applied (missing window,
			// disconnected monitor). Forget it locally so a later size save
			// does not send an unobserved 0,0; the daemon keeps the value.
			m.posApplyPending = false
			m.windowHasPos = false
			m.posKnown = false
			m.posArmed = false
		} else if !m.tryPlaceWindow() {
			return nil
		} else {
			m.posSettleUntil = now.Add(windowPosSettle)
		}
	}

	if !m.posSettleUntil.IsZero() {
		if now.Before(m.posSettleUntil) {
			// The host can move a freshly created window after our first
			// SetWindowPos (for example while it finishes a Windows Terminal
			// new-tab handoff). Put it back instead of saving that as a user
			// move.
			if x, y, ok := m.windowPositionFn(); ok && (x != m.windowSavedX || y != m.windowSavedY) {
				m.tryPlaceWindow()
			}
			return nil
		}
		m.posSettleUntil = time.Time{}
	}

	if !m.connected || !m.windowSupported || m.windowMaximized() {
		return nil
	}
	x, y, ok := m.windowPositionFn()
	if !ok {
		return nil
	}
	if !m.posArmed {
		// The startup placement is the baseline, not a user move.
		m.posArmed = true
		m.posValid = true
		m.windowHasPos = false
		m.windowX, m.windowY = x, y
		return nil
	}
	m.posValid = true
	if x == m.windowX && y == m.windowY {
		return nil
	}
	m.windowX, m.windowY = x, y
	m.windowHasPos = true
	if m.posKnown && x == m.windowSavedX && y == m.windowSavedY {
		m.posDirty = false
		return nil
	}
	m.posDirty = true
	if !m.connected {
		return nil
	}
	return m.scheduleWindowSave()
}

func (m *Model) scheduleWindowSave() tea.Cmd {
	width, height := m.width, m.height
	x, y := m.windowX, m.windowY
	hasPos := m.windowHasPos && m.posValid
	return tea.Tick(windowSaveDelay, func(time.Time) tea.Msg {
		return windowSaveMsg{width: width, height: height, x: x, y: y, hasPos: hasPos}
	})
}

// handleWindowSave persists the placement once the debounce expires. A newer
// resize or move replaces the pending value, so stale timers do nothing.
func (m *Model) handleWindowSave(msg windowSaveMsg) tea.Cmd {
	if !m.connected || !m.windowSupported || m.windowMaximized() {
		return nil
	}
	if msg.width != m.width || msg.height != m.height {
		return nil
	}
	if msg.hasPos && (!m.windowHasPos || msg.x != m.windowX || msg.y != m.windowY) {
		return nil
	}
	width, height := msg.width, msg.height
	var pos *config.WindowPosition
	if msg.hasPos {
		pos = &config.WindowPosition{X: msg.x, Y: msg.y}
	}
	send := m.sendWindow
	return func() tea.Msg {
		return windowSavedMsg{width: width, height: height, pos: pos, err: send(width, height, pos)}
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
	if msg.pos == nil {
		return
	}
	m.windowSavedX, m.windowSavedY = msg.pos.X, msg.pos.Y
	m.posKnown = true
	m.windowHasPos = true
	if m.windowX == msg.pos.X && m.windowY == msg.pos.Y {
		m.posDirty = false
	}
}

// flushWindow makes a last synchronous attempt to keep the placement when the
// TUI exits before the debounce fired.
func (m *Model) flushWindow() {
	if !m.connected || !m.windowSupported || !m.sizeKnown || m.windowMaximized() {
		return
	}
	settling := !m.posSettleUntil.IsZero() && m.now().Before(m.posSettleUntil)
	if m.windowOwned && m.windowPositionFn != nil && !settling {
		if x, y, ok := m.windowPositionFn(); ok {
			m.posValid = true
			if !m.posArmed {
				// The startup placement is a baseline, even when closing early.
				m.posArmed = true
				m.windowHasPos = false
				m.windowX, m.windowY = x, y
			} else {
				if x != m.windowX || y != m.windowY {
					m.windowX, m.windowY = x, y
					m.windowHasPos = true
				}
				if m.posKnown {
					if x == m.windowSavedX && y == m.windowSavedY {
						m.posDirty = false
					} else {
						m.posDirty = true
					}
				} else if m.windowHasPos {
					m.posDirty = true
				}
			}
		}
	}
	if m.windowDirty && m.width == m.windowSavedW && m.height == m.windowSavedH {
		m.windowDirty = false
	}
	if !m.windowDirty && !m.posDirty {
		return
	}
	var pos *config.WindowPosition
	if m.windowHasPos && m.windowOwned && m.posValid && !settling {
		pos = &config.WindowPosition{X: m.windowX, Y: m.windowY}
	}
	if err := m.sendWindow(m.width, m.height, pos); err != nil {
		return
	}
	m.windowSavedW, m.windowSavedH = m.width, m.height
	if pos != nil {
		m.windowSavedX, m.windowSavedY = pos.X, pos.Y
		m.posKnown = true
	}
	m.windowDirty = false
	m.posDirty = false
}

func (m *Model) sendWindowSize(w, h int, pos *config.WindowPosition) error {
	if !m.connected || m.client == nil {
		return nil
	}
	return m.client.Send(protocol.ClientMessage{
		Type:     protocol.TypeSetWindowSize,
		Width:    w,
		Height:   h,
		Position: pos,
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
