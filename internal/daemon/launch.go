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
	"tuick-tack/internal/termwin"
	"tuick-tack/internal/winproc"
	"tuick-tack/internal/wt"
)

// TUILaunchFlag is the internal flag a TUI receives when the launcher (tray,
// direct relaunch or console fallback) has already handled the window choice.
// It stops a second relaunch attempt.
const TUILaunchFlag = "launch-handled"

// tuiLaunchMarker is the command-line form of TUILaunchFlag.
const tuiLaunchMarker = "--" + TUILaunchFlag

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

// OpenTUI launches the terminal UI from the same executable in a new window or
// focuses the running one. It is used by the tray's left click and Custom…
// item. When Windows Terminal is the default terminal, the new window uses the
// user's default profile, so the transparent "none" background keeps their
// acrylic, opacity or background image. The app only reads that profile.
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
	return launchTUI(exe, wt.LaunchDefault, winproc.StartConsole)
}

// launchTUI opens the TUI through Windows Terminal with the user's default
// profile when possible and falls back to a normal console window otherwise. A
// broken or absent Windows Terminal must never leave the user without a TUI.
// Both paths pass the launch marker so the child does not try the Windows
// Terminal handoff again.
func launchTUI(exe string, launch func(string, ...string) (bool, error), fallback func(string, ...string) error) error {
	if ok, _ := launch(exe, tuiLaunchMarker); ok {
		return nil
	}
	if err := fallback(exe, tuiLaunchMarker); err != nil {
		return fmt.Errorf("start TUI: %w", err)
	}
	return nil
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

type relaunchEnv struct {
	running     bool
	ownsConsole bool
	focus       func() bool
	executable  func() (string, error)
	launch      func(string, ...string) (bool, error)
}

func relaunchTUI(env relaunchEnv) (bool, error) {
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
