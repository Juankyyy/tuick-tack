package notify

import "strings"

// appleScriptString quotes s for an AppleScript string literal. Newlines become
// spaces: they are legal inside a literal but make the -e argument hard to read
// in logs, and the notification UI flattens them anyway.
func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return `"` + s + `"`
}
