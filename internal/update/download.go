package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// MaxDownloadSize bounds a release asset. The executables are a few MiB;
	// the limit protects against a wrong URL or a hostile mirror.
	MaxDownloadSize = 128 << 20
	// downloadTimeout bounds one download. The daemon also cancels the request
	// when it exits.
	downloadTimeout = 10 * time.Minute
)

// ErrChecksum reports a downloaded asset that did not match its published
// SHA-256. Callers use it to show a verification error instead of a network
// error.
var ErrChecksum = errors.New("update: checksum mismatch")

// Downloader fetches release assets and their checksums. The zero value is not
// usable; build one with NewDownloader and override the fields in tests.
type Downloader struct {
	Client    *http.Client
	UserAgent string
}

// NewDownloader returns a downloader that identifies itself with version.
func NewDownloader(version string) *Downloader {
	return &Downloader{
		Client:    &http.Client{Timeout: downloadTimeout},
		UserAgent: "tuick-tack/" + version,
	}
}

// Checksums downloads and parses SHA256SUMS.txt.
func (d Downloader) Checksums(ctx context.Context, asset Asset) (map[string]string, error) {
	if asset.URL == "" {
		return nil, errors.New("update: no checksums URL")
	}
	body, err := d.fetch(ctx, asset.URL, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("update: checksums: %w", err)
	}
	defer body.Close()

	sums := make(map[string]string)
	sc := bufio.NewScanner(io.LimitReader(body, 1<<20))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != 64 || !isHex(sum) {
			continue
		}
		sums[fields[1]] = sum
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("update: checksums: %w", err)
	}
	if len(sums) == 0 {
		return nil, errors.New("update: checksums file is empty")
	}
	return sums, nil
}

// ExpectedSHA256 returns the expected hash for asset. It prefers the digest
// GitHub reports for the asset and only downloads SHA256SUMS.txt when that
// digest is absent.
func (d Downloader) ExpectedSHA256(ctx context.Context, asset Asset, checksums *Asset) (string, error) {
	if hasSHA256(asset.Digest) {
		return strings.ToLower(strings.TrimPrefix(asset.Digest, "sha256:")), nil
	}
	if checksums == nil {
		return "", errors.New("update: no checksum available")
	}
	sums, err := d.Checksums(ctx, *checksums)
	if err != nil {
		return "", err
	}
	sum, ok := sums[asset.Name]
	if !ok {
		return "", fmt.Errorf("update: checksums have no entry for %s", asset.Name)
	}
	return sum, nil
}

// Download streams url into dst, verifies expectedSHA256 and only then renames
// the partial file into place. total is the advertised asset size; the
// response Content-Length wins when it is present.
func (d Downloader) Download(ctx context.Context, url, dst, expectedSHA256 string, total int64, progress func(received, total int64)) error {
	if !strings.HasPrefix(url, "https://") {
		return errors.New("update: download URL is not https")
	}
	if expectedSHA256 == "" {
		return errors.New("update: no expected checksum")
	}
	body, err := d.fetch(ctx, url, MaxDownloadSize)
	if err != nil {
		return fmt.Errorf("update: download: %w", err)
	}
	defer body.Close()

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("update: create download dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tuick-download-*")
	if err != nil {
		return fmt.Errorf("update: create download: %w", err)
	}
	tmpName := tmp.Name()
	clean := func() {
		tmp.Close()
		_ = os.Remove(tmpName)
	}
	defer clean()

	hasher := sha256.New()
	written := int64(0)
	buf := make([]byte, 64*1024)
	lastReport := time.Time{}
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			written += int64(n)
			if written > MaxDownloadSize {
				return fmt.Errorf("update: asset is larger than %d MiB", MaxDownloadSize>>20)
			}
			if _, err := tmp.Write(buf[:n]); err != nil {
				return fmt.Errorf("update: write download: %w", err)
			}
			_, _ = hasher.Write(buf[:n])
			if progress != nil && (lastReport.IsZero() || time.Since(lastReport) >= 100*time.Millisecond) {
				lastReport = time.Now()
				progress(written, total)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return fmt.Errorf("update: read download: %w", readErr)
		}
	}
	if progress != nil {
		progress(written, total)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("update: sync download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("update: close download: %w", err)
	}
	got := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(got, expectedSHA256) {
		return fmt.Errorf("%w (got %s, want %s)", ErrChecksum, got, expectedSHA256)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("update: move download into place: %w", err)
	}
	return nil
}

// fetch opens url and applies the shared headers and size limit.
func (d Downloader) fetch(ctx context.Context, rawURL string, limit int64) (io.ReadCloser, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("update: invalid https URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	ua := d.UserAgent
	if ua == "" {
		ua = "tuick-tack"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/octet-stream")

	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("github returned %s", resp.Status)
	}
	if resp.ContentLength > limit {
		resp.Body.Close()
		return nil, fmt.Errorf("asset is larger than %d MiB", limit>>20)
	}
	return resp.Body, nil
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') && !(r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}
