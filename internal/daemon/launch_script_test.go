package daemon

import (
	"strings"
	"testing"
)

func TestTUICommandScript(t *testing.T) {
	tests := []struct {
		name string
		exe  string
	}{
		{"plain", "/Applications/Tuick-Tack.app/Contents/MacOS/tuick-tack"},
		{"spaces", "/Users/A B/Applications/Tuick-Tack.app/Contents/MacOS/tuick-tack"},
		{"quote", "/Users/O'Brien/Applications/Tuick-Tack.app/Contents/MacOS/tuick-tack"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := tuiCommandScript(tt.exe)
			if !strings.HasPrefix(script, "#!/bin/sh\n") {
				t.Fatalf("script has no shebang: %q", script)
			}
			if !strings.Contains(script, shellQuote(tt.exe)) {
				t.Fatalf("script does not quote the executable: %q", script)
			}
			if !strings.HasSuffix(script, " --launch-handled\n") {
				t.Fatalf("script has no launch marker: %q", script)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"plain":   `'plain'`,
		"a b":     `'a b'`,
		"O'Brien": `'O'\''Brien'`,
		"$HOME":   `'$HOME'`,
	}
	for in, want := range tests {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
