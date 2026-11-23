// Package wt has no Windows Terminal integration on macOS; the TUI is opened
// through the daemon's .command launcher.
package wt

// LaunchDefault reports that Windows Terminal is unavailable. macOS opens the
// TUI through its own .command launcher instead.
func LaunchDefault(string, ...string) (bool, error) {
	return false, nil
}
