//go:build windows

package minitimer

import (
	"fmt"
	"image/color"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/theme"
)

const (
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmTimer         = 0x0113
	wmLButtonUp     = 0x0202
	wmRButtonUp     = 0x0205
	wmDisplayChange = 0x007E
	wmSettingChange = 0x001A
	wmDPIChanged    = 0x02E0
	wmNull          = 0x0000

	wsPopup        = 0x80000000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008
	wsExNoActivate = 0x08000000

	swShowNoActivate = 4
	swHide           = 0

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoActivate = 0x0010
	swpNoOwnerZ   = 0x0200

	mfString    = 0x00000000
	mfGrayed    = 0x00000001
	mfSeparator = 0x00000800

	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	timerID        = 1
	timerInterval  = 250
	pollInterval   = 500 * time.Millisecond
	hideAfterClick = 1500 * time.Millisecond

	monitorDefaultToNearest = 2
	dcBrush                 = 18
	idcArrow                = 32512

	menuOpenID  = 1
	menuPauseID = 2
	menuStopID  = 3
	menuHideID  = 4
	menuSepID   = 0

	dpiFallback = 96
	widgetClass = "TuickTackMiniTimer"

	consoleWindowClass  = "ConsoleWindowClass"
	terminalWindowClass = "CASCADIA_HOSTING_WINDOW_CLASS"

	processQueryLimitedInformation = 0x1000
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procSetTimer            = user32.NewProc("SetTimer")
	procKillTimer           = user32.NewProc("KillTimer")
	procInvalidateRect      = user32.NewProc("InvalidateRect")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetWindowTextW      = user32.NewProc("GetWindowTextW")
	procFindWindowW         = user32.NewProc("FindWindowW")
	procIsWindowVisible     = user32.NewProc("IsWindowVisible")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem     = user32.NewProc("GetDpiForSystem")
	procFillRect            = user32.NewProc("FillRect")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procGetStockObject  = gdi32.NewProc("GetStockObject")
	procSetDCBrushColor = gdi32.NewProc("SetDCBrushColor")

	procSHQueryUserNotificationState = shell32.NewProc("SHQueryUserNotificationState")
	procSHAppBarMessage              = shell32.NewProc("SHAppBarMessage")
)

// Widget is the taskbar overlay. All window state is owned by the goroutine
// that calls Run; Close only posts a message.
type Widget struct {
	actions Actions
	logger  *log.Logger

	mu         sync.Mutex
	hwnd       windows.Handle
	closed     bool
	done       chan struct{}
	closeOnce  sync.Once
	finishOnce sync.Once

	// Owned by the message-loop goroutine.
	layout      Layout
	haveLayout  bool
	visible     bool
	menuOpen    bool
	hiddenUntil time.Time
	finishWin   finishWindow
	env         Environment
	tray        trayInfo
	envAt       time.Time
}

type trayInfo struct {
	rect rect
	ok   bool
}

