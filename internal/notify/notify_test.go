package notify

import (
	"testing"

	"github.com/mtunjic/winrt-toast"
)

func TestNotification(t *testing.T) {
	n := notification("Update available", "Version v0.2.0 is available.", "https://example.test/release")
	if AppID != "Juankyyy.TuickTack" {
		t.Fatalf("AppID = %q", AppID)
	}
	if n.AppID != AppID {
		t.Fatalf("toast AppID = %q", n.AppID)
	}
	if n.Title != "Update available" || n.Message != "Version v0.2.0 is available." {
		t.Fatalf("unexpected toast: %+v", n)
	}
	if n.ActivationArguments != "https://example.test/release" {
		t.Fatalf("launch = %q", n.ActivationArguments)
	}
	if n.Duration != toast.DurationShort {
		t.Fatalf("duration = %q", n.Duration)
	}
	if err := n.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
