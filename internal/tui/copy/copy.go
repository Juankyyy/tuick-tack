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
	UpdateAvailable  = Key("update.available")
	UpdateShort      = Key("update.short")
	UpdateArrow      = Key("update.arrow")
	UpdateChecking   = Key("update.checking")
	UpdateHint       = Key("hint.update")
	UpdateHintNew    = Key("hint.update.new")

	UpdateFromTo             = Key("update.fromto")
	UpdateTypeSetup          = Key("update.type.setup")
	UpdateTypePortable       = Key("update.type.portable")
	UpdateStatusInstalled    = Key("update.status.installed")
	UpdateStatusNotInstalled = Key("update.status.notinstalled")
	UpdateStatusInUse        = Key("update.status.inuse")
	UpdateStatusNotInUse     = Key("update.status.notinuse")
	UpdateStatusNotInRelease = Key("update.status.notinrelease")
	UpdateOtherShort         = Key("update.other.short")
	UpdateCurrentSetup       = Key("update.current.setup")
	UpdateCurrentPortable    = Key("update.current.portable")
	UpdateInstallShort       = Key("update.install.short")
	UpdateInstalledBadge     = Key("update.installed")
	UpdatePortableBadge      = Key("update.portable")
	UpdateInstall            = Key("update.install")
	UpdateDownloadPortable   = Key("update.download.portable")
	UpdateInstallSetup       = Key("update.install.setup")
	UpdateReleasePage        = Key("update.release")
	UpdateDownloadShort      = Key("update.download.short")
	UpdateReleaseShort       = Key("update.release.short")
	UpdateLater              = Key("update.later")
	UpdateSetupNote          = Key("update.setup.note")
	UpdatePortableNote       = Key("update.portable.note")
	UpdateChooseInstalled    = Key("update.choose.installed")
	UpdateChoosePortable     = Key("update.choose.portable")
	UpdateTimerTitle         = Key("update.timer.title")
	UpdateTimerBody          = Key("update.timer.body")
	UpdateTimerWarn          = Key("update.timer.warn")
	UpdateTimerPortable      = Key("update.timer.portable")
	UpdateContinue           = Key("update.continue")
	UpdateBack               = Key("update.back")
	UpdateDownloading        = Key("update.downloading")
	UpdateVerifying          = Key("update.verifying")
	UpdateInstalling         = Key("update.installing")
	UpdateRestarting         = Key("update.restarting")
	UpdateInProgress         = Key("update.inprogress")
	UpdatePortableTitle      = Key("update.portable.title")
	UpdatePortableSaved      = Key("update.portable.saved")
	UpdatePortablePath       = Key("update.portable.path")
	UpdatePortableDone       = Key("update.portable.done")
	UpdateOpenFolder         = Key("update.open.folder")
	UpdateFailed             = Key("update.failed")
	UpdateRetry              = Key("update.retry")
	UpdateUnavailable        = Key("update.unavailable")
	UpdateNoAssets           = Key("update.no.assets")
	UpdateTimerActive        = Key("update.timer.active")
	UpdateNetwork            = Key("update.network")
	UpdateChecksum           = Key("update.checksum")
	UpdateWrite              = Key("update.write")
	UpdateBusy               = Key("update.busy")
	UpdateSetupFailed        = Key("update.setup.failed")
	UpdateSize               = Key("update.size")
	UpdatePreparing          = Key("update.preparing")
	UpdateWait               = Key("update.wait")
	UpdateStepsSetup         = Key("update.steps.setup")
	UpdateStepsPortable      = Key("update.steps.portable")
	UpdateNotesTitle         = Key("update.notes.title")
	UpdateHintSelect         = Key("update.hint.select")
	UpdateHintTab            = Key("update.hint.tab")
	UpdateHintNotes          = Key("update.hint.notes")
	UpdateHintGithub         = Key("update.hint.github")
	UpdateHintLater          = Key("update.hint.later")
	UpdateHintWarn           = Key("update.hint.warn")
	UpdateHintPortable       = Key("update.hint.portable")
	UpdateHintFailed         = Key("update.hint.failed")

	NotifyFoundTitle    = Key("notify.found.title")
	NotifyFoundBody     = Key("notify.found.body")
	NotifyUpToDateTitle = Key("notify.uptodate.title")
	NotifyUpToDateBody  = Key("notify.uptodate.body")
	NotifyFailedTitle   = Key("notify.failed.title")
	NotifyFailedBody    = Key("notify.failed.body")

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
	HeroDone         = Key("hero.finished.done")
	HeroFinishedWord = Key("hero.finished.finished")
	HeroTimeOut      = Key("hero.finished.timeout")
	HeroTheEnd       = Key("hero.finished.theend")
	HeroBingo        = Key("hero.finished.bingo")
	LabelFocus       = Key("label.focus")
	RepeatHint       = Key("hint.repeat")
	CloseHint        = Key("hint.close")

	StatsTitle       = Key("stats.title")
	StatsOverview    = Key("stats.overview")
	StatsActivity    = Key("stats.activity")
	StatsPresets     = Key("stats.presets")
	StatsRecent      = Key("stats.recent")
	StatsRange7      = Key("stats.range.7")
	StatsRange30     = Key("stats.range.30")
	StatsRange90     = Key("stats.range.90")
	StatsRangeAll    = Key("stats.range.all")
	StatsLoading     = Key("stats.loading")
	StatsUnavailable = Key("stats.unavailable")
	StatsFocused     = Key("stats.focused")
	StatsSessions    = Key("stats.sessions")
	StatsCompleted   = Key("stats.completed")
	StatsVsPrev      = Key("stats.vs.prev")
	StatsStreak      = Key("stats.streak")
	StatsBestStreak  = Key("stats.best.streak")
	StatsLongest     = Key("stats.longest")
	StatsBusiest     = Key("stats.busiest")
	StatsFocusPerDay = Key("stats.focus.day")
	StatsMax         = Key("stats.max")
	StatsHeatLess    = Key("stats.heat.less")
	StatsHeatMore    = Key("stats.heat.more")
	StatsByHour      = Key("stats.by.hour")
	StatsByWeekday   = Key("stats.by.weekday")
	StatsInsight     = Key("stats.insight")
	StatsWeeks       = Key("stats.weeks")
	StatsPresetsHead = Key("stats.presets.header")
	StatsRuns        = Key("stats.runs")
	StatsMostUsed    = Key("stats.most.used")
	StatsAvgSession  = Key("stats.avg")
	StatsWhen        = Key("stats.when")
	StatsPlanned     = Key("stats.planned")
	StatsLabel       = Key("stats.label")
	StatsOutcome     = Key("stats.outcome")
	StatsDone        = Key("stats.done")
	StatsStopped     = Key("stats.stopped")
	StatsReplaced    = Key("stats.replaced")
	StatsToday       = Key("stats.today")
	StatsYesterday   = Key("stats.yesterday")
	StatsShowing     = Key("stats.showing")
	StatsEmptyTitle  = Key("stats.empty.title")
	StatsEmptyBody   = Key("stats.empty.body")
	StatsHint        = Key("stats.hint")
	StatsHintShort   = Key("stats.hint.short")

	WeekdayMon = Key("weekday.mon")
	WeekdayTue = Key("weekday.tue")
	WeekdayWed = Key("weekday.wed")
	WeekdayThu = Key("weekday.thu")
	WeekdayFri = Key("weekday.fri")
	WeekdaySat = Key("weekday.sat")
	WeekdaySun = Key("weekday.sun")

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

	HelpSectionTimer    = Key("help.section.timer")
	HelpSectionDuration = Key("help.section.duration")
	HelpSectionPanels   = Key("help.section.panels")
	HelpSectionDisplay  = Key("help.section.display")
	HelpSectionApp      = Key("help.section.app")

	HelpStart        = Key("help.start")
	HelpPause        = Key("help.pause")
	HelpCancel       = Key("help.cancel")
	HelpAdjust       = Key("help.adjust")
	HelpRepeat       = Key("help.repeat")
	HelpFormats      = Key("help.formats")
	HelpPickPreset   = Key("help.pick.preset")
	HelpPresets      = Key("help.presets")
	HelpSound        = Key("help.sound")
	HelpMute         = Key("help.mute")
	HelpStats        = Key("help.stats")
	HelpUpdate       = Key("help.update")
	HelpTheme        = Key("help.theme")
	HelpBackground   = Key("help.background")
	HelpReadable     = Key("help.readable")
	HelpReadableNote = Key("help.readable.note")
	HelpQuit         = Key("help.quit")
	HelpTrayNote     = Key("help.tray.note")
	HelpClose        = Key("help.close")

	// The compact help used below the wide two-column layout.
	HelpNarrowStartPause   = Key("help.narrow.startpause")
	HelpNarrowCancelAdjust = Key("help.narrow.canceladjust")
	HelpNarrowRepeat       = Key("help.narrow.repeat")
	HelpNarrowPanels       = Key("help.narrow.panels")
	HelpNarrowPanelsMore   = Key("help.narrow.panels.more")
	HelpNarrowDisplay      = Key("help.narrow.display")
	HelpNarrowReadable     = Key("help.narrow.readable")
	HelpNarrowQuit         = Key("help.narrow.quit")

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

	ErrorDuration   = Key("error.duration")
	ErrorLabel      = Key("error.label")
	ErrorPresets    = Key("error.presets")
	ErrorNoPreset   = Key("error.nopreset")
	ErrorOffline    = Key("error.offline")
	ErrorOpenURL    = Key("error.openurl")
	ErrorOpenFolder = Key("error.openfolder")
	ErrorUpdate     = Key("error.update.check")

	ToastAdded       = Key("toast.added")
	ToastUpdated     = Key("toast.updated")
	ToastDeleted     = Key("toast.deleted")
	ToastRestored    = Key("toast.restored")
	ToastTheme       = Key("toast.theme")
	ToastBackground  = Key("toast.background")
	ToastReadableOn  = Key("toast.readable.on")
	ToastReadableOff = Key("toast.readable.off")
	ToastSoundOn     = Key("toast.sound.on")
	ToastSoundOff    = Key("toast.sound.off")
	ToastReplaced    = Key("toast.replaced")
	ToastUpdateFound = Key("toast.update.found")
	ToastUpToDate    = Key("toast.update.uptodate")

	BackgroundAurora    = Key("background.aurora")
	BackgroundStars     = Key("background.stars")
	BackgroundFireflies = Key("background.fireflies")
	BackgroundRain      = Key("background.rain")
	BackgroundWaves     = Key("background.waves")
	BackgroundGrid      = Key("background.grid")
	BackgroundNone      = Key("background.none")

	Minutes = Key("unit.minutes")

	ExitActive              = Key("exit.active")
	ExitDaemonClosing       = Key("exit.daemon.closing")
	ExitUpdating            = Key("exit.updating")
	ToastPortable           = Key("toast.update.portable")
	NotifyUpdateFailedTitle = Key("notify.update.failed.title")
	NotifyUpdateFailedBody  = Key("notify.update.failed.body")
	AlreadyOpen             = Key("instance.already")
	StartupInstalled        = Key("startup.installed")
	StartupRemoved          = Key("startup.removed")

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
	TrayUpdateAvailable      = Key("tray.update.available")
	TrayUpdateTooltip        = Key("tray.update.tooltip")
	TrayUpdateCheck          = Key("tray.update.check")
	TrayUpdateCheckTooltip   = Key("tray.update.check.tooltip")
	TrayUpdateChecking       = Key("tray.update.checking")
	TrayUpdateUpToDate       = Key("tray.update.uptodate")
	TrayUpdateFailed         = Key("tray.update.failed")
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
	UpdateAvailable:  "Update available",
	UpdateShort:      "Update",
	UpdateArrow:      "→",
	UpdateChecking:   "Checking for updates…",
	UpdateHint:       "[u] check for updates",
	UpdateHintNew:    "[u] update to %s",

	UpdateFromTo:             "%s → %s",
	UpdateTypeSetup:          "Setup",
	UpdateTypePortable:       "Portable",
	UpdateStatusInstalled:    "Installed",
	UpdateStatusNotInstalled: "Not installed",
	UpdateStatusInUse:        "In use",
	UpdateStatusNotInUse:     "Not in use",
	UpdateStatusNotInRelease: "Not available",
	UpdateOtherShort:         "This is not the copy you use; settings carry over.",
	UpdateCurrentSetup:       "Updates this copy.",
	UpdateCurrentPortable:    "Saves the new .exe to Downloads.",
	UpdateInstallShort:       "install",
	UpdateInstalledBadge:     "Installed · %s",
	UpdatePortableBadge:      "Portable · tuick-tack.exe",
	UpdateInstall:            "install update",
	UpdateDownloadPortable:   "download portable",
	UpdateInstallSetup:       "install with setup",
	UpdateReleasePage:        "release page",
	UpdateDownloadShort:      "download",
	UpdateReleaseShort:       "github",
	UpdateLater:              "later",
	UpdateSetupNote:          "Install runs the setup and reopens Tuick-Tack.",
	UpdatePortableNote:       "Download saves the portable .exe to Downloads.",
	UpdateChooseInstalled:    "Install updates this copy. Download saves the portable .exe.",
	UpdateChoosePortable:     "Download saves the .exe. Install adds a Documents copy.",
	UpdateTimerTitle:         "Countdown active",
	UpdateTimerBody:          "Installing the update restarts Tuick-Tack.",
	UpdateTimerWarn:          "The countdown (%s) will be lost.",
	UpdateTimerPortable:      "The portable download does not restart the app.",
	UpdateContinue:           "continue",
	UpdateBack:               "back",
	UpdateDownloading:        "Downloading…",
	UpdateVerifying:          "Verifying…",
	UpdateInstalling:         "Installing… Tuick-Tack will restart.",
	UpdateRestarting:         "Installing update…",
	UpdateInProgress:         "Update in progress",
	UpdatePortableTitle:      "Portable update",
	UpdatePortableSaved:      "Portable update saved",
	UpdatePortablePath:       "Saved to %s",
	UpdatePortableDone:       "The current version keeps running. Replace it when convenient.",
	UpdateOpenFolder:         "open folder",
	UpdateFailed:             "Update failed",
	UpdateRetry:              "try again",
	UpdateUnavailable:        "Update install is not available",
	UpdateNoAssets:           "This release has no downloadable files",
	UpdateTimerActive:        "Confirm the restart to continue",
	UpdateNetwork:            "Could not download the update",
	UpdateChecksum:           "The download failed verification",
	UpdateWrite:              "Could not save the update",
	UpdateBusy:               "Another update is already running",
	UpdateSetupFailed:        "The installer did not finish",
	UpdateSize:               "%s of %s",
	UpdatePreparing:          "Preparing…",
	UpdateWait:               "This may take a moment.",
	UpdateStepsSetup:         "Download · Verify · Install · Reopen",
	UpdateStepsPortable:      "Download · Verify · Done",
	UpdateNotesTitle:         "What's new in %s",
	UpdateHintSelect:         "[Enter] select",
	UpdateHintTab:            "[Tab] type",
	UpdateHintNotes:          "[↑/↓] notes",
	UpdateHintGithub:         "[g] github",
	UpdateHintLater:          "[Esc] later",
	UpdateHintWarn:           "[Enter] continue · [Esc] back",
	UpdateHintPortable:       "[o] open folder · [Enter] close",
	UpdateHintFailed:         "[Enter] try again · [g] github · [Esc] close",
	NotifyFoundTitle:         "Update available",
	NotifyFoundBody:          "Version %s is available. Click to open the release page.",
	NotifyUpToDateTitle:      "You're up to date",
	NotifyUpToDateBody:       "You have the latest version (%s).",
	NotifyFailedTitle:        "Update check failed",
	NotifyFailedBody:         "Could not reach GitHub. Try again later.",

	ControlsIdle:     "[p] presets · [t] theme · [v] sound · [s] stats · [?] help · [q] quit",
	ControlsRunning:  "[Space] pause · [Esc] cancel · [+/-] 1 min · [v] sound · [s] stats",
	ControlsPaused:   "[Space] resume · [Esc] cancel · [+/-] 1 min · [v] sound · [s] stats",
	ControlsFinished: "[v] sound · [s] stats · [?] help · [q] quit",
	CompactIdle:      "[Enter] start · [←→] preset · [q] quit",
	CompactRunning:   "[Space] pause · [Esc] cancel · [q] quit",
	CompactPaused:    "[Space] resume · [Esc] cancel · [q] quit",
	CompactFinished:  "[r] repeat · [Enter] close · [q] quit",
	CompactStopHint:  "[Enter] yes · [Esc] no",
	HeroFinished:     "TIME'S UP!",
	HeroDone:         "DONE!",
	HeroFinishedWord: "FINISHED!",
	HeroTimeOut:      "TIME OUT!",
	HeroTheEnd:       "THE END!",
	HeroBingo:        "BINGO!",
	LabelFocus:       "Focus",
	RepeatHint:       "r repeat",
	CloseHint:        "enter close",

	StatsTitle:       "Statistics",
	StatsOverview:    "overview",
	StatsActivity:    "activity",
	StatsPresets:     "presets",
	StatsRecent:      "recent",
	StatsRange7:      "7 days",
	StatsRange30:     "30 days",
	StatsRange90:     "90 days",
	StatsRangeAll:    "all time",
	StatsLoading:     "Loading…",
	StatsUnavailable: "Statistics are not available",
	StatsFocused:     "focused",
	StatsSessions:    "sessions",
	StatsCompleted:   "completed",
	StatsVsPrev:      "vs prev %s",
	StatsStreak:      "%d-day streak",
	StatsBestStreak:  "best %d days",
	StatsLongest:     "longest %s",
	StatsBusiest:     "busiest %s %02d:00",
	StatsFocusPerDay: "focus per day",
	StatsMax:         "max %s",
	StatsHeatLess:    "less",
	StatsHeatMore:    "more",
	StatsByHour:      "by hour",
	StatsByWeekday:   "by weekday",
	StatsInsight:     "you start most timers on %s around %02d:00",
	StatsWeeks:       "%d weeks",
	StatsPresetsHead: "presets by total focus",
	StatsRuns:        "runs",
	StatsMostUsed:    "most used: %s",
	StatsAvgSession:  "avg session %s",
	StatsWhen:        "when",
	StatsPlanned:     "planned",
	StatsLabel:       "label",
	StatsOutcome:     "outcome",
	StatsDone:        "✓ completed",
	StatsStopped:     "· stopped at %s",
	StatsReplaced:    "· replaced at %s",
	StatsToday:       "today %s",
	StatsYesterday:   "yest %s",
	StatsShowing:     "showing %d of %d sessions",
	StatsEmptyTitle:  "No sessions yet",
	StatsEmptyBody:   "Finish or stop a countdown and it lands here.",
	StatsHint:        "[←/→] section · [t] range · [↑/↓] scroll · [Esc] close",
	StatsHintShort:   "[←/→] section · [Esc] close",

	WeekdayMon: "Mon",
	WeekdayTue: "Tue",
	WeekdayWed: "Wed",
	WeekdayThu: "Thu",
	WeekdayFri: "Fri",
	WeekdaySat: "Sat",
	WeekdaySun: "Sun",

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

	HelpSectionTimer:    "TIMER",
	HelpSectionDuration: "DURATION",
	HelpSectionPanels:   "PANELS",
	HelpSectionDisplay:  "DISPLAY",
	HelpSectionApp:      "APP",

	HelpStart:        "[Enter] start the duration",
	HelpPause:        "[Space] pause or resume",
	HelpCancel:       "[Esc] cancel (asks first)",
	HelpAdjust:       "[+/-] adjust one minute",
	HelpRepeat:       "[r] repeat when it finishes",
	HelpFormats:      "25 · 12:30 · 90s · 1h30m",
	HelpPickPreset:   "[←/→] pick a preset",
	HelpPresets:      "[p] presets",
	HelpSound:        "[v] sound",
	HelpMute:         "[m] mute or unmute",
	HelpStats:        "[s] statistics",
	HelpUpdate:       "[u] check for updates",
	HelpTheme:        "[t] theme",
	HelpBackground:   "[b] background",
	HelpReadable:     "[n] readable bands",
	HelpReadableNote: "only with None background",
	HelpQuit:         "[q] close the TUI",
	HelpTrayNote:     "the tray keeps running",
	HelpClose:        "[Esc] or [Enter] closes this help",

	HelpNarrowStartPause:   "[Enter] start · [Space] pause",
	HelpNarrowCancelAdjust: "[Esc] cancel · [+/-] 1 min",
	HelpNarrowRepeat:       "[r] repeat",
	HelpNarrowPanels:       "[p] presets · [v] sound",
	HelpNarrowPanelsMore:   "[s] stats · [u] update · [m] mute",
	HelpNarrowDisplay:      "[t] theme · [b] background",
	HelpNarrowReadable:     "[n] readable bands",
	HelpNarrowQuit:         "[q] quit · [Esc] close help",

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

	ErrorDuration:   "Use 25, 12:30, 90s or 1h30m",
	ErrorLabel:      "Name must be 1-32 characters",
	ErrorPresets:    "You can keep up to 16 presets",
	ErrorNoPreset:   "No preset selected",
	ErrorOffline:    "The daemon is offline",
	ErrorOpenURL:    "Could not open GitHub",
	ErrorOpenFolder: "Could not open the Downloads folder",
	ErrorUpdate:     "Update check failed",

	ToastAdded:       "“%s” added",
	ToastUpdated:     "“%s” updated",
	ToastDeleted:     "Preset deleted",
	ToastRestored:    "Default presets restored",
	ToastTheme:       "%s theme",
	ToastBackground:  "%s background",
	ToastReadableOn:  "Readable bands on",
	ToastReadableOff: "Readable bands off",
	ToastSoundOn:     "Sound on",
	ToastSoundOff:    "Sound off",
	ToastReplaced:    "Countdown replaced",
	ToastUpdateFound: "%s available",
	ToastUpToDate:    "Up to date (latest %s)",

	BackgroundAurora:    "Aurora",
	BackgroundStars:     "Stars",
	BackgroundFireflies: "Fireflies",
	BackgroundRain:      "Rain",
	BackgroundWaves:     "Tide",
	BackgroundGrid:      "Sunset",
	BackgroundNone:      "None",

	Minutes: "min",

	ExitActive:              "The countdown is still running in the tray.",
	ExitDaemonClosing:       "The daemon quit; closing.",
	ExitUpdating:            "Updating Tuick-Tack; it will reopen.",
	ToastPortable:           "Portable %s saved to Downloads",
	NotifyUpdateFailedTitle: "Update failed",
	NotifyUpdateFailedBody:  "The previous version is still installed. Open Tuick-Tack and try again.",

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
	TrayUpdateAvailable:      "Update available: %s",
	TrayUpdateTooltip:        "Open the release page",
	TrayUpdateCheck:          "Check for updates",
	TrayUpdateCheckTooltip:   "Contact GitHub for the latest release",
	TrayUpdateChecking:       "Checking for updates…",
	TrayUpdateUpToDate:       "Up to date (%s)",
	TrayUpdateFailed:         "Update check failed",
	TrayQuit:                 "Quit",
	TrayQuitTooltip:          "Quit Tuick-Tack",
	TrayCurrentTooltip:       "Current countdown",
}

