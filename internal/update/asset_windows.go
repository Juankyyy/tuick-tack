//go:build windows

package update

// InstallSupported reports whether the release can be installed in place. The
// Windows build downloads the Inno Setup installer and starts the helper.
const InstallSupported = true

// PortableAsset returns the portable executable published by the release.
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
