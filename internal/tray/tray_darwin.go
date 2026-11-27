//go:build darwin && cgo

package tray

import "fyne.io/systray"

// miniTimerSupported is false on macOS: the menu bar already shows the
// remaining time next to its icon.
const miniTimerSupported = false

// initTray leaves the click alone on macOS: the menu-bar button opens the menu,
// and Open is the first item instead of a hidden tap target.
func initTray(Actions) {}

// setMenuBarTitle shows the countdown next to the icon. The empty string
// removes the text so the idle and finished states stay icon-only.
func setMenuBarTitle(text string) {
	systray.SetTitle(text)
}
