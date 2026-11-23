package tui

import (
	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/termwin"
)

// configureWindow wires the terminal-window placement. On Windows the TUI can
// own its console window and remembers where the user put it.
func configureWindow(m *Model) {
	m.windowOwned = termwin.OwnsConsole()
	m.windowPositionFn = func() (int, int, bool) { return termwin.Position(buildinfo.Name) }
	m.moveWindow = func(x, y int) bool { return termwin.Move(buildinfo.Name, x, y) }
	m.windowMaximizedFn = func() bool { return termwin.Maximized(buildinfo.Name) }
}
