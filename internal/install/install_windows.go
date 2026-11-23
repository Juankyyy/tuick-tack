package install

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	// ExecutableName is the installed executable.
	ExecutableName = "tuick-tack.exe"
	// UninstallKey is the root where Inno Setup registers per-user installs.
	UninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
)

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
