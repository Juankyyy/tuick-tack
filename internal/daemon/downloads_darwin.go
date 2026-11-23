package daemon

import (
	"os"
	"path/filepath"
)

// downloadsDir returns ~/Downloads. macOS has no known-folder API in this
// layer, and the directory is a stable per-user location.
func downloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}
