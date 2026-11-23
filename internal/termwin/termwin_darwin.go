// Package termwin locates and moves the console window that hosts the TUI. On
// macOS it is a no-op: the app does not manage the terminal window.
package termwin

// The TUI does not move or resize the terminal window on macOS. These stubs
// keep the package API identical so the shared TUI code compiles; the daemon
// also omits the window object from the handshake.

// OwnsConsole reports false: macOS never gives the app a window to place.
func OwnsConsole() bool { return false }

// Find never finds a window.
func Find(string) (uintptr, bool) { return 0, false }

// Position reports no saved position.
func Position(string) (x, y int, ok bool) { return 0, 0, false }

// Maximized reports false.
func Maximized(string) bool { return false }

// Move does nothing.
func Move(string, int, int) bool { return false }
