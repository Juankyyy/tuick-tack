package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
// would leave a broken entry after the development session.
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
