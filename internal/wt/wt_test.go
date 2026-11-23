//go:build windows

package wt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostFromDelegation(t *testing.T) {
	tests := []struct {
		name     string
		terminal string
		console  string
		want     Host
	}{
		{"stable pair", terminalStableCLSID, consoleStableCLSID, HostWindowsTerminal},
		{"stable terminal only", terminalStableCLSID, "", HostWindowsTerminal},
		{"stable console only", "", consoleStableCLSID, HostWindowsTerminal},
		{"preview pair", terminalPreviewCLSID, consolePreviewCLSID, HostWindowsTerminalPreview},
		{"preview terminal only", terminalPreviewCLSID, "", HostWindowsTerminalPreview},
		{"console host", consoleHostCLSID, consoleHostCLSID, HostConsole},
		{"automatic", "{00000000-0000-0000-0000-000000000000}", "{00000000-0000-0000-0000-000000000000}", HostUnknown},
		{"missing", "", "", HostUnknown},
		{"case and spaces", " " + strings.ToLower(terminalStableCLSID) + " ", "", HostWindowsTerminal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostFromDelegation(tt.terminal, tt.console); got != tt.want {
				t.Fatalf("hostFromDelegation(%q, %q) = %v, want %v", tt.terminal, tt.console, got, tt.want)
			}
		})
	}
}

func TestSettingsPathsAt(t *testing.T) {
	local := filepath.Join("C:", "Users", "juanky", "AppData", "Local")
	stable := settingsPathsAt(local, HostWindowsTerminal)
	if len(stable) != 2 {
		t.Fatalf("stable paths = %v", stable)
	}
	if want := filepath.Join(local, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"); stable[0] != want {
		t.Fatalf("stable package path = %q, want %q", stable[0], want)
	}
	if want := filepath.Join(local, "Microsoft", "Windows Terminal", "settings.json"); stable[1] != want {
		t.Fatalf("stable unpackaged path = %q, want %q", stable[1], want)
	}

	preview := settingsPathsAt(local, HostWindowsTerminalPreview)
	if len(preview) != 2 {
		t.Fatalf("preview paths = %v", preview)
	}
	if want := filepath.Join(local, "Packages", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", "settings.json"); preview[0] != want {
		t.Fatalf("preview package path = %q, want %q", preview[0], want)
	}
	if want := filepath.Join(local, "Microsoft", "Windows Terminal Preview", "settings.json"); preview[1] != want {
		t.Fatalf("preview unpackaged path = %q, want %q", preview[1], want)
	}

	if paths := settingsPathsAt(local, HostConsole); paths != nil {
		t.Fatalf("console paths = %v, want nil", paths)
	}
	if paths := settingsPathsAt(local, HostUnknown); paths != nil {
		t.Fatalf("unknown paths = %v, want nil", paths)
	}
}

func TestParseDefaultProfile(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
		ok   bool
	}{
		{
			name: "plain",
			data: `{"defaultProfile":"{574e775e-4f2a-5b96-ac1e-a2962a402336}"}`,
			want: "{574e775e-4f2a-5b96-ac1e-a2962a402336}",
			ok:   true,
		},
		{
			name: "name",
			data: `{"defaultProfile":"PowerShell"}`,
			want: "PowerShell",
			ok:   true,
		},
		{
			name: "comments and trailing commas",
			data: `{
				// The profile opened by a new terminal.
				"defaultProfile": "PowerShell", /* keep it */
				"profiles": {"list": [],},
			}`,
			want: "PowerShell",
			ok:   true,
		},
		{
			name: "comment markers inside strings",
			data: `{"defaultProfile":"http://example/*x*/","url":"https://example.test//path"}`,
			want: "http://example/*x*/",
			ok:   true,
		},
		{
			name: "escaped quotes",
			data: `{"defaultProfile":"a\"b"}`,
			want: `a"b`,
			ok:   true,
		},
		{
			name: "bom",
			data: "\xEF\xBB\xBF" + `{"defaultProfile":"PowerShell"}`,
			want: "PowerShell",
			ok:   true,
		},
		{
			name: "missing",
			data: `{"themes":[]}`,
			ok:   false,
		},
		{
			name: "empty",
			data: `{"defaultProfile":"  "}`,
			ok:   false,
		},
		{
			name: "malformed",
			data: `{"defaultProfile":`,
			ok:   false,
		},
		{
			name: "unterminated comment",
			data: `{"defaultProfile":"PowerShell" /* no end`,
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDefaultProfile([]byte(tt.data))
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (profile %q)", ok, tt.ok, got)
			}
			if got != tt.want {
				t.Fatalf("profile = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProfileFromPaths(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(bad, []byte(`{"defaultProfile":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte(`{"defaultProfile":"PowerShell"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	profile, ok := profileFromPaths([]string{filepath.Join(dir, "missing.json"), bad, good})
	if !ok || profile != "PowerShell" {
		t.Fatalf("profileFromPaths = %q, %v", profile, ok)
	}

	if _, ok := profileFromPaths([]string{bad, filepath.Join(dir, "missing.json")}); ok {
		t.Fatal("a malformed settings file should not produce a profile")
	}
	if _, ok := profileFromPaths(nil); ok {
		t.Fatal("no paths should not produce a profile")
	}
}

func TestLaunchArgs(t *testing.T) {
	got := launchArgs("PowerShell", `C:\Program Files\tuick-tack\tuick-tack.exe`, []string{"--extra"})
	want := []string{
		"-w", "-1", "new-tab", "--useApplicationTitle", "-p", "PowerShell",
		`C:\Program Files\tuick-tack\tuick-tack.exe`, "--extra",
	}
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}
