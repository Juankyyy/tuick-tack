package notify

import (
	"fmt"
	"os/exec"
	"strings"
)

// Show displays one notification with osascript. launch is accepted for API
// parity with the Windows toast but the v1 notification has no click action.
func Show(title, message, launch string) error {
	script := "display notification " + appleScriptString(message) +
		" with title " + appleScriptString(title)
	out, err := exec.Command("/usr/bin/osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("notify: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
