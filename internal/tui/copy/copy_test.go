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

func TestKeyHintsUseBracketFormat(t *testing.T) {
	keys := []Key{
		ControlsIdle, ControlsRunning, ControlsPaused, ControlsFinished,
		FinishedMeta, PausedMeta,
		HelpIdle, HelpTimer, HelpModal, HelpSound, HelpFinished, HelpClose, HelpPresets,
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
