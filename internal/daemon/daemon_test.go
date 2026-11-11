package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSameExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "tuick-tack-tray.exe")
	if !sameExecutable(`"`+exe+`"`, exe) {
		t.Fatal("quoted path should match")
	}
	if !sameExecutable(exe, exe) {
		t.Fatal("plain path should match")
	}
	if sameExecutable(`"`+filepath.Join(dir, "other.exe")+`"`, exe) {
		t.Fatal("different path should not match")
	}
	if sameExecutable("", exe) {
		t.Fatal("empty value should not match")
	}
}

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
	defer windows.CloseHandle(first)

	second, already, err := AcquireInstance(name)
	if err != nil {
		t.Fatalf("second AcquireInstance: %v", err)
	}
	if !already {
		t.Fatal("second acquire should report already running")
	}
	if second != 0 {
		t.Fatalf("second handle = %v, want 0", second)
	}
}

func TestNewSoundPlayerExtractsChime(t *testing.T) {
	dir := t.TempDir()
	p, err := newSoundPlayer(dir)
	if err != nil {
		t.Fatalf("newSoundPlayer: %v", err)
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		t.Fatalf("read chime: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("chime is suspiciously small: %d", len(data))
	}
	if _, err := newSoundPlayer(dir); err != nil {
		t.Fatalf("second newSoundPlayer: %v", err)
	}
}

func TestLaunchHelpersDoNotPanic(t *testing.T) {
	if got := InstanceMutexName(`\\.\pipe\x`); got != "x-mutex" {
		t.Fatalf("mutex name = %q", got)
	}
	if fileExists(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing file reported as existing")
	}
}
