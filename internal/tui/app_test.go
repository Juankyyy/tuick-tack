package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/theme"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type testClock struct{ t time.Time }

func (c *testClock) Now() time.Time          { return c.t }
func (c *testClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func testModel(w, h int) (*Model, *testClock) {
	clock := &testClock{t: time.Date(2026, 11, 11, 14, 0, 0, 0, time.Local)}
	m := newModel("", clock.Now, 42)
	m.connected = true
	m.connecting = false
	m.width, m.height = w, h
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(config.Default())})
	return m, clock
}

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

func render(m *Model) []string {
	return strings.Split(stripANSI(m.View().Content), "\n")
}

func assertFrame(t *testing.T, m *Model, w, h int) string {
	t.Helper()
	lines := render(m)
	if len(lines) != h {
		t.Fatalf("height = %d, want %d", len(lines), h)
	}
	for i, line := range lines {
		if got := canvas.Width(line); got != w {
			t.Fatalf("line %d width = %d, want %d (%q)", i, got, w, line)
		}
		for _, r := range line {
			if !canvas.SingleCell(r) {
				t.Fatalf("line %d contains a non-single-cell rune %q", i, r)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func applyRunning(m *Model) {
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "running",
		DurationMS: 300000, RemainingMS: 271500, Label: "5 min", PresetID: "p5",
	})
}

func TestViewIdle(t *testing.T) {
	m, _ := testModel(80, 24)
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{"tuick·tack", "Ready when you are", "5 min", "12:30", "[p] presets", copy.T(copy.ControlsIdle)} {
		if !strings.Contains(plain, want) {
			t.Errorf("idle view missing %q", want)
		}
	}
	m.width, m.height = 120, 40
	wide := assertFrame(t, m, 120, 40)
	if !strings.Contains(wide, "10 min") || !strings.Contains(wide, "15 min") {
		t.Error("wide idle view should show all default chips")
	}
}

func TestViewRunningPaused(t *testing.T) {
	m, _ := testModel(80, 24)
	applyRunning(m)
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{"running", "5 min", "remaining", "ends at", "pause", "cancel", copy.T(copy.ControlsRunning)} {
		if !strings.Contains(plain, want) {
			t.Errorf("running view missing %q", want)
		}
	}
	m.state.Status = "paused"
	m.state.RemainingMS = 120000
	plain = assertFrame(t, m, 80, 24)
	for _, want := range []string{"PAUSED", "resume", "[Space] resume", "remaining · [Space] resume", copy.T(copy.ControlsPaused)} {
		if !strings.Contains(plain, want) {
			t.Errorf("paused view missing %q", want)
		}
	}
}

func TestViewFinishedAndCelebrationOnce(t *testing.T) {
	m, _ := testModel(80, 24)
	applyRunning(m)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
	})
	if !m.celebrating {
		t.Fatal("finish should start the celebration")
	}
	if m.confetti == nil || m.confetti.Count() != 220 {
		t.Fatalf("confetti count = %d, want 220", m.confetti.Count())
	}
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{"time's up!", "5 min completed", "repeat", "close", copy.T(copy.ControlsFinished), copy.T(copy.FinishedMeta)} {
		if !strings.Contains(plain, want) {
			t.Errorf("finished view missing %q", want)
		}
	}

	// The same finish_seq must not celebrate twice, even after the animation.
	m.celebrating = false
	m.confetti = nil
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, Label: "5 min", FinishSeq: 1,
	})
	if m.celebrating {
		t.Fatal("duplicate finish_seq started a second celebration")
	}

	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 600000, Label: "10 min", FinishSeq: 2,
	})
	if !m.celebrating {
		t.Fatal("a new finish_seq should celebrate")
	}
}

func TestViewCelebrationResize(t *testing.T) {
	m, _ := testModel(80, 24)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 60000, Label: "1 min", FinishSeq: 1,
	})
	m.width, m.height = 120, 40
	_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	assertFrame(t, m, 120, 40)
	m.width, m.height = 80, 24
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	assertFrame(t, m, 80, 24)
}

func press(m *Model, code rune, text string) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: code, Text: text})
	return cmd
}

