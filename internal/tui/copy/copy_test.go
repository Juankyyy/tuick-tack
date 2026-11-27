package copy

import (
	"strings"
	"testing"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/tui/bigfont"
	"tuick-tack/internal/tui/canvas"
)

func TestSoundLabelCoversConfig(t *testing.T) {
	for _, name := range config.Sounds {
		label := SoundLabel(name)
		if label == "" || label == name {
			t.Errorf("SoundLabel(%q) = %q, want a user-facing label", name, label)
		}
	}
}

func TestSoundLabelsAreSingleCell(t *testing.T) {
	for _, name := range config.Sounds {
		for _, r := range SoundLabel(name) {
			if r > 0x7f {
				t.Errorf("SoundLabel(%q) contains a non-ASCII rune %q", name, r)
			}
		}
	}
}

func TestFinishedChoicesFitHero(t *testing.T) {
	seen := map[string]bool{}
	for _, key := range HeroFinishedChoices {
		word := T(key)
		if word == "" || word == string(key) {
			t.Errorf("copy key %q has no text", key)
			continue
		}
		if seen[word] {
			t.Errorf("duplicate celebration word %q", word)
		}
		seen[word] = true
		for _, r := range word {
			if _, ok := bigfont.Glyph(r); !ok {
				t.Errorf("%q has no bigfont glyph for %q", word, r)
			}
		}
		if w := bigfont.WidthProportional(word, 2); w > 92 {
			t.Errorf("%q is %d columns at 2x, want at most 92", word, w)
		}
	}
	if len(seen) < 6 {
		t.Fatalf("only %d celebration words, want 6", len(seen))
	}
}

func TestSoundAndTrayCopyExists(t *testing.T) {
	keys := []Key{
		SoundTitle, SoundOn, SoundOff, SoundVolume, SoundPreview,
		SoundMute, SoundUnmute, SoundHint, SoundActionHint,
		SoundChime, SoundMarimba, SoundDigital, SoundDeep,
		HelpSound,
		TraySoundSettings, TraySoundSettingsTooltip, TraySoundPresetTooltip,
		TraySoundPreview, TraySoundPreviewTooltip,
		TrayVolume, TrayVolumeTooltip, TrayVolumeValue, TrayVolumeLevel,
		TrayVolumeDown, TrayVolumeDownTooltip, TrayVolumeUp, TrayVolumeUpTooltip,
	}
	for _, key := range keys {
		if T(key) == "" || T(key) == string(key) {
			t.Errorf("copy key %q has no text", key)
		}
	}
}

func TestPresetCopyExists(t *testing.T) {
	keys := []Key{
		PresetsTitle, PresetsEmpty, PresetsHint, PresetsActionHint,
		PresetNewTitle, PresetEditTitle, PresetName, PresetDuration,
		PresetFormHint, PresetDeleteTitle, HelpPresets,
	}
	for _, key := range keys {
		if T(key) == "" || T(key) == string(key) {
			t.Errorf("copy key %q has no text", key)
		}
	}
}

func TestHeaderCopyExists(t *testing.T) {
	for _, key := range []Key{RunningMeta, EndsAt, TimeAt, HeaderOnHold, HeaderNoTimer} {
		if T(key) == "" || T(key) == string(key) {
			t.Errorf("header copy key %q has no text", key)
		}
	}
}

func TestUpdateCopyExists(t *testing.T) {
	for _, key := range []Key{
		UpdateAvailable, UpdateShort, UpdateArrow, UpdateChecking, UpdateHint, ErrorOpenURL, ErrorUpdate,
		UpdateFromTo, UpdateTypeSetup, UpdateTypePortable,
		UpdateStatusInstalled, UpdateStatusNotInstalled, UpdateStatusInUse, UpdateStatusNotInUse,
		UpdateStatusNotInRelease, UpdateOtherShort,
		UpdateCurrentSetup, UpdateCurrentPortable, UpdateInstallShort,
		UpdateNotesTitle, UpdateHintSelect, UpdateHintTab, UpdateHintNotes, UpdateHintGithub,
		UpdateHintLater, UpdateHintWarn, UpdateHintPortable, UpdateHintFailed,
		UpdateMacPortableBadge, UpdateMacCurrent, UpdateMacTitle, UpdateMacSaved,
		UpdateMacDone, UpdateMacUnavailable, UpdateMacDownloadShort, ToastMacPortable,
		ToastUpdateFound, ToastUpToDate,
		NotifyFoundTitle, NotifyFoundBody, NotifyUpToDateTitle, NotifyUpToDateBody,
		NotifyFailedTitle, NotifyFailedBody,
		TrayUpdateAvailable, TrayUpdateTooltip,
		TrayUpdateCheck, TrayUpdateCheckTooltip, TrayUpdateChecking,
		TrayUpdateUpToDate, TrayUpdateFailed,
	} {
		if T(key) == "" || T(key) == string(key) {
			t.Errorf("update copy key %q has no text", key)
		}
	}
	if got, want := T(TrayUpdateAvailable, "v0.2.0"), "Update available: v0.2.0"; got != want {
		t.Errorf("TrayUpdateAvailable = %q, want %q", got, want)
	}
	for _, r := range T(UpdateAvailable) + T(UpdateShort) + T(UpdateHint) {
		if r > 0x7f {
			t.Errorf("update copy contains a non-ASCII rune %q", r)
		}
	}
}

