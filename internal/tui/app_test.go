package tui

import (
	"errors"
	"fmt"
	"image/color"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/config"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/install"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/tui/bigfont"
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

func TestDaemonClosingExitsWithoutReconnect(t *testing.T) {
	m, _ := testModel(80, 24)
	applyRunning(m)
	m.reconnecting = true

	cmd := m.handleServerEvent(serverEventMsg{msg: protocol.NewDaemonClosing()})
	if cmd == nil {
		t.Fatal("daemon closing should quit the TUI")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("daemon closing should return tea.Quit")
	}
	if m.connected || m.reconnecting || !m.DaemonClosing() {
		t.Fatalf("state after closing: connected=%v reconnecting=%v closing=%v",
			m.connected, m.reconnecting, m.DaemonClosing())
	}
	if m.HasActiveTimer() {
		t.Fatal("a quit daemon leaves no timer in the tray")
	}
}

func TestQuitAllKeyClosesEverythingFromEveryScreen(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
	}{
		{"idle", func(*Model) {}},
		{"running", applyRunning},
		{"paused", func(m *Model) {
			m.applyState(protocol.StateMessage{
				Type: protocol.TypeState, Status: "paused",
				DurationMS: 300000, RemainingMS: 271500, Label: "5 min", PresetID: "p5",
			})
		}},
		{"finished", func(m *Model) {
			m.applyState(protocol.StateMessage{
				Type: protocol.TypeState, Status: "finished",
				DurationMS: 300000, Label: "5 min", PresetID: "p5", FinishSeq: 1,
			})
		}},
		{"help", func(m *Model) { m.modal = modalHelp }},
		{"confirm stop", func(m *Model) { m.modal = modalConfirmStop }},
		{"confirm delete", func(m *Model) { m.modal = modalConfirmDelete }},
		{"presets", func(m *Model) { m.modal = modalPresets }},
		{"sound", func(m *Model) { m.modal = modalSound }},
		{"update", func(m *Model) { m.modal = modalUpdate }},
		{"stats", func(m *Model) { m.modal = modalStats }},
		{"compact", func(m *Model) { m.width, m.height = 40, 20 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := testModel(80, 24)
			tt.setup(m)

			cmd := press(m, 'Q', "Q")
			if cmd == nil {
				t.Fatal("Shift+Q should return a command")
			}
			if !m.QuitAll() {
				t.Fatal("Shift+Q should quit the daemon too")
			}
			if m.connected {
				t.Fatal("Shift+Q should drop the TUI connection")
			}
			if m.modal != modalNone {
				t.Fatalf("modal = %v, want none", m.modal)
			}
		})
	}
}

func TestLowercaseQOnlyClosesTheTUI(t *testing.T) {
	m, _ := testModel(80, 24)
	cmd := press(m, 'q', "q")
	if cmd == nil {
		t.Fatal("q should return a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q should return tea.Quit")
	}
	if m.QuitAll() {
		t.Fatal("q should leave the daemon running")
	}
}

func TestShiftQTypesInPresetNameInsteadOfQuitting(t *testing.T) {
	m, _ := testModel(80, 24)
	m.openForm(formAdd, config.Preset{})

	if cmd := press(m, 'Q', "Q"); cmd != nil {
		t.Fatal("Shift+Q in the preset name should not return a command")
	}
	if m.QuitAll() {
		t.Fatal("Shift+Q in the preset name should not quit the daemon")
	}
	if m.modal != modalPresetForm {
		t.Fatalf("modal = %v, want the form", m.modal)
	}
	if m.form.label != "Q" {
		t.Fatalf("label = %q, want Q", m.form.label)
	}

	press(m, tea.KeyTab, "")
	if m.form.focus != 1 {
		t.Fatalf("focus = %d, want the duration", m.form.focus)
	}
	press(m, 'Q', "Q")
	if !m.QuitAll() {
		t.Fatal("Shift+Q outside the name field should quit everything")
	}
}

func TestQuitAllFailedConnectStillTriesToQuitDaemon(t *testing.T) {
	m, _ := testModel(80, 24)
	m.quitting = true

	cmd := m.handleConnected(connectedMsg{err: errors.New("handshake")})
	if cmd == nil {
		t.Fatal("a failed connect while quitting should keep trying")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("failed-connect command = %#v, want a two-command batch", msg)
	}
	if _, ok := batch[0]().(tea.QuitMsg); !ok {
		t.Fatal("the best-effort quit should end the TUI")
	}
}

func TestQuitAllSendFailureQuitsWithoutReconnect(t *testing.T) {
	m, _ := testModel(80, 24)
	m.quitting = true
	m.connected = true

	_, cmd := m.Update(sentMsg{err: errors.New("pipe closed")})
	if cmd == nil {
		t.Fatal("a failed quit send should quit the TUI")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("a failed quit send should return tea.Quit")
	}
	if m.reconnecting || m.connected {
		t.Fatalf("state after failure: reconnecting=%v connected=%v", m.reconnecting, m.connected)
	}
}

func TestQuitAllLostConnectionQuitsWithoutReconnect(t *testing.T) {
	m, _ := testModel(80, 24)
	m.quitting = true

	cmd := m.handleServerEvent(serverEventMsg{err: errors.New("pipe closed")})
	if cmd == nil {
		t.Fatal("a lost pipe while quitting should quit the TUI")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("a lost pipe while quitting should return tea.Quit")
	}
	if m.reconnecting {
		t.Fatal("quit-all should never reconnect")
	}
}

func TestQuitAllTimeoutQuitsTheTUI(t *testing.T) {
	m, _ := testModel(80, 24)
	m.quitting = true

	_, cmd := m.Update(quitAllTimeoutMsg{})
	if cmd == nil {
		t.Fatal("the quit timeout should quit the TUI")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("the quit timeout should return tea.Quit")
	}
	if m.connected {
		t.Fatal("the quit timeout should drop the connection")
	}
	if m.reconnecting {
		t.Fatal("the quit timeout should never reconnect")
	}
}

// quitBackend records the commands that reach the daemon side of the pipe.
type quitBackend struct {
	cmds chan protocol.ClientMessage
}

func (b *quitBackend) Config() config.Config           { return config.Default() }
func (b *quitBackend) Snapshot() engine.State          { return engine.State{} }
func (b *quitBackend) Update() *protocol.UpdatePayload { return nil }
func (b *quitBackend) Command(_ *ipc.Session, msg protocol.ClientMessage) {
	b.cmds <- msg
}

