package install

import (
	"os"
	"path/filepath"
)

// ExecutableName is the binary inside Tuick-Tack.app/Contents/MacOS.
const ExecutableName = "tuick-tack"

// Find reports the app bundle that contains the running executable, but only
// when the bundle lives in /Applications or ~/Applications. A copy opened from
// Downloads or a temporary folder is portable.
func Find() (Info, error) {
	exe, err := os.Executable()
	if err != nil {
		return Info{}, err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return Info{}, err
	}
	info, ok := bundleInfo(exe, appLocations())
	if !ok {
		return Info{}, nil
	}
	return info, nil
}

func appLocations() []string {
	dirs := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	return dirs
}
