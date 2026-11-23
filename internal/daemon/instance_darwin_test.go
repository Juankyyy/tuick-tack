//go:build darwin

package daemon

import (
	"path/filepath"
	"testing"
)

func TestInstanceLockNames(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "daemon-501.sock")
	if got, want := InstanceMutexName(pipe), filepath.Join(dir, "daemon.lock"); got != want {
		t.Fatalf("daemon lock = %q, want %q", got, want)
	}
	if got, want := TUIInstanceMutexName(pipe), filepath.Join(dir, "tui.lock"); got != want {
		t.Fatalf("TUI lock = %q, want %q", got, want)
	}
}

func TestTUIRunning(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "daemon.sock")
	if TUIRunning(pipe) {
		t.Fatal("reported running before acquire")
	}

	release, already, err := AcquireTUIInstance(pipe)
	if err != nil {
		t.Fatalf("AcquireTUIInstance: %v", err)
	}
	if already {
		t.Fatal("first acquire reported already running")
	}
	if !TUIRunning(pipe) {
		t.Fatal("reported not running after acquire")
	}

	second, already, err := AcquireTUIInstance(pipe)
	if err != nil {
		t.Fatalf("second AcquireTUIInstance: %v", err)
	}
	if !already {
		t.Fatal("second acquire should report already running")
	}
	second()

	release()
	if TUIRunning(pipe) {
		t.Fatal("reported running after release")
	}
}

func TestAcquireInstance(t *testing.T) {
	name := filepath.Join(t.TempDir(), "daemon.lock")
	first, already, err := AcquireInstance(name)
	if err != nil {
		t.Fatalf("first AcquireInstance: %v", err)
	}
	if already {
		t.Fatal("first acquire reported already running")
	}
	defer func() { _ = first.Release() }()

	second, already, err := AcquireInstance(name)
	if err != nil {
		t.Fatalf("second AcquireInstance: %v", err)
	}
	if !already {
		t.Fatal("second acquire should report already running")
	}
	if second != nil {
		t.Fatalf("second lock = %v, want nil", second)
	}
}
