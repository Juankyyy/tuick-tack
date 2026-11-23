package tui

// platformMacOS is true on macOS: the release publishes a single universal
// .app zip, so the update dialog has one download action and no type chooser.
const platformMacOS = true

// platformDownloadSuffix is removed when the TUI shows the version of a saved
// download.
const platformDownloadSuffix = "-macos-universal.zip"
