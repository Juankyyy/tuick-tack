package daemon

import "strings"

// tuiCommandName is the script Terminal opens for the TUI on macOS. A .command
// file is a normal executable, so macOS runs it without asking for Automation
// access to Terminal (osascript would trigger the TCC prompt).
const tuiCommandName = "open-tui.command"

// tuiCommandScript is the body of the .command file. The executable is shell
// quoted because an .app path can contain spaces or apostrophes.
func tuiCommandScript(exe string) string {
	return "#!/bin/sh\nexec " + shellQuote(exe) + " " + tuiLaunchMarker + "\n"
}

// shellQuote wraps s in single quotes for /bin/sh. A single quote inside s is
// closed, escaped and reopened.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
