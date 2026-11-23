package daemon

import (
	"golang.org/x/sys/windows"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/instance"
	"tuick-tack/internal/termwin"
)

const (
	vkMenu        = 0x12
	keyEventKeyUp = 0x0002
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procKeybdEvent          = user32.NewProc("keybd_event")
	procSwitchToThisWindow  = user32.NewProc("SwitchToThisWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

// TUIInstanceMutexName is the named mutex held by the single TUI instance for
// a given pipe (user and session).
func TUIInstanceMutexName(pipe string) string {
	return instance.TUIMutexName(pipe)
}

// AcquireTUIInstance takes the TUI singleton mutex. already is true when
// another TUI owns it; release must be called when the TUI exits.
func AcquireTUIInstance(pipe string) (release func(), already bool, err error) {
	return instance.AcquireTUI(pipe)
}

// TUIRunning reports whether a TUI owns the singleton mutex. It only opens the
// existing object; it never creates one.
func TUIRunning(pipe string) bool {
	return instance.TUIRunning(pipe)
}

// FocusTUI brings the open TUI window to the foreground. termwin.Find matches
// the exact window title and console class so other windows whose title merely
// contains the project name (editors, browsers) are ignored.
func FocusTUI() bool {
	target, ok := termwin.Find(buildinfo.Name)
	if !ok {
		return false
	}

	if windows.GetForegroundWindow() == windows.HWND(target) {
		return true
	}
	if r, _, _ := procIsIconic.Call(target); r != 0 {
		_, _, _ = procShowWindow.Call(target, windows.SW_RESTORE)
	}

	// SetForegroundWindow is blocked by the foreground lock when another
	// process owns the foreground. Tapping Alt releases the lock; this is the
	// standard workaround and works even when the tray click was delivered to
	// another thread (the shell's taskbar window, not ours).
	if err := procKeybdEvent.Find(); err == nil {
		_, _, _ = procKeybdEvent.Call(uintptr(vkMenu), 0, 0, 0)
		_, _, _ = procKeybdEvent.Call(uintptr(vkMenu), 0, keyEventKeyUp, 0)
	}
	if r, _, _ := procSetForegroundWindow.Call(target); r != 0 {
		return true
	}
	// SwitchToThisWindow (Alt+Tab's API) is the lock-free fallback for hosts
	// that still reject SetForegroundWindow.
	if err := procSwitchToThisWindow.Find(); err == nil {
		_, _, _ = procSwitchToThisWindow.Call(target, 1)
	}
	return true
}
