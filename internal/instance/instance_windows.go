//go:build windows

package instance

import (
	"errors"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const tuiMutexSuffix = "-tui-mutex"

// ObjectName strips the pipe namespace and makes the name safe for a kernel
// object.
func ObjectName(pipe string) string {
	name := strings.TrimPrefix(pipe, `\\.\pipe\`)
	return strings.ReplaceAll(name, `\`, "-")
}

// DaemonMutexName derives the daemon singleton mutex from the pipe path.
func DaemonMutexName(pipe string) string {
	return ObjectName(pipe) + "-mutex"
}

// TUIMutexName derives the TUI singleton mutex from the pipe path.
func TUIMutexName(pipe string) string {
	return ObjectName(pipe) + tuiMutexSuffix
}

// Lock is an acquired named mutex. Release closes the handle, which frees the
// name for the next process.
type Lock struct {
	handle windows.Handle
}

// Release closes the mutex handle. It is safe on a nil lock and safe to call
// more than once.
func (l *Lock) Release() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	h := l.handle
	l.handle = 0
	return windows.CloseHandle(h)
}

// Acquire creates the named mutex. already is true when another process owns
// it; the returned lock must stay open for the owner's lifetime.
func Acquire(name string) (lock *Lock, already bool, err error) {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, ptr)
	if err != nil {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return &Lock{handle: h}, false, nil
}

// AcquireTUI takes the TUI singleton mutex. already is true when another TUI
// owns it; release must be called when the TUI exits.
func AcquireTUI(pipe string) (release func(), already bool, err error) {
	lock, already, err := Acquire(TUIMutexName(pipe))
	if err != nil || already {
		return func() {}, already, err
	}
	return func() { _ = lock.Release() }, false, nil
}

// TUIRunning reports whether a TUI owns the singleton mutex. It only opens the
// existing object; it never creates one.
func TUIRunning(pipe string) bool {
	name, err := windows.UTF16PtrFromString(TUIMutexName(pipe))
	if err != nil {
		return false
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}

// WaitDaemonFree waits until the daemon mutex can be acquired and holds it for
// the caller. A new daemon cannot start while the returned release is pending.
func WaitDaemonFree(pipe string, timeout time.Duration) (release func(), err error) {
	return waitFree(DaemonMutexName(pipe), timeout)
}

// WaitTUIFree waits until the TUI mutex can be acquired and holds it for the
// caller. Release it before starting a new TUI.
func WaitTUIFree(pipe string, timeout time.Duration) (release func(), err error) {
	return waitFree(TUIMutexName(pipe), timeout)
}

func waitFree(name string, timeout time.Duration) (release func(), err error) {
	deadline := time.Now().Add(timeout)
	for {
		lock, already, err := Acquire(name)
		if err != nil {
			return func() {}, err
		}
		if !already {
			return func() { _ = lock.Release() }, nil
		}
		if time.Now().After(deadline) {
			return func() {}, errors.New("instance: timed out waiting for " + name)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
