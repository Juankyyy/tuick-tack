package daemon

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// downloadSuffix names the downloaded update on this platform. Windows saves
// the portable executable; macOS saves the universal .app zip.
const downloadSuffix = ".exe"

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

// revealDownload is a no-op on Windows: the TUI offers "open folder" after the
// download and the daemon never takes focus on its own.
func revealDownload(string) error {
	return nil
}
