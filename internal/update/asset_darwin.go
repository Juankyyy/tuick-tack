//go:build darwin

package update

// InstallSupported reports whether the release can be installed in place.
// macOS has no silent self-update: the app downloads the zip and the user
// replaces Tuick-Tack.app, so install_update answers unsupported.
const InstallSupported = false

// PortableAsset returns the universal .app zip. The protocol keeps the
// "portable" name for both platforms because macOS has a single download.
func (i Info) PortableAsset() (Asset, bool) {
	return i.asset(AssetMacOS, i.MacOSURL, i.MacOSDigest, i.MacOSSize)
}

// ChecksumsAsset returns the zip's own .sha256 when published.
func (i Info) ChecksumsAsset() (Asset, bool) {
	if i.MacOSSumsURL == "" {
		return Asset{}, false
	}
	return Asset{Name: AssetMacOSSums, URL: i.MacOSSumsURL}, true
}
