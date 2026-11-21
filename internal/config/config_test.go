package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	c := Default()
	if err := Validate(c); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
	if len(c.Presets) != 3 || c.Presets[0].Seconds != 300 || c.Presets[2].Seconds != 900 {
		t.Fatalf("unexpected default presets: %+v", c.Presets)
	}
	if c.Theme != ThemeMidnight || !c.Sound {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.Background != DefaultBackground {
		t.Fatalf("background = %q, want %q", c.Background, DefaultBackground)
	}
	if !c.ReadableText {
		t.Fatal("readable bands should start enabled")
	}
	if c.Version != Version || c.SoundName != DefaultSound || c.Volume != DefaultVolume {
		t.Fatalf("unexpected sound defaults: %+v", c)
	}
}

func TestBackgroundCycle(t *testing.T) {
	if !ValidBackground(DefaultBackground) {
		t.Fatalf("default background %q is not valid", DefaultBackground)
	}
	for i, name := range Backgrounds {
		if !ValidBackground(name) {
			t.Errorf("%q is not valid", name)
		}
		if got, want := NextBackground(name), Backgrounds[(i+1)%len(Backgrounds)]; got != want {
			t.Errorf("NextBackground(%q) = %q, want %q", name, got, want)
		}
	}
	if got := NextBackground("nope"); got != Backgrounds[0] {
		t.Errorf("unknown background fell back to %q, want %q", got, Backgrounds[0])
	}
	if ValidBackground("") {
		t.Error("empty background should be invalid")
	}
}

func TestValidate(t *testing.T) {
	base := Default()
	tests := []struct {
		name string
		mod  func(*Config)
		ok   bool
	}{
		{"valid", func(*Config) {}, true},
		{"no presets", func(c *Config) { c.Presets = nil }, true},
		{"bad version", func(c *Config) { c.Version = Version + 1 }, false},
		{"bad theme", func(c *Config) { c.Theme = "neon" }, false},
		{"bad background", func(c *Config) { c.Background = "neon" }, false},
		{"bad sound", func(c *Config) { c.SoundName = "airhorn" }, false},
		{"low volume", func(c *Config) { c.Volume = -1 }, false},
		{"high volume", func(c *Config) { c.Volume = 101 }, false},
		{"window size", func(c *Config) { c.WindowWidth, c.WindowHeight = 140, 45 }, true},
		{"window width only", func(c *Config) { c.WindowWidth = 140 }, false},
		{"negative window", func(c *Config) { c.WindowWidth, c.WindowHeight = -1, 45 }, false},
		{"small window", func(c *Config) { c.WindowWidth, c.WindowHeight = 1, 1 }, true},
		{"huge window", func(c *Config) {
			c.WindowWidth, c.WindowHeight = MaxWindowCols+1, 45
		}, false},
		{"window position", func(c *Config) {
			c.WindowPosition = &WindowPosition{X: -1920, Y: 50}
		}, true},
		{"window position low", func(c *Config) {
			c.WindowPosition = &WindowPosition{X: MinWindowPos - 1, Y: 50}
		}, false},
		{"window position high", func(c *Config) {
			c.WindowPosition = &WindowPosition{X: 10, Y: MaxWindowPos + 1}
		}, false},
		{"too many", func(c *Config) {
			for i := 0; i < MaxPresets+1; i++ {
				c.Presets = append(c.Presets, Preset{ID: NewPresetID(), Label: "x", Seconds: 1})
			}
		}, false},
		{"empty label", func(c *Config) { c.Presets[0].Label = " " }, false},
		{"long label", func(c *Config) { c.Presets[0].Label = strings.Repeat("a", MaxLabelRunes+1) }, false},
		{"control label", func(c *Config) { c.Presets[0].Label = "a\nb" }, false},
		{"zero seconds", func(c *Config) { c.Presets[0].Seconds = 0 }, false},
		{"too long", func(c *Config) { c.Presets[0].Seconds = MaxSeconds + 1 }, false},
		{"bad id", func(c *Config) { c.Presets[0].ID = "bad id" }, false},
		{"duplicate id", func(c *Config) { c.Presets[1].ID = c.Presets[0].ID }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base.Copy()
			tt.mod(&c)
			err := Validate(c)
			if tt.ok && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
		})
	}
}

