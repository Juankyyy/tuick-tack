package winproc

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// StartHidden starts path in its own session, with stdio on /dev/null, and
// releases the process. Setsid detaches the daemon from the TUI's terminal, so
// closing the TUI does not send it SIGHUP.
func StartHidden(path string, args ...string) error {
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()

	cmd := exec.Command(path, args...)
	cmd.Stdin = null
	cmd.Stdout = null
	cmd.Stderr = null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	return cmd.Process.Release()
}

// StartConsole has no macOS equivalent: the TUI is opened through the .command
// launcher in the daemon package, not by starting a console process.
func StartConsole(path string, _ ...string) error {
	return fmt.Errorf("start %s: no console launcher on macOS", path)
}
