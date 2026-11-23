// Package explorer reveals downloaded files in the platform file manager.
package explorer

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// ShowInFolder reveals path in Finder with the file selected.
func ShowInFolder(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := exec.Command("/usr/bin/open", "-R", abs).Run(); err != nil {
		return fmt.Errorf("explorer: %w", err)
	}
	return nil
}
