//go:build windows

package daemon

import (
	"path/filepath"
	"testing"
)

func TestAutostartCommand(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "tuick-tack.exe")
	want := `"` + exe + `" --daemon --hidden`
	if got := AutostartCommand(exe); got != want {
		t.Fatalf("AutostartCommand = %q, want %q", got, want)
	}
}

func TestAutostartPath(t *testing.T) {
	exe := `C:\Program Files\Tuick-Tack\tuick-tack.exe`
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"quoted with args", `"` + exe + `" --daemon --hidden`, exe},
		{"quoted without args", `"` + exe + `"`, exe},
		{"unquoted with args", `C:\Tuick-Tack\tuick-tack.exe --daemon --hidden`, `C:\Tuick-Tack\tuick-tack.exe`},
		{"unquoted without args", `C:\Tuick-Tack\tuick-tack.exe`, `C:\Tuick-Tack\tuick-tack.exe`},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := autostartPath(tt.value); got != tt.want {
				t.Fatalf("autostartPath(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestAutostartNeedsMigration(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "tuick-tack.exe")
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"legacy tray", `"` + filepath.Join(dir, "tuick-tack-tray.exe") + `"`, true},
		{"current command", AutostartCommand(exe), false},
		{"current exe without args", `"` + exe + `"`, true},
		{"different executable", `"` + filepath.Join(dir, "other.exe") + `" --daemon --hidden`, false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := autostartNeedsMigration(tt.value, exe); got != tt.want {
				t.Fatalf("autostartNeedsMigration(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