func TestBackgroundCopyCoversConfig(t *testing.T) {
	for _, name := range config.Backgrounds {
		label := BackgroundName(name)
		if label == "" || label == name {
			t.Errorf("BackgroundName(%q) = %q, want a user-facing label", name, label)
		}
		for _, r := range label {
			if r > 0x7f {
				t.Errorf("BackgroundName(%q) contains a non-ASCII rune %q", name, r)
			}
		}
	}
	if got, want := T(ToastBackground, BackgroundName(config.BackgroundStars)), "Stars background"; got != want {
		t.Errorf("ToastBackground = %q, want %q", got, want)
	}
	if got := BackgroundName(config.BackgroundGrid); got != "Sunset" {
		t.Errorf("BackgroundName(grid) = %q, want Sunset", got)
	}
}

func TestReadableTextCopyExists(t *testing.T) {
	for _, key := range []Key{HelpReadable, ToastReadableOn, ToastReadableOff} {
		if T(key) == "" || T(key) == string(key) {
			t.Errorf("readable text copy key %q has no text", key)
		}
	}
	if !strings.Contains(T(HelpReadable), "[n]") {
		t.Errorf("HelpReadable does not advertise [n]: %q", T(HelpReadable))
	}
}

func TestKeyHintsUseBracketFormat(t *testing.T) {
	keys := []Key{
		ControlsIdle, ControlsRunning, ControlsPaused, ControlsFinished,
		CompactIdle, CompactRunning, CompactPaused, CompactFinished, CompactStopHint,
		FinishedMeta, PausedMeta,
		HelpStart, HelpPause, HelpCancel, HelpAdjust, HelpRepeat, HelpPickPreset,
		HelpPresets, HelpSound, HelpMute, HelpStats, HelpUpdate, HelpTheme,
		HelpBackground, HelpReadable, HelpQuit, HelpQuitAll, HelpClose,
		HelpNarrowStartPause, HelpNarrowCancelAdjust, HelpNarrowRepeat, HelpNarrowPanels,
		HelpNarrowPanelsMore, HelpNarrowDisplay, HelpNarrowReadable, HelpNarrowQuit,
		HelpNarrowQuitAll,
		UpdateHint,
		SoundHint, SoundActionHint,
		PresetFormHint, PresetsEmpty, PresetsHint, PresetsActionHint,
		StatsHint, StatsHintSection, StatsHintNarrow, StatsHintShort,
	}
	for _, key := range keys {
		text := T(key)
		if !strings.Contains(text, "[") || !strings.Contains(text, "]") {
			t.Errorf("key hint %q does not use [key] labels: %q", key, text)
		}
	}
}

func TestStatsCopyExists(t *testing.T) {
	keys := []Key{
		StatsTitle, StatsOverview, StatsActivity, StatsPresets, StatsRecent,
		StatsRange7, StatsRange30, StatsRange90, StatsRangeAll,
		StatsLoading, StatsUnavailable,
		StatsFocused, StatsSessions, StatsCompleted, StatsOfFinished, StatsVsPrev,
		StatsStreak, StatsBestStreak, StatsLongest, StatsBusiest,
		StatsFocusPerDay, StatsLastDays, StatsRecordsHead,
		StatsMax, StatsHeatLess, StatsHeatMore,
		StatsByHour, StatsByWeekday, StatsInsight, StatsWeeks,
		StatsPresetsHead, StatsRuns, StatsMostUsed, StatsAvgSession,
		StatsWhen, StatsPlanned, StatsLabel, StatsOutcome,
		StatsDone, StatsStopped, StatsReplaced,
		StatsToday, StatsYesterday, StatsYesterdayShort, StatsShowing,
		StatsEmptyTitle, StatsEmptyBody, StatsHint, StatsHintSection, StatsHintNarrow, StatsHintShort,
		WeekdayMon, WeekdayTue, WeekdayWed, WeekdayThu, WeekdayFri, WeekdaySat, WeekdaySun,
		MonthJan, MonthFeb, MonthMar, MonthApr, MonthMay, MonthJun,
		MonthJul, MonthAug, MonthSep, MonthOct, MonthNov, MonthDec,
	}
	for _, key := range keys {
		if T(key) == "" || T(key) == string(key) {
			t.Errorf("stats copy key %q has no text", key)
		}
		for _, r := range T(key) {
			if !canvas.SingleCell(r) {
				t.Errorf("stats copy %q contains a non-single-cell rune %q", key, r)
			}
		}
	}
	if got := T(StatsStreak, 4); got != "4-day streak" {
		t.Errorf("StatsStreak = %q", got)
	}
	if got := WeekdayShort(0); got != "Mon" {
		t.Errorf("WeekdayShort(0) = %q", got)
	}
	if got := WeekdayShort(6); got != "Sun" {
		t.Errorf("WeekdayShort(6) = %q", got)
	}
	if got := WeekdayShort(9); got != "" {
		t.Errorf("WeekdayShort(9) = %q, want empty", got)
	}
	if got := WeekdayNarrow(0); got != "Mo" {
		t.Errorf("WeekdayNarrow(0) = %q", got)
	}
	if got := WeekdayNarrow(9); got != "" {
		t.Errorf("WeekdayNarrow(9) = %q, want empty", got)
	}
	if got := MonthShort(time.January); got != "Jan" {
		t.Errorf("MonthShort(January) = %q", got)
	}
	if got := MonthShort(time.Month(13)); got != "" {
		t.Errorf("MonthShort(13) = %q, want empty", got)
	}
}
