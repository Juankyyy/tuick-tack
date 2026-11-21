package daemon

import (
	"errors"
	"strings"
	"testing"
)

func TestLaunchTUIUsesWindowsTerminal(t *testing.T) {
	var fallback bool
	err := launchTUI(
		`C:\app\tuick-tack.exe`,
		func(exe string, extra ...string) (bool, error) {
			if exe != `C:\app\tuick-tack.exe` {
				t.Fatalf("exe = %q", exe)
			}
			if len(extra) != 1 || extra[0] != tuiLaunchMarker {
				t.Fatalf("marker = %v, want %q", extra, tuiLaunchMarker)
			}
			return true, nil
		},
		func(string, ...string) error {
			fallback = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("launchTUI: %v", err)
	}
	if fallback {
		t.Fatal("the console fallback must not run when Windows Terminal starts")
	}
}

func TestLaunchTUIFallsBackToConsole(t *testing.T) {
	var launched, called bool
	err := launchTUI(
		`C:\app\tuick-tack.exe`,
		func(string, ...string) (bool, error) { launched = true; return false, nil },
		func(exe string, extra ...string) error {
			called = true
			if exe != `C:\app\tuick-tack.exe` || len(extra) != 1 || extra[0] != tuiLaunchMarker {
				t.Fatalf("fallback = %q %v", exe, extra)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("launchTUI: %v", err)
	}
	if !launched || !called {
		t.Fatalf("launched=%v called=%v, want both true", launched, called)
	}
}

func TestLaunchTUIFallsBackOnWindowsTerminalError(t *testing.T) {
	var called bool
	err := launchTUI(
		`C:\app\tuick-tack.exe`,
		func(string, ...string) (bool, error) { return false, errors.New("wt missing") },
		func(string, ...string) error { called = true; return nil },
	)
	if err != nil {
		t.Fatalf("a Windows Terminal error must not block the console fallback: %v", err)
	}
	if !called {
		t.Fatal("the console fallback did not run")
	}
}

func TestLaunchTUIReturnsFallbackError(t *testing.T) {
	err := launchTUI(
		`C:\app\tuick-tack.exe`,
		func(string, ...string) (bool, error) { return false, nil },
		func(string, ...string) error { return errors.New("console failed") },
	)
	if err == nil || !strings.Contains(err.Error(), "start TUI") {
		t.Fatalf("err = %v, want a start TUI error", err)
	}
}

func TestRelaunchTUIFocusesRunningTUI(t *testing.T) {
	focused := false
	handled, err := relaunchTUI(relaunchEnv{
		running:     true,
		focus:       func() bool { focused = true; return true },
		ownsConsole: true,
		executable:  func() (string, error) { t.Fatal("must not launch a second TUI"); return "", nil },
		launch:      func(string, ...string) (bool, error) { t.Fatal("must not launch a second TUI"); return false, nil },
	})
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v, want handled", handled, err)
	}
	if !focused {
		t.Fatal("the running TUI was not focused")
	}
}

func TestRelaunchTUIFocusFailure(t *testing.T) {
	handled, err := relaunchTUI(relaunchEnv{
		running: true,
		focus:   func() bool { return false },
	})
	if !handled || err == nil {
		t.Fatalf("handled=%v err=%v, want handled with an error", handled, err)
	}
}

func TestRelaunchTUISharedConsoleDoesNothing(t *testing.T) {
	handled, err := relaunchTUI(relaunchEnv{
		ownsConsole: false,
		focus:       func() bool { t.Fatal("must not focus"); return false },
		executable:  func() (string, error) { t.Fatal("must not read the executable"); return "", nil },
		launch:      func(string, ...string) (bool, error) { t.Fatal("must not launch"); return false, nil },
	})
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want no action", handled, err)
	}
}

func TestRelaunchTUIOpensDefaultProfileWindow(t *testing.T) {
	marked := false
	handled, err := relaunchTUI(relaunchEnv{
		ownsConsole: true,
		focus:       func() bool { t.Fatal("must not focus"); return false },
		executable:  func() (string, error) { return `C:\app\tuick-tack.exe`, nil },
		launch: func(exe string, extra ...string) (bool, error) {
			marked = true
			if exe != `C:\app\tuick-tack.exe` {
				t.Fatalf("exe = %q", exe)
			}
			if len(extra) != 1 || extra[0] != tuiLaunchMarker {
				t.Fatalf("marker = %v, want %q", extra, tuiLaunchMarker)
			}
			return true, nil
		},
	})
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v, want handled", handled, err)
	}
	if !marked {
		t.Fatal("the relaunch did not pass the launch marker")
	}
}

func TestRelaunchTUIFallsBackWhenWindowsTerminalDoesNotStart(t *testing.T) {
	handled, err := relaunchTUI(relaunchEnv{
		ownsConsole: true,
		focus:       func() bool { return false },
		executable:  func() (string, error) { return `C:\app\tuick-tack.exe`, nil },
		launch:      func(string, ...string) (bool, error) { return false, nil },
	})
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want the current console path", handled, err)
	}
}

func TestRelaunchTUIFallsBackOnWindowsTerminalError(t *testing.T) {
	handled, err := relaunchTUI(relaunchEnv{
		ownsConsole: true,
		focus:       func() bool { return false },
		executable:  func() (string, error) { return `C:\app\tuick-tack.exe`, nil },
		launch:      func(string, ...string) (bool, error) { return false, errors.New("wt failed") },
	})
	if handled {
		t.Fatal("a Windows Terminal error must keep the current window")
	}
	if err == nil {
		t.Fatal("the launch error should be reported to the caller")
	}
}

func TestRelaunchTUIExecutableError(t *testing.T) {
	handled, err := relaunchTUI(relaunchEnv{
		ownsConsole: true,
		focus:       func() bool { return false },
		executable:  func() (string, error) { return "", errors.New("no exe") },
		launch:      func(string, ...string) (bool, error) { t.Fatal("must not launch"); return false, nil },
	})
	if handled || err == nil {
		t.Fatalf("handled=%v err=%v, want no action with an error", handled, err)
	}
}

func TestRelaunchTUISkipsDevelopmentBuild(t *testing.T) {
	handled, err := relaunchTUI(relaunchEnv{
		ownsConsole: true,
		focus:       func() bool { return false },
		executable: func() (string, error) {
			return `C:\Users\juanky\AppData\Local\Temp\go-build123\b001\exe\tuick-tack.exe`, nil
		},
		launch: func(string, ...string) (bool, error) {
			t.Fatal("must not relaunch a go run build")
			return false, nil
		},
	})
	if handled || err != nil {
		t.Fatalf("handled=%v err=%v, want the current console path", handled, err)
	}
}
