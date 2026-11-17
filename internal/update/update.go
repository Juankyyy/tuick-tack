// Package update asks GitHub for the latest Tuick-Tack release and compares it
// with the version this binary was built from. It never downloads or runs
// anything: the app only tells the user that a newer release exists.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// Repo is the GitHub repository that publishes Tuick-Tack releases.
	Repo = "Juankyyy/tuick-tack"
	// APIURL is the "latest release" endpoint for Repo.
	APIURL = "https://api.github.com/repos/" + Repo + "/releases/latest"

	// CheckInterval is how often the daemon looks for a new release.
	CheckInterval = 24 * time.Hour
	// RetryInterval is the shorter delay used after a failed check.
	RetryInterval = 6 * time.Hour

	// requestTimeout bounds one request. The daemon also cancels the request
	// when it exits.
	requestTimeout = 15 * time.Second
)

// Info describes the latest published release.
type Info struct {
	// Version is the release tag as published, for example "v0.2.0".
	Version string
	// URL is the GitHub release page.
	URL string
}

// Checker fetches the latest release. The zero value is not usable; build one
// with NewChecker and override the fields in tests.
type Checker struct {
	Client    *http.Client
	URL       string
	UserAgent string
}

// NewChecker returns a checker that identifies itself with version.
func NewChecker(version string) *Checker {
	return &Checker{
		Client:    &http.Client{Timeout: requestTimeout},
		URL:       APIURL,
		UserAgent: "tuick-tack/" + version,
	}
}

// Latest returns the newest published release. Drafts and prereleases are
// already excluded by the GitHub endpoint.
func (c Checker) Latest(ctx context.Context) (Info, error) {
	if c.URL == "" {
		return Info{}, errors.New("update: no release URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return Info{}, fmt.Errorf("update: request: %w", err)
	}
	ua := c.UserAgent
	if ua == "" {
		ua = "tuick-tack"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, fmt.Errorf("update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Drain a little so the connection can be reused, but do not keep the
		// error body: it can be large and is not user-facing.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		return Info{}, fmt.Errorf("update: github returned %s", resp.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return Info{}, fmt.Errorf("update: decode release: %w", err)
	}
	tag := strings.TrimSpace(payload.TagName)
	url := strings.TrimSpace(payload.HTMLURL)
	if tag == "" {
		return Info{}, errors.New("update: release has no tag")
	}
	if !strings.HasPrefix(url, "https://") {
		return Info{}, errors.New("update: release has no https URL")
	}
	return Info{Version: tag, URL: url}, nil
}

// Delay returns how long to wait before the next check.
func Delay(failed bool) time.Duration {
	if failed {
		return RetryInterval
	}
	return CheckInterval
}

// IsNewer reports whether latest is a later version than current.
func IsNewer(current, latest string) bool {
	return Compare(current, latest) < 0
}

// Compare orders two version strings. It accepts a leading "v", treats missing
// patch components as zero, ignores build metadata and sorts a prerelease
// before its final release. A value that cannot be parsed (for example "dev")
// sorts before every parsable version; two unparsable values compare equal.
// Prerelease identifiers compare lexically, which is enough for this project.
func Compare(a, b string) int {
	va, oka := parseVersion(a)
	vb, okb := parseVersion(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	if va.major != vb.major {
		return cmpInt(va.major, vb.major)
	}
	if va.minor != vb.minor {
		return cmpInt(va.minor, vb.minor)
	}
	if va.patch != vb.patch {
		return cmpInt(va.patch, vb.patch)
	}
	switch {
	case va.pre == "" && vb.pre == "":
		return 0
	case va.pre == "":
		return 1
	case vb.pre == "":
		return -1
	case va.pre < vb.pre:
		return -1
	case va.pre > vb.pre:
		return 1
	}
	return 0
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

type version struct {
	major, minor, patch int
	pre                 string
}

func parseVersion(s string) (version, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return version{}, false
	}
	if s[0] == 'v' || s[0] == 'V' {
		s = s[1:]
	}
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	pre := ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return version{}, false
	}
	nums := [3]int{}
	for i, part := range parts {
		if part == "" {
			return version{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return version{}, false
		}
		nums[i] = n
	}
	return version{major: nums[0], minor: nums[1], patch: nums[2], pre: pre}, true
}
