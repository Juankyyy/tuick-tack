package tray

import (
	"testing"
	"time"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
)

func TestPresetMenuLabel(t *testing.T) {
	tests := []struct {
		preset config.Preset
		want   string
	}{
		{config.Preset{Label: "5 min", Seconds: 300}, "5 min"},
		{config.Preset{Label: "Focus", Seconds: 1500}, "Focus · 25 min"},
		{config.Preset{Label: "", Seconds: 90}, "1:30"},
		{config.Preset{Label: "  Deep work  ", Seconds: 5400}, "Deep work · 1h 30m"},
	}
	for _, tt := range tests {
		if got := presetMenuLabel(tt.preset); got != tt.want {
			t.Errorf("presetMenuLabel(%+v) = %q, want %q", tt.preset, got, tt.want)
		}
	}
}

func TestSamePresets(t *testing.T) {
	a := []config.Preset{{ID: "a", Label: "A", Seconds: 60}}
	if !samePresets(a, append([]config.Preset(nil), a...)) {
		t.Fatal("equal slices should match")
	}
	if samePresets(a, nil) {
		t.Fatal("different lengths should not match")
	}
	b := append([]config.Preset(nil), a...)
	b[0].Seconds = 61
	if samePresets(a, b) {
		t.Fatal("different values should not match")
	}
}

func TestStatusText(t *testing.T) {
	tests := []struct {
		state   engine.State
		menu    string
		tooltip string
	}{
		{engine.State{Status: engine.Idle}, "Tuick-Tack · ready", "Tuick-Tack · ready"},
		{engine.State{Status: engine.Running, Remaining: 4*time.Minute + 32*time.Second, Label: "5 min"},
			"Tuick-Tack · 04:32 remaining", "Tuick-Tack · 04:32 remaining (5 min)"},
		{engine.State{Status: engine.Paused, Remaining: 90 * time.Second, Label: "Focus"},
			"Tuick-Tack · 01:30 paused", "Tuick-Tack · 01:30 paused (Focus)"},
		{engine.State{Status: engine.Finished}, "Tuick-Tack · time's up!", "Tuick-Tack · time's up!"},
	}
	for _, tt := range tests {
		menu, tooltip := statusText(tt.state)
		if menu != tt.menu || tooltip != tt.tooltip {
			t.Errorf("statusText(%+v) = %q, %q; want %q, %q",
				tt.state, menu, tooltip, tt.menu, tt.tooltip)
		}
	}
}

func TestIconFor(t *testing.T) {
	for _, st := range []engine.Status{engine.Idle, engine.Running, engine.Paused, engine.Finished} {
		if len(iconFor(st)) == 0 {
			t.Errorf("iconFor(%s) is empty", st)
		}
	}
}
