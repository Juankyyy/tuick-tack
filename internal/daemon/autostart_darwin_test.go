//go:build darwin

package daemon

import (
	"strings"
	"testing"
)

func TestLaunchAgentPlist(t *testing.T) {
	exe := "/Users/A B/Applications/Tuick-Tack.app/Contents/MacOS/tuick-tack"
	data := launchAgentPlist(exe)
	for _, want := range []string{
		"<string>" + launchAgentLabel + "</string>",
		"<string>" + exe + "</string>",
		"<string>--daemon</string>",
		"<string>--hidden</string>",
		"<key>RunAtLoad</key>\n\t<true/>",
		"<key>KeepAlive</key>\n\t<false/>",
	} {
		if !strings.Contains(data, want) {
			t.Errorf("plist is missing %q:\n%s", want, data)
		}
	}
	if !launchAgentTargets([]byte(data), exe) {
		t.Fatal("the plist should target the executable")
	}
	if launchAgentTargets([]byte(data), "/Applications/Other.app/Contents/MacOS/tuick-tack") {
		t.Fatal("a different executable must not match")
	}
}

func TestLaunchAgentPlistEscapesPath(t *testing.T) {
	exe := "/Users/A&B/Applications/Tuick-Tack.app/Contents/MacOS/tuick-tack"
	data := launchAgentPlist(exe)
	if strings.Contains(data, exe) {
		t.Fatalf("the path must be XML escaped:\n%s", data)
	}
	if !launchAgentTargets([]byte(data), exe) {
		t.Fatal("the escaped path should still match")
	}
}
