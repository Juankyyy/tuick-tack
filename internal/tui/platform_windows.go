package tui

// platformMacOS is false on Windows: the update dialog keeps the Setup and
// Portable chooser and the downloaded file is an .exe.
const platformMacOS = false

// platformMiniTimer is true on Windows: the taskbar overlay exists and the TUI
// can toggle it with M.
const platformMiniTimer = true

// platformDownloadSuffix is removed when the TUI shows the version of a saved
// download.
const platformDownloadSuffix = ".exe"