type point struct {
	X, Y int32
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type msg struct {
	Hwnd     windows.HWND
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type paintStruct struct {
	Hdc       windows.Handle
	Erase     int32
	Paint     rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

type appBarData struct {
	Size            uint32
	Hwnd            windows.Handle
	CallbackMessage uint32
	Edge            uint32
	Rect            rect
	LParam          uintptr
}

// New returns a widget bound to the daemon actions. Run creates the window.
func New(actions Actions) *Widget {
	return &Widget{actions: actions, done: make(chan struct{})}
}

// SetLogger routes widget errors to the daemon log. Call before Run.
func (w *Widget) SetLogger(logger *log.Logger) { w.logger = logger }

// Supported reports whether this build draws the taskbar widget.
func Supported() bool { return true }

func (w *Widget) logf(format string, args ...any) {
	if w.logger != nil {
		w.logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Run creates the popup and blocks on its message loop. The Win32 window must
// live on one OS thread, so this goroutine locks itself before creating it.
func (w *Widget) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if w.actions == nil {
		w.finish()
		return
	}
	if w.isClosed() {
		w.finish()
		return
	}
	if err := w.create(); err != nil {
		w.logf("mini timer: %v", err)
		w.finish()
		return
	}
	w.messageLoop()
	w.finish()
}

// Close asks the message loop to exit and waits briefly for it. It is safe from
// any goroutine and safe to call twice.
func (w *Widget) Close() {
	w.closeOnce.Do(func() {
		w.mu.Lock()
		w.closed = true
		hwnd := w.hwnd
		w.mu.Unlock()
		if hwnd != 0 {
			procPostMessageW.Call(uintptr(hwnd), wmClose, 0, 0)
		}
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
		}
	})
}

func (w *Widget) isClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

func (w *Widget) finish() {
	w.finishOnce.Do(func() { close(w.done) })
}

func (w *Widget) create() error {
	module, _, _ := procGetModuleHandleW.Call(0)
	if module == 0 {
		return fmt.Errorf("GetModuleHandleW failed")
	}
	className, err := windows.UTF16PtrFromString(widgetClass)
	if err != nil {
		return err
	}
	windowName, err := windows.UTF16PtrFromString("Tuick-Tack mini timer")
	if err != nil {
		return err
	}
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc := wndClassEx{
		Size:      uint32(unsafe.Sizeof(wndClassEx{})),
		Style:     0x0002 | 0x0001, // CS_HREDRAW | CS_VREDRAW
		WndProc:   windows.NewCallback(w.wndProc),
		Instance:  windows.Handle(module),
		Cursor:    windows.Handle(cursor),
		ClassName: className,
	}
	atom, _, regErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	_ = atom // already-exists is fine; CreateWindowEx below reports a real failure

	hwnd, _, createErr := procCreateWindowExW.Call(
		wsExToolWindow|wsExTopmost|wsExNoActivate,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsPopup,
		0, 0, 0, 0,
		0, 0, module, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %v (register: %v)", createErr, regErr)
	}

	w.mu.Lock()
	w.hwnd = windows.Handle(hwnd)
	closed := w.closed
	w.mu.Unlock()

	procSetTimer.Call(hwnd, timerID, timerInterval, 0)
	if closed {
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
		return nil
	}
	w.tick()
	return nil
}

func (w *Widget) messageLoop() {
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(r) {
		case -1:
			return
		case 0:
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (w *Widget) wndProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			w.paint(windows.Handle(hdc))
			procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		}
		return 0
	case wmEraseBkgnd:
		return 1
	case wmTimer:
		w.tick()
		return 0
	case wmLButtonUp:
		w.leftClick()
		return 0
	case wmRButtonUp:
		w.rightClick()
		return 0
	case wmClose:
		procDestroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		procKillTimer.Call(uintptr(hwnd), timerID)
		procPostQuitMessage.Call(0)
		return 0
	case wmDisplayChange, wmSettingChange, wmDPIChanged:
		w.envAt = time.Time{}
		w.haveLayout = false
		w.tick()
		return 0
	default:
		return defWindowProc(hwnd, message, wParam, lParam)
	}
}

func defWindowProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return r
}

func (w *Widget) tick() {
	if w.hwnd == 0 || w.menuOpen {
		return
	}
	now := time.Now()
	if w.envAt.IsZero() || now.Sub(w.envAt) >= pollInterval {
		w.env, w.tray = w.probe()
		w.envAt = now
	}

	st := w.actions.Snapshot()
	cfg := w.actions.Config()
	w.finishWin.observe(st, now)
	show := ShouldShow(cfg.MiniTimer, st, w.env) && now.After(w.hiddenUntil) &&
		!w.finishWin.closed(now)
	if !show {
		if w.visible {
			w.hide()
		}
		return
	}

	cell := w.cellSize()
	layout := LayoutForState(cell, st)
	w.layout = layout
	w.haveLayout = true
	w.place(layout)
	if !w.visible {
		w.show()
	}
	procInvalidateRect.Call(uintptr(w.hwnd), 0, 0)
}

func (w *Widget) probe() (Environment, trayInfo) {
	env := Environment{}
	tray, ok := findTaskbar()
	if !ok {
		return env, trayInfo{}
	}
	var tr rect
	if !getWindowRect(tray, &tr) {
		return env, trayInfo{}
	}
	info := monitorFromWindow(tray)
	visible := isWindowVisible(tray) && onMonitor(tr, info)
	horizontal := (tr.Right - tr.Left) >= (tr.Bottom - tr.Top)
	autoHide := taskbarAutoHide()
	if autoHide && visible {
		var pt point
		if getCursorPos(&pt) && !pointInRect(pt, tr) {
			visible = false
		}
	}
	env.TaskbarVisible = visible
	env.TaskbarHorizontal = horizontal
	env.Fullscreen = fullscreenActive()
	env.ShellOverlay = shellOverlayForeground()
	env.TUIFocused = tuiFocused()
	return env, trayInfo{
		rect: tr,
		ok:   true,
	}
}

func (w *Widget) cellSize() int {
	dpi := 0
	if err := procGetDpiForWindow.Find(); err == nil {
		r, _, _ := procGetDpiForWindow.Call(uintptr(w.hwnd))
		dpi = int(r)
	}
	if dpi == 0 {
		if err := procGetDpiForSystem.Find(); err == nil {
			r, _, _ := procGetDpiForSystem.Call()
			dpi = int(r)
		}
	}
	if dpi == 0 {
		dpi = dpiFallback
	}
	return CellForDPI(dpi)
}

func (w *Widget) place(layout Layout) {
	if !w.tray.ok {
		return
	}
	cell := layout.Cell
	x := int(w.tray.rect.Left) + 8*cell/3
	trayH := int(w.tray.rect.Bottom - w.tray.rect.Top)
	y := int(w.tray.rect.Top) + (trayH-layout.Height)/2
	if y < int(w.tray.rect.Top) {
		y = int(w.tray.rect.Top)
	}
	procSetWindowPos.Call(
		uintptr(w.hwnd), ^uintptr(0),
		uintptr(x), uintptr(y), uintptr(layout.Width), uintptr(layout.Height),
		swpNoActivate)
	// Re-assert the topmost position without moving or resizing. The shell can
	// bring its own topmost windows forward after a taskbar refresh.
	procSetWindowPos.Call(
		uintptr(w.hwnd), ^uintptr(0),
		0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate|swpNoOwnerZ)
}

func (w *Widget) show() {
	if w.hwnd == 0 {
		return
	}
	procShowWindow.Call(uintptr(w.hwnd), swShowNoActivate)
	w.visible = true
}

func (w *Widget) hide() {
	if w.hwnd == 0 {
		return
	}
	procShowWindow.Call(uintptr(w.hwnd), swHide)
	w.visible = false
}

func (w *Widget) leftClick() {
	w.hiddenUntil = time.Now().Add(hideAfterClick)
	w.hide()
	w.actions.OpenTUI()
}

func (w *Widget) rightClick() {
	st := w.actions.Snapshot()
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	appendMenu(menu, mfString, menuOpenID, copy.T(copy.TrayOpen))
	appendMenu(menu, mfSeparator, menuSepID, "")
	switch st.Status {
	case engine.Running:
		appendMenu(menu, mfString, menuPauseID, copy.T(copy.TrayPause))
	case engine.Paused:
		appendMenu(menu, mfString, menuPauseID, copy.T(copy.TrayResume))
	default:
		appendMenu(menu, mfString|mfGrayed, menuPauseID, copy.T(copy.TrayPause))
	}
	flags := uintptr(mfString)
	if st.Status == engine.Idle {
		flags |= mfGrayed
	}
	appendMenu(menu, flags, menuStopID, copy.T(copy.TrayStop))
	appendMenu(menu, mfSeparator, menuSepID, "")
	appendMenu(menu, mfString, menuHideID, copy.T(copy.MiniTimerHide))

	var pt point
	if !getCursorPos(&pt) {
		return
	}
	w.menuOpen = true
	defer func() {
		w.menuOpen = false
		w.envAt = time.Time{}
		w.tick()
	}()
	procSetForegroundWindow.Call(uintptr(w.hwnd))
	cmd, _, _ := procTrackPopupMenu.Call(
		menu,
		tpmReturnCmd|tpmRightButton|tpmNoNotify,
		uintptr(int32(pt.X)), uintptr(int32(pt.Y)),
		0,
		uintptr(w.hwnd),
		0,
	)
	procPostMessageW.Call(uintptr(w.hwnd), wmNull, 0, 0)

	switch cmd {
	case menuOpenID:
		w.hiddenUntil = time.Now().Add(hideAfterClick)
		w.hide()
		w.actions.OpenTUI()
	case menuPauseID:
		w.actions.TogglePause()
	case menuStopID:
		w.actions.Stop()
	case menuHideID:
		w.actions.SetMiniTimer(false)
	}
}

func appendMenu(menu uintptr, flags uintptr, id uintptr, text string) {
	var ptr *uint16
	if text != "" {
		ptr, _ = windows.UTF16PtrFromString(text)
	}
	procAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(ptr)))
}

