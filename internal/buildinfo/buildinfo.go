// Package buildinfo exposes version metadata for both executables.
package buildinfo

// Version is the application version. Release builds override it with
// -ldflags "-X tuick-tack/internal/buildinfo.Version=vX.Y.Z".
var Version = "dev"

// Name is the product name used in window titles, logs and menus.
const Name = "Tuick-Tack"

// Protocol is the IPC protocol version spoken by the TUI and the daemon.
const Protocol = 1
