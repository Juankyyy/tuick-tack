// Package config owns the persisted preferences: presets, theme and sound.
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
	Version = 1
	// MaxPresets is the maximum number of stored presets.
	MaxPresets = 16
	// MinSeconds and MaxSeconds bound a preset or countdown.
	MinSeconds = 1
	MaxSeconds = 86400
	// MaxLabelRunes bounds preset labels and countdown names.
	MaxLabelRunes = 32
)

// Theme names are stable identifiers stored in config.json.
const (
	ThemeMidnight = "midnight"
	ThemeEmber    = "ember"
	ThemeMono     = "mono"
)

// Themes lists the supported themes in cycle order.
var Themes = []string{ThemeMidnight, ThemeEmber, ThemeMono}

// Preset is a named duration. IDs are stable across restarts.
type Preset struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Seconds int    `json:"seconds"`
}

// Config is the on-disk configuration.
type Config struct {
	Version int      `json:"version"`
	Theme   string   `json:"theme"`
	Sound   bool     `json:"sound"`
	Presets []Preset `json:"presets"`
}

// Default returns a fresh default configuration.
func Default() Config {
	return Config{
		Version: Version,
		Theme:   ThemeMidnight,
		Sound:   true,
		Presets: []Preset{
			{ID: "p5", Label: "5 min", Seconds: 300},
			{ID: "p10", Label: "10 min", Seconds: 600},
			{ID: "p15", Label: "15 min", Seconds: 900},
		},
	}
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

// ValidSeconds reports whether n is inside the accepted duration range.
func ValidSeconds(n int) bool {
	return n >= MinSeconds && n <= MaxSeconds
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

// Validate checks the schema and every preset. It does not normalize values.
func Validate(c Config) error {
	if c.Version != Version {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if !ValidTheme(c.Theme) {
		return fmt.Errorf("unknown theme %q", c.Theme)
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
	for i := range c.Presets {
		c.Presets[i].Label = strings.TrimSpace(c.Presets[i].Label)
	}
	if c.Presets == nil {
		c.Presets = []Preset{}
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
	return out
}

// ErrInvalid is wrapped by validation failures in the store.
var ErrInvalid = errors.New("invalid config")