func TestNormalizeDropsInvalidWindowPosition(t *testing.T) {
	c := Default()
	c.WindowPosition = &WindowPosition{X: MinWindowPos - 1, Y: 10}
	if got := Normalize(c); got.WindowPosition != nil {
		t.Fatalf("invalid position survived Normalize: %+v", got.WindowPosition)
	}
	c.WindowPosition = &WindowPosition{X: 10, Y: 20}
	if got := Normalize(c); got.WindowPosition == nil || got.WindowPosition.Y != 20 {
		t.Fatalf("valid position was dropped: %+v", got.WindowPosition)
	}
}

func TestNewPreset(t *testing.T) {
	a := NewPreset("  Focus  ", 1500)
	if a.Label != "Focus" || a.Seconds != 1500 || a.ID == "" {
		t.Fatalf("unexpected preset: %+v", a)
	}
	b := NewPreset("Other", 60)
	if a.ID == b.ID {
		t.Fatal("generated IDs should differ")
	}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.json"))
	c, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Presets) != 3 {
		t.Fatalf("expected defaults, got %+v", c)
	}
}

func TestLoadMigratesV1(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	v1 := `{
  "version": 1,
  "theme": "ember",
  "sound": false,
  "presets": [
    { "id": "p5", "label": "5 min", "seconds": 300 }
  ]
}`
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	c, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != Version || c.SoundName != SoundDigital || c.Volume != DefaultVolume {
		t.Fatalf("v1 was not migrated: %+v", c)
	}
	if c.Background != DefaultBackground {
		t.Fatalf("v1 background = %q, want %q", c.Background, DefaultBackground)
	}
	if !c.ReadableText {
		t.Fatal("v1 migration should enable readable bands")
	}
	if c.Theme != ThemeEmber || c.Sound || len(c.Presets) != 1 {
		t.Fatalf("migration lost settings: %+v", c)
	}
	if err := store.Save(c); err != nil {
		t.Fatalf("Save migrated: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"version": 6`) ||
		!strings.Contains(string(data), `"background": "aurora"`) ||
		!strings.Contains(string(data), `"readable_text": true`) ||
		!strings.Contains(string(data), `"sound_name": "digital"`) {
		t.Fatalf("migrated file not written as v6: %s", data)
	}
}

func TestLoadMigratesV2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	v2 := `{
  "version": 2,
  "theme": "mono",
  "sound": false,
  "sound_name": "marimba",
  "volume": 35,
  "presets": [
    { "id": "p5", "label": "5 min", "seconds": 300 }
  ]
}`
	if err := os.WriteFile(path, []byte(v2), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != Version {
		t.Fatalf("version = %d, want %d", c.Version, Version)
	}
	if c.SoundName != SoundMarimba || c.Volume != 35 || c.Sound ||
		c.Theme != ThemeMono || len(c.Presets) != 1 ||
		c.Background != DefaultBackground {
		t.Fatalf("v2 migration lost settings: %+v", c)
	}
	if c.WindowWidth != 0 || c.WindowHeight != 0 || c.WindowPosition != nil {
		t.Fatalf("window placement should start unset: %+v", c)
	}
}

func TestLoadMigratesV3(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	v3 := `{
  "version": 3,
  "theme": "midnight",
  "sound": true,
  "sound_name": "deep",
  "volume": 55,
  "window_width": 140,
  "window_height": 45,
  "presets": []
}`
	if err := os.WriteFile(path, []byte(v3), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != Version || c.WindowWidth != 140 || c.WindowHeight != 45 {
		t.Fatalf("v3 migration lost settings: %+v", c)
	}
	if c.WindowPosition != nil {
		t.Fatalf("position should start unset: %+v", c.WindowPosition)
	}
	if c.Background != DefaultBackground {
		t.Fatalf("v3 background = %q, want %q", c.Background, DefaultBackground)
	}
}

func TestLoadMigratesV4(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	v4 := `{
  "version": 4,
  "theme": "ember",
  "sound": true,
  "sound_name": "deep",
  "volume": 40,
  "window_width": 120,
  "window_height": 40,
  "window_position": { "x": 93, "y": 100 },
  "presets": [
    { "id": "p5", "label": "5 min", "seconds": 300 }
  ]
}`
	if err := os.WriteFile(path, []byte(v4), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != Version || c.Theme != ThemeEmber ||
		c.Background != DefaultBackground || c.SoundName != SoundDeep ||
		c.Volume != 40 || len(c.Presets) != 1 {
		t.Fatalf("v4 migration lost settings: %+v", c)
	}
	if c.WindowPosition == nil || c.WindowPosition.X != 93 || c.WindowPosition.Y != 100 {
		t.Fatalf("v4 migration lost the window position: %+v", c.WindowPosition)
	}
	if !c.ReadableText {
		t.Fatal("v4 migration should enable readable bands")
	}
}

func TestLoadMigratesV5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	v5 := `{
  "version": 5,
  "theme": "mono",
  "background": "none",
  "sound": true,
  "sound_name": "digital",
  "volume": 80,
  "presets": [
    { "id": "p5", "label": "5 min", "seconds": 300 }
  ]
}`
	if err := os.WriteFile(path, []byte(v5), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != Version || c.Background != BackgroundNone ||
		c.Theme != ThemeMono || len(c.Presets) != 1 {
		t.Fatalf("v5 migration lost settings: %+v", c)
	}
	if !c.ReadableText {
		t.Fatal("v5 migration should enable readable bands")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	store := NewStore(path)
	c := Default()
	c.Theme = ThemeEmber
	c.Background = BackgroundStars
	c.ReadableText = false
	c.Sound = false
	c.SoundName = SoundMarimba
	c.Volume = 35
	c.WindowWidth = 140
	c.WindowHeight = 45
	c.WindowPosition = &WindowPosition{X: -120, Y: 100}
	c.Presets = append(c.Presets, NewPreset("Focus", 1500))
	if err := store.Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != ThemeEmber || got.Sound || len(got.Presets) != 4 ||
		got.Background != BackgroundStars || got.ReadableText ||
		got.SoundName != SoundMarimba || got.Volume != 35 ||
		got.WindowWidth != 140 || got.WindowHeight != 45 ||
		got.WindowPosition == nil || got.WindowPosition.X != -120 ||
		got.WindowPosition.Y != 100 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestLoadCorruptBacksUpAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 11, 10, 30, 0, 0, time.UTC)
	store := NewStoreWithClock(path, func() time.Time { return now })
	c, err := store.Load()
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Load error = %v, want ErrInvalid", err)
	}
	if len(c.Presets) != 3 {
		t.Fatalf("expected defaults, got %+v", c)
	}
	backup := filepath.Join(dir, "config.bad-20261111-103000.000.json")
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(data) != "{not json" {
		t.Fatalf("backup content = %q", data)
	}
	// A second corrupt load must not overwrite the first backup.
	if err := os.WriteFile(path, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("second Load error = %v, want ErrInvalid", err)
	}
	backup2 := filepath.Join(dir, "config.bad-20261111-103000.000-1.json")
	if _, err := os.Stat(backup2); err != nil {
		t.Fatalf("second backup missing: %v", err)
	}
}

func TestLoadInvalidSchemaBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	bad := Default()
	bad.Version = 99
	data, _ := json.Marshal(bad)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if _, err := store.Load(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Load error = %v, want ErrInvalid", err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "config.bad-*.json"))
	if len(matches) != 1 {
		t.Fatalf("backups = %v, want 1", matches)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config.json"))
	c := Default()
	c.Theme = "nope"
	if err := store.Save(c); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Save error = %v, want ErrInvalid", err)
	}
	if _, err := os.Stat(store.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid config must not be written: %v", err)
	}
}

func TestNormalizeTrimsLabels(t *testing.T) {
	c := Config{
		Version:    Version,
		Theme:      ThemeMono,
		Background: DefaultBackground,
		SoundName:  DefaultSound,
		Volume:     DefaultVolume,
		Presets:    []Preset{{ID: "p1", Label: "  hi  ", Seconds: 10}},
	}
	c = Normalize(c)
	if c.Presets[0].Label != "hi" {
		t.Fatalf("label = %q", c.Presets[0].Label)
	}
	if err := Validate(c); err != nil {
		t.Fatalf("normalized config invalid: %v", err)
	}
}

func TestNormalizeFillsMissingBackground(t *testing.T) {
	c := Default()
	c.Background = ""
	got := Normalize(c)
	if got.Background != DefaultBackground {
		t.Fatalf("background = %q, want %q", got.Background, DefaultBackground)
	}
	if err := Validate(got); err != nil {
		t.Fatalf("normalized config invalid: %v", err)
	}
}
