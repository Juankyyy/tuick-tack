//go:build darwin

package copy

import "testing"

func TestMacMenuBarCopy(t *testing.T) {
	tests := map[Key]string{
		TrayStartup:      "Start at login",
		StartupInstalled: "Tuick-Tack will start at login.",
		StartupRemoved:   "Tuick-Tack will no longer start at login.",
		ExitActive:       "The countdown is still running in the menu bar.",
		HelpTrayNote:     "the menu bar keeps running",
		NotifyFoundBody:  "Version %s is available.",
	}
	for key, want := range tests {
		if got := T(key); got != want {
			t.Errorf("T(%q) = %q, want %q", key, got, want)
		}
	}
}