func TestQuitAllSendsQuitDaemon(t *testing.T) {
	pipe := testEndpoint(t)
	backend := &quitBackend{cmds: make(chan protocol.ClientMessage, 1)}
	server := ipc.NewServer(pipe, "test", backend, true)
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Close()

	client, _, err := ipc.Dial(pipe)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	m, _ := testModel(80, 24)
	m.pipe = pipe
	m.client = client
	m.connected = true

	cmd := press(m, 'Q', "Q")
	if !m.QuitAll() {
		t.Fatal("Shift+Q should quit the daemon too")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("quit command = %#v, want a two-command batch", msg)
	}
	if sent, ok := batch[0]().(sentMsg); !ok || sent.err != nil {
		t.Fatalf("quit send = %#v, want a successful sentMsg", sent)
	}

	select {
	case got := <-backend.cmds:
		if got.Type != protocol.TypeQuitDaemon {
			t.Fatalf("daemon got %q, want %q", got.Type, protocol.TypeQuitDaemon)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the daemon did not receive quit_daemon")
	}
}

func TestQuitAllDuringReconnectQuitsTheNewDaemon(t *testing.T) {
	pipe := testEndpoint(t)
	backend := &quitBackend{cmds: make(chan protocol.ClientMessage, 1)}
	server := ipc.NewServer(pipe, "test", backend, true)
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Close()

	client, hello, err := ipc.Dial(pipe)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	m, _ := testModel(80, 24)
	m.quitting = true
	cmd := m.handleConnected(connectedMsg{client: client, hello: hello})
	if cmd == nil {
		t.Fatal("a late connection while quitting should exit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("a late connection while quitting should return tea.Quit")
	}

	select {
	case got := <-backend.cmds:
		if got.Type != protocol.TypeQuitDaemon {
			t.Fatalf("daemon got %q, want %q", got.Type, protocol.TypeQuitDaemon)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the daemon started during the reconnect did not receive quit_daemon")
	}
}

func TestViewIdle(t *testing.T) {
	m, _ := testModel(80, 24)
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{"tuick·tack", "Ready when you are", "5 min", "12:30", "[p] presets", copy.T(copy.ControlsIdle), copy.T(copy.UpdateHint)} {
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

func TestHeaderShowsVersion(t *testing.T) {
	old := buildinfo.Version
	buildinfo.Version = "v0.2.0"
	defer func() { buildinfo.Version = old }()

	want := copy.T(copy.Brand) + " v0.2.0"
	for _, size := range [][2]int{{48, 24}, {80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		plain := assertFrame(t, m, size[0], size[1])
		header := strings.Split(plain, "\n")[m.layout(size[0], size[1]).headerY]
		if !strings.Contains(header, want) {
			t.Fatalf("%dx%d header missing %q: %q", size[0], size[1], want, header)
		}
	}

	m, _ := testModel(80, 24)
	l := m.layout(80, 24)
	c := canvas.New(80, 24)
	m.drawHeader(c, l.x, l.headerY, l.w)
	brandX := l.x + 2
	versionX := brandX + canvas.Width(copy.T(copy.Brand)) + 1
	versionCell := c.At(versionX, l.headerY)
	if versionCell.FG != headerVersionColor(m.pal) {
		t.Fatalf("version FG = %+v, want %+v", versionCell.FG, headerVersionColor(m.pal))
	}
	if versionCell.Bold {
		t.Fatal("the version suffix should not be bold")
	}
	if c.At(brandX, l.headerY).FG == versionCell.FG {
		t.Fatal("the version suffix should be quieter than the wordmark")
	}

	// The notice still fits next to the version at the reference widths.
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release"})
	plain := assertFrame(t, m, 80, 24)
	header := strings.Split(plain, "\n")[m.layout(80, 24).headerY]
	if !strings.Contains(header, want) || !strings.Contains(header, "["+copy.T(copy.UpdateAvailable)) {
		t.Fatalf("header should keep the version and the notice: %q", header)
	}
}

func TestUpdateHintAndClick(t *testing.T) {
	m, _ := testModel(80, 24)
	url := "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9"
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: url})
	opened := ""
	m.openURL = func(u string) error {
		opened = u
		return nil
	}

	label := "[" + copy.T(copy.UpdateAvailable) + " " + copy.T(copy.UpdateArrow) + "]"
	plain := assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, label) {
		t.Fatalf("update notice missing:\n%s", plain)
	}
	header := strings.Split(plain, "\n")[m.layout(80, 24).headerY]
	if !strings.Contains(header, copy.T(copy.Brand)) ||
		!strings.Contains(header, label) ||
		!strings.Contains(header, "READY") {
		t.Fatalf("header = %q", header)
	}
	if strings.Index(header, copy.T(copy.Brand)) > strings.Index(header, label) {
		t.Fatalf("the notice should follow the wordmark: %q", header)
	}

	var hit *hitbox
	for i := range m.hits {
		if m.hits[i].action == hitUpdate {
			hit = &m.hits[i]
		}
	}
	if hit == nil {
		t.Fatal("the update notice has no click target")
	}
	_, _ = m.Update(tea.MouseMotionMsg{X: hit.x, Y: hit.y})
	if !m.hoverUpdate {
		t.Fatal("motion over the notice did not set the hover state")
	}
	_, _ = m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	if m.hoverUpdate {
		t.Fatal("motion away from the notice did not clear the hover state")
	}
	_, _ = m.Update(tea.MouseClickMsg{X: hit.x, Y: hit.y, Button: tea.MouseLeft})
	if m.modal != modalUpdate {
		t.Fatalf("clicking the notice opened modal %v, want the update modal", m.modal)
	}
	assertFrame(t, m, 80, 24)
	var release *hitbox
	for i := range m.hits {
		if m.hits[i].action == hitUpdateRelease {
			release = &m.hits[i]
		}
	}
	if release == nil {
		t.Fatal("the update modal has no release target")
	}
	_, _ = m.Update(tea.MouseClickMsg{X: release.x, Y: release.y, Button: tea.MouseLeft})
	if opened != url {
		t.Fatalf("opened %q, want %q", opened, url)
	}

	m.openURL = func(string) error { return errors.New("no browser") }
	m.Update(tea.MouseClickMsg{X: release.x, Y: release.y, Button: tea.MouseLeft})
	if m.toast != copy.T(copy.ErrorOpenURL) {
		t.Fatalf("toast = %q, want %q", m.toast, copy.T(copy.ErrorOpenURL))
	}

	// A build that is already current shows nothing.
	m.setUpdate(&protocol.UpdatePayload{Latest: buildinfo.Version, URL: url})
	if m.update != nil {
		t.Fatalf("same version should clear the notice: %+v", m.update)
	}
	if m.modal != modalNone {
		t.Fatal("clearing the update should close the modal")
	}
}

func TestUpdateChooserRenders(t *testing.T) {
	old := buildinfo.Version
	buildinfo.Version = "v0.2.0"
	defer func() { buildinfo.Version = old }()

	transition := copy.T(copy.UpdateFromTo, "v0.2.0", "v9.9.9")
	for _, size := range [][2]int{{48, 22}, {64, 24}, {80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		m.setUpdate(&protocol.UpdatePayload{
			Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true,
		})
		m.installInfo = install.Info{Installed: true, Dir: `C:\Users\A\Documents\tuick-tack`}
		m.openUpdateModal()
		plain := assertFrame(t, m, size[0], size[1])
		for _, want := range []string{
			transition, "Installed",
			"Setup", "Portable", "Not in use", "install", "github", "later",
		} {
			if !strings.Contains(plain, want) {
				t.Errorf("%dx%d update chooser missing %q:\n%s", size[0], size[1], want, plain)
			}
		}
	}

	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true})
	m.openUpdateModal()
	plain := assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, "Portable · tuick-tack.exe") {
		t.Fatalf("portable badge missing:\n%s", plain)
	}
	if !strings.Contains(plain, "[Enter] select · [Tab] type · [g] github · [Esc] later") {
		t.Fatalf("portable hint should advertise Tab and use the select wording:\n%s", plain)
	}
	if strings.Contains(plain, "[←") || strings.Contains(plain, "[→") {
		t.Fatalf("the arrow keys must not be advertised in the hint:\n%s", plain)
	}
	if m.updateType != updateTypePortable || m.updateFocus != hitUpdatePortable {
		t.Fatalf("portable copy should start on download, type=%q focus=%v", m.updateType, m.updateFocus)
	}
	if !strings.Contains(plain, "In use") || !strings.Contains(plain, "Not installed") {
		t.Fatalf("portable type statuses missing:\n%s", plain)
	}
	press(m, tea.KeyRight, "")
	if m.updateFocus != hitUpdateRelease {
		t.Fatalf("missing install action should be skipped, got %v", m.updateFocus)
	}
}

func TestUpdateTypeSelection(t *testing.T) {
	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true})
	m.installInfo = install.Info{Installed: true, Dir: `C:\Users\A\Documents\tuick-tack`}
	m.openUpdateModal()
	if m.updateType != updateTypeSetup || m.updateFocus != hitUpdateInstall {
		t.Fatalf("installed copy should start on Setup/install, type=%q focus=%v", m.updateType, m.updateFocus)
	}

	// The arrows scroll the notes; Tab is the only key that changes the copy.
	press(m, tea.KeyDown, "")
	if m.updateType != updateTypeSetup || m.updateFocus != hitUpdateInstall {
		t.Fatalf("down must not change the copy, type=%q focus=%v", m.updateType, m.updateFocus)
	}
	press(m, tea.KeyUp, "")
	if m.updateType != updateTypeSetup || m.updateFocus != hitUpdateInstall {
		t.Fatalf("up must not change the copy, type=%q focus=%v", m.updateType, m.updateFocus)
	}

	press(m, tea.KeyTab, "")
	if m.updateType != updateTypePortable || m.updateFocus != hitUpdatePortable {
		t.Fatalf("Tab should select Portable/download, type=%q focus=%v", m.updateType, m.updateFocus)
	}
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{
		copy.T(copy.UpdateOtherShort),
		"download",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("other-copy warning missing %q:\n%s", want, plain)
		}
	}

	press(m, tea.KeyTab, "")
	if m.updateType != updateTypeSetup || m.updateFocus != hitUpdateInstall {
		t.Fatalf("Tab should return to Setup/install, type=%q focus=%v", m.updateType, m.updateFocus)
	}
	plain = assertFrame(t, m, 80, 24)
	if strings.Contains(plain, copy.T(copy.UpdateOtherShort)) {
		t.Fatalf("current-copy view should not show the other-copy warning:\n%s", plain)
	}

	// A missing asset keeps the current type visible but unavailable and starts
	// on the only type that can actually be downloaded.
	m, _ = testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Portable: true})
	m.installInfo = install.Info{Installed: true}
	m.openUpdateModal()
	if m.updateType != updateTypePortable || !strings.Contains(assertFrame(t, m, 80, 24), "Not available") {
		t.Fatalf("missing setup should fall back to Portable, type=%q", m.updateType)
	}
}

func TestUpdateTypeTab(t *testing.T) {
	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true})
	m.installInfo = install.Info{Installed: true, Dir: `C:\Users\A\Documents\tuick-tack`}
	m.openUpdateModal()
	if m.updateType != updateTypeSetup {
		t.Fatalf("installed copy should start on Setup, got %q", m.updateType)
	}

	press(m, tea.KeyTab, "")
	if m.updateType != updateTypePortable || m.updateFocus != hitUpdatePortable {
		t.Fatalf("Tab should select Portable/download, type=%q focus=%v", m.updateType, m.updateFocus)
	}
	plain := assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, copy.T(copy.UpdateOtherShort)) {
		t.Fatalf("Tab should show the other-copy warning:\n%s", plain)
	}
	if !strings.Contains(plain, copy.T(copy.UpdateHintTab)) {
		t.Fatalf("Tab should be advertised when both copies exist:\n%s", plain)
	}

	press(m, tea.KeyTab, "")
	if m.updateType != updateTypeSetup || m.updateFocus != hitUpdateInstall {
		t.Fatalf("Tab should return to Setup/install, type=%q focus=%v", m.updateType, m.updateFocus)
	}

	// A missing setup asset is not selectable; the portable copy is the only
	// available type and the hint must not advertise Tab.
	m, _ = testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Portable: true})
	m.openUpdateModal()
	press(m, tea.KeyTab, "")
	if m.updateType != updateTypePortable {
		t.Fatalf("Tab must not select a missing setup asset, type=%q", m.updateType)
	}
	plain = assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, "Not available") {
		t.Fatalf("unavailable type status missing:\n%s", plain)
	}
	if strings.Contains(plain, copy.T(copy.UpdateHintTab)) {
		t.Fatalf("Tab must not be advertised when only one copy exists:\n%s", plain)
	}
}

