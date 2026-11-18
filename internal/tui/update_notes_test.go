package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/install"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tui/canvas"
	"tuick-tack/internal/tui/copy"
)

const testReleaseNotes = "## 🎉 What's new in v9.9.9\n\n" +
	"### Notes in the dialog\n\n" +
	"The update dialog now shows what changed in the release you are about to install.\n\n" +
	"- The notes pane scrolls with PgUp and PgDn.\n" +
	"- The mouse wheel works too.\n\n" +
	"### A calmer finish\n\n" +
	"- The celebration fades after six seconds.\n" +
	"- Enter closes it without starting another countdown.\n\n" +
	"### Faster tray\n\n" +
	"- The tray icon stops pulsing while the timer is paused.\n" +
	"- Tooltips refresh ten times a second.\n\n" +
	"### LATER-MARKER\n\n" +
	"- This line only appears after scrolling to the end.\n\n" +
	"## ⬆️ Upgrading\n\n" +
	"Not part of the notes.\n\n" +
	"## 📦 Download\n\n" +
	"| File | Size |\n|---|---:|\n| app.exe | 10 MB |\n"

func notesPayload() *protocol.UpdatePayload {
	return &protocol.UpdatePayload{
		Latest: "v9.9.9", URL: "https://example.test/release",
		Setup: true, Portable: true, Notes: testReleaseNotes,
	}
}

func TestUpdateNotesPanelRenders(t *testing.T) {
	old := buildinfo.Version
	buildinfo.Version = "v0.2.0"
	defer func() { buildinfo.Version = old }()

	for _, size := range [][2]int{{48, 22}, {80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		m.setUpdate(notesPayload())
		m.installInfo = install.Info{Installed: true, Dir: `C:\Users\A\Documents\tuick-tack`}
		m.openUpdateModal()
		plain := assertFrame(t, m, size[0], size[1])
		for _, want := range []string{
			copy.T(copy.UpdateNotesTitle, "v9.9.9"),
			"Notes in the dialog",
			"Update available",
		} {
			if !strings.Contains(plain, want) {
				t.Errorf("%dx%d notes dialog missing %q:\n%s", size[0], size[1], want, plain)
			}
		}
		for _, unwanted := range []string{"Not part of the notes.", "Download", "10 MB"} {
			if strings.Contains(plain, unwanted) {
				t.Errorf("%dx%d notes dialog leaked %q:\n%s", size[0], size[1], unwanted, plain)
			}
		}
		if len(m.updateNotes) == 0 {
			t.Fatalf("%dx%d did not render notes", size[0], size[1])
		}
	}
}

func TestUpdateNotesScrolls(t *testing.T) {
	m, _ := testModel(80, 24)
	m.setUpdate(notesPayload())
	m.openUpdateModal()
	plain := assertFrame(t, m, 80, 24)
	if strings.Contains(plain, "LATER-MARKER") {
		t.Fatalf("the marker should start below the first page:\n%s", plain)
	}
	if m.notesScroll != 0 {
		t.Fatalf("initial scroll = %d", m.notesScroll)
	}

	press(m, tea.KeyPgDown, "")
	if m.notesScroll == 0 {
		t.Fatal("PgDown did not scroll the notes")
	}
	page := m.updateNotesPage()
	if page < 1 {
		t.Fatalf("notes page = %d", page)
	}

	press(m, tea.KeyEnd, "")
	if got, want := m.notesScroll, m.updateNotesMaxScroll(); got != want {
		t.Fatalf("End scroll = %d, want %d", got, want)
	}
	plain = assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, "LATER-MARKER") {
		t.Fatalf("the end of the notes is not visible:\n%s", plain)
	}

	press(m, tea.KeyPgDown, "")
	if got := m.notesScroll; got != m.updateNotesMaxScroll() {
		t.Fatalf("PgDown ran past the end: %d > %d", got, m.updateNotesMaxScroll())
	}

	press(m, tea.KeyHome, "")
	if m.notesScroll != 0 {
		t.Fatalf("Home scroll = %d", m.notesScroll)
	}
	plain = assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, "Notes in the dialog") {
		t.Fatalf("Home did not return to the top:\n%s", plain)
	}

	// The wheel is the pointer equivalent of PgUp/PgDn.
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.notesScroll == 0 {
		t.Fatal("wheel down did not scroll the notes")
	}
	_, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if m.notesScroll != 0 {
		t.Fatalf("wheel up did not return to the top: %d", m.notesScroll)
	}
}

