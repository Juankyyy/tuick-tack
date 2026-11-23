package tray

import (
	"tuick-tack/assets"
	"tuick-tack/internal/engine"
)

// iconFor returns the 32 px PNG for the menu bar. systray sizes it to 16 pt, so
// the Retina display uses the full bitmap.
func iconFor(status engine.Status) []byte {
	switch status {
	case engine.Running:
		return assets.RunningIconPNG
	case engine.Paused:
		return assets.PausedIconPNG
	case engine.Finished:
		return assets.FinishedIconPNG
	default:
		return assets.IdleIconPNG
	}
}