func TestUpdateTypeHorizontalActions(t *testing.T) {
	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true})
	m.installInfo = install.Info{Installed: true}
	m.openUpdateModal()
	press(m, tea.KeyTab, "")
	press(m, tea.KeyRight, "")
	if m.updateFocus != hitUpdateRelease {
		t.Fatalf("right should move to github, got %v", m.updateFocus)
	}
	press(m, tea.KeyRight, "")
	if m.updateFocus != hitUpdateLater {
		t.Fatalf("right should move to later, got %v", m.updateFocus)
	}
	press(m, tea.KeyRight, "")
	if m.updateFocus != hitUpdatePortable {
		t.Fatalf("right should wrap to download, got %v", m.updateFocus)
	}
	press(m, tea.KeyLeft, "")
	if m.updateFocus != hitUpdateLater {
		t.Fatalf("left should wrap to later, got %v", m.updateFocus)
	}
}

func TestUpdateGitHubShortcut(t *testing.T) {
	m, _ := testModel(80, 24)
	url := "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9"
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: url, Setup: true, Portable: true})
	m.openUpdateModal()
	opened := ""
	m.openURL = func(u string) error { opened = u; return nil }

	press(m, 'g', "g")
	if opened != url {
		t.Fatalf("g opened %q, want %q", opened, url)
	}
	if m.modal != modalUpdate {
		t.Fatal("opening GitHub should keep the update chooser visible")
	}

	opened = ""
	press(m, 'o', "o")
	if opened != "" {
		t.Fatalf("o should not open GitHub anymore, got %q", opened)
	}

	m.updateStep = updateStepFailed
	m.updateErr = "network"
	press(m, 'g', "g")
	if opened != url {
		t.Fatalf("g in the failure view opened %q, want %q", opened, url)
	}
}

func TestUpdateChooserArrowFocus(t *testing.T) {
	m, _ := testModel(80, 24)
	url := "https://example.test/release"
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: url, Setup: true, Portable: true})
	m.installInfo = install.Info{Installed: true, Dir: `C:\Users\A\Documents\tuick-tack`}
	m.openUpdateModal()
	if m.updateFocus != hitUpdateInstall || m.updateType != updateTypeSetup {
		t.Fatalf("installed copy should start on Setup/install, type=%q focus=%v", m.updateType, m.updateFocus)
	}
	press(m, tea.KeyRight, "")
	if m.updateFocus != hitUpdateRelease {
		t.Fatalf("right should focus github, got %v", m.updateFocus)
	}
	press(m, tea.KeyRight, "")
	if m.updateFocus != hitUpdateLater {
		t.Fatalf("right should focus later, got %v", m.updateFocus)
	}
	press(m, tea.KeyRight, "")
	if m.updateFocus != hitUpdateInstall {
		t.Fatalf("right should wrap to install, got %v", m.updateFocus)
	}
	press(m, tea.KeyLeft, "")
	if m.updateFocus != hitUpdateLater {
		t.Fatalf("left should wrap to later, got %v", m.updateFocus)
	}

	opened := ""
	m.openURL = func(u string) error { opened = u; return nil }
	m.updateFocus = hitUpdateRelease
	press(m, tea.KeyEnter, "")
	if opened != url {
		t.Fatalf("enter on github opened %q, want %q", opened, url)
	}
	if m.modal != modalUpdate {
		t.Fatal("opening GitHub should keep the chooser visible")
	}

	m.updateFocus = hitUpdateLater
	press(m, tea.KeyEnter, "")
	if m.modal != modalNone {
		t.Fatal("enter on later should close the chooser")
	}

	// Without assets the only real action is the release page; Enter must not
	// fall back to a download that does not exist.
	m, _ = testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: url})
	m.openUpdateModal()
	if m.updateFocus != hitUpdateRelease {
		t.Fatalf("release-only chooser should start on github, got %v", m.updateFocus)
	}
	opened = ""
	m.openURL = func(u string) error { opened = u; return nil }
	press(m, tea.KeyEnter, "")
	if opened != url {
		t.Fatalf("release-only enter opened %q, want %q", opened, url)
	}
}

func TestUpdateTransitionAnimation(t *testing.T) {
	old := buildinfo.Version
	buildinfo.Version = "v0.2.0"
	defer func() { buildinfo.Version = old }()

	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release"})

	static := canvas.New(60, 3)
	m.drawUpdateTransition(static, 0, 1, 60, false)
	for x := 0; x < 60; x++ {
		cell := static.At(x, 1)
		if cell.Ch != ' ' && cell.FG != m.pal.Muted {
			t.Fatalf("static transition cell %d is not muted: %+v", x, cell)
		}
	}

	m.animTime = 0
	first := canvas.New(60, 3)
	m.drawUpdateTransition(first, 0, 1, 60, true)
	m.animTime = 1.2
	second := canvas.New(60, 3)
	m.drawUpdateTransition(second, 0, 1, 60, true)
	changed := 0
	for x := 0; x < 60; x++ {
		a, b := first.At(x, 1), second.At(x, 1)
		if a.Ch != b.Ch {
			t.Fatalf("transition text changed between frames: %q vs %q", a.Ch, b.Ch)
		}
		if a.FG != b.FG {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("the transitioning line did not animate")
	}
}

func TestUpdateKeyOpensChooser(t *testing.T) {
	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true})
	press(m, 'u', "u")
	if m.modal != modalUpdate || m.updateStep != updateStepChoose {
		t.Fatalf("u opened modal %v step %v", m.modal, m.updateStep)
	}
	m.width, m.height = 48, 22
	assertFrame(t, m, 48, 22)
}

func TestUpdateTimerWarning(t *testing.T) {
	m, _ := testModel(80, 24)
	applyRunning(m)
	m.installInfo = install.Info{Installed: true}
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true})
	m.openUpdateModal()
	press(m, tea.KeyEnter, "")
	if m.updateStep != updateStepWarn {
		t.Fatalf("step = %v, want warning", m.updateStep)
	}
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{"Countdown active", "will be lost", "continue", "back"} {
		if !strings.Contains(plain, want) {
			t.Errorf("warning missing %q:\n%s", want, plain)
		}
	}
	press(m, tea.KeyEscape, "")
	if m.updateStep != updateStepChoose {
		t.Fatalf("escape should return to the chooser, got %v", m.updateStep)
	}
}

func TestUpdateProgressAndPortableResult(t *testing.T) {
	old := buildinfo.Version
	buildinfo.Version = "v0.2.0"
	defer func() { buildinfo.Version = old }()

	m, _ := testModel(80, 24)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release", Setup: true, Portable: true})
	m.openUpdateModal()
	m.updateStep = updateStepProgress
	m.updateKind = "setup"
	m.handleUpdateProgress(protocol.NewUpdateProgress(protocol.UpdateProgressMessage{
		Kind: "setup", Phase: "downloading", Received: 42, Total: 100,
	}))
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{
		copy.T(copy.UpdateFromTo, "v0.2.0", "v9.9.9"),
		"Downloading", "42%", "Download · Verify · Install · Reopen",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("progress missing %q:\n%s", want, plain)
		}
	}

	m.handleUpdateProgress(protocol.NewUpdateProgress(protocol.UpdateProgressMessage{
		Kind: "setup", Phase: "failed", Error: "checksum",
	}))
	if m.updateStep != updateStepFailed {
		t.Fatalf("checksum failure step = %v", m.updateStep)
	}
	failure := assertFrame(t, m, 80, 24)
	if !strings.Contains(failure, copy.T(copy.UpdateChecksum)) {
		t.Fatal("checksum failure text missing")
	}
	if !strings.Contains(failure, "[g] github") {
		t.Fatalf("failure hint should use the GitHub shortcut:\n%s", failure)
	}

	m.updateStep = updateStepProgress
	m.updateKind = "portable"
	path := `C:\Users\A\Downloads\tuick-tack-v9.9.9.exe`
	m.handleUpdateProgress(protocol.NewUpdateProgress(protocol.UpdateProgressMessage{
		Kind: "portable", Phase: "done", Received: 100, Total: 100, Path: path,
	}))
	if m.updateStep != updateStepPortableDone {
		t.Fatalf("portable step = %v", m.updateStep)
	}
	plain = assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, "Portable update saved") || !strings.Contains(plain, "tuick-tack-v9.9.9.exe") {
		t.Fatalf("portable result missing:\n%s", plain)
	}

	opened := ""
	m.openPath = func(p string) error { opened = p; return nil }
	press(m, 'o', "o")
	if opened != path {
		t.Fatalf("opened folder %q, want %q", opened, path)
	}
	press(m, tea.KeyEnter, "")
	if m.modal != modalNone {
		t.Fatal("enter should close the portable result")
	}
}

func TestUpdateClosingReason(t *testing.T) {
	m, _ := testModel(80, 24)
	cmd := m.handleServerEvent(serverEventMsg{msg: protocol.NewDaemonClosing(protocol.CloseReasonUpdate)})
	if cmd == nil {
		t.Fatal("update closing should quit the TUI")
	}
	if !m.Updating() || !m.DaemonClosing() {
		t.Fatalf("updating = %v, closing = %v", m.Updating(), m.DaemonClosing())
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("update closing should return tea.Quit")
	}
}

func TestUpdateNoticeShortensAtMinimum(t *testing.T) {
	m, _ := testModel(48, 24)
	m.setUpdate(&protocol.UpdatePayload{
		Latest: "v9.9.9",
		URL:    "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9",
	})
	m.state = protocol.StateMessage{Type: protocol.TypeState, Status: string(engine.Finished)}
	m.hasState = true

	plain := assertFrame(t, m, 48, 24)
	header := strings.Split(plain, "\n")[m.layout(48, 24).headerY]
	short := "[" + copy.T(copy.UpdateShort) + " " + copy.T(copy.UpdateArrow) + "]"
	long := "[" + copy.T(copy.UpdateAvailable) + " " + copy.T(copy.UpdateArrow) + "]"
	if !strings.Contains(header, short) {
		t.Fatalf("narrow header is missing the short notice: %q", header)
	}
	if strings.Contains(header, long) {
		t.Fatalf("narrow header should not use the long notice: %q", header)
	}
}

