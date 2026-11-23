//go:build windows

package update

import "testing"

// checkPlatformCapabilities asserts the JSON-to-capability mapping for the
// Windows release: the setup and the portable executable are both usable.
func checkPlatformCapabilities(t *testing.T, info Info) {
	t.Helper()
	if !info.CanInstall() || !info.CanDownloadPortable() {
		t.Fatalf("the release should support install and download: %+v", info)
	}
}

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
	if !InstallSupported {
		t.Fatal("windows should support in-place installs")
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
	if asset, ok := info.PortableAsset(); !ok || asset.Name != AssetPortable || asset.URL != info.PortableURL {
		t.Fatalf("portable asset = %+v, %v", asset, ok)
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