// HeroFinishedChoices are the big celebration words. The TUI picks one per
// finish; every one fits the hero at scale 2 with proportional spacing.
var HeroFinishedChoices = []Key{
	HeroFinished,
	HeroDone,
	HeroFinishedWord,
	HeroTimeOut,
	HeroTheEnd,
	HeroBingo,
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

// WeekdayShort returns the short weekday name for Monday=0..Sunday=6.
func WeekdayShort(index int) string {
	switch index {
	case 0:
		return T(WeekdayMon)
	case 1:
		return T(WeekdayTue)
	case 2:
		return T(WeekdayWed)
	case 3:
		return T(WeekdayThu)
	case 4:
		return T(WeekdayFri)
	case 5:
		return T(WeekdaySat)
	case 6:
		return T(WeekdaySun)
	default:
		return ""
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

// BackgroundName returns the display name of a background ID. Unknown IDs fall
// back to the original aurora so the toast always has a label.
func BackgroundName(name string) string {
	switch name {
	case config.BackgroundStars:
		return T(BackgroundStars)
	case config.BackgroundFireflies:
		return T(BackgroundFireflies)
	case config.BackgroundRain:
		return T(BackgroundRain)
	case config.BackgroundWaves:
		return T(BackgroundWaves)
	case config.BackgroundGrid:
		return T(BackgroundGrid)
	case config.BackgroundNone:
		return T(BackgroundNone)
	default:
		return T(BackgroundAurora)
	}
}
