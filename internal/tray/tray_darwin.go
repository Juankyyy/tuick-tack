//go:build darwin && cgo

package tray

import "fyne.io/systray"

// initTray leaves the click alone on macOS: the menu-bar button opens the menu,
// and Open is the first item instead of a hidden tap target.
func initTray(Actions) {}

// setMenuBarTitle shows the countdown next to the icon. The empty string
// removes the text so the idle and finished states stay icon-only.
func setMenuBarTitle(text string) {
	systray.SetTitle(text)
}
