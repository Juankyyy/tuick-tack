package daemon

import (
	"errors"
	"fmt"
	"time"

	"tuick-tack/internal/instance"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
)

// TUILaunchFlag is the internal flag a TUI receives when the launcher (tray,
// direct relaunch or console fallback) has already handled the window choice.
// It stops a second relaunch attempt.
const TUILaunchFlag = "launch-handled"

// tuiLaunchMarker is the command-line form of TUILaunchFlag.
const tuiLaunchMarker = "--" + TUILaunchFlag

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

// OpenTUI launches the terminal UI from the same executable in a new window or
// focuses the running one. It is used by the tray's left click and Custom…
// item. Each platform decides how to open a terminal; Windows prefers the
// user's Windows Terminal default profile, macOS writes and opens a .command
// script so AppleScript and its automation permission are not needed.
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
	return openTUI(exe)
}

// launchTUI opens the TUI through a preferred launcher and falls back to the
// normal console window if the preferred path is unavailable. A broken or
// absent terminal must never leave the user without a TUI. Both paths pass the
// launch marker so the child does not try the handoff again.
func launchTUI(exe string, launch func(string, ...string) (bool, error), fallback func(string, ...string) error) error {
	if ok, _ := launch(exe, tuiLaunchMarker); ok {
		return nil
	}
	if err := fallback(exe, tuiLaunchMarker); err != nil {
		return fmt.Errorf("start TUI: %w", err)
	}
	return nil
}

// relaunchEnv is the platform-independent part of RelaunchTUI. The Windows
// launcher fills it with the console and Windows Terminal checks; macOS returns
// handled=false directly and never calls it.
type relaunchEnv struct {
	running     bool
	ownsConsole bool
	focus       func() bool
	executable  func() (string, error)
	launch      func(string, ...string) (bool, error)
}

// relaunchTUI decides whether the current process should hand the TUI over to
// a new window instead of running in the current console. handled is true when
// the caller should exit: an existing TUI was focused, or the new window was
// started. When handled is false the caller should run normally in the current
// window. err is only meaningful when handled is true: it reports a focus
// failure.
func relaunchTUI(env relaunchEnv) (handle bool, err error) {
	if env.running {
		if !env.focus() {
			return true, errors.New("TUI is already open but its window was not found")
		}
		return true, nil
	}
	if !env.ownsConsole {
		// A shell tab shares its console. The tab already has the user's
		// profile and the TUI must not move or replace the whole window.
		return false, nil
	}
	exe, err := env.executable()
	if err != nil {
		return false, err
	}
	if isDevelopmentPath(exe) {
		// go run's temporary binary may disappear when this process exits;
		// keep the current console instead of relaunching it.
		return false, nil
	}
	started, err := env.launch(exe, tuiLaunchMarker)
	if err != nil || !started {
		// The current console window is still open and usable; fall back to
		// it rather than leaving the user without a TUI.
		return false, err
	}
	return true, nil
}

// InstanceMutexName derives a per-session lock name from the pipe path.
func InstanceMutexName(pipe string) string {
	return instance.DaemonMutexName(pipe)
}

// AcquireInstance creates the lock used to enforce a single daemon.
// alreadyRunning is true when another process owns it; the returned lock must
// stay open for the daemon's lifetime.
func AcquireInstance(name string) (lock *instance.Lock, alreadyRunning bool, err error) {
	return instance.Acquire(name)
}
