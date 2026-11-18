// Package install describes how the running executable was installed. The
// Inno Setup installer writes the standard per-user uninstall entry; a
// portable copy has no such entry. The TUI uses this to pick the primary update
// action, and the updater uses it to find the executable to reopen.
package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	// DisplayName is the value the installer writes in Apps & features.
	DisplayName = "Tuick-Tack"
	// ExecutableName is the installed executable.
	ExecutableName = "tuick-tack.exe"
	// UninstallKey is the root where Inno Setup registers per-user installs.
	UninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
)

// Info describes the installed copy known to the registry.
type Info struct {
	// Installed is true when the running executable is the registered copy.
	Installed bool
	// Dir is the directory registered by the installer.
	Dir string
	// Exe is the executable inside Dir.
	Exe string
	// Version is the DisplayVersion value, when present.
	Version string
}

// Current inspects the running executable. A missing or unreadable uninstall
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

// Find scans the per-user uninstall entries for the Tuick-Tack installer.
// A missing entry is not an error: it simply means no installed copy exists.
func Find() (Info, error) {
	root, err := registry.OpenKey(registry.CURRENT_USER, UninstallKey, registry.READ)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return Info{}, nil
		}
		return Info{}, err
	}
	defer root.Close()

	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return Info{}, err
	}
	for _, name := range names {
		key, err := registry.OpenKey(root, name, registry.READ)
		if err != nil {
			continue
		}
		display, _, _ := key.GetStringValue("DisplayName")
		if !isAppDisplayName(display) {
			key.Close()
			continue
		}
		location, _, _ := key.GetStringValue("InstallLocation")
		icon, _, _ := key.GetStringValue("DisplayIcon")
		uninstall, _, _ := key.GetStringValue("UninstallString")
		version, _, _ := key.GetStringValue("DisplayVersion")
		key.Close()

		info := infoFromValues(display, location, icon, uninstall, version)
		if info.Dir == "" {
			continue
		}
		return info, nil
	}
	return Info{}, nil
}

// isAppDisplayName accepts the exact installer name and Inno's usual
// "Tuick-Tack version x.y.z" form, but not unrelated apps that share a prefix.
func isAppDisplayName(name string) bool {
	name = strings.TrimSpace(name)
	if name == DisplayName {
		return true
	}
	if !strings.HasPrefix(name, DisplayName+" ") {
		return false
	}
	rest := strings.TrimPrefix(name, DisplayName+" ")
	return strings.HasPrefix(rest, "version ")
}

// infoFromValues derives the install directory from the uninstall entry.
// InstallLocation wins; DisplayIcon and UninstallString are fallbacks for
// installers that omit it.
func infoFromValues(display, location, icon, uninstall, version string) Info {
	dir := strings.TrimSpace(location)
	if dir == "" {
		dir = dirFromDisplayIcon(icon)
	}
	if dir == "" {
		dir = dirFromUninstallString(uninstall)
	}
	info := Info{Dir: dir, Version: strings.TrimSpace(version)}
	if dir != "" {
		info.Exe = filepath.Join(dir, ExecutableName)
	}
	return info
}

func dirFromDisplayIcon(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Trim(value, `"`)
	if i := strings.LastIndex(value, ","); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	value = strings.Trim(value, `"`)
	if strings.EqualFold(filepath.Base(value), ExecutableName) {
		return filepath.Dir(value)
	}
	return ""
}

func dirFromUninstallString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if value[0] == '"' {
		if end := strings.IndexByte(value[1:], '"'); end >= 0 {
			value = value[1 : end+1]
		} else {
			value = strings.Trim(value, `"`)
		}
	} else if i := strings.Index(value, " --"); i >= 0 {
		value = value[:i]
	}
	if value == "" {
		return ""
	}
	return filepath.Dir(value)
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
