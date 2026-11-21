// Package config owns the persisted preferences: presets, theme, background,
// sound selection, volume and the TUI window placement.
//
// The daemon is the only writer. Load falls back to defaults when the file is
// missing, and backs up an unreadable or invalid file before doing the same.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// Version is the schema version understood by this build.
	Version = 5
	// MaxPresets is the maximum number of stored presets.
	MaxPresets = 16
	// MinSeconds and MaxSeconds bound a preset or countdown.
	MinSeconds = 1
	MaxSeconds = 86400
	// MaxLabelRunes bounds preset labels and countdown names.
	MaxLabelRunes = 32
)

// Sound names are stable identifiers stored in config.json.
const (
	SoundChime   = "chime"
	SoundMarimba = "marimba"
	SoundDigital = "digital"
	SoundDeep    = "deep"
)

// Sounds lists the supported completion sounds in UI order.
var Sounds = []string{SoundChime, SoundMarimba, SoundDigital, SoundDeep}

// Volume bounds and the defaults used by a fresh or migrated config.
const (
	DefaultSound  = SoundDigital
	DefaultVolume = 80
	MinVolume     = 0
	MaxVolume     = 100
)

// Theme names are stable identifiers stored in config.json.
const (
	ThemeMidnight = "midnight"
	ThemeEmber    = "ember"
	ThemeMono     = "mono"
)

// Themes lists the supported themes in cycle order.
var Themes = []string{ThemeMidnight, ThemeEmber, ThemeMono}

// Background names are stable identifiers stored in config.json. They are
// separate from the theme: the theme owns the palette and the background owns
// the ambient layer painted behind the stage.
const (
	BackgroundAurora    = "aurora"
	BackgroundStars     = "stars"
	BackgroundFireflies = "fireflies"
	BackgroundRain      = "rain"
	BackgroundWaves     = "waves"
	BackgroundGrid      = "grid"
	BackgroundNone      = "none"

	DefaultBackground = BackgroundAurora
)

// Backgrounds lists the supported backgrounds in cycle order. Aurora is the
// original background and stays first so the familiar look is always one step
// away.
var Backgrounds = []string{
	BackgroundAurora,
	BackgroundStars,
	BackgroundFireflies,
	BackgroundRain,
	BackgroundWaves,
	BackgroundGrid,
	BackgroundNone,
}

// Window size bounds. A stored width and height of zero means the TUI has not
// recorded a preference yet.
const (
	MinWindowCols = 1
	MaxWindowCols = 1000
	MinWindowRows = 1
	MaxWindowRows = 1000
)

// Window position bounds in screen pixels. Real desktops fit well inside this
// range, including negative coordinates from monitors placed to the left or
// above the primary one.
const (
	MinWindowPos = -32768
	MaxWindowPos = 32767
)

// WindowPosition is the outer top-left of the TUI window in screen pixels. It
// is display state stored next to the cell size and does not describe the
// maximized or minimized state.
type WindowPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Preset is a named duration. IDs are stable across restarts.
type Preset struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Seconds int    `json:"seconds"`
}

// Config is the on-disk configuration.
type Config struct {
	Version        int             `json:"version"`
	Theme          string          `json:"theme"`
	Background     string          `json:"background"`
	Sound          bool            `json:"sound"`
	SoundName      string          `json:"sound_name"`
	Volume         int             `json:"volume"`
	WindowWidth    int             `json:"window_width,omitempty"`
	WindowHeight   int             `json:"window_height,omitempty"`
	WindowPosition *WindowPosition `json:"window_position,omitempty"`
	Presets        []Preset        `json:"presets"`
}

// Default returns a fresh default configuration.
func Default() Config {
	return Config{
		Version:    Version,
		Theme:      ThemeMidnight,
		Background: DefaultBackground,
		Sound:      true,
		SoundName:  DefaultSound,
		Volume:     DefaultVolume,
		Presets: []Preset{
			{ID: "p5", Label: "5 min", Seconds: 300},
			{ID: "p10", Label: "10 min", Seconds: 600},
			{ID: "p15", Label: "15 min", Seconds: 900},
		},
	}
}

// ValidSound reports whether name is a supported completion sound.
func ValidSound(name string) bool {
	for _, s := range Sounds {
		if s == name {
			return true
		}
	}
	return false
}

// ValidVolume reports whether n is inside the accepted volume range.
func ValidVolume(n int) bool {
	return n >= MinVolume && n <= MaxVolume
}

// ValidTheme reports whether name is a supported theme.
func ValidTheme(name string) bool {
	for _, t := range Themes {
		if t == name {
			return true
		}
	}
	return false
}

// ValidBackground reports whether name is a supported ambient background.
func ValidBackground(name string) bool {
	for _, b := range Backgrounds {
		if b == name {
			return true
		}
	}
	return false
}

// NextBackground returns the next background in cycle order.
func NextBackground(name string) string {
	for i, b := range Backgrounds {
		if b == name {
			return Backgrounds[(i+1)%len(Backgrounds)]
		}
	}
	return Backgrounds[0]
}

// ValidSeconds reports whether n is inside the accepted duration range.
func ValidSeconds(n int) bool {
	return n >= MinSeconds && n <= MaxSeconds
}

