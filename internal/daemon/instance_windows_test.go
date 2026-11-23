//go:build windows

package daemon

import (
	"path/filepath"
	"testing"
)

func TestInstanceMutexName(t *testing.T) {
	got := InstanceMutexName(`\\.\pipe\tuick-tack-juanky-1`)
	if got != "tuick-tack-juanky-1-mutex" {
		t.Fatalf("mutex name = %q", got)
	}
}

func TestTUIInstanceMutexName(t *testing.T) {
	got := TUIInstanceMutexName(`\\.\pipe\tuick-tack-juanky-1`)
	if got != "tuick-tack-juanky-1-tui-mutex" {
		t.Fatalf("TUI mutex name = %q", got)
	}
}

func TestTUIRunning(t *testing.T) {
	pipe := `\\.\pipe\tuick-tack-test-` + filepath.Base(t.TempDir())
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
	name := "tuick-tack-test-mutex-" + filepath.Base(t.TempDir())
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
