//go:build darwin

package instance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// Lock is an acquired advisory file lock. Release closes the descriptor, which
// frees the lock for the next process.
type Lock struct {
	file *os.File
}

// Release closes the lock file. It is safe on a nil lock and safe to call more
// than once.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}

// DaemonMutexName derives the daemon lock file from the socket path. Both live
// in the per-user cache directory, so one exists for each daemon identity.
func DaemonMutexName(pipe string) string {
	return lockPath(pipe, "daemon.lock")
}

// TUIMutexName derives the TUI lock file from the socket path.
func TUIMutexName(pipe string) string {
	return lockPath(pipe, "tui.lock")
}

func lockPath(pipe, name string) string {
	return filepath.Join(filepath.Dir(pipe), name)
}

// Acquire takes an advisory lock on name, creating the file when needed.
// already is true when another process holds it; the returned lock must stay
// open for the owner's lifetime.
func Acquire(name string) (lock *Lock, already bool, err error) {
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := flock(f, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("instance: lock %s: %w", name, err)
	}
	return &Lock{file: f}, false, nil
}

// AcquireTUI takes the TUI singleton lock. already is true when another TUI
// owns it; release must be called when the TUI exits.
func AcquireTUI(pipe string) (release func(), already bool, err error) {
	lock, already, err := Acquire(TUIMutexName(pipe))
	if err != nil || already {
		return func() {}, already, err
	}
	return func() { _ = lock.Release() }, false, nil
}

// TUIRunning reports whether a TUI holds the singleton lock. It never creates
// the lock file, so a missing file means no TUI is running.
func TUIRunning(pipe string) bool {
	f, err := os.OpenFile(TUIMutexName(pipe), os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	if err := flock(f, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return errors.Is(err, unix.EWOULDBLOCK)
	}
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return false
}

// WaitDaemonFree waits until the daemon lock can be acquired and holds it for
// the caller. A new daemon cannot start while the returned release is pending.
func WaitDaemonFree(pipe string, timeout time.Duration) (release func(), err error) {
	return waitFree(DaemonMutexName(pipe), timeout)
}

// WaitTUIFree waits until the TUI lock can be acquired and holds it for the
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

// flock retries on EINTR, which can interrupt a blocking flock.
func flock(f *os.File, how int) error {
	for {
		err := unix.Flock(int(f.Fd()), how)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
