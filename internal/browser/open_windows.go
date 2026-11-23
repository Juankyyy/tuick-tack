package browser

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// openURL hands the validated URL to the shell.
func openURL(target string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	if err := windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	return nil
}
