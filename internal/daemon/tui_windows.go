package daemon

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"tuick-tack/internal/buildinfo"
)

const (
	tuiMutexSuffix = "-tui-mutex"

	// Console host classes: classic conhost and Windows Terminal.
	consoleWindowClass  = "ConsoleWindowClass"
	terminalWindowClass = "CASCADIA_HOSTING_WINDOW_CLASS"

	vkMenu        = 0x12
	keyEventKeyUp = 0x0002
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procIsIconic            = user32.NewProc("IsIconic")
	procShowWindow          = user32.NewProc("ShowWindow")
	procKeybdEvent          = user32.NewProc("keybd_event")
	procSwitchToThisWindow  = user32.NewProc("SwitchToThisWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

// TUIInstanceMutexName is the named mutex held by the single TUI instance for
// a given pipe (user and session).
func TUIInstanceMutexName(pipe string) string {
	return objectName(pipe) + tuiMutexSuffix
}

// AcquireTUIInstance takes the TUI singleton mutex. already is true when
// another TUI owns it; release must be called when the TUI exits.
func AcquireTUIInstance(pipe string) (release func(), already bool, err error) {
	h, already, err := AcquireInstance(TUIInstanceMutexName(pipe))
	if err != nil || already {
		return func() {}, already, err
	}
	return func() { _ = windows.CloseHandle(h) }, false, nil
}

// TUIRunning reports whether a TUI owns the singleton mutex. It only opens the
// existing object; it never creates one.
func TUIRunning(pipe string) bool {
	name, err := windows.UTF16PtrFromString(TUIInstanceMutexName(pipe))
	if err != nil {
		return false
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}

// FocusTUI brings the open TUI window to the foreground. It matches the exact
// window title and console class so other windows whose title merely contains
// the project name (editors, browsers) are ignored.
func FocusTUI() bool {
	var target windows.HWND
	callback := windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		if isTUIWindow(hwnd) {
			target = hwnd
			return 0
		}
		return 1
	})
	_ = windows.EnumWindows(callback, nil)
	if target == 0 {
		return false
	}

	if windows.GetForegroundWindow() == target {
		return true
	}
	if r, _, _ := procIsIconic.Call(uintptr(target)); r != 0 {
		_, _, _ = procShowWindow.Call(uintptr(target), windows.SW_RESTORE)
	}

	// SetForegroundWindow is blocked by the foreground lock when another
	// process owns the foreground. Tapping Alt releases the lock; this is the
	// standard workaround and works even when the tray click was delivered to
	// another thread (the shell's taskbar window, not ours).
	if err := procKeybdEvent.Find(); err == nil {
		_, _, _ = procKeybdEvent.Call(uintptr(vkMenu), 0, 0, 0)
		_, _, _ = procKeybdEvent.Call(uintptr(vkMenu), 0, keyEventKeyUp, 0)
	}
	if r, _, _ := procSetForegroundWindow.Call(uintptr(target)); r != 0 {
		return true
	}
	// SwitchToThisWindow (Alt+Tab's API) is the lock-free fallback for hosts
	// that still reject SetForegroundWindow.
	if err := procSwitchToThisWindow.Find(); err == nil {
		_, _, _ = procSwitchToThisWindow.Call(uintptr(target), 1)
	}
	return true
}

func isTUIWindow(hwnd windows.HWND) bool {
	var title [256]uint16
	n, _, _ := procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
	if n == 0 || windows.UTF16ToString(title[:int(n)]) != buildinfo.Name {
		return false
	}
	var class [128]uint16
	copied, err := windows.GetClassName(hwnd, &class[0], int32(len(class)))
	if err != nil || copied == 0 {
		return false
	}
	name := windows.UTF16ToString(class[:copied])
	if name != consoleWindowClass && name != terminalWindowClass {
		return false
	}
	r, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return r != 0
}