func (w *Widget) paint(hdc windows.Handle) {
	if !w.haveLayout {
		w.layout = LayoutForState(w.cellSize(), w.actions.Snapshot())
		w.haveLayout = true
	}
	l := w.layout
	cfg := w.actions.Config()
	st := w.actions.Snapshot()
	pal := theme.Get(cfg.Theme)
	grid := RenderAt(st, pal, float64(time.Now().UnixMilli())/260, l.Cells)

	brush, _, _ := procGetStockObject.Call(dcBrush)
	fillRect(hdc, brush, 0, 0, l.Width, l.Height, pal.Panel)
	// 1 px border; the bar can overdraw the bottom pixel inside its width.
	fillRect(hdc, brush, 0, 0, l.Width, 1, pal.Border)
	fillRect(hdc, brush, 0, l.Height-1, l.Width, 1, pal.Border)
	fillRect(hdc, brush, 0, 0, 1, l.Height, pal.Border)
	fillRect(hdc, brush, l.Width-1, 0, 1, l.Height, pal.Border)

	x0, y0 := l.ContentX(), l.ContentY()
	for y := 0; y < grid.H; y++ {
		for x := 0; x < grid.W; x++ {
			cell := grid.At(x, y)
			px, py := x0+x*l.Cell, y0+y*l.Cell
			if cell.HasBG {
				fillRect(hdc, brush, px, py, l.Cell, l.Cell, cell.BG)
			}
			if cell.HasFG {
				fillRect(hdc, brush, px, py, l.Cell, l.Cell, cell.FG)
			}
		}
	}
}