func TestUpdateColors(t *testing.T) {
	m, _ := testModel(80, 24)
	colors := m.updateColors(18)
	if len(colors) != 18 {
		t.Fatalf("colors = %d, want 18", len(colors))
	}
	for _, c := range colors {
		if c.A != 0xFF {
			t.Fatalf("alpha = %d", c.A)
		}
		spread := int(maxUint8(c.R, c.G, c.B)) - int(minUint8(c.R, c.G, c.B))
		if spread > 140 {
			t.Fatalf("color %+v is too saturated (spread %d)", c, spread)
		}
	}
	if colors[0] == colors[len(colors)-1] {
		t.Fatal("the gradient should change across the label")
	}

	m.pal = theme.Get("mono")
	for _, c := range m.updateColors(18) {
		if absInt(int(c.R)-int(c.G)) > 12 || absInt(int(c.G)-int(c.B)) > 12 {
			t.Fatalf("mono notice is not gray enough: %+v", c)
		}
	}

	m.pal = theme.Get("midnight")
	m.animTime = 0
	first := m.updateColors(18)
	m.animTime = 1
	moved := m.updateColors(18)
	if first[0] == moved[0] {
		t.Fatal("the rainbow should move with animTime")
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestUpdateNoticeInCompact(t *testing.T) {
	m, _ := testModel(40, 12)
	m.setUpdate(&protocol.UpdatePayload{Latest: "v9.9.9", URL: "https://example.test/release"})
	plain := assertFrame(t, m, 40, 12)
	if !strings.Contains(plain, copy.T(copy.UpdateAvailable)) {
		t.Fatalf("compact view is missing the notice:\n%s", plain)
	}
	if len(m.hits) != 0 {
		t.Fatalf("compact view registered %d mouse hits", len(m.hits))
	}
}

func maxUint8(v ...uint8) uint8 {
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func minUint8(v ...uint8) uint8 {
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

func TestManualUpdateCheck(t *testing.T) {
	m, _ := testModel(80, 24)

	// Without a daemon the key reports the offline state instead of leaving a
	// spinner behind.
	press(m, 'u', "u")
	if m.checkingUpdate {
		t.Fatal("offline check should not start")
	}
	if m.toast != copy.T(copy.ErrorOffline) {
		t.Fatalf("toast = %q, want offline", m.toast)
	}

	m.connected = true
	m.checkingUpdate = true
	m.toast = ""
	plain := assertFrame(t, m, 80, 24)
	l := m.layout(80, 24)
	if line := strings.Split(plain, "\n")[l.statusY]; !strings.Contains(line, copy.T(copy.UpdateChecking)) {
		t.Fatalf("checking line = %q", line)
	}

	url := "https://github.com/Juankyyy/tuick-tack/releases/tag/v9.9.9"
	_, cmd := m.Update(serverEventMsg{msg: protocol.NewUpdateResult(protocol.UpdateResultMessage{
		Current: buildinfo.Version,
		Latest:  "v9.9.9",
		URL:     url,
	})})
	if cmd != nil {
		t.Fatalf("unexpected command after result: %v", cmd)
	}
	if m.checkingUpdate {
		t.Fatal("result did not clear the checking state")
	}
	if m.update == nil || m.update.URL != url || m.update.Latest != "v9.9.9" {
		t.Fatalf("update = %+v", m.update)
	}
	if m.toast != copy.T(copy.ToastUpdateFound, "v9.9.9") {
		t.Fatalf("toast = %q", m.toast)
	}
}

func TestManualUpdateResultUpToDate(t *testing.T) {
	old := buildinfo.Version
	buildinfo.Version = "v2.0.0"
	defer func() { buildinfo.Version = old }()

	m, _ := testModel(80, 24)
	m.checkingUpdate = true
	m.update = &protocol.UpdatePayload{Latest: "v1.9.0", URL: "https://example.test/old"}
	_, _ = m.Update(serverEventMsg{msg: protocol.NewUpdateResult(protocol.UpdateResultMessage{
		Current: "v2.0.0",
		Latest:  "v1.9.0",
		URL:     "https://example.test/v1.9.0",
	})})
	if m.checkingUpdate {
		t.Fatal("result did not clear the checking state")
	}
	if m.update != nil {
		t.Fatalf("up-to-date result left a notice: %+v", m.update)
	}
	if m.toast != copy.T(copy.ToastUpToDate, "v1.9.0") {
		t.Fatalf("toast = %q", m.toast)
	}

	m.checkingUpdate = true
	_, _ = m.Update(serverEventMsg{msg: protocol.NewUpdateResult(protocol.UpdateResultMessage{Error: "failed"})})
	if m.checkingUpdate || m.toast != copy.T(copy.ErrorUpdate) {
		t.Fatalf("failed result: checking=%v toast=%q", m.checkingUpdate, m.toast)
	}
}

func TestManualUpdateCheckInCompact(t *testing.T) {
	m, _ := testModel(40, 12)
	m.connected = true
	m.checkingUpdate = true
	plain := assertFrame(t, m, 40, 12)
	if !strings.Contains(plain, copy.T(copy.UpdateChecking)) {
		t.Fatalf("compact view is missing the checking line:\n%s", plain)
	}
}

func TestKeyHintsSitBelowPresets(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		gap  int
	}{
		{80, 24, 1},
		{80, 30, 1},
		{80, 32, 2},
		{120, 40, 2},
	} {
		m, _ := testModel(tc.w, tc.h)
		l := m.layout(tc.w, tc.h)
		if got, want := l.hintY-l.chipsY, 1+tc.gap; got != want {
			t.Fatalf("%dx%d hint is %d rows under the presets, want %d", tc.w, tc.h, got, want)
		}
		if l.statusY != l.hintY-1 {
			t.Fatalf("%dx%d status row = %d, want %d", tc.w, tc.h, l.statusY, l.hintY-1)
		}
		if l.subHintY != l.hintY+2 {
			t.Fatalf("%dx%d sub-hint row = %d, want %d", tc.w, tc.h, l.subHintY, l.hintY+2)
		}
		if l.subHintY >= tc.h {
			t.Fatalf("%dx%d sub-hint row = %d, outside the frame", tc.w, tc.h, l.subHintY)
		}
		if l.hintY >= tc.h {
			t.Fatalf("%dx%d hint row = %d, outside the frame", tc.w, tc.h, l.hintY)
		}
		lines := render(m)
		hintLine := -1
		for i, line := range lines {
			if strings.Contains(line, copy.T(copy.ControlsIdle)) {
				hintLine = i
			}
		}
		if hintLine != l.hintY {
			t.Fatalf("%dx%d hints render at row %d, want %d", tc.w, tc.h, hintLine, l.hintY)
		}
		if strings.Contains(lines[l.hintY+1], copy.T(copy.UpdateHint)) {
			t.Fatalf("%dx%d update hint should have a blank row above it", tc.w, tc.h)
		}
		if !strings.Contains(lines[l.subHintY], copy.T(copy.UpdateHint)) {
			t.Fatalf("%dx%d sub-hint missing at row %d: %q", tc.w, tc.h, l.subHintY, lines[l.subHintY])
		}
		if strings.Index(lines[l.hintY], copy.T(copy.UpdateHint)) >= 0 {
			t.Fatalf("%dx%d update hint should be below the key line", tc.w, tc.h)
		}
	}
}

func TestMinimumStageFitsAndDropsMeta(t *testing.T) {
	m, _ := testModel(48, 22)
	l := m.layout(48, 22)
	if l.metaY != -1 {
		t.Fatalf("metaY = %d, want -1 in the tight layout", l.metaY)
	}
	if l.subHintY != -1 {
		t.Fatalf("subHintY = %d, want -1 in the tight layout", l.subHintY)
	}
	if l.chipsY != 19 || l.statusY != 20 || l.hintY != 21 {
		t.Fatalf("tight rows: chips=%d status=%d hint=%d, want 19/20/21",
			l.chipsY, l.statusY, l.hintY)
	}
	lines := render(m)
	assertFrame(t, m, 48, 22)
	if !strings.Contains(lines[l.chipsY], "5 min") {
		t.Fatalf("preset chips missing at row %d: %q", l.chipsY, lines[l.chipsY])
	}
	if !strings.Contains(lines[l.hintY], "[p] presets") {
		t.Fatalf("hints missing at row %d: %q", l.hintY, lines[l.hintY])
	}
	if strings.Contains(strings.Join(lines, "\n"), copy.T(copy.UpdateHint)) {
		t.Fatal("the tight layout should omit the secondary update hint")
	}
	if strings.Contains(strings.Join(lines, "\n"), copy.T(copy.ReadyMeta)) {
		t.Fatal("the tight layout should drop the meta line")
	}

	m.width, m.height = 48, 24
	if l := m.layout(48, 24); l.metaY < 0 {
		t.Fatal("the normal layout should restore the meta line from 24 rows")
	}
	if l := m.layout(48, 24); l.subHintY != l.hintY+2 {
		t.Fatalf("48x24 sub-hint row = %d, hint = %d", l.subHintY, l.hintY)
	}
	if !strings.Contains(strings.Join(render(m), "\n"), copy.T(copy.ReadyMeta)) {
		t.Fatal("24 rows should render the meta line")
	}
	if !strings.Contains(strings.Join(render(m), "\n"), copy.T(copy.UpdateHint)) {
		t.Fatal("24 rows should render the secondary update hint")
	}
}

func TestCompactBelowMinimum(t *testing.T) {
	for _, tc := range []struct{ w, h int }{
		{48, 21}, {47, 22}, {40, 12}, {20, 5}, {10, 2}, {1, 1},
	} {
		m, _ := testModel(tc.w, tc.h)
		applyRunning(m)
		plain := assertFrame(t, m, tc.w, tc.h)
		if tc.w >= 5 && !strings.Contains(plain, "04:32") {
			t.Fatalf("%dx%d compact view missing the clock:\n%s", tc.w, tc.h, plain)
		}
		if tc.w >= 5 && tc.h >= 2 && !strings.Contains(plain, "5 min") {
			t.Fatalf("%dx%d compact view missing the goal:\n%s", tc.w, tc.h, plain)
		}
		if len(m.hits) != 0 {
			t.Fatalf("%dx%d compact view registered %d mouse hits", tc.w, tc.h, len(m.hits))
		}
	}
}

func TestCompactKeepsTypingAndBlocksOverlays(t *testing.T) {
	m, _ := testModel(40, 12)
	press(m, '2', "2")
	press(m, '5', "5")
	plain := assertFrame(t, m, 40, 12)
	if !strings.Contains(plain, "› 25_") {
		t.Fatalf("compact view missing the typed input:\n%s", plain)
	}
	for _, key := range []struct {
		code rune
		text string
	}{{'p', "p"}, {'v', "v"}, {'?', "?"}} {
		press(m, key.code, key.text)
		if m.modal != modalNone {
			t.Fatalf("compact mode opened modal %v with %q", m.modal, key.text)
		}
	}
}

func TestCompactStopConfirmation(t *testing.T) {
	m, _ := testModel(40, 12)
	applyRunning(m)
	press(m, tea.KeyEscape, "")
	if m.modal != modalConfirmStop {
		t.Fatalf("modal = %v, want the stop confirmation", m.modal)
	}
	plain := assertFrame(t, m, 40, 12)
	for _, want := range []string{"04:32", copy.T(copy.StopTitle), copy.T(copy.CompactStopHint)} {
		if !strings.Contains(plain, want) {
			t.Fatalf("compact confirmation missing %q:\n%s", want, plain)
		}
	}
	press(m, tea.KeyEscape, "")
	if m.modal != modalNone {
		t.Fatal("escape should dismiss the compact confirmation")
	}
}

func TestCompactHidesAndRestoresModal(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, '?', "?")
	if m.modal != modalHelp {
		t.Fatalf("modal = %v, want help", m.modal)
	}
	m.width, m.height = 40, 12
	plain := assertFrame(t, m, 40, 12)
	if strings.Contains(plain, copy.T(copy.HelpTitle)) {
		t.Fatal("the help overlay should be hidden in compact mode")
	}
	press(m, 'p', "p")
	if m.modal != modalHelp {
		t.Fatalf("compact keys changed the hidden modal to %v", m.modal)
	}
	m.width, m.height = 80, 24
	plain = assertFrame(t, m, 80, 24)
	if !strings.Contains(plain, copy.T(copy.HelpTitle)) {
		t.Fatal("the help overlay should return when the terminal grows")
	}
}

func TestViewHasNoWindowFrame(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		for _, status := range []string{"idle", "running"} {
			if status == "running" {
				applyRunning(m)
			}
			plain := assertFrame(t, m, size[0], size[1])
			if strings.ContainsAny(plain, "╭╮╰╯┌┐└┘") {
				t.Fatalf("%dx%d %s view draws a window frame", size[0], size[1], status)
			}
		}
	}
}

func TestViewRunningPaused(t *testing.T) {
	m, _ := testModel(80, 24)
	applyRunning(m)
	plain := assertFrame(t, m, 80, 24)
	for _, want := range []string{"RUNNING", "5 min", "remaining", "ends at", "pause", "cancel", copy.T(copy.ControlsRunning)} {
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
	if m.confetti == nil || m.confetti.Count() != 110 {
		t.Fatalf("confetti count = %d, want 110", m.confetti.Count())
	}
	plain := assertFrame(t, m, 80, 24)
	// The header keeps the status copy; the hero word is drawn in blocks, so
	// its literal text never appears in the frame.
	for _, want := range []string{"TIME'S UP!", "5 min completed", "repeat", "close", copy.T(copy.ControlsFinished), copy.T(copy.FinishedMeta)} {
		if !strings.Contains(plain, want) {
			t.Errorf("finished view missing %q", want)
		}
	}
	if !validFinishWord(m.finishWord) {
		t.Errorf("finish word %q is not one of the celebration words", m.finishWord)
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

func validFinishWord(word string) bool {
	for _, key := range copy.HeroFinishedChoices {
		if copy.T(key) == word {
			return true
		}
	}
	return false
}

func TestFinishWordStaysForTheFinish(t *testing.T) {
	m, _ := testModel(120, 40)
	applyRunning(m)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
	})
	word := m.finishWord
	if word == "" {
		t.Fatal("finish did not pick a celebration word")
	}
	m.celebrating = false
	m.confetti = nil
	first := assertFrame(t, m, 120, 40)
	second := assertFrame(t, m, 120, 40)
	if first != second {
		t.Fatal("the finished frame changed between renders")
	}
	if m.finishWord != word {
		t.Fatalf("word changed from %q to %q while the finish stood", word, m.finishWord)
	}
}

func TestFinishWordsVaryAndNeverRepeat(t *testing.T) {
	m, _ := testModel(120, 40)
	applyRunning(m)
	seen := map[string]bool{}
	prev := ""
	for seq := int64(1); seq <= 12; seq++ {
		m.applyState(protocol.StateMessage{
			Type: protocol.TypeState, Status: "finished",
			DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: seq,
		})
		word := m.finishWord
		if !validFinishWord(word) {
			t.Fatalf("finish %d picked %q, not a celebration word", seq, word)
		}
		if word == prev {
			t.Fatalf("finish %d repeated %q", seq, word)
		}
		seen[word] = true
		prev = word
	}
	if len(seen) < 4 {
		t.Fatalf("only %d distinct words in 12 finishes: %v", len(seen), seen)
	}
}

func TestFinishedWordUsesWideScale(t *testing.T) {
	for _, size := range [][2]int{{100, 30}, {106, 30}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		applyRunning(m)
		m.applyState(protocol.StateMessage{
			Type: protocol.TypeState, Status: "finished",
			DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
		})
		m.celebrating = false
		m.confetti = nil
		plain := assertFrame(t, m, size[0], size[1])
		lines := strings.Split(plain, "\n")
		l := m.layout(size[0], size[1])
		minX, maxX := size[0], -1
		for _, line := range lines[l.heroY : l.heroY+bigfont.GlyphH] {
			for x, r := range []rune(line) {
				if r == '█' {
					if x < minX {
						minX = x
					}
					if x > maxX {
						maxX = x
					}
				}
			}
		}
		want := bigfont.WidthProportional(m.finishWord, 2)
		if got := maxX - minX + 1; got != want {
			t.Fatalf("%q spans %d columns at %dx%d, want %d", m.finishWord, got, size[0], size[1], want)
		}
	}
}

func TestEveryHeroWordFitsAtHundredColumns(t *testing.T) {
	m, _ := testModel(100, 30)
	applyRunning(m)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
	})
	m.celebrating = false
	m.confetti = nil
	for _, key := range copy.HeroFinishedChoices {
		word := copy.T(key)
		m.finishWord = word
		plain := assertFrame(t, m, 100, 30)
		lines := strings.Split(plain, "\n")
		l := m.layout(100, 30)
		minX, maxX := 100, -1
		for _, line := range lines[l.heroY : l.heroY+bigfont.GlyphH] {
			for x, r := range []rune(line) {
				if r == '█' {
					if x < minX {
						minX = x
					}
					if x > maxX {
						maxX = x
					}
				}
			}
		}
		want := bigfont.WidthProportional(word, 2)
		if want > 92 {
			t.Fatalf("%q is %d columns at 2x, want at most 92", word, want)
		}
		if got := maxX - minX + 1; got != want {
			t.Fatalf("%q spans %d columns at 100x30, want %d", word, got, want)
		}
	}
}

