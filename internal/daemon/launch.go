package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
)

const (
	createNewConsole = 0x00000010
	createNoWindow   = 0x08000000
)

// LaunchDaemon starts the tray daemon without a console. It prefers
// tuick-tack-tray.exe next to the current executable and falls back to the
// current executable with --daemon for `go run` development.
func LaunchDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	if candidate := filepath.Join(dir, "tuick-tack-tray.exe"); fileExists(candidate) {
		return startHidden(candidate)
	}
	if strings.EqualFold(filepath.Base(exe), "tuick-tack-tray.exe") {
		return nil
	}
	return startHidden(exe, "--daemon")
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func startHidden(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	return cmd.Process.Release()
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

// OpenTUI launches the terminal UI in a new console window. It is used by the
// tray's left click and Custom… item.
func OpenTUI() error {
	exe, err := TUIExecutable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start TUI: %w", err)
	}
	return cmd.Process.Release()
}

// InstanceMutexName derives a per-session mutex name from the pipe path.
func InstanceMutexName(pipe string) string {
	name := strings.TrimPrefix(pipe, `\\.\pipe\`)
	name = strings.ReplaceAll(name, `\`, "-")
	return name + "-mutex"
}

// AcquireInstance creates the named mutex used to enforce a single daemon.
// alreadyRunning is true when another process owns it; the returned handle must
// stay open for the daemon's lifetime.
func AcquireInstance(name string) (handle windows.Handle, alreadyRunning bool, err error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, err
	}
	h, err := windows.CreateMutex(nil, false, ptr)
	if err != nil {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		if err == windows.ERROR_ALREADY_EXISTS {
			return 0, true, nil
		}
		return 0, false, fmt.Errorf("create mutex: %w", err)
	}
	return h, false, nil
}