func fillRect(hdc windows.Handle, brush uintptr, x, y, w, h int, c color.RGBA) {
	if w <= 0 || h <= 0 {
		return
	}
	r := rect{Left: int32(x), Top: int32(y), Right: int32(x + w), Bottom: int32(y + h)}
	procSetDCBrushColor.Call(uintptr(hdc), colorRef(c))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&r)), brush)
}

func colorRef(c color.RGBA) uintptr {
	return uintptr(uint32(c.R) | uint32(c.G)<<8 | uint32(c.B)<<16)
}

func findTaskbar() (windows.Handle, bool) {
	class, err := windows.UTF16PtrFromString("Shell_TrayWnd")
	if err != nil {
		return 0, false
	}
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(class)), 0)
	if h == 0 {
		return 0, false
	}
	return windows.Handle(h), true
}

func getWindowRect(hwnd windows.Handle, r *rect) bool {
	ok, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(r)))
	return ok != 0
}

func isWindowVisible(hwnd windows.Handle) bool {
	ok, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return ok != 0
}

func monitorFromWindow(hwnd windows.Handle) monitorInfo {
	hmon, _, _ := procMonitorFromWindow.Call(uintptr(hwnd), monitorDefaultToNearest)
	if hmon == 0 {
		return monitorInfo{}
	}
	mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	ok, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi)))
	if ok == 0 {
		return monitorInfo{}
	}
	return mi
}

