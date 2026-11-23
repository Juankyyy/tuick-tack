package daemon

import (
	"os"

	"tuick-tack/internal/termwin"
	"tuick-tack/internal/winproc"
	"tuick-tack/internal/wt"
)

// LaunchDaemon starts the background daemon as a hidden copy of the current
// executable. CREATE_NO_WINDOW keeps its console out of sight.
func LaunchDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return winproc.StartHidden(exe, "--daemon", "--hidden")
}

// openTUI opens the TUI through Windows Terminal with the user's default
// profile when possible and falls back to a normal console window otherwise. A
// broken or absent Windows Terminal must never leave the user without a TUI.
// Both paths pass the launch marker so the child does not try the Windows
// Terminal handoff again.
func openTUI(exe string) error {
	return launchTUI(exe, wt.LaunchDefault, winproc.StartConsole)
}

// RelaunchTUI reopens a directly launched TUI in a Windows Terminal window
// that uses the user's default profile. A shortcut, Explorer or the installer
// starts tuick-tack.exe in a console window that Windows Terminal hosts with
// the "Defaults" profile, so it loses the user's acrylic. Relaunching through
// wt gives the TUI the same appearance as a normal terminal without writing
// any terminal setting.
//
// handled is true when the caller should exit: an existing TUI was focused, or
// the new window was started. When handled is false the caller should run
// normally in the current window. err is only meaningful when handled is true:
// it reports a focus failure.
func RelaunchTUI(pipe string) (handled bool, err error) {
	return relaunchTUI(relaunchEnv{
		running:     TUIRunning(pipe),
		ownsConsole: termwin.OwnsConsole(),
		focus:       FocusTUI,
		executable:  SelfExecutable,
		launch:      wt.LaunchDefault,
	})
}
