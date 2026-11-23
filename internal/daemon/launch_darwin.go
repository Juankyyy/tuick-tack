package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"tuick-tack/internal/winproc"
)

// LaunchDaemon starts the background daemon in its own session, detached from
// the TUI's terminal.
func LaunchDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return winproc.StartHidden(exe, "--daemon", "--hidden")
}

// openTUI writes a small .command script and asks LaunchServices to open it.
// Terminal then runs the real executable in a new window.
func openTUI(exe string) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "tuick-tack")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	script := filepath.Join(dir, tuiCommandName)
	if err := os.WriteFile(script, []byte(tuiCommandScript(exe)), 0o700); err != nil {
		return err
	}
	// WriteFile keeps the mode of an existing file; make sure a stale copy that
	// lost its execute bit is usable.
	if err := os.Chmod(script, 0o700); err != nil {
		return err
	}
	if err := exec.Command("/usr/bin/open", script).Run(); err != nil {
		return fmt.Errorf("open TUI: %w", err)
	}
	return nil
}

// RelaunchTUI does nothing on macOS: there is no terminal handoff to perform,
// and the TUI keeps the window it was started in.
func RelaunchTUI(string) (handled bool, err error) {
	return false, nil
}
