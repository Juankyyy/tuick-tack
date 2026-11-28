package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/config"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/theme"
)

func TestThemePickerPreviewAndCancel(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 't', "t")
	if m.modal != modalTheme || m.themeCursor != 0 || m.themeOriginal != config.DefaultTheme {
		t.Fatalf("picker did not open on the default: modal=%v cursor=%d original=%q",
			m.modal, m.themeCursor, m.themeOriginal)
	}

	press(m, tea.KeyDown, "")
	if m.pal.Name != config.ThemeTokyoNight {
		t.Fatalf("preview = %q, want %q", m.pal.Name, config.ThemeTokyoNight)
	}
	if m.cfg.Theme != config.DefaultTheme {
		t.Fatalf("preview changed the persisted theme: %q", m.cfg.Theme)
	}

	press(m, tea.KeyEscape, "")
	if m.modal != modalNone || m.pal.Name != config.DefaultTheme || m.cfg.Theme != config.DefaultTheme {
		t.Fatalf("cancel did not restore: modal=%v pal=%q cfg=%q",
			m.modal, m.pal.Name, m.cfg.Theme)
	}
}

func TestThemePickerApply(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 't', "t")
	press(m, tea.KeyDown, "")
	press(m, tea.KeyEnter, "")
	if m.modal != modalNone || m.cfg.Theme != config.ThemeTokyoNight ||
		m.pal.Name != config.ThemeTokyoNight {
		t.Fatalf("apply failed: modal=%v cfg=%q pal=%q",
			m.modal, m.cfg.Theme, m.pal.Name)
	}
}

func TestThemePickerOfflineKeepsPreview(t *testing.T) {
	m, _ := testModel(80, 24)
	m.connected = false
	press(m, 't', "t")
	press(m, tea.KeyDown, "")
	press(m, tea.KeyEnter, "")
	if m.modal != modalTheme || m.cfg.Theme != config.DefaultTheme ||
		m.pal.Name != config.ThemeTokyoNight || m.toast != copy.T(copy.ErrorOffline) {
		t.Fatalf("offline apply changed state: modal=%v cfg=%q pal=%q toast=%q",
			m.modal, m.cfg.Theme, m.pal.Name, m.toast)
	}
}

func TestThemePickerSurvivesConfigBroadcast(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 't', "t")
	press(m, tea.KeyDown, "")
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(config.Default())})
	if m.pal.Name != config.ThemeTokyoNight {
		t.Fatalf("config broadcast reset the preview: %q", m.pal.Name)
	}
	if m.themeOriginal != config.DefaultTheme {
		t.Fatalf("persisted reference = %q, want %q", m.themeOriginal, config.DefaultTheme)
	}
	press(m, tea.KeyEscape, "")
	if m.pal.Name != config.DefaultTheme {
		t.Fatalf("cancel after a broadcast = %q, want %q", m.pal.Name, config.DefaultTheme)
	}
}

func TestThemePickerMouseAndWheel(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 't', "t")
	m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.themeCursor != 1 || m.pal.Name != config.ThemeTokyoNight {
		t.Fatalf("wheel did not move the preview: cursor=%d pal=%q", m.themeCursor, m.pal.Name)
	}
	m.performHit(hitbox{action: hitThemeRow, index: 2})
	if m.themeCursor != 2 || m.pal.Name != config.ThemeDracula {
		t.Fatalf("row click did not preview: cursor=%d pal=%q", m.themeCursor, m.pal.Name)
	}
	m.performHit(hitbox{action: hitThemeClose})
	if m.modal != modalNone || m.pal.Name != config.DefaultTheme {
		t.Fatalf("close hit did not cancel: modal=%v pal=%q", m.modal, m.pal.Name)
	}

	press(m, 't', "t")
	m.performHit(hitbox{action: hitThemeRow, index: 1})
	m.performHit(hitbox{action: hitThemeApply})
	if m.modal != modalNone || m.cfg.Theme != config.ThemeTokyoNight {
		t.Fatalf("apply hit failed: modal=%v cfg=%q", m.modal, m.cfg.Theme)
	}
}

func TestThemePickerKeys(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, 't', "t")
	press(m, tea.KeyEnd, "")
	if m.themeCursor != len(config.Themes)-1 || m.pal.Name != config.ThemeVantablack {
		t.Fatalf("End = cursor %d pal %q", m.themeCursor, m.pal.Name)
	}
	press(m, tea.KeyHome, "")
	if m.themeCursor != 0 || m.pal.Name != config.DefaultTheme {
		t.Fatalf("Home = cursor %d pal %q", m.themeCursor, m.pal.Name)
	}
	press(m, tea.KeyPgDown, "")
	if m.themeCursor != themePage {
		t.Fatalf("PgDown = cursor %d, want %d", m.themeCursor, themePage)
	}
	press(m, tea.KeyPgUp, "")
	if m.themeCursor != 0 {
		t.Fatalf("PgUp = cursor %d, want 0", m.themeCursor)
	}
	press(m, tea.KeyPgUp, "")
	if m.themeCursor != 0 {
		t.Fatalf("PgUp at the top = cursor %d, want 0", m.themeCursor)
	}
	press(m, tea.KeyEnd, "")
	press(m, tea.KeyPgDown, "")
	if m.themeCursor != len(config.Themes)-1 {
		t.Fatalf("PgDown at the bottom = cursor %d, want %d", m.themeCursor, len(config.Themes)-1)
	}
}

func TestThemePickerRenders(t *testing.T) {
	for _, size := range [][2]int{{48, 22}, {80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		press(m, 't', "t")
		plain := assertFrame(t, m, size[0], size[1])
		for _, want := range []string{
			copy.T(copy.ThemeTitle),
			copy.ThemeLabel(config.DefaultTheme),
			copy.ThemeLabel(config.ThemeVantablack),
			copy.T(copy.ThemeHint),
		} {
			if !strings.Contains(plain, want) {
				t.Errorf("%dx%d picker missing %q", size[0], size[1], want)
			}
		}
	}
}

func TestAllThemesRenderAtSupportedSizes(t *testing.T) {
	for _, name := range config.Themes {
		for _, size := range [][2]int{{48, 22}, {80, 24}, {120, 40}} {
			m, _ := testModel(size[0], size[1])
			m.cfg.Theme = name
			m.pal = theme.Get(name)
			assertFrame(t, m, size[0], size[1])
		}
	}
}

func TestThemeIndexUnknown(t *testing.T) {
	if got := themeIndex("nope"); got != 0 {
		t.Fatalf("themeIndex(unknown) = %d, want 0", got)
	}
}
