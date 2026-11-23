package tray

import (
	"tuick-tack/assets"
	"tuick-tack/internal/engine"
)

// iconFor returns the notification-area icon for a status. Windows systray
// loads the 2x HICON from the multi-size ICO pack.
func iconFor(status engine.Status) []byte {
	switch status {
	case engine.Running:
		return assets.RunningIcon
	case engine.Paused:
		return assets.PausedIcon
	case engine.Finished:
		return assets.FinishedIcon
	default:
		return assets.IdleIcon
	}
}
