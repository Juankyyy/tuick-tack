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
		{"bad version", func(c *Config) { c.Version = 2 }, false},
		{"bad theme", func(c *Config) { c.Theme = "neon" }, false},
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

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	store := NewStore(path)
	c := Default()
	c.Theme = ThemeEmber
	c.Sound = false
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
	if got.Theme != ThemeEmber || got.Sound || len(got.Presets) != 4 {
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
	c := Config{Version: 1, Theme: ThemeMono, Presets: []Preset{{ID: "p1", Label: "  hi  ", Seconds: 10}}}
	c = Normalize(c)
	if c.Presets[0].Label != "hi" {
		t.Fatalf("label = %q", c.Presets[0].Label)
	}
	if err := Validate(c); err != nil {
		t.Fatalf("normalized config invalid: %v", err)
	}
}
