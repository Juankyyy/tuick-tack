// Package copy holds every user-facing string in the TUI. Widgets must not
// hardcode labels, hints, dialogs or toasts.
package copy

import (
	"fmt"

	"tuick-tack/internal/config"
)

// Key identifies a message.
type Key string

// UI strings. Keep them one-cell and English for v1.
const (
	Brand = Key("brand")

	StatusReady      = Key("status.ready")
	StatusRunning    = Key("status.running")
	StatusPaused     = Key("status.paused")
	StatusFinished   = Key("status.finished")
	StatusOffline    = Key("status.offline")
	StatusConnecting = Key("status.connecting")

	ReadyTitle       = Key("ready.title")
	ReadyMeta        = Key("ready.meta")
	InputPlaceholder = Key("input.placeholder")
	PausedMeta       = Key("paused.meta")
	RunningMeta      = Key("running.meta")
	FinishedTitle    = Key("finished.title")
	FinishedMeta     = Key("finished.meta")
	EndsAt           = Key("ends.at")
	TimeAt           = Key("time.at")
	HeaderOnHold     = Key("header.onhold")
	HeaderNoTimer    = Key("header.notimer")
	Completed        = Key("completed")

	ControlsIdle     = Key("controls.idle")
	ControlsRunning  = Key("controls.running")
	ControlsPaused   = Key("controls.paused")
	ControlsFinished = Key("controls.finished")
	CompactIdle      = Key("compact.idle")
	CompactRunning   = Key("compact.running")
	CompactPaused    = Key("compact.paused")
	CompactFinished  = Key("compact.finished")
	CompactStopHint  = Key("compact.stop.hint")
	HeroFinished     = Key("hero.finished")
	LabelFocus       = Key("label.focus")
	RepeatHint       = Key("hint.repeat")
	CloseHint        = Key("hint.close")
	HelpFormats      = Key("help.formats")
	HelpFinished     = Key("help.finished")
	HelpSound        = Key("help.sound")
	HelpPresets      = Key("help.presets")

	SoundTitle      = Key("sound.title")
	SoundOn         = Key("sound.on")
	SoundOff        = Key("sound.off")
	SoundVolume     = Key("sound.volume")
	SoundPreview    = Key("sound.preview")
	SoundMute       = Key("sound.mute")
	SoundUnmute     = Key("sound.unmute")
	SoundHint       = Key("sound.hint")
	SoundActionHint = Key("sound.action.hint")
	SoundChime      = Key("sound.chime")
	SoundMarimba    = Key("sound.marimba")
	SoundDigital    = Key("sound.digital")
	SoundDeep       = Key("sound.deep")

	Pause    = Key("action.pause")
	Resume   = Key("action.resume")
	PlusMin  = Key("action.plus")
	MinusMin = Key("action.minus")
	Cancel   = Key("action.cancel")
	Add      = Key("action.add")
	Edit     = Key("action.edit")
	Delete   = Key("action.delete")
	Restore  = Key("action.restore")
	Close    = Key("action.close")
	Start    = Key("action.start")
	Repeat   = Key("action.repeat")
	Quit     = Key("action.quit")
	Yes      = Key("action.yes")
	No       = Key("action.no")
	Save     = Key("action.save")
	Back     = Key("action.back")

	HelpTitle = Key("help.title")
	HelpIdle  = Key("help.idle")
	HelpTimer = Key("help.timer")
	HelpModal = Key("help.modal")
	HelpClose = Key("help.close")

	PresetsTitle      = Key("presets.title")
	PresetsEmpty      = Key("presets.empty")
	PresetsHint       = Key("presets.hint")
	PresetsActionHint = Key("presets.action.hint")
	PresetNewTitle    = Key("preset.new")
	PresetEditTitle   = Key("preset.edit")
	PresetName        = Key("preset.name")
	PresetDuration    = Key("preset.duration")
	PresetFormHint    = Key("preset.formhint")
	PresetDeleteTitle = Key("preset.delete")

	StopTitle = Key("confirm.stop")

	Disconnected = Key("conn.disconnected")
	Connecting   = Key("conn.connecting")
	DaemonOnline = Key("conn.online")
	DaemonLost   = Key("conn.lost")

	ErrorDuration = Key("error.duration")
	ErrorLabel    = Key("error.label")
	ErrorPresets  = Key("error.presets")
	ErrorNoPreset = Key("error.nopreset")
	ErrorOffline  = Key("error.offline")

	ToastAdded    = Key("toast.added")
	ToastUpdated  = Key("toast.updated")
	ToastDeleted  = Key("toast.deleted")
	ToastRestored = Key("toast.restored")
	ToastTheme    = Key("toast.theme")
	ToastSoundOn  = Key("toast.sound.on")
	ToastSoundOff = Key("toast.sound.off")
	ToastReplaced = Key("toast.replaced")

	Minutes = Key("unit.minutes")

	ExitActive       = Key("exit.active")
	AlreadyOpen      = Key("instance.already")
	StartupInstalled = Key("startup.installed")
	StartupRemoved   = Key("startup.removed")

	TrayReady                = Key("tray.ready")
	TrayRunning              = Key("tray.running")
	TrayPaused               = Key("tray.paused")
	TrayFinished             = Key("tray.finished")
	TrayRunningTooltip       = Key("tray.running.tooltip")
	TrayPausedTooltip        = Key("tray.paused.tooltip")
	TrayOpen                 = Key("tray.open")
	TrayOpenTooltip          = Key("tray.open.tooltip")
	TrayStart                = Key("tray.start")
	TrayStartTooltip         = Key("tray.start.tooltip")
	TrayStartPreset          = Key("tray.start.preset")
	TrayCustom               = Key("tray.custom")
	TrayCustomTooltip        = Key("tray.custom.tooltip")
	TrayPause                = Key("tray.pause")
	TrayResume               = Key("tray.resume")
	TrayPauseTooltip         = Key("tray.pause.tooltip")
	TrayStop                 = Key("tray.stop")
	TrayStopTooltip          = Key("tray.stop.tooltip")
	TraySound                = Key("tray.sound")
	TraySoundTooltip         = Key("tray.sound.tooltip")
	TraySoundSettings        = Key("tray.sound.settings")
	TraySoundSettingsTooltip = Key("tray.sound.settings.tooltip")
	TraySoundPresetTooltip   = Key("tray.sound.preset.tooltip")
	TraySoundPreview         = Key("tray.sound.preview")
	TraySoundPreviewTooltip  = Key("tray.sound.preview.tooltip")
	TrayVolume               = Key("tray.volume")
	TrayVolumeTooltip        = Key("tray.volume.tooltip")
	TrayVolumeValue          = Key("tray.volume.value")
	TrayVolumeLevel          = Key("tray.volume.level")
	TrayVolumeDown           = Key("tray.volume.down")
	TrayVolumeDownTooltip    = Key("tray.volume.down.tooltip")
	TrayVolumeUp             = Key("tray.volume.up")
	TrayVolumeUpTooltip      = Key("tray.volume.up.tooltip")
	TrayStartup              = Key("tray.startup")
	TrayStartupTooltip       = Key("tray.startup.tooltip")
	TrayQuit                 = Key("tray.quit")
	TrayQuitTooltip          = Key("tray.quit.tooltip")
	TrayCurrentTooltip       = Key("tray.current.tooltip")
)

