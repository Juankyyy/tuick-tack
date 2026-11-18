package daemon

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"

	"tuick-tack/internal/instance"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/winproc"
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

// Ensure connects to the daemon, launching it when necessary and retrying for
// up to three seconds. The returned client has completed the handshake.
func Ensure(pipe string) (*ipc.Client, protocol.HelloMessage, error) {
	if c, hello, err := ipc.DialWithin(pipe, 200*time.Millisecond); err == nil {
		return c, hello, nil
	}
	if err := LaunchDaemon(); err != nil {
		return nil, protocol.HelloMessage{}, err
	}
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		c, hello, err := ipc.DialWithin(pipe, 250*time.Millisecond)
		if err == nil {
			return c, hello, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("daemon did not start")
	}
	return nil, protocol.HelloMessage{}, lastErr
}

// OpenTUI launches the terminal UI from the same executable in a new console
// window or focuses the running one. It is used by the tray's left click and
// Custom… item.
func OpenTUI() error {
	pipe, err := ipc.PipePath()
	if err != nil {
		return err
	}
	if instance.TUIRunning(pipe) {
		if !FocusTUI() {
			return errors.New("TUI is already open but its window was not found")
		}
		return nil
	}
	exe, err := SelfExecutable()
	if err != nil {
		return err
	}
	if err := winproc.StartConsole(exe); err != nil {
		return fmt.Errorf("start TUI: %w", err)
	}
	return nil
}

// InstanceMutexName derives a per-session mutex name from the pipe path.
func InstanceMutexName(pipe string) string {
	return instance.DaemonMutexName(pipe)
}

// AcquireInstance creates the named mutex used to enforce a single daemon.
// alreadyRunning is true when another process owns it; the returned handle must
// stay open for the daemon's lifetime.
func AcquireInstance(name string) (handle windows.Handle, alreadyRunning bool, err error) {
	return instance.Acquire(name)
}
