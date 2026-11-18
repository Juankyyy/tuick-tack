// Package explorer opens File Explorer at a downloaded file.
package explorer

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ShowInFolder opens Explorer with path selected. Explorer is a GUI process,
// so it does not need a console.
func ShowInFolder(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	cmd := exec.Command("explorer.exe", "/select,", abs)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("explorer: %w", err)
	}
	return nil
}
