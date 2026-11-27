//go:build darwin

package copy

// macOS starts the app at login with a LaunchAgent and has a menu bar instead
// of a system tray, so the Windows wording would be wrong.
func init() {
	texts[TrayStartup] = "Start at login"
	texts[StartupInstalled] = "Tuick-Tack will start at login."
	texts[StartupRemoved] = "Tuick-Tack will no longer start at login."
	texts[ExitActive] = "The countdown is still running in the menu bar."
	texts[HelpTrayNote] = "the menu bar keeps running"
	texts[HelpNarrowReadable] = "[n] readable bands"
	// osascript notifications ignore the launch URL, so a "Click to open"
	// body would promise an action that does not exist.
	texts[NotifyFoundBody] = "Version %s is available."
}
