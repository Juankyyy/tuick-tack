package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfoAssets(t *testing.T) {
	info := Info{
		Version:        "v9.9.9",
		URL:            "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9",
		SetupURL:       "https://example.test/setup.exe",
		SetupDigest:    "",
		PortableURL:    "https://example.test/tuick-tack.exe",
		PortableDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ChecksumsURL:   "https://example.test/SHA256SUMS.txt",
	}
	if !info.CanInstall() {
		t.Fatal("setup with checksums should be installable")
	}
	if !info.CanDownloadPortable() {
		t.Fatal("portable with digest should be downloadable")
	}
	if asset, ok := info.SetupAsset(); !ok || asset.Name != AssetSetup || asset.URL != info.SetupURL {
		t.Fatalf("setup asset = %+v, %v", asset, ok)
	}
	if asset, ok := info.ChecksumsAsset(); !ok || asset.Name != AssetChecksums {
		t.Fatalf("checksums asset = %+v, %v", asset, ok)
	}

	info.SetupURL = ""
	info.PortableURL = ""
	if info.CanInstall() || info.CanDownloadPortable() {
		t.Fatal("empty URLs should disable both actions")
	}
}

func TestCheckerLatestAssets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"tag_name": "v9.9.9",
			"html_url": "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9",
			"assets": [
				{"name":"tuick-tack-setup.exe","browser_download_url":"https://example.test/setup.exe","size":12,"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				{"name":"tuick-tack.exe","browser_download_url":"https://example.test/tuick-tack.exe","size":34},
				{"name":"SHA256SUMS.txt","browser_download_url":"https://example.test/SHA256SUMS.txt"},
				{"name":"ignored.bin","browser_download_url":"http://insecure.test/ignored"}
			]
		}`))
	}))
	defer srv.Close()

	c := Checker{Client: srv.Client(), URL: srv.URL}
	info, err := c.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.SetupURL != "https://example.test/setup.exe" || info.SetupSize != 12 {
		t.Fatalf("setup = %+v", info)
	}
	if !strings.HasPrefix(info.SetupDigest, "sha256:") {
		t.Fatalf("setup digest = %q", info.SetupDigest)
	}
	if info.PortableURL != "https://example.test/tuick-tack.exe" || info.PortableSize != 34 {
		t.Fatalf("portable = %+v", info)
	}
	if info.ChecksumsURL != "https://example.test/SHA256SUMS.txt" {
		t.Fatalf("checksums = %q", info.ChecksumsURL)
	}
	if !info.CanInstall() || !info.CanDownloadPortable() {
		t.Fatal("the release should support both actions")
	}
}

func TestDownloaderChecksums(t *testing.T) {
	content := []byte("portable")
	sum := sha256.Sum256(content)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sums" {
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  tuick-tack.exe\r\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	d := Downloader{Client: srv.Client(), UserAgent: "test"}
	sums, err := d.Checksums(context.Background(), Asset{Name: AssetChecksums, URL: srv.URL + "/sums"})
	if err != nil {
		t.Fatal(err)
	}
	if sums[AssetPortable] != hex.EncodeToString(sum[:]) {
		t.Fatalf("sums = %+v", sums)
	}
}

func TestDownloaderDownload(t *testing.T) {
	content := []byte("new portable executable")
	sum := sha256.Sum256(content)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/file" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", "23")
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "tuick-tack.exe")
	var lastReceived, lastTotal int64
	d := Downloader{Client: srv.Client()}
	err := d.Download(context.Background(), srv.URL+"/file", dst, hex.EncodeToString(sum[:]), int64(len(content)), func(received, total int64) {
		lastReceived, lastTotal = received, total
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("file = %q", got)
	}
	if lastReceived != int64(len(content)) || lastTotal != int64(len(content)) {
		t.Fatalf("progress = %d/%d", lastReceived, lastTotal)
	}
}

func TestDownloaderChecksumMismatch(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "file.exe")
	err := Downloader{Client: srv.Client()}.Download(context.Background(), srv.URL+"/file", dst, strings.Repeat("0", 64), 8, nil)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("mismatched download should not exist: %v", statErr)
	}
}

func TestDownloaderExpectedSHA256(t *testing.T) {
	content := []byte("portable")
	sum := sha256.Sum256(content)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  tuick-tack.exe\n"))
	}))
	defer srv.Close()

	d := Downloader{Client: srv.Client()}
	asset := Asset{Name: AssetPortable, URL: srv.URL + "/file", Digest: "sha256:" + strings.Repeat("b", 64)}
	got, err := d.ExpectedSHA256(context.Background(), asset, &Asset{URL: srv.URL + "/sums"})
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.Repeat("b", 64) {
		t.Fatalf("digest should win: %q", got)
	}

	asset.Digest = ""
	got, err = d.ExpectedSHA256(context.Background(), asset, &Asset{URL: srv.URL + "/sums"})
	if err != nil {
		t.Fatal(err)
	}
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %q", got)
	}
}

func TestDownloaderRejectsHTTP(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "file.exe")
	err := Downloader{}.Download(context.Background(), "http://example.test/file", dst, strings.Repeat("0", 64), 1, nil)
	if err == nil {
		t.Fatal("expected an error for an insecure URL")
	}
}
