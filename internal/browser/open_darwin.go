package browser

import (
	"fmt"
	"os/exec"
)

// openURL hands the validated URL to LaunchServices.
func openURL(target string) error {
	if err := exec.Command("/usr/bin/open", target).Run(); err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	return nil
}
