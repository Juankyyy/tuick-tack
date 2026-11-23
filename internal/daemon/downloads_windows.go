package daemon

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// downloadsDir returns the user's Downloads known folder, with a conservative
// fallback when the shell lookup is unavailable.
func downloadsDir() (string, error) {
	if dir, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, windows.KF_FLAG_DEFAULT); err == nil && dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}
