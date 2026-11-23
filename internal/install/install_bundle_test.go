package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnclosingApp(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "Tuick-Tack.app")
	exe := filepath.Join(app, "Contents", "MacOS", "tuick-tack")
	if got, ok := enclosingApp(exe); !ok || got != app {
		t.Fatalf("enclosingApp(%q) = %q, %v; want %q, true", exe, got, ok, app)
	}
	for _, path := range []string{
		filepath.Join(root, "Tuick-Tack.app", "tuick-tack"),
		filepath.Join(root, "Tuick-Tack.app", "Contents", "tuick-tack"),
		filepath.Join(root, "Tuick-Tack", "Contents", "MacOS", "tuick-tack"),
	} {
		if got, ok := enclosingApp(path); ok {
			t.Fatalf("enclosingApp(%q) = %q, want no bundle", path, got)
		}
	}
}

func TestBundleInfo(t *testing.T) {
	root := t.TempDir()
	apps := filepath.Join(root, "Applications")
	app := filepath.Join(apps, "Tuick-Tack.app")
	exe := filepath.Join(app, "Contents", "MacOS", "tuick-tack")
	if err := os.MkdirAll(filepath.Dir(exe), 0o700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>Tuick-Tack</string>
	<key>CFBundleShortVersionString</key>
	<string>0.4.0</string>
	<key>CFBundleVersion</key>
	<string>4</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o600); err != nil {
		t.Fatal(err)
	}

	info, ok := bundleInfo(exe, []string{apps})
	if !ok {
		t.Fatal("bundle inside Applications was not detected")
	}
	if info.Dir != app || info.Exe != exe || info.Version != "0.4.0" {
		t.Fatalf("bundleInfo = %+v", info)
	}

	if _, ok := bundleInfo(exe, []string{filepath.Join(root, "Other")}); ok {
		t.Fatal("a bundle outside the app locations must be portable")
	}
}

func TestPlistString(t *testing.T) {
	data := []byte(`<plist><dict>
	<key>First</key><string>one</string>
	<key>CFBundleShortVersionString</key><string>v1 &amp; 2</string>
</dict></plist>`)
	if got, want := plistString(data, "CFBundleShortVersionString"), "v1 & 2"; got != want {
		t.Fatalf("plistString = %q, want %q", got, want)
	}
	if got := plistString(data, "Missing"); got != "" {
		t.Fatalf("missing key = %q, want empty", got)
	}
	if got := plistString([]byte("not a plist"), "CFBundleShortVersionString"); got != "" {
		t.Fatalf("malformed plist = %q, want empty", got)
	}
}
