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
	"tuick-tack/internal/tui/copy"
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
	SetSoundName(string)
	SetVolume(int)
	PreviewSound()
	ToggleAutostart()
	Quit()
}

var volumeLevels = []int{0, 25, 50, 75, 100}

// Tray is the single notification-area controller.
type Tray struct {
	actions Actions

	mu            sync.Mutex
	ready         bool
	status        *systray.MenuItem
	start         *systray.MenuItem
	pause         *systray.MenuItem
	stop          *systray.MenuItem
	sound         *systray.MenuItem
	soundSettings *systray.MenuItem
	soundItems    map[string]*systray.MenuItem
	preview       *systray.MenuItem
	volume        *systray.MenuItem
	volumeDown    *systray.MenuItem
	volumeUp      *systray.MenuItem
	volumeItems   map[int]*systray.MenuItem
	startup       *systray.MenuItem
	quit          *systray.MenuItem
	children      []*systray.MenuItem

	lastStatus  engine.Status
	lastPresets []config.Preset
	soundOn     bool
	startupOn   bool
	haveChecks  bool

	soundNameOn     string
	volumeOn        int
	haveSoundChecks bool
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
	systray.SetIcon(assets.IdleIcon)

	t.status = systray.AddMenuItem(copy.T(copy.TrayReady), copy.T(copy.TrayCurrentTooltip))
	t.status.Disable()
	systray.AddSeparator()
	open := systray.AddMenuItem(copy.T(copy.TrayOpen), copy.T(copy.TrayOpenTooltip))
	t.start = systray.AddMenuItem(copy.T(copy.TrayStart), copy.T(copy.TrayStartTooltip))
	t.pause = systray.AddMenuItem(copy.T(copy.TrayPause), copy.T(copy.TrayPauseTooltip))
	t.stop = systray.AddMenuItem(copy.T(copy.TrayStop), copy.T(copy.TrayStopTooltip))
	t.stop.Disable()
	t.sound = systray.AddMenuItemCheckbox(copy.T(copy.TraySound), copy.T(copy.TraySoundTooltip), true)

	t.soundSettings = systray.AddMenuItem(copy.T(copy.TraySoundSettings), copy.T(copy.TraySoundSettingsTooltip))
	t.soundItems = make(map[string]*systray.MenuItem, len(config.Sounds))
	for _, name := range config.Sounds {
		name := name
		label := copy.SoundLabel(name)
		item := t.soundSettings.AddSubMenuItemCheckbox(label,
			copy.T(copy.TraySoundPresetTooltip, label), false)
		t.soundItems[name] = item
		go clicks(item, func() { t.actions.SetSoundName(name) })
	}
	t.soundSettings.AddSeparator()
	t.preview = t.soundSettings.AddSubMenuItem(copy.T(copy.TraySoundPreview), copy.T(copy.TraySoundPreviewTooltip))

	t.volume = systray.AddMenuItem(copy.T(copy.TrayVolume), copy.T(copy.TrayVolumeTooltip))
	t.volumeDown = t.volume.AddSubMenuItem(copy.T(copy.TrayVolumeDown), copy.T(copy.TrayVolumeDownTooltip))
	t.volumeUp = t.volume.AddSubMenuItem(copy.T(copy.TrayVolumeUp), copy.T(copy.TrayVolumeUpTooltip))
	t.volume.AddSeparator()
	t.volumeItems = make(map[int]*systray.MenuItem, len(volumeLevels))
	for _, level := range volumeLevels {
		level := level
		item := t.volume.AddSubMenuItemCheckbox(copy.T(copy.TrayVolumeLevel, level),
			copy.T(copy.TrayVolumeTooltip), false)
		t.volumeItems[level] = item
		go clicks(item, func() { t.actions.SetVolume(level) })
	}

	t.startup = systray.AddMenuItemCheckbox(copy.T(copy.TrayStartup), copy.T(copy.TrayStartupTooltip), false)
	systray.AddSeparator()
	t.quit = systray.AddMenuItem(copy.T(copy.TrayQuit), copy.T(copy.TrayQuitTooltip))
	t.ready = true
	t.mu.Unlock()

	go clicks(open, t.actions.OpenTUI)
	go clicks(t.pause, t.actions.TogglePause)
	go clicks(t.stop, t.actions.Stop)
	go clicks(t.sound, t.actions.ToggleSound)
	go clicks(t.preview, t.actions.PreviewSound)
	go clicks(t.volumeDown, func() { t.adjustVolume(-5) })
	go clicks(t.volumeUp, func() { t.adjustVolume(5) })
	go clicks(t.startup, t.actions.ToggleAutostart)
	go clicks(t.quit, t.actions.Quit)

	t.Refresh()
}

func (t *Tray) adjustVolume(delta int) {
	cfg := t.actions.Config()
	volume := cfg.Volume + delta
	if volume < config.MinVolume {
		volume = config.MinVolume
	}
	if volume > config.MaxVolume {
		volume = config.MaxVolume
	}
	t.actions.SetVolume(volume)
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
	t.updateChecksLocked(sound, startup, cfg.SoundName, cfg.Volume)
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
		t.pause.SetTitle(copy.T(copy.TrayResume))
	} else {
		t.pause.SetTitle(copy.T(copy.TrayPause))
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

func (t *Tray) updateChecksLocked(sound, startup bool, soundName string, volume int) {
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

	if !t.haveSoundChecks || t.soundNameOn != soundName {
		for name, item := range t.soundItems {
			if name == soundName {
				item.Check()
			} else {
				item.Uncheck()
			}
		}
		t.soundNameOn = soundName
	}
	if !t.haveSoundChecks || t.volumeOn != volume {
		for level, item := range t.volumeItems {
			if level == volume {
				item.Check()
			} else {
				item.Uncheck()
			}
		}
		t.volume.SetTitle(copy.T(copy.TrayVolumeValue, volume))
		if volume <= config.MinVolume {
			t.volumeDown.Disable()
		} else {
			t.volumeDown.Enable()
		}
		if volume >= config.MaxVolume {
			t.volumeUp.Disable()
		} else {
			t.volumeUp.Enable()
		}
		t.volumeOn = volume
	}
	t.haveSoundChecks = true
}

func (t *Tray) rebuildStartLocked(presets []config.Preset) {
	for _, item := range t.children {
		item.Remove()
	}
	t.children = nil
	for _, p := range presets {
		p := p
		item := t.start.AddSubMenuItem(presetMenuLabel(p), copy.T(copy.TrayStartPreset, p.Label))
		t.children = append(t.children, item)
		go clicks(item, func() { t.actions.StartPreset(p) })
	}
	custom := t.start.AddSubMenuItem(copy.T(copy.TrayCustom), copy.T(copy.TrayCustomTooltip))
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