func TestIdleInputKeys(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, '1', "1")
	press(m, '2', "2")
	press(m, '3', "3")
	if m.input != "123" {
		t.Fatalf("input = %q", m.input)
	}
	press(m, tea.KeyBackspace, "")
	if m.input != "12" {
		t.Fatalf("after backspace input = %q", m.input)
	}
	press(m, tea.KeyEscape, "")
	if m.input != "" {
		t.Fatalf("escape should clear input, got %q", m.input)
	}
	press(m, '9', "9")
	press(m, '0', "0")
	press(m, 's', "s")
	if m.input != "90s" {
		t.Fatalf("unit s should append, got %q", m.input)
	}
	press(m, tea.KeyEscape, "")
	press(m, 't', "t")
	if m.cfg.Theme != "ember" || m.pal.Name != "ember" {
		t.Fatalf("theme did not cycle: %q %q", m.cfg.Theme, m.pal.Name)
	}
}

func TestInvalidDurationShowsError(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, '1', "1")
	press(m, ':', ":")
	press(m, tea.KeyEnter, "")
	if m.inputErr == "" {
		t.Fatal("expected an inline error")
	}
	plain := assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, "Use 25") {
		t.Fatalf("error not rendered: %q", plain)
	}
}

func TestModalsOpenAndClose(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 'p', "p")
	if m.modal != modalPresets {
		t.Fatalf("modal = %v, want presets", m.modal)
	}
	assertFrame(t, m, 80, 24)
	press(m, 'a', "a")
	if m.modal != modalPresetForm {
		t.Fatalf("modal = %v, want form", m.modal)
	}
	assertFrame(t, m, 80, 24)
	press(m, tea.KeyEscape, "")
	if m.modal != modalPresets {
		t.Fatalf("escape should return to presets, got %v", m.modal)
	}
	press(m, tea.KeyEscape, "")
	if m.modal != modalNone {
		t.Fatalf("escape should close presets, got %v", m.modal)
	}
	press(m, '?', "?")
	if m.modal != modalHelp {
		t.Fatalf("modal = %v, want help", m.modal)
	}
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{
		copy.T(copy.HelpIdle), copy.T(copy.HelpTimer), copy.T(copy.HelpModal),
		copy.T(copy.HelpSound), copy.T(copy.HelpPresets),
		copy.T(copy.HelpFormats), copy.T(copy.HelpFinished),
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("help is missing %q", want)
		}
	}
	press(m, tea.KeyEnter, "")
	if m.modal != modalNone {
		t.Fatalf("help did not close")
	}
}

func TestPresetKeyboardNavigation(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 'p', "p")
	if m.presetFocus != presetFocusList {
		t.Fatalf("initial preset focus = %d, want list", m.presetFocus)
	}
	plain := assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, copy.T(copy.PresetsHint)) {
		t.Fatal("presets modal is missing the keyboard hint")
	}

	press(m, tea.KeyTab, "")
	if m.presetFocus != 0 {
		t.Fatalf("tab focus = %d, want add", m.presetFocus)
	}
	plain = assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, copy.T(copy.PresetsActionHint)) {
		t.Fatalf("action bar hint missing: %q", plain)
	}

	press(m, tea.KeyEnter, "")
	if m.modal != modalPresetForm {
		t.Fatalf("enter on add opened %v, want form", m.modal)
	}
	press(m, tea.KeyEscape, "")
	if m.modal != modalPresets {
		t.Fatalf("escape from form = %v, want presets", m.modal)
	}

	press(m, tea.KeyRight, "")
	press(m, tea.KeyRight, "")
	if m.presetFocus != 2 {
		t.Fatalf("right focus = %d, want delete", m.presetFocus)
	}
	press(m, tea.KeyEnter, "")
	if m.modal != modalConfirmDelete {
		t.Fatalf("enter on delete = %v, want confirmation", m.modal)
	}
	press(m, tea.KeyEscape, "")
	if m.modal != modalPresets || m.presetFocus != 2 {
		t.Fatalf("cancel should return to delete: modal=%v focus=%d", m.modal, m.presetFocus)
	}

	press(m, tea.KeyRight, "")
	if m.presetFocus != 3 {
		t.Fatalf("focus = %d, want defaults", m.presetFocus)
	}
	press(m, tea.KeyEnter, "")
	if m.modal != modalPresets {
		t.Fatalf("defaults should keep the modal open, got %v", m.modal)
	}
	press(m, tea.KeyRight, "")
	if m.presetFocus != 4 {
		t.Fatalf("focus = %d, want start", m.presetFocus)
	}
	press(m, tea.KeyEnter, "")
	if m.modal != modalNone {
		t.Fatalf("start should close the modal, got %v", m.modal)
	}

	press(m, 'p', "p")
	press(m, tea.KeyTab, "")
	press(m, tea.KeyDown, "")
	if m.presetFocus != presetFocusList {
		t.Fatalf("down focus = %d, want list", m.presetFocus)
	}
	if m.presetCursor != 1 {
		t.Fatalf("down cursor = %d, want 1", m.presetCursor)
	}

	press(m, tea.KeyTab, "")
	press(m, tea.KeyLeft, "")
	if m.presetFocus != 5 {
		t.Fatalf("left focus = %d, want close", m.presetFocus)
	}
	press(m, tea.KeyEnter, "")
	if m.modal != modalNone {
		t.Fatalf("close action left modal %v open", m.modal)
	}

	m.width, m.height = 120, 40
	press(m, 'p', "p")
	press(m, tea.KeyTab, "")
	assertFrame(t, m, 120, 40)
}

