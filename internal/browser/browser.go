// Package browser opens URLs in the user's default browser.
package browser

import (
	"errors"
	"net/url"
)

// ErrInvalidURL is returned for anything that is not an absolute https URL.
var ErrInvalidURL = errors.New("browser: only https URLs are supported")

// Open launches raw in the default browser. Only https URLs are accepted: the
// URL comes from the network, and handing an arbitrary scheme to the platform
// opener could run a local file or a registered protocol handler.
func Open(raw string) error {
	target, err := validate(raw)
	if err != nil {
		return err
	}
	return openURL(target)
}

// validate returns the URL to open after checking its scheme and host.
func validate(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", ErrInvalidURL
	}
	return u.String(), nil
}
