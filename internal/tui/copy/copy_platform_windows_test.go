//go:build windows

package copy

import "testing"

func TestWindowsNotifyCopy(t *testing.T) {
	if got, want := T(NotifyFoundBody, "v0.2.0"),
		"Version v0.2.0 is available. Click to open the release page."; got != want {
		t.Fatalf("NotifyFoundBody = %q, want %q", got, want)
	}
}