func TestUpdateNotesNoPanelWithoutNotes(t *testing.T) {
	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{
		Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true,
	})
	m.openUpdateModal()
	if m.updateHasNotes() {
		t.Fatal("a payload without a body should not report notes")
	}
	if got := m.updateChooserHeight(); got != 18 {
		t.Fatalf("chooser height = %d, want 18", got)
	}
	plain := assertFrame(t, m, 80, 24)
	if strings.Contains(plain, copy.T(copy.UpdateNotesTitle, "v9.9.9")) {
		t.Fatalf("notes header drawn without notes:\n%s", plain)
	}
	if !strings.Contains(plain, copy.T(copy.UpdateHintTab)) {
		t.Fatalf("Tab should be advertised when both copies exist:\n%s", plain)
	}
	press(m, tea.KeyPgDown, "")
	press(m, tea.KeyEnd, "")
	if m.notesScroll != 0 {
		t.Fatalf("scroll moved without notes: %d", m.notesScroll)
	}
}

func TestUpdateNotesHint(t *testing.T) {
	old := buildinfo.Version
	buildinfo.Version = "v0.2.0"
	defer func() { buildinfo.Version = old }()

	m, _ := testModel(80, 24)
	m.setUpdate(notesPayload())
	m.openUpdateModal()
	assertFrame(t, m, 80, 24)
	layout := m.updateChooserLayout()
	hint := m.updateChooserHint(layout)
	for _, want := range []string{
		copy.T(copy.UpdateHintSelect),
		copy.T(copy.UpdateHintTab),
		copy.T(copy.UpdateHintNotes),
		copy.T(copy.UpdateHintLater),
	} {
		if !strings.Contains(hint, want) {
			t.Errorf("80x24 hint missing %q: %q", want, hint)
		}
	}
	if strings.Contains(hint, copy.T(copy.UpdateHintGithub)) {
		t.Errorf("80x24 hint should drop github before Tab or notes: %q", hint)
	}
	if canvas.Width(hint) > layout.innerW {
		t.Errorf("hint width = %d > %d: %q", canvas.Width(hint), layout.innerW, hint)
	}

	// Without notes there is room for github as well.
	m, _ = testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{
		Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true,
	})
	m.openUpdateModal()
	assertFrame(t, m, 80, 24)
	hint = m.updateChooserHint(m.updateChooserLayout())
	if !strings.Contains(hint, copy.T(copy.UpdateHintGithub)) ||
		!strings.Contains(hint, copy.T(copy.UpdateHintTab)) {
		t.Fatalf("no-notes hint = %q", hint)
	}

	// A single copy cannot switch with Tab, so it is not advertised.
	m, _ = testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{
		Latest: "v9.9.9", URL: "https://example.test/release", Portable: true, Notes: testReleaseNotes,
	})
	m.openUpdateModal()
	assertFrame(t, m, 80, 24)
	hint = m.updateChooserHint(m.updateChooserLayout())
	if strings.Contains(hint, copy.T(copy.UpdateHintTab)) {
		t.Fatalf("single-copy hint advertises Tab: %q", hint)
	}
	if !strings.Contains(hint, copy.T(copy.UpdateHintNotes)) {
		t.Fatalf("single-copy hint lost the notes: %q", hint)
	}

	// At 48 columns the full line does not fit; the short form is still valid
	// for the frame (Tab is documented in the help and README).
	m, _ = testModel(48, 22)
	m.setUpdate(notesPayload())
	m.openUpdateModal()
	assertFrame(t, m, 48, 22)
	layout = m.updateChooserLayout()
	hint = m.updateChooserHint(layout)
	if canvas.Width(hint) > layout.innerW {
		t.Fatalf("48x22 hint width = %d > %d: %q", canvas.Width(hint), layout.innerW, hint)
	}
}

func TestUpdateNotesCompactTypeRow(t *testing.T) {
	m, _ := testModel(80, 24)
	m.setUpdate(notesPayload())
	m.openUpdateModal()
	plain := assertFrame(t, m, 80, 24)
	found := false
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Setup ·") && strings.Contains(line, "Portable ·") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("80x24 should keep both type chips on one row:\n%s", plain)
	}

	m, _ = testModel(48, 22)
	m.setUpdate(notesPayload())
	m.openUpdateModal()
	plain = assertFrame(t, m, 48, 22)
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Setup") && strings.Contains(line, "Portable") {
			t.Fatalf("48x22 should stack the type rows:\n%s", plain)
		}
	}

	// The other-copy warning wraps to two lines at 44 columns and pushes the
	// notes pane down without covering the buttons.
	press(m, tea.KeyDown, "")
	plain = assertFrame(t, m, 48, 22)
	for _, want := range []string{"This is not the copy you use;", "carry over.", "[ install ]", "[Enter] select"} {
		if !strings.Contains(plain, want) {
			t.Errorf("48x22 other-copy view missing %q:\n%s", want, plain)
		}
	}
}

