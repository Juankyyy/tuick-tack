//go:build windows

package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.exe")
	dst := filepath.Join(dir, "dst.exe")
	content := []byte("helper executable")
	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("copy = %q", got)
	}
}

func TestCopyFileMissingSource(t *testing.T) {
	dir := t.TempDir()
	if err := copyFile(filepath.Join(dir, "missing"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("expected an error for a missing source")
	}
}

func TestSetupArgs(t *testing.T) {
	tests := []struct {
		name    string
		logPath string
	}{
		{"with log", `C:\Users\A\AppData\Local\tuick-tack\logs\setup.log`},
		{"space in profile", `C:\Users\A B\AppData\Local\tuick-tack\logs\setup.log`},
		{"without log", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := setupArgs(tt.logPath)
			for _, arg := range args {
				if strings.Contains(arg, `"`) {
					t.Fatalf("argument %q contains literal quotes", arg)
				}
			}
			for _, want := range []string{"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/CLOSEAPPLICATIONS"} {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %s in %q", want, args)
				}
			}
			wantLog := tt.logPath != ""
			foundLog := false
			for _, arg := range args {
				if arg == "/LOG="+tt.logPath {
					foundLog = true
				}
			}
			if foundLog != wantLog {
				t.Fatalf("log argument = %v, want %v in %q", foundLog, wantLog, args)
			}
		})
	}
}
