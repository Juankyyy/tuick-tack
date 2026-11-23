package daemon

import (
	"os/exec"

	"tuick-tack/internal/instance"
)

// TUIInstanceMutexName is the lock held by the single TUI instance for a given
// socket (user and session).
func TUIInstanceMutexName(pipe string) string {
	return instance.TUIMutexName(pipe)
}

// AcquireTUIInstance takes the TUI singleton lock. already is true when another
// TUI owns it; release must be called when the TUI exits.
func AcquireTUIInstance(pipe string) (release func(), already bool, err error) {
	return instance.AcquireTUI(pipe)
}

// TUIRunning reports whether a TUI owns the singleton lock.
func TUIRunning(pipe string) bool {
	return instance.TUIRunning(pipe)
}

// FocusTUI brings Terminal to the front. macOS does not let a background
// process locate another app's window reliably, so this is a best effort: the
// plan explicitly gives up the window memory on this platform.
func FocusTUI() bool {
	if err := exec.Command("/usr/bin/open", "-a", "Terminal").Run(); err != nil {
		return false
	}
	return true
}