func TestFinishAutoClosesAfterTenSeconds(t *testing.T) {
	m, clock := testModel(120, 40)
	applyRunning(m)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
	})
	if !m.finishAutoArmed {
		t.Fatal("a new finish should arm the automatic close")
	}
	clock.Advance(9 * time.Second)
	m.handleTick()
	if !m.finishAutoArmed {
		t.Fatal("the celebration closed before ten seconds")
	}
	clock.Advance(time.Second)
	m.handleTick()
	if m.finishAutoArmed {
		t.Fatal("the automatic close did not fire at ten seconds")
	}
	if m.celebrating || m.confetti != nil {
		t.Fatal("the automatic close left the celebration running")
	}
	// The test model has no IPC client, so send() reports the disconnect; that
	// proves the dismiss was attempted.
	if m.toast != copy.T(copy.ErrorOffline) {
		t.Fatalf("automatic dismiss was not sent: toast = %q", m.toast)
	}
	m.toast = ""
	m.handleTick()
	if m.toast != "" {
		t.Fatalf("automatic close repeated: toast = %q", m.toast)
	}
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 600000, RemainingMS: 0, Label: "10 min", FinishSeq: 2,
	})
	if !m.finishAutoArmed {
		t.Fatal("the next finish should re-arm the automatic close")
	}
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "running",
		DurationMS: 600000, RemainingMS: 590000, Label: "10 min", FinishSeq: 2,
	})
	if m.finishAutoArmed {
		t.Fatal("leaving the finished screen should disarm the automatic close")
	}
}

func TestFinishAutoCloseWaitsForModal(t *testing.T) {
	m, clock := testModel(120, 40)
	applyRunning(m)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
	})
	m.modal = modalStats
	clock.Advance(11 * time.Second)
	m.handleTick()
	if !m.finishAutoArmed {
		t.Fatal("the automatic close fired while a modal was open")
	}
	m.modal = modalNone
	m.handleTick()
	if m.finishAutoArmed {
		t.Fatal("the automatic close did not fire after closing the modal")
	}
}

