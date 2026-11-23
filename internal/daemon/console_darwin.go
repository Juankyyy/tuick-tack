package daemon

// DetachConsole does nothing on macOS: the process owns no console to release
// and --hidden is accepted for command-line compatibility with Windows.
func DetachConsole() {}
