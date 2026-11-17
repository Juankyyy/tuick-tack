// Package browser opens the release page in the user's default browser.
package browser

import (
	"errors"
	"fmt"
	"net/url"

	"golang.org/x/sys/windows"
)

// ErrInvalidURL is returned for anything that is not an absolute https URL.
var ErrInvalidURL = errors.New("browser: only https URLs are supported")

// Open launches raw in the default browser. Only https URLs are accepted: the
// URL comes from the network, and handing an arbitrary scheme to ShellExecute
// could run a local file or a registered protocol handler.
func Open(raw string) error {
	target, err := validate(raw)
	if err != nil {
		return err
	}
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

// validate returns the URL to open after checking its scheme and host.
func validate(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", ErrInvalidURL
	}
	return u.String(), nil
}