func TestFinishAutoCloseCancelledByRepeat(t *testing.T) {
	m, clock := testModel(120, 40)
	applyRunning(m)
	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "finished",
		DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
	})
	clock.Advance(11 * time.Second)
	press(m, 'r', "r")
	if m.finishAutoArmed {
		t.Fatal("repeat did not disarm the automatic close")
	}
	m.toast = ""
	m.handleTick()
	if m.toast != "" {
		t.Fatalf("repeat still sent the automatic dismiss: toast = %q", m.toast)
	}
}

func TestViewCelebrationRendersConfetti(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		applyRunning(m)
		m.applyState(protocol.StateMessage{
			Type: protocol.TypeState, Status: "finished",
			DurationMS: 300000, RemainingMS: 0, Label: "5 min", FinishSeq: 1,
		})
		for i := 0; i < 20; i++ {
			m.confetti.Update(0.033)
		}
		plain := assertFrame(t, m, size[0], size[1])
		if !strings.ContainsAny(plain, "█▀▄▌▘") {
			t.Fatalf("%dx%d celebration did not draw any confetti", size[0], size[1])
		}
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
	for _, key := range []copy.Key{
		copy.HelpStart, copy.HelpPause, copy.HelpCancel, copy.HelpRepeat,
		copy.HelpPickPreset, copy.HelpPresets, copy.HelpSound, copy.HelpMute,
		copy.HelpStats, copy.HelpUpdate, copy.HelpTheme, copy.HelpBackground,
		copy.HelpReadable, copy.HelpQuit, copy.HelpQuitAll,
	} {
		k, desc := splitHelpKey(copy.T(key))
		if !strings.Contains(plain, k) || !strings.Contains(plain, desc) {
			t.Errorf("help is missing %q", copy.T(key))
		}
	}
	if platformMiniTimer {
		k, desc := splitHelpKey(copy.T(copy.HelpMiniTimer))
		if !strings.Contains(plain, k) || !strings.Contains(plain, desc) {
			t.Errorf("help is missing %q", copy.T(copy.HelpMiniTimer))
		}
	}
	if !strings.Contains(plain, copy.T(copy.HelpFormats)) {
		t.Errorf("help is missing %q", copy.T(copy.HelpFormats))
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
	cfg.Background = config.BackgroundGrid
	cfg.Sound = false
	cfg.SoundName = config.SoundMarimba
	cfg.Volume = 35
	cfg.Presets = cfg.Presets[:1]
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	if m.pal.Name != config.ThemeEmber || m.cfg.Sound || len(m.cfg.Presets) != 1 ||
		m.cfg.Background != config.BackgroundGrid || m.backdrop.Name() != config.BackgroundGrid ||
		m.cfg.SoundName != config.SoundMarimba || m.cfg.Volume != 35 {
		t.Fatalf("config not applied: %+v", m.cfg)
	}
}

func TestApplyConfigKeepsLocalBackgroundForOldDaemon(t *testing.T) {
	m, _ := testModel(80, 24)
	cfg := config.Default()
	cfg.Background = config.BackgroundFireflies
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	if !m.backgroundSupported || m.backdrop.Name() != config.BackgroundFireflies {
		t.Fatalf("background not applied: supported=%v name=%q",
			m.backgroundSupported, m.backdrop.Name())
	}

	// An older daemon omits the field: keep the local choice for this session.
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.ConfigPayload{
		Theme: config.ThemeMidnight, Sound: true,
	}})
	if m.backgroundSupported {
		t.Fatal("a missing background should mark the daemon as unsupported")
	}
	if m.cfg.Background != config.BackgroundFireflies ||
		m.backdrop.Name() != config.BackgroundFireflies {
		t.Fatalf("missing background reset the local choice: %+v", m.cfg)
	}

	// A malformed non-empty value falls back to aurora but still marks the
	// daemon as capable: the field is present, so set_background can be sent.
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.ConfigPayload{
		Theme: config.ThemeMidnight, Background: "neon", Sound: true,
	}})
	if !m.backgroundSupported {
		t.Fatal("a non-empty background should mark the daemon as supported")
	}
	if m.cfg.Background != config.DefaultBackground ||
		m.backdrop.Name() != config.DefaultBackground {
		t.Fatalf("invalid background did not fall back: %+v", m.cfg)
	}
}

func TestCycleBackground(t *testing.T) {
	m, _ := testModel(80, 24)
	if m.cfg.Background != config.BackgroundAurora {
		t.Fatalf("default background = %q", m.cfg.Background)
	}
	press(m, 'b', "b")
	if m.cfg.Background != config.BackgroundStars || m.backdrop.Name() != config.BackgroundStars {
		t.Fatalf("b did not cycle to stars: cfg=%q backdrop=%q",
			m.cfg.Background, m.backdrop.Name())
	}
	if !m.toastActive() {
		t.Fatal("cycling the background should show a toast")
	}

	// While typing a duration, b must not steal the shortcut.
	press(m, '5', "5")
	press(m, 'b', "b")
	if m.input != "5" || m.cfg.Background != config.BackgroundStars {
		t.Fatalf("input/background changed while typing: input=%q background=%q",
			m.input, m.cfg.Background)
	}
	m.input = ""

	// Running timers can change it too.
	applyRunning(m)
	press(m, 'b', "b")
	if m.cfg.Background != config.BackgroundFireflies {
		t.Fatalf("running b did not advance: %q", m.cfg.Background)
	}
}

func TestViewNoneIsTransparent(t *testing.T) {
	m, _ := testModel(80, 24)
	cfg := config.Default()
	cfg.Background = config.BackgroundNone
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	v := m.View()
	if v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatalf("none should leave the terminal colors alone: bg=%v fg=%v",
			v.BackgroundColor, v.ForegroundColor)
	}
	themeBG := fmt.Sprintf("48;2;%d;%d;%d", m.pal.BG.R, m.pal.BG.G, m.pal.BG.B)
	if strings.Contains(v.Content, themeBG) {
		t.Fatalf("none painted the theme background %q in the view", themeBG)
	}

	c := canvas.New(80, 24)
	m.drawBackdrop(c, 80, 24)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			cell := c.At(x, y)
			if cell.HasFG || cell.HasBG {
				t.Fatalf("none painted cell %d,%d: %+v", x, y, cell)
			}
		}
	}

	// The celebration flash must not fill the transparent background. Set a
	// real start so the flash window is active: otherwise the test would pass
	// even without the "none" guard in drawFlash.
	m.celebrating = true
	m.celebrationStart = m.now()
	c = canvas.New(80, 24)
	m.drawBackdrop(c, 80, 24)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			cell := c.At(x, y)
			if cell.HasFG || cell.HasBG {
				t.Fatalf("none painted cell %d,%d during the flash: %+v", x, y, cell)
			}
		}
	}

	// The hero keeps its original shadow even with none: it is a deliberate
	// readability element, not an ambient background.
	hero := canvas.New(80, 24)
	l := m.layout(80, 24)
	m.drawHero(hero, l.heroY, l.x, l.w)
	shadow := theme.Scale(m.pal.BG, 0.5)
	shadowCells := 0
	for y := 0; y < hero.H; y++ {
		for x := 0; x < hero.W; x++ {
			cell := hero.At(x, y)
			if !cell.HasBG {
				continue
			}
			if cell.BG != shadow {
				t.Fatalf("unexpected hero background at %d,%d: %+v", x, y, cell)
			}
			shadowCells++
		}
	}
	if shadowCells == 0 {
		t.Fatal("none should keep the hero shadow")
	}
}

func TestCompactNoneIsTransparent(t *testing.T) {
	m, _ := testModel(40, 12)
	cfg := config.Default()
	cfg.Background = config.BackgroundNone
	cfg.ReadableText = false
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	c := canvas.New(40, 12)
	m.drawCompact(c, 40, 12)
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y).HasBG {
				t.Fatalf("compact none without bands painted a background at %d,%d", x, y)
			}
		}
	}
	v := m.View()
	if v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatalf("compact none should leave terminal colors alone: bg=%v fg=%v",
			v.BackgroundColor, v.ForegroundColor)
	}

	// Without bands, the light stage still darkens the compact copy.
	m.termBGKnown = true
	m.termLight = true
	m.termBG = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	c = canvas.New(40, 12)
	m.drawCompact(c, 40, 12)
	found := false
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			cell := c.At(x, y)
			if !cell.HasFG || cell.Ch == ' ' {
				continue
			}
			found = true
			if got := theme.Contrast(cell.FG, m.termBG); got < stageTextContrast {
				t.Fatalf("compact ink at %d,%d has contrast %.2f, want %.1f", x, y, got, stageTextContrast)
			}
		}
	}
	if !found {
		t.Fatal("compact light stage painted no text")
	}

	// With bands, every compact glyph sits on solid black and remains light.
	m.cfg.ReadableText = true
	c = canvas.New(40, 12)
	m.drawCompact(c, 40, 12)
	found = false
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			cell := c.At(x, y)
			if !cell.HasFG || cell.Ch == ' ' {
				continue
			}
			found = true
			if !cell.HasBG || cell.BG != readableBandBG {
				t.Fatalf("compact glyph at %d,%d has no black band: %+v", x, y, cell)
			}
			if got := theme.Contrast(cell.FG, readableBandBG); got < stageTextContrast {
				t.Fatalf("compact band ink at %d,%d has contrast %.2f", x, y, got)
			}
		}
	}
	if !found {
		t.Fatal("compact bands painted no text")
	}
}

