// Package tray owns the notification-area or menu-bar icon and its menu.
//
// systray is global and must run on the main goroutine. The tray pulls the
// current snapshot/config through Actions; the daemon calls Refresh on changes
// and once per second while a countdown runs. macOS without cgo has no systray,
// so a no-op implementation keeps the daemon running headless.
package tray

import (
	"strings"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/duration"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/tui/copy"
)

// Actions is the daemon API used by menu items.
type Actions interface {
	Snapshot() engine.State
	Config() config.Config
	SoundEnabled() bool
	AutostartEnabled() bool
	MiniTimerEnabled() bool
	UpdateAvailable() (latest, url string, ok bool)
	// UpdateCheck returns idle, checking, up_to_date or failed plus the current
	// version for the menu label.
	UpdateCheck() (state, current string)
	OpenTUI()
	StartPreset(config.Preset)
	StartCustom()
	TogglePause()
	Stop()
	ToggleSound()
	SetSoundName(string)
	SetVolume(int)
	PreviewSound()
	ToggleAutostart()
	ToggleMiniTimer()
	OpenUpdate()
	CheckUpdates()
	Quit()
}

var volumeLevels = []int{0, 25, 50, 75, 100}

func samePresets(a, b []config.Preset) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func presetMenuLabel(p config.Preset) string {
	short := duration.Short(time.Duration(p.Seconds) * time.Second)
	label := strings.TrimSpace(p.Label)
	if label == "" || strings.EqualFold(label, short) {
		return short
	}
	return label + " · " + short
}

// updateMenuLabel is the tray entry for a newer release.
func updateMenuLabel(latest string) string {
	return copy.T(copy.TrayUpdateAvailable, latest)
}

// updateCheckMenu renders the manual check entry for the daemon state.
func updateCheckMenu(state, current string) (label string, enabled bool) {
	switch state {
	case "checking":
		return copy.T(copy.TrayUpdateChecking), false
	case "up_to_date":
		return copy.T(copy.TrayUpdateUpToDate, current), true
	case "failed":
		return copy.T(copy.TrayUpdateFailed), true
	default:
		return copy.T(copy.TrayUpdateCheck), true
	}
}

func statusText(st engine.State) (menu, tooltip string) {
	label := strings.TrimSpace(st.Label)
	switch st.Status {
	case engine.Running:
		left := duration.Format(st.Remaining)
		if label != "" {
			return copy.T(copy.TrayRunning, left), copy.T(copy.TrayRunningTooltip, left, label)
		}
		return copy.T(copy.TrayRunning, left), copy.T(copy.TrayRunning, left)
	case engine.Paused:
		left := duration.Format(st.Remaining)
		if label != "" {
			return copy.T(copy.TrayPaused, left), copy.T(copy.TrayPausedTooltip, left, label)
		}
		return copy.T(copy.TrayPaused, left), copy.T(copy.TrayPaused, left)
	case engine.Finished:
		return copy.T(copy.TrayFinished), copy.T(copy.TrayFinished)
	default:
		return copy.T(copy.TrayReady), copy.T(copy.TrayReady)
	}
}

// menuBarTitle is the compact text next to the menu-bar icon on macOS: the
// remaining time while a countdown runs or is paused. Windows ignores it, so a
// countdown never widens the notification area.
func menuBarTitle(st engine.State) string {
	switch st.Status {
	case engine.Running, engine.Paused:
		return duration.Format(st.Remaining)
	}
	return ""
}
