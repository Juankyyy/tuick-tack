package install

import (
	"path/filepath"
	"testing"
)

func TestIsAppDisplayName(t *testing.T) {
	tests := map[string]bool{
		"Tuick-Tack":               true,
		"Tuick-Tack version 0.3.0": true,
		"Tuick-Tack Helper":        false,
		"Other Tuick-Tack":         false,
		"":                         false,
		" Tuick-Tack ":             true,
		"Tuick-Tack version":       false,
	}
	for name, want := range tests {
		if got := isAppDisplayName(name); got != want {
			t.Errorf("isAppDisplayName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestInfoFromValues(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tuick-tack")
	info := infoFromValues("Tuick-Tack", dir, "", "", "0.3.0")
	if info.Dir != dir || info.Version != "0.3.0" {
		t.Fatalf("info = %+v", info)
	}
	if info.Exe != filepath.Join(dir, ExecutableName) {
		t.Fatalf("exe = %q", info.Exe)
	}

	info = infoFromValues("Tuick-Tack", "", `"`+filepath.Join(dir, ExecutableName)+`",0`, "", "")
	if info.Dir != dir {
		t.Fatalf("DisplayIcon dir = %q, want %q", info.Dir, dir)
	}

	info = infoFromValues("Tuick-Tack", "", "", `"`+filepath.Join(dir, "unins000.exe")+`" /SILENT`, "")
	if info.Dir != dir {
		t.Fatalf("UninstallString dir = %q, want %q", info.Dir, dir)
	}
}

func TestDirFromDisplayIcon(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tuick-tack")
	exe := filepath.Join(dir, ExecutableName)
	tests := map[string]string{
		exe:                         dir,
		`"` + exe + `"`:             dir,
		`"` + exe + `",0`:           dir,
		exe + `,0`:                  dir,
		`"` + dir + `\other.exe",0`: "",
		"":                          "",
	}
	for in, want := range tests {
		if got := dirFromDisplayIcon(in); got != want {
			t.Errorf("dirFromDisplayIcon(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDirFromUninstallString(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tuick-tack")
	uninstaller := filepath.Join(dir, "unins000.exe")
	tests := map[string]string{
		uninstaller:                     dir,
		`"` + uninstaller + `"`:         dir,
		`"` + uninstaller + `" /SILENT`: dir,
		"":                              "",
	}
	for in, want := range tests {
		if got := dirFromUninstallString(in); got != want {
			t.Errorf("dirFromUninstallString(%q) = %q, want %q", in, got, want)
		}
	}
}