func TestBackgroundColorMsg(t *testing.T) {
	m, _ := testModel(80, 24)
	white := color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	_, _ = m.Update(tea.BackgroundColorMsg{Color: white})
	if !m.termBGKnown || !m.termLight || m.termBG != white {
		t.Fatalf("light background not stored: known=%v light=%v bg=%+v",
			m.termBGKnown, m.termLight, m.termBG)
	}
	dark := color.RGBA{R: 0x07, G: 0x0B, B: 0x14, A: 0xFF}
	_, _ = m.Update(tea.BackgroundColorMsg{Color: dark})
	if m.termLight || m.termBG != dark {
		t.Fatalf("dark background not stored: light=%v bg=%+v", m.termLight, m.termBG)
	}
	// Saturated backgrounds are classified by WCAG contrast, not HSL
	// lightness: pure blue is dark enough that white text reads better.
	for _, saturated := range []color.RGBA{
		{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF},
	} {
		_, _ = m.Update(tea.BackgroundColorMsg{Color: saturated})
		if m.termLight {
			t.Fatalf("saturated background %+v was classified as light", saturated)
		}
	}
}

func TestBackgroundColorMsgNil(t *testing.T) {
	m, _ := testModel(80, 24)
	_, _ = m.Update(tea.BackgroundColorMsg{})
	if m.termBGKnown {
		t.Fatal("a nil color should not mark the terminal background as known")
	}
}

func TestViewNeverSetsTerminalColors(t *testing.T) {
	m, _ := testModel(80, 24)
	// The app paints its own background cell by cell. It must never change the
	// terminal's default colors: that would alter the user's profile and can
	// drop its acrylic or background image.
	if v := m.View(); v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatalf("a themed background changed terminal colors: bg=%v fg=%v",
			v.BackgroundColor, v.ForegroundColor)
	}
	cfg := config.Default()
	cfg.Background = config.BackgroundNone
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	if v := m.View(); v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatalf("none changed terminal colors: bg=%v fg=%v",
			v.BackgroundColor, v.ForegroundColor)
	}
	cfg.Background = config.BackgroundAurora
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	if v := m.View(); v.BackgroundColor != nil || v.ForegroundColor != nil {
		t.Fatalf("returning to a themed background changed terminal colors: bg=%v fg=%v",
			v.BackgroundColor, v.ForegroundColor)
	}
}

func lightStageModel(t *testing.T) *Model {
	t.Helper()
	m, _ := testModel(80, 24)
	cfg := config.Default()
	cfg.Background = config.BackgroundNone
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	m.termBGKnown = true
	m.termLight = true
	m.termBG = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	return m
}

func TestLightStagePalette(t *testing.T) {
	m := lightStageModel(t)
	white := m.termBG
	if !m.lightStage() {
		t.Fatal("none over a light terminal should use the light stage")
	}
	if got := m.stageInk(m.pal.Text, stageTextContrast); got == m.pal.Text {
		t.Fatal("body text was not darkened for the light terminal")
	}
	for _, tc := range []struct {
		name string
		col  color.RGBA
		min  float64
	}{
		{"text", m.pal.Text, stageTextContrast},
		{"accent", m.pal.Accent, stageAccentContrast},
		{"accent2", m.pal.Accent2, stageAccentContrast},
		{"warn", m.pal.Warn, stageAccentContrast},
		{"danger", m.pal.Danger, stageAccentContrast},
	} {
		ink := m.stageInk(tc.col, tc.min)
		if got := theme.Contrast(ink, white); got < tc.min {
			t.Errorf("%s ink %+v has contrast %.2f, want %.1f", tc.name, ink, got, tc.min)
		}
	}
	// The update notice is tested on the adaptive path; bands have their own
	// tests and use the raw theme colors.
	m.cfg.ReadableText = false
	for i, c := range m.updateColors(12) {
		if got := theme.Contrast(c, white); got < stageMutedContrast {
			t.Errorf("update color %d has contrast %.2f, want %.1f", i, got, stageMutedContrast)
		}
	}
	// A dark terminal and a themed backdrop keep the active palette verbatim.
	m.termLight = false
	if got := m.stageInk(m.pal.Text, stageTextContrast); got != m.pal.Text {
		t.Fatalf("dark terminal changed the palette: %+v", got)
	}
	m.termLight = true
	m.termBGKnown = false
	if got := m.stageInk(m.pal.Text, stageTextContrast); got != m.pal.Text {
		t.Fatalf("unknown terminal background changed the palette: %+v", got)
	}
	m.termBGKnown = true
	cfg := config.Default()
	cfg.Background = config.BackgroundAurora
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	if got := m.stageInk(m.pal.Text, stageTextContrast); got != m.pal.Text {
		t.Fatalf("a themed backdrop changed the stage palette: %+v", got)
	}
}

// rowMinContrast returns the lowest contrast of the foreground cells with ink
// on a row. It fails when the row has no text at all.
func rowMinContrast(t *testing.T, c *canvas.Canvas, y int, bg color.RGBA) float64 {
	t.Helper()
	low := 21.0
	found := false
	for x := 0; x < c.W; x++ {
		cell := c.At(x, y)
		if !cell.HasFG || cell.Ch == ' ' {
			continue
		}
		found = true
		if ratio := theme.Contrast(cell.FG, bg); ratio < low {
			low = ratio
		}
	}
	if !found {
		t.Fatalf("row %d has no foreground text", y)
	}
	return low
}

func TestLightStageFrame(t *testing.T) {
	m := lightStageModel(t)
	// This test covers the adaptive ink used when the bands are off.
	m.cfg.ReadableText = false
	l := m.layout(80, 24)
	c := canvas.New(80, 24)
	m.drawContent(c, l)
	white := m.termBG

	if low := rowMinContrast(t, c, l.headerY, white); low < stageDimContrast {
		t.Errorf("header contrast = %.2f, want at least %.1f", low, stageDimContrast)
	}
	if got := theme.Contrast(c.At(l.x+2, l.headerY).FG, white); got < stageTextContrast {
		t.Errorf("brand contrast = %.2f, want at least %.1f", got, stageTextContrast)
	}
	if low := rowMinContrast(t, c, l.labelY, white); low < stageMutedContrast {
		t.Errorf("label contrast = %.2f, want at least %.1f", low, stageMutedContrast)
	}
	if low := rowMinContrast(t, c, l.inputY, white); low < stageMutedContrast {
		t.Errorf("input contrast = %.2f, want at least %.1f", low, stageMutedContrast)
	}
	if low := rowMinContrast(t, c, l.metaY, white); low < stageMutedContrast {
		t.Errorf("meta contrast = %.2f, want at least %.1f", low, stageMutedContrast)
	}
	if low := rowMinContrast(t, c, l.hintY, white); low < stageDimContrast {
		t.Errorf("hint contrast = %.2f, want at least %.1f", low, stageDimContrast)
	}
	if low := rowMinContrast(t, c, l.subHintY, white); low < stageDimContrast {
		t.Errorf("sub-hint contrast = %.2f, want at least %.1f", low, stageDimContrast)
	}

	// The countdown gradient is dark ink too, and its block shadow must stay
	// light so a dark glyph never sits on a dark shadow cell.
	hero := canvas.New(80, 24)
	m.drawHero(hero, l.heroY, l.x, l.w)
	darkShadow := theme.Scale(m.pal.BG, 0.5)
	shadowCells := 0
	for y := l.heroY; y < l.heroY+bigfont.GlyphH; y++ {
		for x := 0; x < hero.W; x++ {
			cell := hero.At(x, y)
			if cell.HasBG {
				if cell.BG == darkShadow {
					t.Fatalf("light stage kept the dark hero shadow at %d,%d", x, y)
				}
				shadowCells++
			}
			if !cell.HasFG || cell.Ch == ' ' {
				continue
			}
			bg := white
			if cell.HasBG {
				bg = cell.BG
			}
			if got := theme.Contrast(cell.FG, bg); got < stageDimContrast {
				t.Fatalf("hero ink at %d,%d has contrast %.2f against its cell", x, y, got)
			}
		}
	}
	if shadowCells == 0 {
		t.Fatal("light stage lost the hero block shadow")
	}

	// The transient status line adapts as well.
	m.toast = "hello"
	m.toastUntil = m.now().Add(time.Minute)
	c = canvas.New(80, 24)
	m.drawContent(c, l)
	if low := rowMinContrast(t, c, l.statusY, white); low < stageAccentContrast {
		t.Errorf("status contrast = %.2f, want at least %.1f", low, stageAccentContrast)
	}
}

func rowHasBand(c *canvas.Canvas, y int) bool {
	for x := 0; x < c.W; x++ {
		cell := c.At(x, y)
		if cell.HasBG && cell.BG == readableBandBG {
			return true
		}
	}
	return false
}

func assertBandRow(t *testing.T, c *canvas.Canvas, y int) {
	t.Helper()
	found := false
	for x := 0; x < c.W; x++ {
		cell := c.At(x, y)
		if !cell.HasFG || cell.Ch == ' ' {
			continue
		}
		found = true
		if !cell.HasBG || cell.BG != readableBandBG {
			t.Fatalf("row %d glyph at %d has no black band: %+v", y, x, cell)
		}
		if got := theme.Contrast(cell.FG, readableBandBG); got < stageDimContrast {
			t.Fatalf("row %d glyph at %d has contrast %.2f on its band", y, x, got)
		}
	}
	if !found {
		t.Fatalf("row %d has no glyphs to check", y)
	}
}

