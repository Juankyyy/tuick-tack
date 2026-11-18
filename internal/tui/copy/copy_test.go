package copy

import (
	"strings"
	"testing"

	"tuick-tack/internal/config"
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
	if got, want := T(NotifyFoundBody, "v0.2.0"),
		"Version v0.2.0 is available. Click to open the release page."; got != want {
		t.Errorf("NotifyFoundBody = %q, want %q", got, want)
	}
	for _, r := range T(UpdateAvailable) + T(UpdateShort) + T(UpdateHint) {
		if r > 0x7f {
			t.Errorf("update copy contains a non-ASCII rune %q", r)
		}
	}
}

func TestKeyHintsUseBracketFormat(t *testing.T) {
	keys := []Key{
		ControlsIdle, ControlsRunning, ControlsPaused, ControlsFinished,
		CompactIdle, CompactRunning, CompactPaused, CompactFinished, CompactStopHint,
		FinishedMeta, PausedMeta,
		HelpIdle, HelpTimer, HelpModal, HelpSound, HelpFinished, HelpClose, HelpPresets,
		HelpUpdate, UpdateHint,
		SoundHint, SoundActionHint,
		PresetFormHint, PresetsEmpty, PresetsHint, PresetsActionHint,
	}
	for _, key := range keys {
		text := T(key)
		if !strings.Contains(text, "[") || !strings.Contains(text, "]") {
			t.Errorf("key hint %q does not use [key] labels: %q", key, text)
		}
	}
}
