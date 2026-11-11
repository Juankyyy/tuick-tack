// Package tray owns the Windows notification-area icon and its menu.
//
// systray is global and must run on the main goroutine. The tray pulls the
// current snapshot/config through Actions; the daemon calls Refresh on changes
// and once per second while a countdown runs.
package tray

import (
	"log"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"

	"tuick-tack/assets"
	"tuick-tack/internal/config"
	"tuick-tack/internal/duration"
	"tuick-tack/internal/engine"
)

// Actions is the daemon API used by menu items.
type Actions interface {
	Snapshot() engine.State
	Config() config.Config
	SoundEnabled() bool
	AutostartEnabled() bool
	OpenTUI()
	StartPreset(config.Preset)
	StartCustom()
	TogglePause()
	Stop()
	ToggleSound()
	ToggleAutostart()
	Quit()
}

// Tray is the single notification-area controller.
type Tray struct {
	actions Actions

	mu       sync.Mutex
	ready    bool
	status   *systray.MenuItem
	start    *systray.MenuItem
	pause    *systray.MenuItem
	stop     *systray.MenuItem
	sound    *systray.MenuItem
	startup  *systray.MenuItem
	quit     *systray.MenuItem
	children []*systray.MenuItem

	lastStatus  engine.Status
	lastPresets []config.Preset
	soundOn     bool
	startupOn   bool
	haveChecks  bool
}

// New returns a tray bound to actions.
func New(actions Actions) *Tray {
	return &Tray{actions: actions, lastStatus: engine.Idle}
}

// Run blocks on the systray event loop. Call it from the main goroutine.
func (t *Tray) Run() {
	systray.Run(t.onReady, func() { log.Println("tray: event loop exited") })
}

// Quit asks systray to exit. It is safe from any goroutine.
func (t *Tray) Quit() {
	systray.Quit()
}

func (t *Tray) onReady() {
	t.mu.Lock()
	if t.ready {
		t.mu.Unlock()
		return
	}
	systray.SetOnTapped(t.actions.OpenTUI)

	t.status = systray.AddMenuItem("Tuick-Tack · ready", "Current countdown")
	t.status.Disable()
	systray.AddSeparator()
	open := systray.AddMenuItem("Open Tuick-Tack", "Open the terminal UI")
	t.start = systray.AddMenuItem("Start", "Start a preset")
	t.pause = systray.AddMenuItem("Pause", "Pause the countdown")
	t.stop = systray.AddMenuItem("Stop", "Stop the countdown")
	t.stop.Disable()
	t.sound = systray.AddMenuItemCheckbox("Sound", "Play the chime at zero", true)
	t.startup = systray.AddMenuItemCheckbox("Start with Windows", "Launch the daemon at sign-in", false)
	systray.AddSeparator()
	t.quit = systray.AddMenuItem("Quit", "Quit Tuick-Tack")
	t.ready = true
	t.mu.Unlock()

	go clicks(open, t.actions.OpenTUI)
	go clicks(t.pause, t.actions.TogglePause)
	go clicks(t.stop, t.actions.Stop)
	go clicks(t.sound, t.actions.ToggleSound)
	go clicks(t.startup, t.actions.ToggleAutostart)
	go clicks(t.quit, t.actions.Quit)

	t.Refresh()
}

func clicks(item *systray.MenuItem, fn func()) {
	for range item.ClickedCh {
		fn()
	}
}

// Refresh repaints the menu from the daemon state. It is safe before the
// systray loop is ready; the call is ignored until onReady builds the menu.
func (t *Tray) Refresh() {
	if t == nil || t.actions == nil {
		return
	}
	st := t.actions.Snapshot()
	cfg := t.actions.Config()
	sound := t.actions.SoundEnabled()
	startup := t.actions.AutostartEnabled()

	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ready {
		return
	}
	t.updateLocked(st, cfg)
	t.updateChecksLocked(sound, startup)
}

func (t *Tray) updateLocked(st engine.State, cfg config.Config) {
	statusTitle, tooltip := statusText(st)
	t.status.SetTitle(statusTitle)
	systray.SetTooltip(tooltip)

	if st.Status != t.lastStatus {
		t.lastStatus = st.Status
		systray.SetIcon(iconFor(st.Status))
	}

	switch st.Status {
	case engine.Running, engine.Paused:
		t.pause.Enable()
	default:
		t.pause.Disable()
	}
	if st.Status == engine.Paused {
		t.pause.SetTitle("Resume")
	} else {
		t.pause.SetTitle("Pause")
	}
	if st.Status == engine.Idle {
		t.stop.Disable()
	} else {
		t.stop.Enable()
	}

	if !samePresets(t.lastPresets, cfg.Presets) {
		t.rebuildStartLocked(cfg.Presets)
		t.lastPresets = append([]config.Preset(nil), cfg.Presets...)
	}
}

func (t *Tray) updateChecksLocked(sound, startup bool) {
	if !t.haveChecks || t.soundOn != sound {
		if sound {
			t.sound.Check()
		} else {
			t.sound.Uncheck()
		}
		t.soundOn = sound
	}
	if !t.haveChecks || t.startupOn != startup {
		if startup {
			t.startup.Check()
		} else {
			t.startup.Uncheck()
		}
		t.startupOn = startup
	}
	t.haveChecks = true
}

func (t *Tray) rebuildStartLocked(presets []config.Preset) {
	for _, item := range t.children {
		item.Remove()
	}
	t.children = nil
	for _, p := range presets {
		p := p
		item := t.start.AddSubMenuItem(presetMenuLabel(p), "Start "+p.Label)
		t.children = append(t.children, item)
		go clicks(item, func() { t.actions.StartPreset(p) })
	}
	custom := t.start.AddSubMenuItem("Custom…", "Type a duration in the TUI")
	t.children = append(t.children, custom)
	go clicks(custom, t.actions.StartCustom)
}

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

func statusText(st engine.State) (menu, tooltip string) {
	label := strings.TrimSpace(st.Label)
	suffix := ""
	if label != "" {
		suffix = " (" + label + ")"
	}
	switch st.Status {
	case engine.Running:
		left := duration.Format(st.Remaining)
		return "Tuick-Tack · " + left + " remaining",
			"Tuick-Tack · " + left + " remaining" + suffix
	case engine.Paused:
		left := duration.Format(st.Remaining)
		return "Tuick-Tack · " + left + " paused",
			"Tuick-Tack · " + left + " paused" + suffix
	case engine.Finished:
		return "Tuick-Tack · time's up!", "Tuick-Tack · time's up!"
	default:
		return "Tuick-Tack · ready", "Tuick-Tack · ready"
	}
}

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