func TestReadableBandsOnlyWithNone(t *testing.T) {
	m := lightStageModel(t)
	l := m.layout(80, 24)
	c := canvas.New(80, 24)
	m.drawContent(c, l)
	for _, y := range []int{l.headerY, l.labelY, l.inputY, l.metaY, l.hintY, l.subHintY} {
		if !rowHasBand(c, y) {
			t.Fatalf("none should paint a black band on row %d", y)
		}
		assertBandRow(t, c, y)
	}
	// The band adds exactly one cell of margin on each side of the run.
	b := canvas.New(20, 1)
	m.band(b, 5, 0, 3)
	for x := 4; x <= 8; x++ {
		if cell := b.At(x, 0); !cell.HasBG || cell.BG != readableBandBG {
			t.Fatalf("band margin at %d is missing: %+v", x, cell)
		}
	}
	for _, x := range []int{3, 9} {
		if cell := b.At(x, 0); cell.HasBG {
			t.Fatalf("band leaked beyond its margin at %d: %+v", x, cell)
		}
	}

	// The hero keeps its own shadow and never gets a readable band.
	for y := l.heroY; y < l.heroY+bigfont.GlyphH; y++ {
		for x := 0; x < c.W; x++ {
			if cell := c.At(x, y); cell.HasBG && cell.BG == readableBandBG {
				t.Fatalf("hero row %d got a readable band at %d", y, x)
			}
		}
	}

	// Disabling the bands returns to the transparent adaptive path.
	m.cfg.ReadableText = false
	c = canvas.New(80, 24)
	m.drawContent(c, l)
	for y := 0; y < c.H; y++ {
		if rowHasBand(c, y) {
			t.Fatalf("disabled bands still painted row %d", y)
		}
	}

	// A themed backdrop never gets readable bands, even when enabled.
	m.cfg.ReadableText = true
	cfg := config.Default()
	cfg.Background = config.BackgroundAurora
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	c = canvas.New(80, 24)
	m.drawContent(c, m.layout(80, 24))
	for y := 0; y < c.H; y++ {
		if rowHasBand(c, y) {
			t.Fatalf("aurora should not paint a black band on row %d", y)
		}
	}
}

func TestReadableBandsFrame(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		cfg := config.Default()
		cfg.Background = config.BackgroundNone
		m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
		assertFrame(t, m, size[0], size[1])
		l := m.layout(size[0], size[1])
		c := canvas.New(size[0], size[1])
		m.drawContent(c, l)
		if !rowHasBand(c, l.hintY) {
			t.Fatalf("%dx%d hint row has no black band", size[0], size[1])
		}
		for x := 0; x < c.W; x++ {
			cell := c.At(x, l.hintY)
			if !cell.HasFG || cell.Ch == ' ' {
				continue
			}
			if !cell.HasBG || cell.BG != readableBandBG {
				t.Fatalf("%dx%d hint glyph at %d has no black band: %+v", size[0], size[1], x, cell)
			}
			if got := theme.Contrast(cell.FG, readableBandBG); got < stageMutedContrast {
				t.Fatalf("%dx%d hint ink at %d has contrast %.2f", size[0], size[1], x, got)
			}
		}
	}
}

func TestHelpShowsReadableBands(t *testing.T) {
	m, _ := testModel(80, 24)
	press(m, '?', "?")
	plain := assertFrame(t, m, 80, 24)
	key, desc := splitHelpKey(copy.T(copy.HelpReadable))
	if !strings.Contains(plain, key) || !strings.Contains(plain, desc) {
		t.Fatalf("help is missing the readable-bands hint:\n%s", plain)
	}
}

func TestHelpWideLayoutAlignsDescriptions(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		m, _ := testModel(size[0], size[1])
		press(m, '?', "?")
		lines := render(m)
		columnX := func(needle string) int {
			t.Helper()
			for _, line := range lines {
				if i := strings.Index(line, needle); i >= 0 {
					return canvas.Width(line[:i])
				}
			}
			t.Fatalf("%dx%d help is missing %q", size[0], size[1], needle)
			return 0
		}
		left := []string{
			"start the duration", "pause or resume", "cancel (asks first)",
			"adjust one minute", "repeat when it finishes", "pick a preset",
		}
		wantLeft := columnX(left[0])
		for _, desc := range left[1:] {
			if got := columnX(desc); got != wantLeft {
				t.Errorf("%dx%d %q starts at x=%d, want %d", size[0], size[1], desc, got, wantLeft)
			}
		}
		right := []string{"presets", "sound", "mute or unmute", "statistics", "check for updates"}
		wantRight := columnX(right[0])
		for _, desc := range right[1:] {
			if got := columnX(desc); got != wantRight {
				t.Errorf("%dx%d %q starts at x=%d, want %d", size[0], size[1], desc, got, wantRight)
			}
		}
		if wantRight <= wantLeft {
			t.Errorf("%dx%d right column x=%d should follow the left column x=%d", size[0], size[1], wantRight, wantLeft)
		}
		assertFrame(t, m, size[0], size[1])
	}
}

func TestHelpNarrowFallbackAtMinimumWindow(t *testing.T) {
	m, _ := testModel(48, 22)
	press(m, '?', "?")
	plain := assertFrame(t, m, 48, 22)
	for _, want := range []string{
		copy.T(copy.HelpNarrowStartPause),
		copy.T(copy.HelpNarrowCancelAdjust),
		copy.T(copy.HelpNarrowPanelsMore),
		copy.T(copy.HelpNarrowQuit),
		copy.T(copy.HelpNarrowQuitAll),
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("narrow help is missing %q", want)
		}
	}
	if strings.Contains(plain, copy.T(copy.HelpFormats)) {
		t.Error("narrow help should not render the wide duration examples")
	}
	for _, want := range []string{
		copy.T(copy.HelpNarrowDisplay),
		copy.T(copy.HelpNarrowReadable),
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("narrow help is missing %q", want)
		}
	}
}

func TestHelpCopyComplete(t *testing.T) {
	check := func(line helpLine) {
		if line.kind == helpLineBlank || line.text == "" {
			return
		}
		if got := copy.T(line.text); got == "" || got == string(line.text) {
			t.Errorf("help copy key %q has no text", line.text)
		}
	}
	for _, col := range helpColumns {
		for _, line := range col {
			check(line)
		}
	}
	for _, line := range helpNarrow {
		check(line)
	}
}

func TestToggleReadableText(t *testing.T) {
	m, _ := testModel(80, 24)
	if !m.cfg.ReadableText || !m.readableSupported {
		t.Fatalf("readable bands should start on and supported: %+v", m.cfg)
	}
	press(m, 'n', "n")
	if m.cfg.ReadableText {
		t.Fatal("n did not turn the bands off")
	}
	if m.toast != copy.T(copy.ToastReadableOff) {
		t.Fatalf("toast = %q, want %q", m.toast, copy.T(copy.ToastReadableOff))
	}
	press(m, 'n', "n")
	if !m.cfg.ReadableText || m.toast != copy.T(copy.ToastReadableOn) {
		t.Fatalf("n did not turn the bands back on: %+v toast=%q", m.cfg, m.toast)
	}
}

func TestApplyConfigReadableText(t *testing.T) {
	m, _ := testModel(80, 24)
	cfg := config.Default()
	cfg.ReadableText = false
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
	if m.cfg.ReadableText || !m.readableSupported {
		t.Fatalf("new daemon readable_text not applied: %+v supported=%v", m.cfg, m.readableSupported)
	}

	// An older daemon omits the pointer: keep the local choice and mark the
	// command unsupported.
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.ConfigPayload{
		Theme: config.ThemeMidnight, Background: config.BackgroundNone,
		Sound: true, SoundName: config.DefaultSound, Volume: config.DefaultVolume,
	}})
	if m.cfg.ReadableText || m.readableSupported {
		t.Fatalf("old daemon reset the local choice: %+v supported=%v", m.cfg, m.readableSupported)
	}
}

func TestNoTextShadow(t *testing.T) {
	m := lightStageModel(t)
	c := canvas.New(80, 24)
	style := canvas.Fg(m.pal.Text)
	m.drawCenteredIn(c, 0, 5, 80, "abc", style)
	for x := 0; x < c.W; x++ {
		cell := c.At(x, 6)
		if cell.HasFG || cell.HasBG {
			t.Fatalf("drawCentered painted a shadow cell at %d,6: %+v", x, cell)
		}
	}
}

func TestViewBackdrops(t *testing.T) {
	for _, name := range config.Backgrounds {
		t.Run(name, func(t *testing.T) {
			m, _ := testModel(80, 24)
			cfg := config.Default()
			cfg.Background = name
			m.applyConfig(protocol.ConfigMessage{ConfigPayload: protocol.NewConfigPayload(cfg)})
			assertFrame(t, m, 80, 24)
			m.width, m.height = 120, 40
			assertFrame(t, m, 120, 40)
		})
	}
}

func TestShowDustByBackground(t *testing.T) {
	m, _ := testModel(80, 24)
	for _, name := range []string{config.BackgroundAurora, config.BackgroundWaves} {
		m.setBackdrop(name)
		if !m.showDust() {
			t.Errorf("%s should keep the ambient dust", name)
		}
	}
	for _, name := range []string{
		config.BackgroundNone, config.BackgroundStars, config.BackgroundFireflies,
		config.BackgroundRain, config.BackgroundGrid,
	} {
		m.setBackdrop(name)
		if m.showDust() {
			t.Errorf("%s should not add the dust field", name)
		}
	}
}

func TestBackdropClockSlowsWhenPaused(t *testing.T) {
	m, clock := testModel(80, 24)
	applyRunning(m)

	start := m.backdropTime
	clock.Advance(100 * time.Millisecond)
	m.handleTick()
	if got := m.backdropTime - start; got < 0.099 || got > 0.101 {
		t.Fatalf("running backdrop clock advanced %.3f, want ~0.1", got)
	}

	m.applyState(protocol.StateMessage{
		Type: protocol.TypeState, Status: "paused",
		DurationMS: 300000, RemainingMS: 200000,
	})
	start = m.backdropTime
	clock.Advance(100 * time.Millisecond)
	m.handleTick()
	if got := m.backdropTime - start; got < 0.024 || got > 0.026 {
		t.Fatalf("paused backdrop clock advanced %.3f, want ~0.025", got)
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