func onMonitor(r rect, mi monitorInfo) bool {
	if mi.Size == 0 {
		return true
	}
	m := mi.Monitor
	// A taskbar pushed completely off its monitor is auto-hidden.
	if r.Bottom <= m.Top+1 || r.Top >= m.Bottom-1 ||
		r.Right <= m.Left+1 || r.Left >= m.Right-1 {
		return false
	}
	return true
}

func taskbarAutoHide() bool {
	abd := appBarData{Size: uint32(unsafe.Sizeof(appBarData{}))}
	r, _, _ := procSHAppBarMessage.Call(0x00000004, uintptr(unsafe.Pointer(&abd))) // ABM_GETSTATE
	return r&1 != 0                                                                // ABS_AUTOHIDE
}

func fullscreenActive() bool {
	if err := procSHQueryUserNotificationState.Find(); err != nil {
		return false
	}
	var state uint32
	hr, _, _ := procSHQueryUserNotificationState.Call(uintptr(unsafe.Pointer(&state)))
	if int32(hr) != 0 {
		return false
	}
	switch state {
	case 1, 2, 3, 4, 6: // not present, busy, D3D fullscreen, presentation, quiet time
		return true
	default:
		return false
	}
}

func shellOverlayForeground() bool {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return false
	}
	class := windowClass(hwnd)
	switch class {
	case "Shell_TrayWnd",
		"Shell_SecondaryTrayWnd",
		"StartMenuExperienceHost",
		"XamlExplorerHostIslandWindow",
		"Windows.Internal.Shell.TabProxyWindow",
		"NotifyIconOverflowWindow",
		"TopLevelWindowForOverflowXamlIsland",
		"#32768":
		return true
	case "Windows.UI.Core.CoreWindow":
		// Many UWP apps use this class too (Calculator, Settings, Photos...).
		// Only the shell windows should hide the widget.
		return shellProcess(foregroundProcessName())
	default:
		return false
	}
}

func shellProcess(name string) bool {
	switch name {
	case "explorer.exe",
		"shellexperiencehost.exe",
		"startmenuexperiencehost.exe",
		"searchhost.exe",
		"searchui.exe",
		"textinputhost.exe":
		return true
	default:
		return false
	}
}

// tuiFocused checks only the foreground window: title and console class. This
// is equivalent to termwin.Find for the focused case and avoids EnumWindows,
// whose per-call callback would exhaust the runtime callback table in a
// long-running daemon.
func tuiFocused() bool {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 || windowText(hwnd) != buildinfo.Name {
		return false
	}
	switch windowClass(hwnd) {
	case consoleWindowClass, terminalWindowClass:
		return true
	default:
		return false
	}
}

func windowText(hwnd windows.HWND) string {
	var buf [256]uint16
	n, _, _ := procGetWindowTextW.Call(
		uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:int(n)])
}

func windowClass(hwnd windows.HWND) string {
	var class [128]uint16
	n, err := windows.GetClassName(hwnd, &class[0], int32(len(class)))
	if err != nil || n == 0 {
		return ""
	}
	return windows.UTF16ToString(class[:n])
}

func foregroundProcessName() string {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return ""
	}
	var pid uint32
	_, _ = windows.GetWindowThreadProcessId(hwnd, &pid)
	if pid == 0 {
		return ""
	}
	h, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
}

func getCursorPos(pt *point) bool {
	ok, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(pt)))
	return ok != 0
}

func pointInRect(pt point, r rect) bool {
	return pt.X >= r.Left && pt.X < r.Right && pt.Y >= r.Top && pt.Y < r.Bottom
}
