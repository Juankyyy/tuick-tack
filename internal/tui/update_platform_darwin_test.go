//go:build darwin

package tui

import "testing"

func TestMacUpdatePlatform(t *testing.T) {
	if !platformMacOS {
		t.Fatal("the darwin build must use the single-update dialog")
	}
	if m := New(""); !m.singleUpdate {
		t.Fatal("New should enable the single-update dialog on macOS")
	}
	path := "/Users/a/Downloads/tuick-tack-v9.9.9-macos-universal.zip"
	if got := versionFromPath(path); got != "v9.9.9" {
		t.Fatalf("versionFromPath = %q, want v9.9.9", got)
	}
}
