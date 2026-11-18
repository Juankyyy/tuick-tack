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

	// AssetSetup and AssetPortable are the stable release asset names. The
	// updater finds them by exact name so the release tag stays free-form.
	AssetSetup     = "tuick-tack-setup.exe"
	AssetPortable  = "tuick-tack.exe"
	AssetChecksums = "SHA256SUMS.txt"

	// requestTimeout bounds one request. The daemon also cancels the request
	// when it exits.
	requestTimeout = 15 * time.Second
)

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name   string
	URL    string
	Size   int64
	Digest string // GitHub's "sha256:<hex>" digest, when present.
}

// Info describes the latest published release.
type Info struct {
	// Version is the release tag as published, for example "v0.2.0".
	Version string
	// URL is the GitHub release page.
	URL string

	// The setup installer and the portable executable are optional: an older
	// release may not publish them under the stable names. The UI disables the
	// matching action when the asset or its checksum is missing.
	SetupURL       string
	SetupDigest    string
	SetupSize      int64
	PortableURL    string
	PortableDigest string
	PortableSize   int64
	ChecksumsURL   string
}

// SetupAsset returns the installer asset when the release publishes it.
func (i Info) SetupAsset() (Asset, bool) {
	return i.asset(AssetSetup, i.SetupURL, i.SetupDigest, i.SetupSize)
}

// PortableAsset returns the portable executable asset when published.
func (i Info) PortableAsset() (Asset, bool) {
	return i.asset(AssetPortable, i.PortableURL, i.PortableDigest, i.PortableSize)
}

// ChecksumsAsset returns SHA256SUMS.txt when published.
func (i Info) ChecksumsAsset() (Asset, bool) {
	if i.ChecksumsURL == "" {
		return Asset{}, false
	}
	return Asset{Name: AssetChecksums, URL: i.ChecksumsURL}, true
}

// CanInstall reports whether the setup asset can be downloaded and verified.
func (i Info) CanInstall() bool {
	asset, ok := i.SetupAsset()
	if !ok {
		return false
	}
	return hasSHA256(asset.Digest) || i.ChecksumsURL != ""
}

// CanDownloadPortable reports whether the portable executable can be
// downloaded and verified.
func (i Info) CanDownloadPortable() bool {
	asset, ok := i.PortableAsset()
	if !ok {
		return false
	}
	return hasSHA256(asset.Digest) || i.ChecksumsURL != ""
}

func (i Info) asset(name, url, digest string, size int64) (Asset, bool) {
	if url == "" {
		return Asset{}, false
	}
	return Asset{Name: name, URL: url, Size: size, Digest: digest}, true
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
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
			Digest             string `json:"digest"`
		} `json:"assets"`
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
	info := Info{Version: tag, URL: url}
	for _, a := range payload.Assets {
		name := strings.TrimSpace(a.Name)
		assetURL := strings.TrimSpace(a.BrowserDownloadURL)
		if name == "" || !strings.HasPrefix(assetURL, "https://") {
			continue
		}
		switch name {
		case AssetSetup:
			info.SetupURL, info.SetupSize, info.SetupDigest = assetURL, a.Size, strings.TrimSpace(a.Digest)
		case AssetPortable:
			info.PortableURL, info.PortableSize, info.PortableDigest = assetURL, a.Size, strings.TrimSpace(a.Digest)
		case AssetChecksums:
			info.ChecksumsURL = assetURL
		}
	}
	return info, nil
}

// hasSHA256 reports whether a GitHub asset digest is a usable SHA-256 value.
func hasSHA256(digest string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		return false
	}
	hex := strings.TrimSpace(digest[len(prefix):])
	if len(hex) != 64 {
		return false
	}
	for _, r := range hex {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') && !(r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
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