var texts = map[Key]string{
	Brand: "tuick·tack",

	StatusReady:      "ready",
	StatusRunning:    "running",
	StatusPaused:     "PAUSED",
	StatusFinished:   "time's up!",
	StatusOffline:    "offline",
	StatusConnecting: "connecting",

	ReadyTitle:       "Ready when you are",
	ReadyMeta:        "Type a duration or pick a preset",
	InputPlaceholder: "25 · 12:30 · 90s · 1h30m",
	PausedMeta:       "%s remaining · [Space] resume",
	RunningMeta:      "%s remaining",
	FinishedTitle:    "%s completed",
	FinishedMeta:     "[r] repeat · [Enter] close",
	EndsAt:           "ends at %s",
	TimeAt:           "at %s",
	HeaderOnHold:     "on hold",
	HeaderNoTimer:    "no countdown",
	Completed:        "%s completed",

	ControlsIdle:     "[p] presets · [t] theme · [v] sound · [s] mute · [?] help · [q] quit",
	ControlsRunning:  "[Space] pause · [Esc] cancel · [+/-] 1 min · [v] sound · [q] quit",
	ControlsPaused:   "[Space] resume · [Esc] cancel · [+/-] 1 min · [v] sound · [q] quit",
	ControlsFinished: "[v] sound · [?] help · [q] quit",
	CompactIdle:      "[Enter] start · [←→] preset · [q] quit",
	CompactRunning:   "[Space] pause · [Esc] cancel · [q] quit",
	CompactPaused:    "[Space] resume · [Esc] cancel · [q] quit",
	CompactFinished:  "[r] repeat · [Enter] close · [q] quit",
	CompactStopHint:  "[Enter] yes · [Esc] no",
	HeroFinished:     "TIME'S UP!",
	LabelFocus:       "Focus",
	RepeatHint:       "r repeat",
	CloseHint:        "enter close",
	HelpFormats:      "25 min · 12:30 · 90s · 1h30m",
	HelpFinished:     "finished: [r] repeat · [Enter] close",
	HelpSound:        "sound: [v] settings · [s] mute · [Space] preview",
	HelpPresets:      "presets: [a] add · [e] edit · [d] delete · [r] defaults",

	SoundTitle:      "Sound",
	SoundOn:         "Sound on",
	SoundOff:        "Sound off",
	SoundVolume:     "Volume",
	SoundPreview:    "preview",
	SoundMute:       "mute",
	SoundUnmute:     "unmute",
	SoundHint:       "[↑↓] sound · [←→] volume",
	SoundActionHint: "[Space] preview · [m] mute · [Esc] close",
	SoundChime:      "Chime",
	SoundMarimba:    "Marimba",
	SoundDigital:    "Digital",
	SoundDeep:       "Deep",

	Pause:    "pause",
	Resume:   "resume",
	PlusMin:  "+1 min",
	MinusMin: "-1 min",
	Cancel:   "cancel",
	Add:      "add",
	Edit:     "edit",
	Delete:   "delete",
	Restore:  "defaults",
	Close:    "close",
	Start:    "start",
	Repeat:   "repeat",
	Quit:     "quit",
	Yes:      "yes",
	No:       "no",
	Save:     "save",
	Back:     "back",

	HelpTitle: "Help",
	HelpIdle:  "type a duration · [Enter] start · [←→] select preset",
	HelpTimer: "[Space] pause/resume · [Esc] cancel · [+/-] adjust 1 min",
	HelpModal: "[p] presets · [t] theme · [v] sound · [?] help · [q] quit",
	HelpClose: "[Esc] or [Enter] to close",

	PresetsTitle:      "Presets",
	PresetsEmpty:      "No presets yet. Press [a] to add one.",
	PresetsHint:       "[↑↓] move · [Tab] buttons · [Enter] start · [Esc] close",
	PresetsActionHint: "[←→] choose · [Enter] run · [Tab] back · [Esc] close",
	PresetNewTitle:    "New preset",
	PresetEditTitle:   "Edit preset",
	PresetName:        "Name",
	PresetDuration:    "Duration",
	PresetFormHint:    "[Tab] field · [Enter] save · [Esc] cancel",
	PresetDeleteTitle: "Delete “%s”?",

	StopTitle: "Stop the countdown?",

	Disconnected: "Daemon disconnected — retrying…",
	Connecting:   "Connecting to the daemon…",
	DaemonOnline: "Connected",
	DaemonLost:   "Connection lost — retrying…",

	ErrorDuration: "Use 25, 12:30, 90s or 1h30m",
	ErrorLabel:    "Name must be 1-32 characters",
	ErrorPresets:  "You can keep up to 16 presets",
	ErrorNoPreset: "No preset selected",
	ErrorOffline:  "The daemon is offline",

	ToastAdded:    "“%s” added",
	ToastUpdated:  "“%s” updated",
	ToastDeleted:  "Preset deleted",
	ToastRestored: "Default presets restored",
	ToastTheme:    "%s theme",
	ToastSoundOn:  "Sound on",
	ToastSoundOff: "Sound off",
	ToastReplaced: "Countdown replaced",

	Minutes: "min",

	ExitActive: "The countdown is still running in the tray.",

	AlreadyOpen: "Tuick-Tack is already open.",

	StartupInstalled: "Tuick-Tack will start with Windows.",
	StartupRemoved:   "Tuick-Tack will no longer start with Windows.",

	TrayReady:                "Tuick-Tack · ready",
	TrayRunning:              "Tuick-Tack · %s remaining",
	TrayPaused:               "Tuick-Tack · %s paused",
	TrayFinished:             "Tuick-Tack · time's up!",
	TrayRunningTooltip:       "Tuick-Tack · %s remaining (%s)",
	TrayPausedTooltip:        "Tuick-Tack · %s paused (%s)",
	TrayOpen:                 "Open Tuick-Tack",
	TrayOpenTooltip:          "Open the terminal UI",
	TrayStart:                "Start",
	TrayStartTooltip:         "Start a preset",
	TrayStartPreset:          "Start %s",
	TrayCustom:               "Custom…",
	TrayCustomTooltip:        "Type a duration in the TUI",
	TrayPause:                "Pause",
	TrayResume:               "Resume",
	TrayPauseTooltip:         "Pause the countdown",
	TrayStop:                 "Stop",
	TrayStopTooltip:          "Stop the countdown",
	TraySound:                "Sound",
	TraySoundTooltip:         "Play the selected sound at zero",
	TraySoundSettings:        "Sound settings",
	TraySoundSettingsTooltip: "Choose the completion sound",
	TraySoundPresetTooltip:   "Use %s at zero",
	TraySoundPreview:         "Preview",
	TraySoundPreviewTooltip:  "Play the selected sound",
	TrayVolume:               "Volume",
	TrayVolumeTooltip:        "Completion sound volume",
	TrayVolumeValue:          "Volume %d%%",
	TrayVolumeLevel:          "%d%%",
	TrayVolumeDown:           "-5%",
	TrayVolumeDownTooltip:    "Lower the volume by 5%",
	TrayVolumeUp:             "+5%",
	TrayVolumeUpTooltip:      "Raise the volume by 5%",
	TrayStartup:              "Start with Windows",
	TrayStartupTooltip:       "Launch the daemon at sign-in",
	TrayQuit:                 "Quit",
	TrayQuitTooltip:          "Quit Tuick-Tack",
	TrayCurrentTooltip:       "Current countdown",
}

// SoundLabel returns the user-facing label for a stable sound ID.
func SoundLabel(id string) string {
	switch id {
	case config.SoundChime:
		return T(SoundChime)
	case config.SoundMarimba:
		return T(SoundMarimba)
	case config.SoundDigital:
		return T(SoundDigital)
	case config.SoundDeep:
		return T(SoundDeep)
	default:
		return id
	}
}

// T returns the localized message. Format arguments are applied with
// fmt.Sprintf; no arguments returns the raw string.
func T(key Key, args ...any) string {
	s, ok := texts[key]
	if !ok {
		return string(key)
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}
