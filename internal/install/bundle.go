package install

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
)

// bundleInfo returns the .app info when exe is inside Contents/MacOS of a
// bundle under one of appDirs. It is pure so the detection is testable without
// writing to /Applications. Only macOS calls it today; the code has no platform
// dependency, so its test runs on every host.
func bundleInfo(exe string, appDirs []string) (Info, bool) {
	bundle, ok := enclosingApp(exe)
	if !ok {
		return Info{}, false
	}
	for _, dir := range appDirs {
		if pathWithin(bundle, dir) {
			return Info{Dir: bundle, Exe: exe, Version: bundleVersion(bundle)}, true
		}
	}
	return Info{}, false
}

// enclosingApp walks the executable path back to its .app bundle.
func enclosingApp(exe string) (string, bool) {
	macos := filepath.Dir(exe)
	if filepath.Base(macos) != "MacOS" {
		return "", false
	}
	contents := filepath.Dir(macos)
	if filepath.Base(contents) != "Contents" {
		return "", false
	}
	app := filepath.Dir(contents)
	if !strings.HasSuffix(app, ".app") {
		return "", false
	}
	return app, true
}

func pathWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// bundleVersion reads CFBundleShortVersionString from Info.plist. A missing or
// malformed plist leaves the version empty; the caller only displays it.
func bundleVersion(bundle string) string {
	data, err := os.ReadFile(filepath.Join(bundle, "Contents", "Info.plist"))
	if err != nil {
		return ""
	}
	return plistString(data, "CFBundleShortVersionString")
}

// plistString returns the string value for key in a plist dict. It streams the
// XML tokens so it does not need a plist dependency.
func plistString(data []byte, key string) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "key" {
			continue
		}
		var name string
		if err := dec.DecodeElement(&name, &start); err != nil {
			return ""
		}
		if name != key {
			continue
		}
		for {
			tok, err := dec.Token()
			if err != nil {
				return ""
			}
			switch t := tok.(type) {
			case xml.StartElement:
				if t.Name.Local == "string" {
					var value string
					if err := dec.DecodeElement(&value, &t); err != nil {
						return ""
					}
					return value
				}
				if err := dec.Skip(); err != nil {
					return ""
				}
			case xml.EndElement:
				if t.Name.Local == "dict" {
					return ""
				}
			}
		}
	}
}
