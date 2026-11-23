// Package updater is the short-lived helper that finishes an installer update
// on Windows. macOS has no silent self-update: the app downloads the release
// zip and the user replaces Tuick-Tack.app, so the helper is unsupported there.
package updater

import (
	"errors"
	"os"
	"path/filepath"
)

// Apply reports that the update helper is not available on macOS.
func Apply([]string) int {
	return 2
}

// Spawn reports that macOS has no installer helper.
func Spawn(setupPath, fallback, pipe string) error {
	return errors.New("updater: not supported on macOS")
}

// UpdateDir is where the daemon downloads update artifacts.
func UpdateDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "tuick-tack", "updates"), nil
}
