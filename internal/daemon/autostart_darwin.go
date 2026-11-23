package daemon

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// launchAgentLabel is the reverse-DNS identity of the login item. It matches
// the .app bundle identifier.
const launchAgentLabel = "com.juankyyy.tuick-tack"

// launchAgentPath returns ~/Library/LaunchAgents/com.juankyyy.tuick-tack.plist.
func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

// launchAgentPlist renders the LaunchAgent. RunAtLoad starts the daemon at
// sign-in; KeepAlive is false so quitting it from the tray does not restart it.
func launchAgentPlist(exePath string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(exePath))
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchAgentLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + escaped.String() + `</string>
		<string>--daemon</string>
		<string>--hidden</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>
`
}

// AutostartEnabled reports whether the LaunchAgent exists and starts exePath.
func AutostartEnabled(exePath string) bool {
	path, err := launchAgentPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return launchAgentTargets(data, exePath)
}

// launchAgentTargets reports whether data is a plist written for exePath. The
// only executable string in the file this app writes is the first
// ProgramArguments entry.
func launchAgentTargets(data []byte, exePath string) bool {
	if exePath == "" {
		return false
	}
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(exePath))
	return bytes.Contains(data, []byte("<string>"+escaped.String()+"</string>"))
}

// SetAutostart writes or removes the LaunchAgent and reloads the job. Writes are
// atomic: a partial plist would make launchd ignore the login item.
func SetAutostart(exePath string, enabled bool) error {
	if enabled && exePath == "" {
		return errors.New("executable path is empty")
	}
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	if !enabled {
		bootoutLaunchAgent()
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove LaunchAgent: %w", err)
		}
		return nil
	}
	if err := writeLaunchAgent(path, exePath); err != nil {
		return err
	}
	// Make the change effective now. bootout is ignored when the label is not
	// loaded; bootstrap can race a just-written plist but the file is what the
	// next login reads, so a failure is not fatal.
	bootoutLaunchAgent()
	_ = exec.Command("/bin/launchctl", "bootstrap", launchAgentDomain(), path).Run()
	return nil
}

// MigrateAutostart rewrites the LaunchAgent when the app moved. It returns true
// when the plist was changed. The running daemon is left alone; the new path is
// used from the next login.
func MigrateAutostart(exePath string) (bool, error) {
	if exePath == "" {
		return false, nil
	}
	path, err := launchAgentPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if launchAgentTargets(data, exePath) {
		return false, nil
	}
	if err := writeLaunchAgent(path, exePath); err != nil {
		return false, err
	}
	return true, nil
}

func writeLaunchAgent(path, exePath string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tuick-tack-*.plist")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.WriteString(launchAgentPlist(exePath)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("write LaunchAgent: %w", err)
	}
	return nil
}

func launchAgentDomain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

func bootoutLaunchAgent() {
	// launchd sets XPC_SERVICE_NAME for the job it started. Booting out that
	// job would SIGTERM the daemon while the user toggles the setting, so when
	// this process is the agent, removing the plist is enough for the next
	// login and the loaded job stays until logout.
	if os.Getenv("XPC_SERVICE_NAME") == launchAgentLabel {
		return
	}
	_ = exec.Command("/bin/launchctl", "bootout", launchAgentDomain()+"/"+launchAgentLabel).Run()
}
