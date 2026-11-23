package tui

// platformMacOS is false on Windows: the update dialog keeps the Setup and
// Portable chooser and the downloaded file is an .exe.
const platformMacOS = false

// platformDownloadSuffix is removed when the TUI shows the version of a saved
// download.
const platformDownloadSuffix = ".exe"
