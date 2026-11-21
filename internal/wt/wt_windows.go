// Package wt reads Windows Terminal's configuration and opens the TUI in a
// window that uses the user's default profile. It is read-only: it never
// writes settings.json or the registry.
package wt

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"tuick-tack/internal/winproc"
)

// Host identifies the terminal application registered as the Windows default.
type Host int

const (
	// HostUnknown means the delegation is missing, automatic or unrecognised.
	HostUnknown Host = iota
	// HostConsole means the classic Windows Console Host is the default.
	HostConsole
	// HostWindowsTerminal means stable Windows Terminal is the default.
	HostWindowsTerminal
	// HostWindowsTerminalPreview means Windows Terminal Preview is the default.
	HostWindowsTerminalPreview
)

// Delegation CLSIDs from the "Default terminal application" setting. Both the
// terminal and the console values are normally written together; matching
// either keeps detection working if a Windows build writes only one.
const (
	consoleHostCLSID     = "{B23D10C0-E52E-411E-9D5B-C09FDF709C7D}"
	terminalStableCLSID  = "{E12CFF52-A866-4C77-9A90-F570A7AA2C6B}"
	consoleStableCLSID   = "{2EACA947-7F5F-4CFA-BA87-8F7FBEEFBE69}"
	terminalPreviewCLSID = "{86633F1F-6454-40EC-89CE-DA4EBA977EE2}"
	consolePreviewCLSID  = "{06EC847C-C0A5-46B8-92CB-7C92F6E35CD5}"
)

// DefaultHost reports which terminal application Windows uses for console
// launches. It reads HKCU\Console\%%Startup, the same key the Settings UI
// writes for "Default terminal application".
func DefaultHost() Host {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Console\%%Startup`, registry.QUERY_VALUE)
	if err != nil {
		return HostUnknown
	}
	defer key.Close()

	terminal, _, _ := key.GetStringValue("DelegationTerminal")
	console, _, _ := key.GetStringValue("DelegationConsole")
	return hostFromDelegation(terminal, console)
}

func hostFromDelegation(terminal, console string) Host {
	switch {
	case equalCLSID(terminal, terminalStableCLSID) || equalCLSID(console, consoleStableCLSID):
		return HostWindowsTerminal
	case equalCLSID(terminal, terminalPreviewCLSID) || equalCLSID(console, consolePreviewCLSID):
		return HostWindowsTerminalPreview
	case equalCLSID(terminal, consoleHostCLSID) || equalCLSID(console, consoleHostCLSID):
		return HostConsole
	default:
		return HostUnknown
	}
}

func equalCLSID(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), b)
}

// profileFor returns the profile name or GUID configured as Windows Terminal's
// default for the given host. It reports false when the host is not Windows
// Terminal or no settings file can be read.
func profileFor(host Host) (string, bool) {
	return profileFromPaths(settingsPaths(host))
}

func profileFromPaths(paths []string) (string, bool) {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if profile, ok := parseDefaultProfile(data); ok {
			return profile, true
		}
	}
	return "", false
}

// settingsPaths returns the candidate settings files for host, in order: the
// Microsoft Store package first and the unpackaged build second.
func settingsPaths(host Host) []string {
	local, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	return settingsPathsAt(local, host)
}

func settingsPathsAt(local string, host Host) []string {
	switch host {
	case HostWindowsTerminal:
		return []string{
			filepath.Join(local, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
			filepath.Join(local, "Microsoft", "Windows Terminal", "settings.json"),
		}
	case HostWindowsTerminalPreview:
		return []string{
			filepath.Join(local, "Packages", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", "settings.json"),
			filepath.Join(local, "Microsoft", "Windows Terminal Preview", "settings.json"),
		}
	default:
		return nil
	}
}

// parseDefaultProfile accepts Windows Terminal's settings.json, which may be
// JSONC (comments and trailing commas) and may start with a UTF-8 BOM.
func parseDefaultProfile(data []byte) (string, bool) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	clean := stripTrailingCommas(stripComments(data))

	var settings struct {
		DefaultProfile string `json:"defaultProfile"`
	}
	if err := json.Unmarshal(clean, &settings); err != nil {
		return "", false
	}

	profile := strings.TrimSpace(settings.DefaultProfile)
	return profile, profile != ""
}

// stripComments removes // and /* */ comments outside JSON strings. Newlines
// are kept so structural whitespace survives; a block comment becomes a single
// space so it cannot join two tokens.
func stripComments(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	escaped := false

	for i := 0; i < len(in); i++ {
		c := in[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(in) {
			switch in[i+1] {
			case '/':
				i += 2
				for i < len(in) && in[i] != '\n' {
					i++
				}
				if i < len(in) {
					out = append(out, '\n')
				}
				continue
			case '*':
				i += 2
				for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
					if in[i] == '\n' {
						out = append(out, '\n')
					} else {
						out = append(out, ' ')
					}
					i++
				}
				if i+1 < len(in) {
					i++ // the '/' of the closing */
				}
				out = append(out, ' ')
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// stripTrailingCommas removes a comma that only has whitespace before a
// closing brace or bracket. It runs after stripComments, so a comment cannot
// hide the closing token.
func stripTrailingCommas(in []byte) []byte {
	out := make([]byte, 0, len(in))
	inString := false
	escaped := false

	for i := 0; i < len(in); i++ {
		c := in[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(in) && (in[j] == ' ' || in[j] == '\t' || in[j] == '\r' || in[j] == '\n') {
				j++
			}
			if j < len(in) && (in[j] == '}' || in[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// launchArgs builds the wt.exe command line that opens a new window with the
// given profile and runs exe. --useApplicationTitle lets the TUI keep its own
// window title, which termwin uses to find and place the window.
func launchArgs(profile, exe string, extra []string) []string {
	args := []string{"-w", "-1", "new-tab", "--useApplicationTitle", "-p", profile, exe}
	return append(args, extra...)
}

// LaunchDefault opens a new Windows Terminal window with the default profile
// and runs exe. It reports false when Windows Terminal is not the default
// terminal or is unavailable, so the caller can use a normal console window.
func LaunchDefault(exe string, extra ...string) (bool, error) {
	profile, ok := profileFor(DefaultHost())
	if !ok {
		return false, nil
	}
	wtPath, ok := findWT()
	if !ok {
		return false, nil
	}
	if err := winproc.StartHidden(wtPath, launchArgs(profile, exe, extra)...); err != nil {
		return false, err
	}
	return true, nil
}

// findWT returns the Windows Terminal launcher. The app execution alias is the
// supported entry point; PATH is only a fallback for development shells.
func findWT() (string, bool) {
	if local, err := os.UserCacheDir(); err == nil {
		alias := filepath.Join(local, "Microsoft", "WindowsApps", "wt.exe")
		if info, err := os.Stat(alias); err == nil && !info.IsDir() {
			return alias, true
		}
	}
	if path, err := exec.LookPath("wt.exe"); err == nil {
		return path, true
	}
	return "", false
}
