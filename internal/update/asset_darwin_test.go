//go:build darwin

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDarwinChecksumLookup(t *testing.T) {
	content := []byte("universal zip")
	sum := sha256.Sum256(content)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  " + AssetMacOS + "\n"))
	}))
	defer srv.Close()

	info := Info{MacOSURL: srv.URL + "/zip"}
	asset, ok := info.PortableAsset()
	if !ok {
		t.Fatal("the macOS zip should be the portable asset")
	}
	got, err := (Downloader{Client: srv.Client()}).ExpectedSHA256(
		context.Background(), asset, &Asset{Name: AssetMacOSSums, URL: srv.URL + "/sums"})
	if err != nil {
		t.Fatal(err)
	}
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %q", got)
	}
}

// checkPlatformCapabilities asserts the JSON-to-capability mapping for the
// macOS release: there is one download and no in-place install.
func checkPlatformCapabilities(t *testing.T, info Info) {
	t.Helper()
	if info.CanInstall() {
		t.Fatal("macOS must not support in-place installs")
	}
	if !info.CanDownloadPortable() {
		t.Fatalf("the macOS zip should be downloadable: %+v", info)
	}
}

func TestInfoAssets(t *testing.T) {
	info := Info{
		Version:      "v9.9.9",
		URL:          "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9",
		SetupURL:     "https://example.test/setup.exe",
		SetupDigest:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		PortableURL:  "https://example.test/tuick-tack.exe",
		MacOSURL:     "https://example.test/tuick-tack-macos-universal.zip",
		MacOSDigest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MacOSSumsURL: "https://example.test/tuick-tack-macos-universal.zip.sha256",
	}
	if InstallSupported {
		t.Fatal("macOS should not support in-place installs")
	}
	if info.CanInstall() {
		t.Fatal("macOS must never offer the setup installer")
	}
	if !info.CanDownloadPortable() {
		t.Fatal("the macOS zip with a digest should be downloadable")
	}
	if asset, ok := info.PortableAsset(); !ok || asset.Name != AssetMacOS || asset.URL != info.MacOSURL {
		t.Fatalf("macOS asset = %+v, %v", asset, ok)
	}
	if asset, ok := info.ChecksumsAsset(); !ok || asset.Name != AssetMacOSSums || asset.URL != info.MacOSSumsURL {
		t.Fatalf("macOS checksums = %+v, %v", asset, ok)
	}

	// The Windows checksum file does not list the zip, so it must not make the
	// macOS download look verifiable on its own.
	info.MacOSDigest = ""
	info.ChecksumsURL = "https://example.test/SHA256SUMS.txt"
	if !info.CanDownloadPortable() {
		t.Fatal("the dedicated .sha256 should keep the download available")
	}
	info.MacOSSumsURL = ""
	if info.CanDownloadPortable() {
		t.Fatal("without a macOS checksum the download is unavailable")
	}

	info.MacOSURL = ""
	if info.CanDownloadPortable() {
		t.Fatal("an empty zip URL should disable the download")
	}
}
