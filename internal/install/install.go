// Package install describes how the running executable was installed. The
// Windows Inno Setup installer writes the standard per-user uninstall entry; a
// portable copy has no such entry. On macOS a copy inside Tuick-Tack.app under
// /Applications or ~/Applications is installed; anything else is portable. The
// TUI uses this to pick the primary update action.
package install

import (
	"os"
	"path/filepath"
	"strings"
)

// DisplayName is the product name shown to the user.
const DisplayName = "Tuick-Tack"

// Info describes the installed copy known to the platform.
type Info struct {
	// Installed is true when the running executable is the registered copy.
	Installed bool
	// Dir is the directory registered by the installer, or the .app bundle on
	// macOS.
	Dir string
	// Exe is the executable inside Dir.
	Exe string
	// Version is the registered version, when present.
	Version string
}

// Current inspects the running executable. A missing or unreadable install
// entry means the copy is portable; the error is returned for logging but the
// caller can use Installed as the answer.
func Current() (Info, error) {
	exe, err := os.Executable()
	if err != nil {
		return Info{}, err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return Info{}, err
	}
	info, err := Find()
	if err != nil {
		return Info{Dir: filepath.Dir(exe), Exe: exe}, err
	}
	if info.Exe == "" {
		info.Exe = filepath.Join(info.Dir, ExecutableName)
	}
	info.Installed = samePath(info.Exe, exe)
	return info, nil
}

func samePath(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB))
}
