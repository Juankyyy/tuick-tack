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
)

// AutostartEnabled reports whether the current executable is registered for
// sign-in. A registration for a different path is treated as off.
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
	return sameExecutable(value, exePath)
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
	if exePath == "" {
		return errors.New("executable path is empty")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, autostartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer key.Close()
	if enabled {
		quoted := `"` + exePath + `"`
		if err := key.SetStringValue(autostartValue, quoted); err != nil {
			return fmt.Errorf("write Run value: %w", err)
		}
		return nil
	}
	if err := key.DeleteValue(autostartValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("delete Run value: %w", err)
	}
	return nil
}

// TrayExecutable finds tuick-tack-tray.exe next to the current executable.
func TrayExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := strings.ToLower(filepath.Base(exe))
	if base == "tuick-tack-tray.exe" {
		return exe, nil
	}
	candidate := filepath.Join(filepath.Dir(exe), "tuick-tack-tray.exe")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("tuick-tack-tray.exe not found next to %s", exe)
	}
	return candidate, nil
}

// TUIExecutable finds tuick-tack.exe next to the current executable.
func TUIExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := strings.ToLower(filepath.Base(exe))
	if base == "tuick-tack.exe" {
		return exe, nil
	}
	candidate := filepath.Join(filepath.Dir(exe), "tuick-tack.exe")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("tuick-tack.exe not found next to %s", exe)
	}
	return candidate, nil
}

// InstallStartup registers the tray daemon for sign-in.
func InstallStartup() error {
	exe, err := TrayExecutable()
	if err != nil {
		return err
	}
	return SetAutostart(exe, true)
}

// UninstallStartup removes the sign-in registration.
func UninstallStartup() error {
	exe, err := TrayExecutable()
	if err != nil {
		return err
	}
	return SetAutostart(exe, false)
}