func TestPresetLetterShortcuts(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 'p', "p")
	press(m, 'e', "e")
	if m.modal != modalPresetForm {
		t.Fatalf("e opened %v, want the edit form", m.modal)
	}
	press(m, tea.KeyEscape, "")
	press(m, 'd', "d")
	if m.modal != modalConfirmDelete {
		t.Fatalf("d opened %v, want the delete confirmation", m.modal)
	}
	press(m, tea.KeyEscape, "")
	press(m, 'a', "a")
	if m.modal != modalPresetForm {
		t.Fatalf("a opened %v, want the add form", m.modal)
	}
	press(m, tea.KeyEscape, "")
	press(m, 'r', "r")
	if m.modal != modalPresets {
		t.Fatalf("r left modal %v open", m.modal)
	}
	press(m, tea.KeyTab, "")
	press(m, tea.KeyEscape, "")
	if m.modal != modalNone {
		t.Fatalf("escape from the action bar left modal %v open", m.modal)
	}
}

func TestPresetKeyboardSkipsDisabledActions(t *testing.T) {
	m, _ := testModel(80, 24)
	m.cfg.Presets = nil
	press(m, 'p', "p")
	press(m, tea.KeyTab, "")
	if m.presetFocus != 0 {
		t.Fatalf("tab focus = %d, want add", m.presetFocus)
	}
	press(m, tea.KeyRight, "")
	if m.presetFocus != 3 {
		t.Fatalf("focus = %d, want defaults (edit/delete skipped)", m.presetFocus)
	}
	press(m, tea.KeyRight, "")
	if m.presetFocus != 5 {
		t.Fatalf("focus = %d, want close (start skipped)", m.presetFocus)
	}
	press(m, tea.KeyRight, "")
	if m.presetFocus != 0 {
		t.Fatalf("focus = %d, want add after wrapping", m.presetFocus)
	}
	plain := assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, copy.T(copy.PresetsEmpty)) {
		t.Fatal("empty state not rendered")
	}
}

func TestPresetFocusClampsWhenPresetsDisappear(t *testing.T) {
	m, _ := testModel(80, 24)
	m.presetFocus = 1 // edit
	m.cfg.Presets = nil
	m.clampPresetFocus()
	if m.presetFocus != 3 {
		t.Fatalf("focus = %d, want defaults", m.presetFocus)
	}
}

func TestRunningKeys(t *testing.T) {
	m, _ := testModel(80, 24)
	applyRunning(m)
	press(m, tea.KeyEscape, "")
	if m.modal != modalConfirmStop {
		t.Fatalf("escape should ask for confirmation, got %v", m.modal)
	}
	press(m, tea.KeyEscape, "")
	if m.modal != modalNone {
		t.Fatal("second escape should cancel")
	}
	press(m, '+', "+")
	if m.status() != engine.Running {
		t.Fatal("adjust should not change local status without a daemon")
	}
}

func TestFinishedKeys(t *testing.T) {
	m, _ := testModel(80, 24)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 60000, Label: "1 min", FinishSeq: 1,
	})
	press(m, 'r', "r")
	if m.celebrating {
		t.Fatal("repeat should end the celebration locally")
	}
}

func TestApplyConfig(t *testing.T) {
	m, _ := testModel(80, 24)
	cfg := config.Default()
	cfg.Theme = config.ThemeEmber
	cfg.Sound = false
	cfg.SoundName = config.SoundMarimba
	cfg.Volume = 35
	cfg.Presets = cfg.Presets[:1]
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	if m.pal.Name != config.ThemeEmber || m.cfg.Sound || len(m.cfg.Presets) != 1 ||
		m.cfg.SoundName != config.SoundMarimba || m.cfg.Volume != 35 {
		t.Fatalf("config not applied: %+v", m.cfg)
	}
}

