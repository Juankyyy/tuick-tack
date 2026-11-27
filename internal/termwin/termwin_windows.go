// Package termwin locates and moves the console window that hosts the TUI. It
// is the only place that touches the user32 window APIs for the TUI window.
package termwin

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// Console host classes: classic conhost and Windows Terminal. The latter
	// owns the real top-level window; the console process only sees a hidden
	// PseudoConsoleWindow.
	consoleWindowClass  = "ConsoleWindowClass"
	terminalWindowClass = "CASCADIA_HOSTING_WINDOW_CLASS"

	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procFindWindowW           = user32.NewProc("FindWindowW")
	procIsWindowVisible       = user32.NewProc("IsWindowVisible")
	procGetWindowRect         = user32.NewProc("GetWindowRect")
	procSetWindowPos          = user32.NewProc("SetWindowPos")
	procIsZoomed              = user32.NewProc("IsZoomed")
	procIsIconic              = user32.NewProc("IsIconic")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
)

type rect struct {
	left, top, right, bottom int32
}

// OwnsConsole reports whether this process is the only client attached to its
// console. A TUI launched in a new console (from the tray or Explorer) owns its
// window; one running inside a shell tab shares the console and must not move
// the whole terminal window.
func OwnsConsole() bool {
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return n == 1
}

// Find returns the visible top-level window that hosts the TUI. It matches the
// exact window title and the console host class, so other windows whose title
// merely contains the app name are ignored. FindWindowW is used instead of
// EnumWindows: the latter needs a new Go callback on every call, and a
// long-running daemon would eventually exhaust the runtime callback table. The
// result is not cached. The handle is returned as uintptr so the package API
// does not expose a Win32 type; Windows callers convert it to windows.HWND.
func Find(name string) (uintptr, bool) {
	title, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, false
	}
	for _, className := range []string{consoleWindowClass, terminalWindowClass} {
		class, err := windows.UTF16PtrFromString(className)
		if err != nil {
			continue
		}
		h, _, _ := procFindWindowW.Call(
			uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)))
		if h == 0 {
			continue
		}
		if r, _, _ := procIsWindowVisible.Call(h); r != 0 {
			return h, true
		}
	}
	return 0, false
}

var cached struct {
	mu   sync.Mutex
	name string
	hwnd uintptr
}

// window returns the window handle, resolving it once and reusing it while it
// stays alive. The TUI owns its window for the process lifetime.
func window(name string) uintptr {
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if cached.hwnd != 0 && cached.name == name {
		if windows.IsWindow(windows.HWND(cached.hwnd)) {
			return cached.hwnd
		}
		cached.hwnd = 0
	}
	hwnd, ok := Find(name)
	if !ok {
		return 0
	}
	cached.name = name
	cached.hwnd = hwnd
	return hwnd
}

// Position returns the outer top-left of the window in screen pixels. It reports
// false when the window is maximized, minimized or not found, because those
// states do not describe a normal placement.
func Position(name string) (x, y int, ok bool) {
	hwnd := window(name)
	if hwnd == 0 || isIconic(hwnd) || isZoomed(hwnd) {
		return 0, 0, false
	}
	var r rect
	if r1, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r))); r1 == 0 {
		return 0, 0, false
	}
	return int(r.left), int(r.top), true
}

// Maximized reports whether the window is currently maximized.
func Maximized(name string) bool {
	hwnd := window(name)
	return hwnd != 0 && isZoomed(hwnd)
}

// Move places the outer top-left of the window at x,y without resizing,
// reordering or activating it. It does nothing when the window is maximized,
// minimized, not found or the target is outside every monitor.
func Move(name string, x, y int) bool {
	hwnd := window(name)
	if hwnd == 0 || isIconic(hwnd) || isZoomed(hwnd) {
		return false
	}
	if !onScreen(x, y) {
		return false
	}
	r1, _, _ := procSetWindowPos.Call(
		uintptr(hwnd), 0,
		uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
	return r1 != 0
}

func isZoomed(hwnd uintptr) bool {
	r, _, _ := procIsZoomed.Call(hwnd)
	return r != 0
}

func isIconic(hwnd uintptr) bool {
	r, _, _ := procIsIconic.Call(hwnd)
	return r != 0
}

// onScreen reports whether a point near the window's top-left falls inside the
// virtual screen. It keeps a saved position from a disconnected monitor from
// sending the window off-screen.
func onScreen(x, y int) bool {
	x0 := int(int32(metric(smXVirtualScreen)))
	y0 := int(int32(metric(smYVirtualScreen)))
	w := int(int32(metric(smCXVirtualScreen)))
	h := int(int32(metric(smCYVirtualScreen)))
	if w <= 0 || h <= 0 {
		return true
	}
	px, py := x+8, y+8
	return px >= x0 && py >= y0 && px < x0+w && py < y0+h
}

func metric(index int) int {
	r, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(r)
}
