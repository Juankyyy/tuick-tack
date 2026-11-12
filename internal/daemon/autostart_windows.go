package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	autostartKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartValue = "TuickTack"

	// autostartArgs runs the daemon without a console. The Run key starts the
	// console executable at sign-in, so --hidden detaches it from the console
	// Windows creates for it.
	autostartArgs = " --daemon --hidden"
)

// SelfExecutable returns the absolute path of the running executable.
func SelfExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

// AutostartExecutable returns the executable path to register for sign-in. It
// rejects `go run` builds: their cache path is temporary, so registering one
// would leave a broken Run entry after the development session.
func AutostartExecutable() (string, error) {
	exe, err := SelfExecutable()
	if err != nil {
		return "", err
	}
	if isDevelopmentPath(exe) {
		return "", errors.New("autostart is unavailable for development builds")
	}
	return exe, nil
}

func isDevelopmentPath(exe string) bool {
	lower := strings.ToLower(exe)
	return strings.Contains(lower, `\go-build`) || strings.Contains(lower, `/go-build`)
}

// AutostartCommand returns the Run value that starts the daemon at sign-in.
func AutostartCommand(exePath string) string {
	return `"` + exePath + `"` + autostartArgs
}

// AutostartEnabled reports whether the given executable is registered for
// sign-in. The Run value may include arguments; only the executable path is
// compared. A registration for a different path is treated as off.
func AutostartEnabled(exePath string) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(autostartValue)
	if err != nil {
		return false
	}
	return sameExecutable(autostartPath(value), exePath)
}

// autostartPath extracts the executable path from a Run value. Values written
// by Tuick-Tack are quoted and may carry arguments; legacy values are plain
// paths.
func autostartPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if value[0] == '"' {
		if end := strings.IndexByte(value[1:], '"'); end >= 0 {
			return value[1 : end+1]
		}
		return strings.Trim(value, `"`)
	}
	// An unquoted path cannot contain spaces before its arguments; cut at the
	// first argument separator.
	if i := strings.Index(value, " --"); i >= 0 {
		value = value[:i]
	}
	return strings.TrimSpace(value)
}

func sameExecutable(value, exePath string) bool {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	value = strings.TrimSpace(value)
	if value == "" || exePath == "" {
		return false
	}
	a, errA := filepath.Abs(value)
	b, errB := filepath.Abs(exePath)
	if errA != nil || errB != nil {
		return strings.EqualFold(value, exePath)
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// SetAutostart adds or removes HKCU\...\Run\TuickTack without admin rights.
func SetAutostart(exePath string, enabled bool) error {
	if enabled && exePath == "" {
		return errors.New("executable path is empty")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer key.Close()
	if enabled {
		if err := key.SetStringValue(autostartValue, AutostartCommand(exePath)); err != nil {
			return fmt.Errorf("write Run value: %w", err)
		}
		return nil
	}
	if err := key.DeleteValue(autostartValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("delete Run value: %w", err)
	}
	return nil
}

// autostartNeedsMigration reports whether an existing Run value predates the
// single-executable layout (tuick-tack-tray.exe) or points at this executable
// without the daemon arguments.
func autostartNeedsMigration(value, exePath string) bool {
	oldPath := autostartPath(value)
	if oldPath == "" || exePath == "" {
		return false
	}
	if strings.EqualFold(filepath.Base(oldPath), "tuick-tack-tray.exe") {
		return true
	}
	if !sameExecutable(oldPath, exePath) {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(value), AutostartCommand(exePath))
}

// MigrateAutostart rewrites a legacy Run value so an upgrade keeps the
// sign-in preference. It returns true when the registry was changed.
func MigrateAutostart(exePath string) (bool, error) {
	if exePath == "" {
		return false, nil
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("open Run key: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue(autostartValue)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read Run value: %w", err)
	}
	if !autostartNeedsMigration(value, exePath) {
		return false, nil
	}
	if err := key.SetStringValue(autostartValue, AutostartCommand(exePath)); err != nil {
		return false, fmt.Errorf("write Run value: %w", err)
	}
	return true, nil
}

// InstallStartup registers the daemon for sign-in.
func InstallStartup() error {
	exe, err := AutostartExecutable()
	if err != nil {
		return err
	}
	return SetAutostart(exe, true)
}

// UninstallStartup removes the sign-in registration.
func UninstallStartup() error {
	return SetAutostart("", false)
}