func TestApplyConfigFallsBackMissingSound(t *testing.T) {
	m, _ := testModel(80, 24)
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.ConfigPayload{
		Theme: config.ThemeMono,
		Sound: true,
	}})
	if m.cfg.SoundName != config.DefaultSound || m.cfg.Volume != config.DefaultVolume {
		t.Fatalf("missing sound fields did not fall back: %+v", m.cfg)
	}
}

func TestSoundModal(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 'v', "v")
	if m.modal != modalSound {
		t.Fatalf("modal = %v, want sound", m.modal)
	}
	if m.soundCursor != 2 {
		t.Fatalf("sound cursor = %d, want digital", m.soundCursor)
	}
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{"Sound", "Digital", "Volume", "80%", "preview", "mute", copy.T(copy.SoundHint), copy.T(copy.SoundActionHint)} {
		if !strings.Contains(plain, want) {
			t.Errorf("sound modal missing %q", want)
		}
	}

	press(m, tea.KeyDown, "")
	if m.cfg.SoundName != config.SoundDeep || m.soundCursor != 3 {
		t.Fatalf("down did not select deep: %+v", m.cfg)
	}
	press(m, tea.KeyUp, "")
	if m.cfg.SoundName != config.SoundDigital {
		t.Fatalf("up did not return to digital: %+v", m.cfg)
	}
	press(m, tea.KeyLeft, "")
	if m.cfg.Volume != 75 {
		t.Fatalf("left volume = %d, want 75", m.cfg.Volume)
	}
	press(m, tea.KeyRight, "")
	if m.cfg.Volume != 80 {
		t.Fatalf("right volume = %d, want 80", m.cfg.Volume)
	}
	press(m, 'm', "m")
	if m.cfg.Sound {
		t.Fatal("m should mute the sound")
	}
	press(m, tea.KeyEscape, "")
	if m.modal != modalNone {
		t.Fatal("escape should close the sound modal")
	}

	m.width, m.height = 120, 40
	press(m, 'v', "v")
	assertFrame(t, m, 120, 40)
}

func TestSoundModalDuringRunning(t *testing.T) {
	m, _ := testModel(80, 24)
	applyRunning(m)
	press(m, 'v', "v")
	if m.modal != modalSound {
		t.Fatalf("modal = %v, want sound during running", m.modal)
	}
	assertFrame(t, m, 80, 24)
}

func TestVolumeClampsAtBounds(t *testing.T) {
	m, _ := testModel(80, 24)
	m.cfg.Volume = 0
	press(m, 'v', "v")
	press(m, tea.KeyLeft, "")
	if m.cfg.Volume != 0 {
		t.Fatalf("volume below zero = %d", m.cfg.Volume)
	}
	m.cfg.Volume = 100
	press(m, tea.KeyRight, "")
	if m.cfg.Volume != 100 {
		t.Fatalf("volume above 100 = %d", m.cfg.Volume)
	}
}

func TestManyPresetsKeepAddChip(t *testing.T) {
	m, _ := testModel(80, 24)
	for i := 0; i < 16; i++ {
		m.cfg.Presets = append(m.cfg.Presets, config.Preset{
			ID: "x" + string(rune('a'+i)), Label: "Preset " + string(rune('A'+i)), Seconds: 60 + i,
		})
	}
	for _, selected := range []int{0, 7, 18} {
		m.selected = selected
		plain := assertFrame(t, m, 80, 24)
		if !strings.Contains(plain, "[ + ]") {
			t.Fatalf("add chip missing with selected=%d", selected)
		}
	}
}

func TestFormAndThemesRender(t *testing.T) {
	for _, name := range []string{"midnight", "ember", "mono"} {
		m, _ := testModel(80, 24)
		m.cfg.Theme = name
		m.pal = theme.Get(name)
		press(m, 'p', "p")
		press(m, 'a', "a")
		plain := assertFrame(t, m, 80, 24)
		for _, want := range []string{"New preset", "Name", "Duration", "save", "cancel", copy.T(copy.PresetFormHint)} {
			if !strings.Contains(plain, want) {
				t.Errorf("%s form missing %q", name, want)
			}
		}
	}
}

func TestRemainingInterpolates(t *testing.T) {
	m, clock := testModel(80, 24)
	applyRunning(m)
	if got := m.remaining(); got != 271500*time.Millisecond {
		t.Fatalf("remaining = %v", got)
	}
	clock.Advance(1500 * time.Millisecond)
	if got := m.remaining(); got != 270*time.Second {
		t.Fatalf("interpolated remaining = %v, want 4m30s", got)
	}
	m.connected = false
	if got := m.remaining(); got != 271500*time.Millisecond {
		t.Fatalf("disconnected remaining should freeze, got %v", got)
	}
}