// ValidWindowSize reports whether w by h is a size the TUI can store. Both
// zero means the preference is still unset.
func ValidWindowSize(w, h int) bool {
	if w == 0 && h == 0 {
		return true
	}
	return w >= MinWindowCols && w <= MaxWindowCols &&
		h >= MinWindowRows && h <= MaxWindowRows
}

// ValidWindowPosition reports whether p is a plausible screen position.
func ValidWindowPosition(p WindowPosition) bool {
	return p.X >= MinWindowPos && p.X <= MaxWindowPos &&
		p.Y >= MinWindowPos && p.Y <= MaxWindowPos
}

// ValidLabel validates a user-facing label: 1..32 printable runes, trimmed by
// the caller or here for convenience.
func ValidLabel(label string) bool {
	label = strings.TrimSpace(label)
	if label == "" || utf8.RuneCountInString(label) > MaxLabelRunes {
		return false
	}
	for _, r := range label {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// ValidPresetID reports whether an ID can be stored. Generated IDs are
// lowercase hex, but imported configs may use short readable IDs.
func ValidPresetID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// Migrate upgrades a decoded configuration to the current schema. Version 1
// had no sound selection or volume, so both take the v2 defaults. Version 3
// adds the optional window size and version 4 the optional window position;
// both stay unset until the TUI records them. Version 5 adds the ambient
// background, which starts as the original aurora. It returns the migrated
// config and whether anything changed.
func Migrate(c Config) (Config, bool) {
	if c.Version >= Version {
		return c, false
	}
	if c.Version < 2 {
		c.SoundName = DefaultSound
		c.Volume = DefaultVolume
	}
	if c.Version < 5 {
		c.Background = DefaultBackground
	}
	c.Version = Version
	return c, true
}

// Validate checks the schema and every preset. It does not normalize values.
func Validate(c Config) error {
	if c.Version != Version {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if !ValidTheme(c.Theme) {
		return fmt.Errorf("unknown theme %q", c.Theme)
	}
	if !ValidBackground(c.Background) {
		return fmt.Errorf("unknown background %q", c.Background)
	}
	if !ValidSound(c.SoundName) {
		return fmt.Errorf("unknown sound %q", c.SoundName)
	}
	if !ValidVolume(c.Volume) {
		return fmt.Errorf("volume %d is outside %d..%d", c.Volume, MinVolume, MaxVolume)
	}
	if !ValidWindowSize(c.WindowWidth, c.WindowHeight) {
		return fmt.Errorf("invalid window size %dx%d", c.WindowWidth, c.WindowHeight)
	}
	if c.WindowPosition != nil && !ValidWindowPosition(*c.WindowPosition) {
		return fmt.Errorf("invalid window position %d,%d", c.WindowPosition.X, c.WindowPosition.Y)
	}
	if len(c.Presets) > MaxPresets {
		return fmt.Errorf("too many presets: %d (max %d)", len(c.Presets), MaxPresets)
	}
	seen := make(map[string]bool, len(c.Presets))
	for i, p := range c.Presets {
		if !ValidPresetID(p.ID) {
			return fmt.Errorf("preset %d has invalid id %q", i, p.ID)
		}
		if seen[p.ID] {
			return fmt.Errorf("duplicate preset id %q", p.ID)
		}
		seen[p.ID] = true
		if !ValidLabel(p.Label) {
			return fmt.Errorf("preset %q has an invalid label", p.ID)
		}
		if !ValidSeconds(p.Seconds) {
			return fmt.Errorf("preset %q has invalid seconds %d", p.ID, p.Seconds)
		}
	}
	return nil
}

// Normalize trims labels and fills a missing preset list. It is used after
// decoding so harmless whitespace does not invalidate a config.
func Normalize(c Config) Config {
	if c.Background == "" {
		// A v5 file edited by hand may lose the field; keep the rest of the
		// settings instead of backing up the whole config.
		c.Background = DefaultBackground
	}
	for i := range c.Presets {
		c.Presets[i].Label = strings.TrimSpace(c.Presets[i].Label)
	}
	if c.Presets == nil {
		c.Presets = []Preset{}
	}
	// The position is display state: a corrupt value is dropped instead of
	// invalidating the rest of the configuration.
	if c.WindowPosition != nil && !ValidWindowPosition(*c.WindowPosition) {
		c.WindowPosition = nil
	}
	return c
}

// NewPresetID returns a random stable ID for a custom preset.
func NewPresetID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("p%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// NewPreset builds a custom preset with a generated ID and trimmed label.
func NewPreset(label string, seconds int) Preset {
	return Preset{ID: NewPresetID(), Label: strings.TrimSpace(label), Seconds: seconds}
}

// Copy returns a deep copy safe to mutate without touching the original.
func (c Config) Copy() Config {
	out := c
	out.Presets = append([]Preset(nil), c.Presets...)
	if c.WindowPosition != nil {
		pos := *c.WindowPosition
		out.WindowPosition = &pos
	}
	return out
}

// ErrInvalid is wrapped by validation failures in the store.
var ErrInvalid = errors.New("invalid config")
