package tui

// configureWindow leaves the placement disabled on macOS: the app neither
// moves nor resizes the terminal window, and the daemon omits the window object
// from the handshake so the stored size is never applied.
func configureWindow(m *Model) {
	m.windowOwned = false
	m.windowPositionFn = func() (int, int, bool) { return 0, 0, false }
	m.moveWindow = func(int, int) bool { return false }
	m.windowMaximizedFn = func() bool { return false }
}
