package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// downloadSuffix names the downloaded update on this platform. Windows saves
// the portable executable; macOS saves the universal .app zip.
const downloadSuffix = "-macos-universal.zip"

// downloadsDir returns ~/Downloads. macOS has no known-folder API in this
// layer, and the directory is a stable per-user location.
func downloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

// revealDownload opens Finder with the verified zip selected. There is no
// in-place install on macOS, so showing the file is the end of the update.
func revealDownload(path string) error {
	if err := exec.Command("/usr/bin/open", "-R", path).Run(); err != nil {
		return fmt.Errorf("daemon: reveal download: %w", err)
	}
	return nil
}
