// Package instance owns the per-user locks that keep one daemon and one TUI
// per session. The updater also uses them to wait for the old processes to exit
// before it replaces or reinstalls the executable.
package instance
