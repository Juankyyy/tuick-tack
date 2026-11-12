package copy

import (
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
		SoundMute, SoundUnmute, SoundHint,
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