func TestUpdateNotesManualResult(t *testing.T) {
	m, _ := testModel(80, 24)
	_, _ = m.Update(serverEventMsg{msg: protocol.NewUpdateResult(protocol.UpdateResultMessage{
		Current:  buildinfo.Version,
		Latest:   "v9.9.9",
		URL:      "https://example.test/release",
		Setup:    true,
		Portable: true,
		Notes:    testReleaseNotes,
	})})
	if m.update == nil || m.update.Notes != testReleaseNotes {
		t.Fatalf("manual result lost the notes: %+v", m.update)
	}
	m.openUpdateModal()
	if !strings.Contains(assertFrame(t, m, 80, 24), copy.T(copy.UpdateNotesTitle, "v9.9.9")) {
		t.Fatal("manual result did not open the notes panel")
	}
}

func TestUpdateTypeChipFollowsTab(t *testing.T) {
	assertSelected := func(t *testing.T, plain, selected, other string) {
		t.Helper()
		if !strings.Contains(plain, "› "+selected) {
			t.Errorf("the %s chip is not selected:\n%s", selected, plain)
		}
		if strings.Contains(plain, "› "+other) {
			t.Errorf("the %s chip should not be selected:\n%s", other, plain)
		}
	}

	// Installed copy: Setup is in use, so Tab must select Portable.
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		m.setUpdate(notesPayload())
		m.installInfo = install.Info{Installed: true, Dir: `C:\Users\A\Documents\tuick-tack`}
		m.openUpdateModal()
		plain := assertFrame(t, m, size[0], size[1])
		assertSelected(t, plain, "Setup", "Portable")
		if strings.Contains(plain, copy.T(copy.UpdateOtherShort)) {
			t.Fatalf("%dx%d current-copy view shows the warning:\n%s", size[0], size[1], plain)
		}

		press(m, tea.KeyTab, "")
		plain = assertFrame(t, m, size[0], size[1])
		assertSelected(t, plain, "Portable", "Setup")
		if !strings.Contains(plain, copy.T(copy.UpdateOtherShort)) ||
			!strings.Contains(plain, "[ download ]") {
			t.Fatalf("%dx%d Tab did not select Portable/download:\n%s", size[0], size[1], plain)
		}

		press(m, tea.KeyTab, "")
		plain = assertFrame(t, m, size[0], size[1])
		assertSelected(t, plain, "Setup", "Portable")
		if !strings.Contains(plain, "[ install ]") ||
			strings.Contains(plain, copy.T(copy.UpdateOtherShort)) {
			t.Fatalf("%dx%d Tab did not return to Setup/install:\n%s", size[0], size[1], plain)
		}
	}

	// Portable copy: Tab must select Setup and its install action.
	m, _ := testModel(80, 24)
	m.setUpdate(notesPayload())
	m.openUpdateModal()
	plain := assertFrame(t, m, 80, 24)
	assertSelected(t, plain, "Portable", "Setup")
	press(m, tea.KeyTab, "")
	plain = assertFrame(t, m, 80, 24)
	assertSelected(t, plain, "Setup", "Portable")
	if !strings.Contains(plain, copy.T(copy.UpdateOtherShort)) ||
		!strings.Contains(plain, "[ install ]") {
		t.Fatalf("Tab did not select Setup/install:\n%s", plain)
	}

	// At 48x22 the chips stack; the selected row must still move.
	m, _ = testModel(48, 22)
	m.setUpdate(notesPayload())
	m.openUpdateModal()
	plain = assertFrame(t, m, 48, 22)
	assertSelected(t, plain, "Portable", "Setup")
	press(m, tea.KeyTab, "")
	plain = assertFrame(t, m, 48, 22)
	assertSelected(t, plain, "Setup", "Portable")
	if !strings.Contains(plain, "[ install ]") {
		t.Fatalf("48x22 Tab did not select install:\n%s", plain)
	}
}

func TestUpdateNotesNeverLeaksWideRunes(t *testing.T) {
	notes := "## What's new in v9.9.9\n\n" +
		"Emoji 🚀🎉 and CJK 日本語 and combining e\u0301 and bidi \u202eRTL\u202c.\n\n" +
		"\x1b]0;title\x07 control \x00 text.\n\n" +
		"## Download\n\n| File | Size |\n|---|---:|\n| app.exe | 10 MB |\n"
	for _, size := range [][2]int{{48, 22}, {80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		m.setUpdate(&protocol.UpdatePayload{
			Latest: "v9.9.9", URL: "https://example.test/release",
			Setup: true, Portable: true, Notes: notes,
		})
		m.openUpdateModal()
		plain := assertFrame(t, m, size[0], size[1])
		for _, unwanted := range []string{"🚀", "日本", "\x1b", "\x00", "10 MB"} {
			if strings.Contains(plain, unwanted) {
				t.Errorf("%dx%d frame contains %q", size[0], size[1], unwanted)
			}
		}
	}
}
