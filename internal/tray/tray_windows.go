package tray

import "fyne.io/systray"

// miniTimerSupported reports whether the taskbar overlay exists on this
// platform. The menu item is only added on Windows.
const miniTimerSupported = true

// initTray wires the left-click action. The Windows systray supports a tap
// callback, so a click on the icon opens the TUI directly.
func initTray(actions Actions) {
	systray.SetOnTapped(actions.OpenTUI)
}

// setMenuBarTitle is macOS-only; the notification area has no title text.
func setMenuBarTitle(string) {}
