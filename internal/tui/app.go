package tui

import (
	"image/color"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"tuick-tack/internal/browser"
	"tuick-tack/internal/buildinfo"
	"tuick-tack/internal/config"
	"tuick-tack/internal/daemon"
	"tuick-tack/internal/duration"
	"tuick-tack/internal/engine"
	"tuick-tack/internal/explorer"
	"tuick-tack/internal/install"
	"tuick-tack/internal/ipc"
	"tuick-tack/internal/protocol"
	"tuick-tack/internal/termwin"
	"tuick-tack/internal/tui/copy"
	"tuick-tack/internal/tui/fx"
	"tuick-tack/internal/tui/notes"
	"tuick-tack/internal/tui/theme"
	release "tuick-tack/internal/update"
)

type modalKind int

const (
	modalNone modalKind = iota
	modalHelp
	modalConfirmStop
	modalConfirmDelete
	modalPresets
	modalPresetForm
	modalSound
	modalUpdate
	modalStats
)

// updateStep is the screen shown inside modalUpdate.
type updateStep int

const (
	updateStepChoose updateStep = iota
	updateStepWarn
	updateStepProgress
	updateStepPortableDone
	updateStepFailed
)

type formMode int

const (
	formAdd formMode = iota
	formEdit
)

type formState struct {
	mode     formMode
	id       string
	label    string
	duration string
	focus    int
	err      string
}

type hitAction int

const (
	hitNone hitAction = iota
	hitChip
	hitPresetRow
	hitPlus
	hitPause
	hitAdjustPlus
	hitAdjustMinus
	hitCancel
	hitClose
	hitConfirmYes
	hitConfirmNo
	hitPresetAdd
	hitPresetEdit
	hitPresetDelete
	hitPresetRestore
	hitPresetStart
	hitFormName
	hitFormDuration
	hitFormSave
	hitFormCancel
	hitRepeat
	hitDismiss
	hitSoundRow
	hitSoundBar
	hitSoundVolumeDown
	hitSoundVolumeUp
	hitSoundPreview
	hitSoundMute
	hitSoundClose
	hitUpdate
	hitUpdateInstall
	hitUpdatePortable
	hitUpdateRelease
	hitUpdateLater
	hitUpdateContinue
	hitUpdateBack
	hitUpdateOpenFolder
	hitUpdateRetry
	hitStatsTab
)

// presetFocusList means the preset list owns the keyboard focus; any other
// value is an index into presetActionDefs.
const presetFocusList = -1

// presetActionDef describes one button in the presets modal. The view and the
// keyboard handler share the list so both stay in sync.
type presetActionDef struct {
	label       copy.Key
	action      hitAction
	needsPreset bool
}

var presetActionDefs = []presetActionDef{
	{copy.Add, hitPresetAdd, false},
	{copy.Edit, hitPresetEdit, true},
	{copy.Delete, hitPresetDelete, true},
	{copy.Restore, hitPresetRestore, false},
	{copy.Start, hitPresetStart, true},
	{copy.Close, hitClose, false},
}

type hitbox struct {
	x, y, w, h int
	action     hitAction
	index      int
}

type connectedMsg struct {
	client *ipc.Client
	hello  protocol.HelloMessage
	err    error
}

type serverEventMsg struct {
	msg protocol.ServerMessage
	err error
}

type sentMsg struct{ err error }

type tickMsg time.Time

// Model is the Bubble Tea model. All timer truth comes from the daemon; the
// model only interpolates between snapshots.
type Model struct {
	pipe string
	now  func() time.Time
	seed int64

	client       *ipc.Client
	connected    bool
	connecting   bool
	reconnecting bool

	state    protocol.StateMessage
	hasState bool
	snapAt   time.Time
	cfg      config.Config
	pal      theme.Theme
	// termLight is true when the terminal background is light and
	// termBG is its RGB color. They come from the OSC 11 reply and let the
	// transparent "none" background switch its stage text to a dark palette.
	termLight   bool
	termBG      color.RGBA
	termBGKnown bool
	// backgroundSupported is true when the daemon's config carries a
	// background, so set_background is safe to send. An older daemon leaves it
	// empty and the TUI keeps the choice local for this session.
	backgroundSupported bool

	width, height int
	hits          []hitbox

	// Terminal placement preference: sizeKnown is set by the first
	// WindowSizeMsg, windowSupported by a daemon that advertises window in the
	// hello. The placement is applied once per TUI session; later resizes and
	// moves are persisted after windowSaveDelay, and again on a clean exit.
	// windowOwned is false when the TUI shares a console (a terminal tab) and
	// must not move the user's whole terminal window.
	sizeKnown            bool
	windowConnected      bool
	windowSupported      bool
	windowArmed          bool
	windowApplyPending   bool
	windowDirty          bool
	windowChangedOffline bool
	windowSavedW         int
	windowSavedH         int
	windowOwned          bool
	posArmed             bool
	posApplyPending      bool
	posApplyUntil        time.Time
	posReapply           bool
	posDirty             bool
	posKnown             bool
	posValid             bool
	windowHasPos         bool
	windowX              int
	windowY              int
	windowSavedX         int
	windowSavedY         int
	lastPosSample        time.Time
	applyWindow          func(w, h int)
	moveWindow           func(x, y int) bool
	windowPositionFn     func() (x, y int, ok bool)
	windowMaximizedFn    func() bool
	sendWindow           func(w, h int, pos *config.WindowPosition) error

	input    string
	inputErr string
	selected int

	modal        modalKind
	presetCursor int
	presetScroll int
	presetFocus  int
	soundCursor  int
	form         formState
	deleteTarget config.Preset

	finishSeen       int64
	finishWord       string
	finishAutoArmed  bool
	celebrating      bool
	celebrationStart time.Time
	finishedAt       time.Time
	confetti         *fx.Confetti
	dust             *fx.Dust
	backdrop         fx.Backdrop
	animTime         float64
	backdropTime     float64
	lastTick         time.Time

	toast      string
	toastUntil time.Time

	update      *protocol.UpdatePayload
	openURL     func(string) error
	openPath    func(string) error
	installInfo install.Info

	updateStep     updateStep
	updateKind     string
	updatePhase    string
	updateReceived int64
	updateTotal    int64
	updatePath     string
	updateErr      string
	updateType     string
	updateFocus    hitAction

	updateNotes    []notes.Line
	updateNotesKey string
	notesScroll    int

	checkingUpdate bool
	hoverUpdate    bool
	daemonClosing  bool
	updating       bool

	// Statistics tab. The daemon sends the aggregate on demand and the TUI
	// caches it; statsTab, statsRange and statsScroll are the view state.
	statsData    *protocol.StatsPayload
	statsLoaded  bool
	statsLoading bool
	statsTab     int
	statsRange   int
	statsScroll  int
}

// New creates the TUI model. It reads the per-user uninstall entry so the
// update dialog can highlight the action that matches this copy.
func New(pipe string) *Model {
	m := newModel(pipe, time.Now, time.Now().UnixNano())
	info, _ := install.Current()
	m.installInfo = info
	m.applyWindow = resizeTerminal
	m.windowOwned = termwin.OwnsConsole()
	m.windowPositionFn = func() (int, int, bool) { return termwin.Position(buildinfo.Name) }
	m.moveWindow = func(x, y int) bool { return termwin.Move(buildinfo.Name, x, y) }
	m.windowMaximizedFn = func() bool { return termwin.Maximized(buildinfo.Name) }
	return m
}

func newModel(pipe string, now func() time.Time, seed int64) *Model {
	if now == nil {
		now = time.Now
	}
	pal := theme.Get("midnight")
	cfg := config.Default()
	cfg.Theme = "midnight"
	m := &Model{
		pipe:        pipe,
		now:         now,
		seed:        seed,
		cfg:         cfg,
		pal:         pal,
		width:       80,
		height:      24,
		presetFocus: presetFocusList,
		connecting:  true,
		dust:        fx.NewDust(seed, 80, 24),
		backdrop:    fx.NewBackdrop(cfg.Background, seed, 80, 24),
		lastTick:    now(),
		openURL:     browser.Open,
		openPath:    explorer.ShowInFolder,
		applyWindow: func(int, int) {},
	}
	m.sendWindow = m.sendWindowSize
	return m
}

// Init starts the connection, the render tick and the terminal background
// query used by the transparent "none" background.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(connectCmd(m.pipe), tickCmd(100*time.Millisecond), tea.RequestBackgroundColor)
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, m.handleWindowSize(msg.Width, msg.Height)
	case tea.BackgroundColorMsg:
		if msg.Color == nil {
			// A malformed OSC 11 reply can carry a nil color; RGBA would
			// panic, and an unknown background must keep the active palette.
			return m, nil
		}
		r, g, b, _ := msg.RGBA()
		m.termBG = color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 0xFF}
		m.termLight = lightBackground(m.termBG)
		m.termBGKnown = true
		return m, nil
	case windowSaveMsg:
		return m, m.handleWindowSave(msg)
	case windowSavedMsg:
		m.handleWindowSaved(msg)
		return m, nil
	case tickMsg:
		return m, m.handleTick()
	case connectedMsg:
		return m, m.handleConnected(msg)
	case serverEventMsg:
		return m, m.handleServerEvent(msg)
	case sentMsg:
		if msg.err != nil {
			m.checkingUpdate = false
			if m.modal == modalUpdate && m.updateStep == updateStepProgress {
				m.updateStep = updateStepFailed
				m.updateErr = copy.T(copy.UpdateNetwork)
			}
			m.showToast(copy.T(copy.DaemonLost))
		}
		return m, nil
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	case tea.MouseClickMsg:
		return m, m.handleMouse(msg)
	case tea.MouseWheelMsg:
		m.handleMouseWheel(msg)
		return m, nil
	case tea.MouseMotionMsg:
		m.handleMouseMotion(msg)
		return m, nil
	}
	return m, nil
}

func connectCmd(pipe string) tea.Cmd {
	return func() tea.Msg {
		c, hello, err := daemon.Ensure(pipe)
		return connectedMsg{client: c, hello: hello, err: err}
	}
}

func reconnectCmd(pipe string) tea.Cmd {
	return func() tea.Msg {
		for {
			c, hello, err := daemon.Ensure(pipe)
			if err == nil {
				return connectedMsg{client: c, hello: hello}
			}
			time.Sleep(time.Second)
		}
	}
}

func readCmd(c *ipc.Client) tea.Cmd {
	return func() tea.Msg {
		msg, err := c.ReadMessage()
		return serverEventMsg{msg: msg, err: err}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) handleConnected(msg connectedMsg) tea.Cmd {
	if msg.err != nil {
		m.connected = false
		m.connecting = false
		m.checkingUpdate = false
		if m.client != nil {
			_ = m.client.Close()
			m.client = nil
		}
		m.showToast(copy.T(copy.Disconnected))
		if m.reconnecting {
			return nil
		}
		m.reconnecting = true
		return reconnectCmd(m.pipe)
	}
	m.client = msg.client
	m.connected = true
	m.connecting = false
	m.reconnecting = false
	m.checkingUpdate = false
	m.applyConfig(protocol.ConfigMessage{ConfigPayload: msg.hello.Config})
	m.setUpdate(msg.hello.Update)
	cmds := []tea.Cmd{readCmd(m.client)}
	if m.noneBackground() {
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	if cmd := m.noteHelloWindow(msg.hello.Window); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.modal == modalStats {
		if cmd := m.requestStats(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleServerEvent(msg serverEventMsg) tea.Cmd {
	if msg.err != nil {
		if m.client != nil {
			_ = m.client.Close()
			m.client = nil
		}
		m.connected = false
		m.connecting = false
		m.checkingUpdate = false
		m.showToast(copy.T(copy.DaemonLost))
		if m.reconnecting {
			return nil
		}
		m.reconnecting = true
		return reconnectCmd(m.pipe)
	}
	switch ev := msg.msg.(type) {
	case protocol.StateMessage:
		prev := m.state
		hadState := m.hasState
		m.applyState(ev)
		if m.modal == modalStats && hadState && statsChanged(prev, ev) {
			return tea.Batch(readCmd(m.client), m.requestStats())
		}
	case protocol.ConfigMessage:
		m.applyConfig(ev)
	case protocol.ErrorMessage:
		switch {
		case m.modal == modalStats:
			m.modal = modalNone
			m.statsLoading = false
			m.showToast(copy.T(copy.StatsUnavailable))
		case m.modal == modalUpdate && ev.Code == protocol.CodeInvalidCommand:
			m.updateStep = updateStepFailed
			m.updateErr = copy.T(copy.UpdateUnavailable)
		case m.checkingUpdate && ev.Code == protocol.CodeInvalidCommand:
			m.checkingUpdate = false
			m.showToast(copy.T(copy.ErrorUpdate))
		default:
			m.showToast(ev.Message)
		}
	case protocol.StatsMessage:
		payload := ev.Stats
		m.statsData = &payload
		m.statsLoaded = true
		m.statsLoading = false
		m.clampStatsScroll()
	case protocol.UpdateMessage:
		m.setUpdate(&ev.UpdatePayload)
	case protocol.UpdateResultMessage:
		m.handleUpdateResult(ev)
	case protocol.UpdateProgressMessage:
		m.handleUpdateProgress(ev)
	case protocol.DaemonClosingMessage:
		// The user quit the daemon from the tray or an update is closing it.
		// Exit instead of reconnecting: a reconnect would launch a new daemon.
		m.daemonClosing = true
		m.updating = ev.Reason == protocol.CloseReasonUpdate
		m.connected = false
		m.connecting = false
		m.reconnecting = false
		m.checkingUpdate = false
		return tea.Quit
	}
	if m.client != nil {
		return readCmd(m.client)
	}
	return nil
}

// finishAutoClose is how long the finished screen stays before the TUI
// dismisses it on its own. It starts when the finish arrives (or when the TUI
// connects to an already finished timer) and leaves the 6 s confetti burst
// untouched inside it.
const finishAutoClose = 10 * time.Second

func (m *Model) handleTick() tea.Cmd {
	now := m.now()
	dt := now.Sub(m.lastTick)
	if dt < 0 || dt > time.Second {
		dt = 100 * time.Millisecond
	}
	m.lastTick = now
	m.animTime += dt.Seconds()
	backdropPace := 1.0
	if m.status() == engine.Paused {
		// Slow the ambient clock instead of scaling absolute time inside the
		// effect: that would make the particles jump on pause and resume.
		backdropPace = 0.25
	}
	m.backdropTime += dt.Seconds() * backdropPace
	if m.dust != nil {
		m.dust.Update(dt.Seconds())
	}
	if m.celebrating && m.confetti != nil {
		m.confetti.Update(dt.Seconds())
	}
	if m.toast != "" && now.After(m.toastUntil) {
		m.toast = ""
	}
	if m.celebrating && now.Sub(m.celebrationStart) > 6*time.Second {
		m.confetti = nil
	}
	var autoClose tea.Cmd
	if m.finishAutoArmed && m.status() == engine.Finished && m.modal == modalNone &&
		!m.finishedAt.IsZero() && now.Sub(m.finishedAt) >= finishAutoClose {
		m.leaveFinished()
		autoClose = m.send(protocol.ClientMessage{Type: protocol.TypeDismiss})
	}
	next := 100 * time.Millisecond
	if m.celebrating && now.Sub(m.celebrationStart) < 6*time.Second {
		next = 33 * time.Millisecond
	}
	return tea.Batch(tickCmd(next), m.sampleWindow(), autoClose)
}

func (m *Model) applyConfig(cfg protocol.ConfigMessage) {
	// An older daemon has no background field. Keep the local choice in that
	// case instead of resetting the screen on every unrelated config broadcast.
	background := cfg.Background
	supported := background != ""
	if !config.ValidBackground(background) {
		if background == "" && config.ValidBackground(m.cfg.Background) {
			background = m.cfg.Background
		} else {
			background = config.DefaultBackground
		}
	}
	c := config.Config{
		Version:    config.Version,
		Theme:      cfg.Theme,
		Background: background,
		Sound:      cfg.Sound,
		SoundName:  cfg.SoundName,
		Volume:     cfg.Volume,
		Presets:    append([]config.Preset(nil), cfg.Presets...),
	}
	if !config.ValidTheme(c.Theme) {
		c.Theme = "midnight"
	}
	if c.SoundName == "" {
		// An older daemon does not send the sound fields at all.
		c.SoundName = config.DefaultSound
		c.Volume = config.DefaultVolume
	} else if !config.ValidSound(c.SoundName) {
		c.SoundName = config.DefaultSound
	}
	if !config.ValidVolume(c.Volume) {
		c.Volume = config.DefaultVolume
	}
	m.backgroundSupported = supported
	m.cfg = c
	m.pal = theme.Get(c.Theme)
	m.setBackdrop(c.Background)
	if m.modal == modalSound {
		m.soundCursor = soundIndex(c.SoundName)
	}
	if m.selected >= len(c.Presets) {
		m.selected = maxInt(0, len(c.Presets)-1)
	}
	if m.presetCursor >= len(c.Presets) {
		m.presetCursor = maxInt(0, len(c.Presets)-1)
	}
	m.clampPresetFocus()
	if m.form.mode == formEdit {
		found := false
		for _, p := range c.Presets {
			if p.ID == m.form.id {
				found = true
				break
			}
		}
		if !found {
			m.modal = modalNone
		}
	}
}

func (m *Model) applyState(st protocol.StateMessage) {
	first := !m.hasState
	prev := m.state
	m.state = st
	m.hasState = true
	m.snapAt = m.now()

	switch {
	case st.Status == string(engine.Finished) && st.FinishSeq != m.finishSeen:
		m.finishSeen = st.FinishSeq
		m.startCelebration(first)
	case first && st.Status == string(engine.Finished):
		m.startCelebration(true)
	case st.FinishSeq > m.finishSeen:
		// The TUI missed the finish because it was closed; do not celebrate a
		// timer that has already been restarted.
		m.finishSeen = st.FinishSeq
	}

	if st.Status != string(engine.Finished) {
		m.celebrating = false
		m.confetti = nil
		m.finishAutoArmed = false
	} else if m.modal != modalNone && m.modal != modalUpdate {
		m.modal = modalNone
		m.form.err = ""
	}

	if !first && prev.Status == string(engine.Running) && st.Status == string(engine.Running) &&
		st.FinishSeq == prev.FinishSeq &&
		(st.DurationMS != prev.DurationMS || st.Label != prev.Label) {
		m.showToast(copy.T(copy.ToastReplaced))
	}
}

// setUpdate stores the release the daemon announced. The TUI compares it with
// its own build so a daemon from a different version cannot advertise a release
// this binary already is.
func (m *Model) setUpdate(p *protocol.UpdatePayload) {
	if p == nil || p.URL == "" || !release.IsNewer(buildinfo.Version, p.Latest) {
		m.update = nil
		m.hoverUpdate = false
		m.resetUpdateNotes()
		if m.modal == modalUpdate {
			m.modal = modalNone
		}
		return
	}
	m.update = &protocol.UpdatePayload{
		Latest:   p.Latest,
		URL:      p.URL,
		Setup:    p.Setup,
		Portable: p.Portable,
		Notes:    p.Notes,
	}
	m.resetUpdateNotes()
}

// resetUpdateNotes drops the rendered pane and its scroll position when the
// announced release changes.
func (m *Model) resetUpdateNotes() {
	m.updateNotes = nil
	m.updateNotesKey = ""
	m.notesScroll = 0
}

// handleUpdateResult finishes a manual check: it updates the notice and gives
// the user a short confirmation in the status line.
func (m *Model) handleUpdateResult(msg protocol.UpdateResultMessage) {
	m.checkingUpdate = false
	if msg.Error != "" || msg.Latest == "" {
		m.showToast(copy.T(copy.ErrorUpdate))
		return
	}
	if release.IsNewer(buildinfo.Version, msg.Latest) && msg.URL != "" {
		m.setUpdate(&protocol.UpdatePayload{
			Latest:   msg.Latest,
			URL:      msg.URL,
			Setup:    msg.Setup,
			Portable: msg.Portable,
			Notes:    msg.Notes,
		})
		m.showToast(copy.T(copy.ToastUpdateFound, msg.Latest))
		return
	}
	m.setUpdate(nil)
	m.showToast(copy.T(copy.ToastUpToDate, msg.Latest))
}

// openUpdateModal opens the install/download chooser for the cached release.
func (m *Model) openUpdateModal() {
	if m.update == nil {
		return
	}
	m.modal = modalUpdate
	m.updateStep = updateStepChoose
	m.updateErr = ""
	m.updatePath = ""
	m.updateReceived = 0
	m.updateTotal = 0
	m.notesScroll = 0
	m.updateType = m.defaultUpdateType()
	m.updateFocus = m.primaryUpdateAction()
}

const (
	updateTypeSetup    = "setup"
	updateTypePortable = "portable"
)

// currentUpdateType is the kind of copy that is running. The type chooser
// shows it first and selects it by default.
func (m *Model) currentUpdateType() string {
	if m.installInfo.Installed {
		return updateTypeSetup
	}
	return updateTypePortable
}

func (m *Model) otherUpdateType(updateType string) string {
	if updateType == updateTypeSetup {
		return updateTypePortable
	}
	return updateTypeSetup
}

func (m *Model) updateTypeAvailable(updateType string) bool {
	if m.update == nil {
		return false
	}
	switch updateType {
	case updateTypeSetup:
		return m.update.Setup
	case updateTypePortable:
		return m.update.Portable
	}
	return false
}

func (m *Model) defaultUpdateType() string {
	current := m.currentUpdateType()
	if m.updateTypeAvailable(current) {
		return current
	}
	other := m.otherUpdateType(current)
	if m.updateTypeAvailable(other) {
		return other
	}
	return current
}

func (m *Model) ensureUpdateType() {
	if m.update == nil {
		m.updateType = ""
		return
	}
	if m.updateType == "" || !m.updateTypeAvailable(m.updateType) {
		m.updateType = m.defaultUpdateType()
	}
}

// toggleUpdateType is the Tab shortcut: it switches between the copy in use
// and the other one when both assets exist. The chooser hint advertises it at
// widths that can hold the label.
func (m *Model) toggleUpdateType() {
	m.ensureUpdateType()
	current := m.currentUpdateType()
	other := m.otherUpdateType(current)
	if !m.updateTypeAvailable(current) || !m.updateTypeAvailable(other) {
		return
	}
	if m.updateType == current {
		m.updateType = other
	} else {
		m.updateType = current
	}
	if m.updateFocus == hitUpdateInstall || m.updateFocus == hitUpdatePortable {
		m.updateFocus = m.primaryUpdateAction()
		return
	}
	m.ensureUpdateFocus()
}

// updateActions is the horizontal action row for the selected type.
func (m *Model) updateActions() []hitAction {
	m.ensureUpdateType()
	actions := make([]hitAction, 0, 3)
	if m.updateTypeAvailable(m.updateType) {
		if m.updateType == updateTypeSetup {
			actions = append(actions, hitUpdateInstall)
		} else {
			actions = append(actions, hitUpdatePortable)
		}
	}
	return append(actions, hitUpdateRelease, hitUpdateLater)
}

// primaryUpdateAction is the action Enter uses when the chooser opens and the
// action the highlight starts on.
func (m *Model) primaryUpdateAction() hitAction {
	m.ensureUpdateType()
	if m.updateTypeAvailable(m.updateType) {
		if m.updateType == updateTypeSetup {
			return hitUpdateInstall
		}
		return hitUpdatePortable
	}
	return hitUpdateRelease
}

// ensureUpdateFocus keeps the highlight inside the actions that are currently
// visible. It also repairs the focus after a release gains or loses an asset.
func (m *Model) ensureUpdateFocus() {
	m.ensureUpdateType()
	actions := m.updateActions()
	for _, action := range actions {
		if action == m.updateFocus {
			return
		}
	}
	m.updateFocus = m.primaryUpdateAction()
}

func (m *Model) moveUpdateFocus(delta int) {
	actions := m.updateActions()
	if len(actions) == 0 {
		m.updateFocus = hitNone
		return
	}
	m.ensureUpdateFocus()
	index := 0
	for i, action := range actions {
		if action == m.updateFocus {
			index = i
			break
		}
	}
	index = (index + delta) % len(actions)
	if index < 0 {
		index += len(actions)
	}
	m.updateFocus = actions[index]
}

func (m *Model) activateUpdateFocus() tea.Cmd {
	m.ensureUpdateFocus()
	switch m.updateFocus {
	case hitUpdateInstall:
		return m.startInstallUpdate(false)
	case hitUpdatePortable:
		return m.startPortableDownload()
	case hitUpdateRelease:
		m.openReleasePage()
		return nil
	case hitUpdateLater:
		m.modal = modalNone
		return nil
	default:
		return m.startPrimaryUpdate()
	}
}

func (m *Model) handleUpdateKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch m.updateStep {
	case updateStepChoose:
		switch {
		case key.Code == tea.KeyEnter:
			return m.activateUpdateFocus()
		case key.Code == tea.KeyUp:
			m.scrollUpdateNotes(-1)
		case key.Code == tea.KeyDown:
			m.scrollUpdateNotes(1)
		case key.Code == tea.KeyTab:
			m.toggleUpdateType()
		case key.Code == tea.KeyLeft:
			m.moveUpdateFocus(-1)
		case key.Code == tea.KeyRight:
			m.moveUpdateFocus(1)
		case key.Code == tea.KeyPgUp:
			if page := m.updateNotesPage(); page > 0 {
				m.scrollUpdateNotes(-maxInt(1, page-1))
			}
		case key.Code == tea.KeyPgDown:
			if page := m.updateNotesPage(); page > 0 {
				m.scrollUpdateNotes(maxInt(1, page-1))
			}
		case key.Code == tea.KeyHome:
			m.notesScroll = 0
		case key.Code == tea.KeyEnd:
			m.notesScroll = m.updateNotesMaxScroll()
		case text == "g":
			m.openReleasePage()
		case key.Code == tea.KeyEscape:
			m.modal = modalNone
		}
	case updateStepWarn:
		switch {
		case key.Code == tea.KeyEnter:
			return m.startInstallUpdate(true)
		case key.Code == tea.KeyEscape:
			m.updateStep = updateStepChoose
		}
	case updateStepPortableDone:
		switch {
		case key.Code == tea.KeyEnter, key.Code == tea.KeyEscape:
			m.modal = modalNone
		case text == "o":
			m.openUpdateFolder()
		}
	case updateStepFailed:
		switch {
		case key.Code == tea.KeyEnter:
			m.updateStep = updateStepChoose
			m.updateErr = ""
		case text == "g":
			m.openReleasePage()
		case key.Code == tea.KeyEscape:
			m.modal = modalNone
		}
	}
	return nil
}

func (m *Model) startPrimaryUpdate() tea.Cmd {
	m.ensureUpdateType()
	if m.updateTypeAvailable(m.updateType) {
		if m.updateType == updateTypeSetup {
			return m.startInstallUpdate(false)
		}
		return m.startPortableDownload()
	}
	if m.update != nil {
		m.showToast(copy.T(copy.UpdateNoAssets))
	}
	return nil
}

func (m *Model) startInstallUpdate(force bool) tea.Cmd {
	if m.update == nil || !m.update.Setup {
		m.showToast(copy.T(copy.UpdateUnavailable))
		return nil
	}
	m.updateType = updateTypeSetup
	m.updateFocus = hitUpdateInstall
	if !force && (m.status() == engine.Running || m.status() == engine.Paused) {
		m.updateStep = updateStepWarn
		m.updateErr = ""
		return nil
	}
	if !m.connected || m.client == nil {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.updateStep = updateStepProgress
	m.updateKind = "setup"
	m.updatePhase = "downloading"
	m.updateReceived = 0
	m.updateTotal = 0
	m.updateErr = ""
	m.updatePath = ""
	return m.send(protocol.ClientMessage{Type: protocol.TypeInstallUpdate, Force: force})
}

func (m *Model) startPortableDownload() tea.Cmd {
	if m.update == nil || !m.update.Portable {
		m.showToast(copy.T(copy.UpdateUnavailable))
		return nil
	}
	m.updateType = updateTypePortable
	m.updateFocus = hitUpdatePortable
	if !m.connected || m.client == nil {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.updateStep = updateStepProgress
	m.updateKind = "portable"
	m.updatePhase = "downloading"
	m.updateReceived = 0
	m.updateTotal = 0
	m.updateErr = ""
	m.updatePath = ""
	return m.send(protocol.ClientMessage{Type: protocol.TypeDownloadPortable})
}

func (m *Model) handleUpdateProgress(msg protocol.UpdateProgressMessage) {
	if m.modal != modalUpdate {
		if msg.Phase == "done" {
			m.showToast(copy.T(copy.ToastPortable, versionFromPath(msg.Path)))
		}
		return
	}
	if msg.Kind != "" {
		m.updateKind = msg.Kind
	}
	m.updatePhase = msg.Phase
	m.updateReceived = msg.Received
	m.updateTotal = msg.Total
	m.updatePath = msg.Path
	switch msg.Phase {
	case "failed":
		if msg.Error == "timer_active" && m.updateKind == "setup" {
			m.updateStep = updateStepWarn
			m.updateErr = ""
			return
		}
		m.updateStep = updateStepFailed
		m.updateErr = updateErrorText(msg.Error)
	case "done":
		m.updateStep = updateStepPortableDone
	default:
		m.updateStep = updateStepProgress
	}
}

func updateErrorText(code string) string {
	switch code {
	case "timer_active":
		return copy.T(copy.UpdateTimerActive)
	case "busy":
		return copy.T(copy.UpdateBusy)
	case "checksum":
		return copy.T(copy.UpdateChecksum)
	case "write":
		return copy.T(copy.UpdateWrite)
	case "unavailable", "unsupported":
		return copy.T(copy.UpdateUnavailable)
	case "setup_failed":
		return copy.T(copy.UpdateSetupFailed)
	default:
		return copy.T(copy.UpdateNetwork)
	}
}

func (m *Model) openReleasePage() {
	if m.update == nil {
		return
	}
	if err := m.openURL(m.update.URL); err != nil {
		m.showToast(copy.T(copy.ErrorOpenURL))
	}
}

func (m *Model) openUpdateFolder() {
	if m.updatePath == "" {
		return
	}
	if err := m.openPath(m.updatePath); err != nil {
		m.showToast(copy.T(copy.ErrorOpenFolder))
	}
}

func versionFromPath(path string) string {
	base := path
	if i := strings.LastIndexAny(base, `\/`); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimPrefix(base, "tuick-tack-")
	return base
}

// updateKey opens the cached update when the styled view has room for a modal;
// otherwise it asks for a fresh check.
func (m *Model) updateKey() tea.Cmd {
	if m.update != nil && !m.compact() {
		m.openUpdateModal()
		return nil
	}
	return m.checkUpdates()
}

// checkUpdates asks the daemon for a fresh release check. The daemon owns the
// request so the tray and every TUI share one result.
func (m *Model) checkUpdates() tea.Cmd {
	if m.checkingUpdate {
		return nil
	}
	if !m.connected || m.client == nil {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.checkingUpdate = true
	return m.send(protocol.ClientMessage{Type: protocol.TypeCheckUpdate})
}

func (m *Model) startCelebration(short bool) {
	m.finishWord = m.pickFinishWord()
	m.finishAutoArmed = true
	m.celebrating = true
	m.celebrationStart = m.now()
	m.finishedAt = m.now()
	if m.modal != modalUpdate {
		m.modal = modalNone
	}
	m.form.err = ""
	m.input = ""
	m.inputErr = ""
	w, h := m.viewSize()
	seed := m.seed + m.state.FinishSeq*7919
	m.confetti = fx.NewCelebration(seed, w, h, m.pal.Confetti, short)
	m.dust = fx.NewDust(seed, w, h)
}

// pickFinishWord chooses the big celebration word for the current finish. The
// session seed and finish_seq make it deterministic in tests, and the next
// word never repeats the previous one so two finishes in a row feel distinct.
func (m *Model) pickFinishWord() string {
	words := copy.HeroFinishedChoices
	n := len(words)
	if n == 0 {
		return copy.T(copy.HeroFinished)
	}
	rng := rand.New(rand.NewSource(m.seed + m.state.FinishSeq*104729))
	prev := -1
	for i, key := range words {
		if copy.T(key) == m.finishWord {
			prev = i
			break
		}
	}
	if prev < 0 || n == 1 {
		return copy.T(words[rng.Intn(n)])
	}
	i := rng.Intn(n - 1)
	if i >= prev {
		i++
	}
	return copy.T(words[i])
}

// leaveFinished clears the local celebration for the current finish and
// disarms the automatic close, so a manual repeat or dismiss cannot race the
// ten-second deadline.
func (m *Model) leaveFinished() {
	m.celebrating = false
	m.confetti = nil
	m.finishAutoArmed = false
}

func (m *Model) showToast(s string) {
	m.toast = s
	m.toastUntil = m.now().Add(3 * time.Second)
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Key()
	text := key.Text

	if m.compact() {
		if text == "q" {
			m.Close()
			return tea.Quit
		}
		if m.modal == modalConfirmStop {
			return m.handleModalKey(key)
		}
		if m.modal != modalNone {
			// The overlay cannot be drawn here; keep its state untouched for
			// when the terminal grows back.
			return nil
		}
		switch text {
		case "p", "v", "s", "b", "?":
			return nil
		}
	} else {
		if m.modal != modalNone {
			return m.handleModalKey(key)
		}
		if text == "q" {
			m.Close()
			return tea.Quit
		}
	}

	switch m.status() {
	case engine.Running, engine.Paused:
		return m.handleTimerKey(key)
	case engine.Finished:
		return m.handleFinishedKey(key)
	default:
		return m.handleIdleKey(key)
	}
}

func (m *Model) handleIdleKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeyEscape:
		m.input = ""
		m.inputErr = ""
		return nil
	case key.Code == tea.KeyBackspace:
		m.input = dropLastRune(m.input)
		m.inputErr = ""
		return nil
	case key.Code == tea.KeyEnter:
		return m.startTypedOrSelected()
	case key.Code == tea.KeyLeft:
		if len(m.cfg.Presets) > 0 {
			m.selected = (m.selected - 1 + len(m.cfg.Presets)) % len(m.cfg.Presets)
		}
		return nil
	case key.Code == tea.KeyRight:
		if len(m.cfg.Presets) > 0 {
			m.selected = (m.selected + 1) % len(m.cfg.Presets)
		}
		return nil
	case text == "?":
		m.modal = modalHelp
		return nil
	case text == "v":
		m.openSound()
		return nil
	case text == "p":
		m.openPresets()
		return nil
	case text == "t":
		return m.cycleTheme()
	case text == "b":
		if m.input != "" {
			return nil
		}
		return m.cycleBackground()
	case text == "u":
		return m.updateKey()
	case text == "s":
		if m.acceptsUnit() {
			m.appendInput("s")
			return nil
		}
		return m.openStats()
	case text == "m":
		if m.acceptsUnit() {
			m.appendInput("m")
			return nil
		}
		return m.toggleSound()
	case isDigitText(text) || text == ":" || text == "h":
		m.appendInput(text)
		return nil
	}
	return nil
}

func (m *Model) handleTimerKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeySpace || text == " ":
		return m.togglePause()
	case key.Code == tea.KeyEscape:
		m.modal = modalConfirmStop
		return nil
	case text == "+" || text == "=":
		return m.adjust(60)
	case text == "-":
		return m.adjust(-60)
	case text == "?":
		m.modal = modalHelp
		return nil
	case text == "v":
		m.openSound()
		return nil
	case text == "b":
		return m.cycleBackground()
	case text == "s":
		return m.openStats()
	case text == "m":
		return m.toggleSound()
	case text == "u":
		return m.updateKey()
	}
	return nil
}

func (m *Model) handleFinishedKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeyEnter || key.Code == tea.KeyEscape:
		m.leaveFinished()
		return m.send(protocol.ClientMessage{Type: protocol.TypeDismiss})
	case text == "r":
		m.leaveFinished()
		return m.send(protocol.ClientMessage{Type: protocol.TypeRestart})
	case text == "?":
		m.modal = modalHelp
		return nil
	case text == "v":
		m.openSound()
		return nil
	case text == "b":
		return m.cycleBackground()
	case text == "s":
		return m.openStats()
	case text == "m":
		return m.toggleSound()
	case text == "u":
		return m.updateKey()
	}
	return nil
}

func (m *Model) handleModalKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch m.modal {
	case modalHelp:
		if key.Code == tea.KeyEscape || key.Code == tea.KeyEnter || text == "?" {
			m.modal = modalNone
		}
		return nil
	case modalConfirmStop:
		switch {
		case key.Code == tea.KeyEnter:
			m.modal = modalNone
			return m.send(protocol.ClientMessage{Type: protocol.TypeStop})
		case key.Code == tea.KeyEscape:
			m.modal = modalNone
		}
		return nil
	case modalConfirmDelete:
		switch {
		case key.Code == tea.KeyEnter:
			return m.confirmDelete()
		case key.Code == tea.KeyEscape:
			m.modal = modalPresets
		}
		return nil
	case modalPresets:
		return m.handlePresetListKey(key)
	case modalPresetForm:
		return m.handleFormKey(key)
	case modalSound:
		return m.handleSoundKey(key)
	case modalUpdate:
		return m.handleUpdateKey(key)
	case modalStats:
		return m.handleStatsKey(key)
	}
	return nil
}

func (m *Model) handlePresetListKey(key tea.Key) tea.Cmd {
	text := key.Text
	n := len(m.cfg.Presets)
	m.clampPresetFocus()
	switch {
	case key.Code == tea.KeyEscape:
		m.modal = modalNone
	case key.Code == tea.KeyTab:
		// Tab moves between the list and the action bar; the bar is entered
		// at the first action that is available.
		if m.presetFocus == presetFocusList {
			m.presetFocus = m.firstPresetAction()
		} else {
			m.presetFocus = presetFocusList
		}
	case key.Code == tea.KeyLeft:
		if m.presetFocus != presetFocusList {
			m.movePresetFocus(-1)
		}
	case key.Code == tea.KeyRight:
		if m.presetFocus != presetFocusList {
			m.movePresetFocus(1)
		}
	case key.Code == tea.KeyUp:
		m.presetFocus = presetFocusList
		if n > 0 {
			m.presetCursor = (m.presetCursor - 1 + n) % n
		}
	case key.Code == tea.KeyDown:
		m.presetFocus = presetFocusList
		if n > 0 {
			m.presetCursor = (m.presetCursor + 1) % n
		}
	case key.Code == tea.KeyEnter:
		if m.presetFocus != presetFocusList {
			return m.activatePresetAction(m.presetFocus)
		}
		if p, ok := m.cursorPreset(); ok {
			m.selected = m.presetCursor
			m.modal = modalNone
			return m.startPreset(p)
		}
	case text == "a":
		return m.activatePresetAction(presetActionIndex(hitPresetAdd))
	case text == "e":
		return m.activatePresetAction(presetActionIndex(hitPresetEdit))
	case text == "d":
		return m.activatePresetAction(presetActionIndex(hitPresetDelete))
	case text == "r":
		return m.activatePresetAction(presetActionIndex(hitPresetRestore))
	}
	m.clampPresetCursor()
	return nil
}

func presetActionIndex(action hitAction) int {
	for i, def := range presetActionDefs {
		if def.action == action {
			return i
		}
	}
	return presetFocusList
}

func (m *Model) presetActionEnabled(i int) bool {
	if i < 0 || i >= len(presetActionDefs) {
		return false
	}
	return !presetActionDefs[i].needsPreset || len(m.cfg.Presets) > 0
}

func (m *Model) firstPresetAction() int {
	for i := 0; i < len(presetActionDefs); i++ {
		if m.presetActionEnabled(i) {
			return i
		}
	}
	return presetFocusList
}

// clampPresetFocus keeps the focus on an available action. When a preset is
// deleted while edit/delete/start is focused, focus moves to the next action.
func (m *Model) clampPresetFocus() {
	if m.presetFocus == presetFocusList || m.presetActionEnabled(m.presetFocus) {
		return
	}
	m.movePresetFocus(1)
}

func (m *Model) movePresetFocus(delta int) {
	if m.presetFocus == presetFocusList {
		m.presetFocus = m.firstPresetAction()
		return
	}
	for step := 1; step <= len(presetActionDefs); step++ {
		next := (m.presetFocus + delta*step) % len(presetActionDefs)
		if next < 0 {
			next += len(presetActionDefs)
		}
		if m.presetActionEnabled(next) {
			m.presetFocus = next
			return
		}
	}
}

// activatePresetAction routes keyboard activation through the same handler as
// a mouse click, so both input paths behave identically.
func (m *Model) activatePresetAction(i int) tea.Cmd {
	if !m.presetActionEnabled(i) {
		return nil
	}
	return m.performHit(hitbox{action: presetActionDefs[i].action})
}

func (m *Model) handleFormKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch key.Code {
	case tea.KeyEscape:
		m.modal = modalPresets
		m.form.err = ""
		return nil
	case tea.KeyTab:
		m.form.focus = (m.form.focus + 1) % 2
		return nil
	case tea.KeyEnter:
		return m.saveForm()
	case tea.KeyBackspace:
		if m.form.focus == 0 {
			m.form.label = dropLastRune(m.form.label)
		} else {
			m.form.duration = dropLastRune(m.form.duration)
		}
		m.form.err = ""
		return nil
	}
	if text == "" || len([]rune(text)) != 1 {
		return nil
	}
	r := []rune(text)[0]
	if unicode.IsControl(r) {
		return nil
	}
	if m.form.focus == 0 {
		if len([]rune(m.form.label)) < config.MaxLabelRunes {
			m.form.label += text
			m.form.err = ""
		}
		return nil
	}
	if len(m.form.duration) < 16 && (isDigitRune(r) || r == ':' || r == 'h' || r == 'm' || r == 's') {
		m.form.duration += string(r)
		m.form.err = ""
	}
	return nil
}

func (m *Model) handleSoundKey(key tea.Key) tea.Cmd {
	text := key.Text
	switch {
	case key.Code == tea.KeyEscape || key.Code == tea.KeyEnter:
		return m.closeSound()
	case key.Code == tea.KeyUp:
		m.soundCursor = (m.soundCursor - 1 + len(config.Sounds)) % len(config.Sounds)
		return m.selectSound(config.Sounds[m.soundCursor])
	case key.Code == tea.KeyDown:
		m.soundCursor = (m.soundCursor + 1) % len(config.Sounds)
		return m.selectSound(config.Sounds[m.soundCursor])
	case key.Code == tea.KeyLeft || text == "-":
		return m.setVolume(m.cfg.Volume - 5)
	case key.Code == tea.KeyRight || text == "+" || text == "=":
		return m.setVolume(m.cfg.Volume + 5)
	case key.Code == tea.KeySpace || text == " ":
		return m.previewSound()
	case text == "m" || text == "M":
		return m.toggleSound()
	}
	return nil
}

func (m *Model) handleMouse(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button != tea.MouseLeft {
		return nil
	}
	for i := len(m.hits) - 1; i >= 0; i-- {
		hit := m.hits[i]
		if msg.X < hit.x || msg.X >= hit.x+hit.w || msg.Y < hit.y || msg.Y >= hit.y+hit.h {
			continue
		}
		return m.performHit(hit)
	}
	return nil
}

// handleMouseMotion tracks whether the pointer is over the update notice, so
// it can show the hover background. All-motion mode is enabled in View.
func (m *Model) handleMouseMotion(msg tea.MouseMotionMsg) {
	hover := false
	for i := len(m.hits) - 1; i >= 0; i-- {
		hit := m.hits[i]
		if hit.action != hitUpdate {
			continue
		}
		if msg.X >= hit.x && msg.X < hit.x+hit.w && msg.Y >= hit.y && msg.Y < hit.y+hit.h {
			hover = true
		}
		break
	}
	m.hoverUpdate = hover
}

// handleMouseWheel scrolls the release notes while the chooser is open, or the
// Statistics section it is showing. It is the pointer equivalent of PgUp/PgDn.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) {
	if m.compact() {
		return
	}
	if m.modal == modalStats {
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scrollStats(-3)
		case tea.MouseWheelDown:
			m.scrollStats(3)
		}
		return
	}
	if m.modal != modalUpdate || m.updateStep != updateStepChoose {
		return
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.scrollUpdateNotes(-3)
	case tea.MouseWheelDown:
		m.scrollUpdateNotes(3)
	}
}

func (m *Model) performHit(hit hitbox) tea.Cmd {
	switch hit.action {
	case hitChip:
		if hit.index >= 0 && hit.index < len(m.cfg.Presets) {
			m.selected = hit.index
			return m.startPreset(m.cfg.Presets[hit.index])
		}
	case hitPlus:
		m.openForm(formAdd, config.Preset{})
	case hitPresetRow:
		m.presetCursor = hit.index
		m.presetFocus = presetFocusList
		m.clampPresetCursor()
	case hitPause:
		return m.togglePause()
	case hitAdjustPlus:
		return m.adjust(60)
	case hitAdjustMinus:
		return m.adjust(-60)
	case hitCancel:
		m.modal = modalConfirmStop
	case hitClose:
		m.closeModal()
	case hitConfirmYes:
		if m.modal == modalConfirmDelete {
			return m.confirmDelete()
		}
		m.modal = modalNone
		return m.send(protocol.ClientMessage{Type: protocol.TypeStop})
	case hitConfirmNo:
		if m.modal == modalConfirmDelete {
			m.modal = modalPresets
		} else {
			m.modal = modalNone
		}
	case hitPresetAdd:
		m.openForm(formAdd, config.Preset{})
	case hitPresetEdit:
		if p, ok := m.cursorPreset(); ok {
			m.openForm(formEdit, p)
		}
	case hitPresetDelete:
		if p, ok := m.cursorPreset(); ok {
			m.deleteTarget = p
			m.modal = modalConfirmDelete
		}
	case hitPresetRestore:
		m.showToast(copy.T(copy.ToastRestored))
		return m.send(protocol.ClientMessage{Type: protocol.TypePresetsRestoreDefault})
	case hitPresetStart:
		if p, ok := m.cursorPreset(); ok {
			m.modal = modalNone
			return m.startPreset(p)
		}
	case hitFormName:
		m.form.focus = 0
	case hitFormDuration:
		m.form.focus = 1
	case hitFormSave:
		return m.saveForm()
	case hitFormCancel:
		m.modal = modalPresets
		m.form.err = ""
	case hitRepeat:
		m.leaveFinished()
		return m.send(protocol.ClientMessage{Type: protocol.TypeRestart})
	case hitDismiss:
		m.leaveFinished()
		return m.send(protocol.ClientMessage{Type: protocol.TypeDismiss})
	case hitSoundRow:
		if hit.index >= 0 && hit.index < len(config.Sounds) {
			m.soundCursor = hit.index
			return m.selectSound(config.Sounds[hit.index])
		}
	case hitSoundBar:
		return m.setVolume(hit.index)
	case hitSoundVolumeDown:
		return m.setVolume(m.cfg.Volume - 5)
	case hitSoundVolumeUp:
		return m.setVolume(m.cfg.Volume + 5)
	case hitSoundPreview:
		return m.previewSound()
	case hitSoundMute:
		return m.toggleSound()
	case hitSoundClose:
		return m.closeSound()
	case hitUpdate:
		m.openUpdateModal()
	case hitUpdateInstall:
		return m.startInstallUpdate(false)
	case hitUpdatePortable:
		return m.startPortableDownload()
	case hitUpdateRelease:
		m.openReleasePage()
	case hitUpdateLater, hitUpdateBack:
		if m.updateStep == updateStepFailed || m.updateStep == updateStepPortableDone {
			m.modal = modalNone
		} else if m.updateStep == updateStepWarn {
			m.updateStep = updateStepChoose
		} else {
			m.modal = modalNone
		}
	case hitUpdateContinue:
		return m.startInstallUpdate(true)
	case hitUpdateOpenFolder:
		m.openUpdateFolder()
	case hitUpdateRetry:
		m.updateStep = updateStepChoose
		m.updateErr = ""
	case hitStatsTab:
		m.setStatsTab(hit.index)
	}
	return nil
}

func (m *Model) startTypedOrSelected() tea.Cmd {
	if strings.TrimSpace(m.input) != "" {
		d, err := duration.Parse(m.input)
		if err != nil {
			m.inputErr = copy.T(copy.ErrorDuration)
			return nil
		}
		m.input = ""
		m.inputErr = ""
		return m.send(protocol.ClientMessage{
			Type:    protocol.TypeStart,
			Seconds: int(d.Seconds()),
			Label:   duration.Short(d),
		})
	}
	if p, ok := m.selectedPreset(); ok {
		return m.startPreset(p)
	}
	m.inputErr = copy.T(copy.ErrorNoPreset)
	return nil
}

func (m *Model) startPreset(p config.Preset) tea.Cmd {
	if !config.ValidSeconds(p.Seconds) {
		m.showToast(copy.T(copy.ErrorDuration))
		return nil
	}
	return m.send(protocol.ClientMessage{
		Type:     protocol.TypeStart,
		Seconds:  p.Seconds,
		Label:    p.Label,
		PresetID: p.ID,
	})
}

func (m *Model) togglePause() tea.Cmd {
	switch m.status() {
	case engine.Running:
		return m.send(protocol.ClientMessage{Type: protocol.TypePause})
	case engine.Paused:
		return m.send(protocol.ClientMessage{Type: protocol.TypeResume})
	}
	return nil
}

func (m *Model) adjust(seconds int) tea.Cmd {
	switch m.status() {
	case engine.Running, engine.Paused:
		return m.send(protocol.ClientMessage{Type: protocol.TypeAdjust, DeltaSeconds: seconds})
	}
	return nil
}

func (m *Model) cycleTheme() tea.Cmd {
	next := theme.Next(m.cfg.Theme)
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.cfg.Theme = next
	m.pal = theme.Get(next)
	m.showToast(copy.T(copy.ToastTheme, next))
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetTheme, Name: next})
}

// setBackdrop swaps the ambient layer only when the name really changes, so a
// daemon broadcast does not reset the effect's phase.
func (m *Model) setBackdrop(name string) {
	if !config.ValidBackground(name) {
		name = config.DefaultBackground
	}
	if m.backdrop != nil && m.backdrop.Name() == name {
		return
	}
	w, h := m.viewSize()
	m.backdrop = fx.NewBackdrop(name, m.seed, w, h)
}

func (m *Model) cycleBackground() tea.Cmd {
	next := config.NextBackground(m.cfg.Background)
	m.cfg.Background = next
	m.setBackdrop(next)
	m.showToast(copy.T(copy.ToastBackground, copy.BackgroundName(next)))
	var cmds []tea.Cmd
	if m.connected && m.client != nil && m.backgroundSupported {
		cmds = append(cmds, m.send(protocol.ClientMessage{Type: protocol.TypeSetBackground, Name: next}))
	}
	if next == config.BackgroundNone {
		// The terminal profile may have changed; refresh the light/dark stage
		// palette when the user enters the transparent background.
		cmds = append(cmds, tea.RequestBackgroundColor)
	}
	return tea.Batch(cmds...)
}

func (m *Model) openSound() {
	m.modal = modalSound
	m.soundCursor = soundIndex(m.cfg.SoundName)
}

func (m *Model) closeSound() tea.Cmd {
	m.modal = modalNone
	if !m.connected {
		return nil
	}
	return m.send(protocol.ClientMessage{Type: protocol.TypeStopSound})
}

func (m *Model) selectSound(name string) tea.Cmd {
	if !config.ValidSound(name) {
		return nil
	}
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	m.soundCursor = soundIndex(name)
	if m.cfg.SoundName == name {
		return m.previewSound()
	}
	m.cfg.SoundName = name
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetSoundName, SoundName: name})
}

func (m *Model) setVolume(volume int) tea.Cmd {
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	if volume < config.MinVolume {
		volume = config.MinVolume
	}
	if volume > config.MaxVolume {
		volume = config.MaxVolume
	}
	volume = ((volume + 2) / 5) * 5
	if volume == m.cfg.Volume {
		return nil
	}
	m.cfg.Volume = volume
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetVolume, Volume: &volume})
}

func (m *Model) previewSound() tea.Cmd {
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	return m.send(protocol.ClientMessage{Type: protocol.TypePreviewSound})
}

func (m *Model) toggleSound() tea.Cmd {
	if !m.connected {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	next := !m.cfg.Sound
	m.cfg.Sound = next
	if next {
		m.showToast(copy.T(copy.ToastSoundOn))
	} else {
		m.showToast(copy.T(copy.ToastSoundOff))
	}
	return m.send(protocol.ClientMessage{Type: protocol.TypeSetSound, Enabled: &next})
}

func (m *Model) send(msg protocol.ClientMessage) tea.Cmd {
	if !m.connected || m.client == nil {
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	c := m.client
	return func() tea.Msg { return sentMsg{err: c.Send(msg)} }
}

func (m *Model) openPresets() {
	m.modal = modalPresets
	m.presetCursor = 0
	m.presetScroll = 0
	m.presetFocus = presetFocusList
	if len(m.cfg.Presets) == 0 {
		m.presetCursor = 0
	} else if m.selected >= 0 && m.selected < len(m.cfg.Presets) {
		m.presetCursor = m.selected
	}
}

func (m *Model) openForm(mode formMode, p config.Preset) {
	m.modal = modalPresetForm
	m.form = formState{
		mode:     mode,
		id:       p.ID,
		label:    p.Label,
		duration: formDuration(p.Seconds),
		focus:    0,
	}
	if mode == formAdd {
		m.form.label = ""
		m.form.duration = ""
	}
}

func (m *Model) saveForm() tea.Cmd {
	label := strings.TrimSpace(m.form.label)
	if !config.ValidLabel(label) {
		m.form.err = copy.T(copy.ErrorLabel)
		return nil
	}
	d, err := duration.Parse(m.form.duration)
	if err != nil {
		m.form.err = copy.T(copy.ErrorDuration)
		return nil
	}
	if !m.connected {
		m.form.err = copy.T(copy.ErrorOffline)
		return nil
	}
	seconds := int(d.Seconds())
	if m.form.mode == formAdd {
		if len(m.cfg.Presets) >= config.MaxPresets {
			m.form.err = copy.T(copy.ErrorPresets)
			return nil
		}
		m.showToast(copy.T(copy.ToastAdded, label))
		m.modal = modalPresets
		m.form.err = ""
		return m.send(protocol.ClientMessage{Type: protocol.TypePresetAdd, Label: label, Seconds: seconds})
	}
	m.showToast(copy.T(copy.ToastUpdated, label))
	m.modal = modalPresets
	m.form.err = ""
	return m.send(protocol.ClientMessage{
		Type:    protocol.TypePresetUpdate,
		ID:      m.form.id,
		Label:   label,
		Seconds: seconds,
	})
}

func (m *Model) confirmDelete() tea.Cmd {
	if !m.connected {
		m.modal = modalPresets
		m.showToast(copy.T(copy.ErrorOffline))
		return nil
	}
	id := m.deleteTarget.ID
	m.modal = modalPresets
	m.showToast(copy.T(copy.ToastDeleted))
	m.deleteTarget = config.Preset{}
	return m.send(protocol.ClientMessage{Type: protocol.TypePresetDelete, ID: id})
}

func (m *Model) closeModal() {
	switch m.modal {
	case modalHelp, modalConfirmStop:
		m.modal = modalNone
	case modalConfirmDelete, modalPresetForm:
		m.modal = modalPresets
	case modalPresets:
		m.modal = modalNone
	default:
		m.modal = modalNone
	}
	m.form.err = ""
}

func (m *Model) appendInput(s string) {
	if len(m.input)+len(s) > 16 {
		return
	}
	m.input += s
	m.inputErr = ""
}

func (m *Model) acceptsUnit() bool {
	if m.input == "" {
		return false
	}
	hasDigit := false
	for _, r := range m.input {
		if isDigitRune(r) {
			hasDigit = true
		}
	}
	if !hasDigit {
		return false
	}
	last := m.input[len(m.input)-1]
	return last != 'h' && last != 'm' && last != 's'
}

func (m *Model) selectedPreset() (config.Preset, bool) {
	if len(m.cfg.Presets) == 0 {
		return config.Preset{}, false
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.cfg.Presets) {
		m.selected = len(m.cfg.Presets) - 1
	}
	return m.cfg.Presets[m.selected], true
}

func (m *Model) cursorPreset() (config.Preset, bool) {
	if len(m.cfg.Presets) == 0 {
		return config.Preset{}, false
	}
	m.clampPresetCursor()
	return m.cfg.Presets[m.presetCursor], true
}

func (m *Model) clampPresetCursor() {
	if len(m.cfg.Presets) == 0 {
		m.presetCursor = 0
		m.presetScroll = 0
		return
	}
	if m.presetCursor < 0 {
		m.presetCursor = 0
	}
	if m.presetCursor >= len(m.cfg.Presets) {
		m.presetCursor = len(m.cfg.Presets) - 1
	}
}

func (m *Model) status() engine.Status {
	switch m.state.Status {
	case string(engine.Running):
		return engine.Running
	case string(engine.Paused):
		return engine.Paused
	case string(engine.Finished):
		return engine.Finished
	default:
		return engine.Idle
	}
}

func (m *Model) remaining() time.Duration {
	left := time.Duration(m.state.RemainingMS) * time.Millisecond
	if m.connected && m.state.Status == string(engine.Running) && m.hasState {
		left -= m.now().Sub(m.snapAt)
	}
	if left < 0 {
		left = 0
	}
	return left
}

func (m *Model) total() time.Duration {
	return time.Duration(m.state.DurationMS) * time.Millisecond
}

func (m *Model) progress() float64 {
	total := m.total()
	if total <= 0 {
		return 0
	}
	left := m.remaining()
	if left > total {
		left = total
	}
	p := 1 - float64(left)/float64(total)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

func (m *Model) finishLabel() string {
	if label := strings.TrimSpace(m.state.Label); label != "" {
		return label
	}
	return duration.Short(m.total())
}

func (m *Model) toastActive() bool {
	return m.toast != "" && m.now().Before(m.toastUntil)
}

func (m *Model) viewSize() (int, int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

// Close releases the pipe connection. Called after the Bubble Tea program
// exits so the daemon keeps running. A pending placement is sent first so a
// resize or move followed by a quick quit is not lost.
func (m *Model) Close() {
	if m.client != nil {
		if m.connected {
			m.flushWindow()
		}
		_ = m.client.Close()
		m.client = nil
	}
	m.connected = false
}

// HasActiveTimer reports whether the TUI is leaving a running or paused timer
// behind in the tray.
func (m *Model) HasActiveTimer() bool {
	if m.daemonClosing {
		return false
	}
	switch m.status() {
	case engine.Running, engine.Paused:
		return true
	}
	return false
}

// DaemonClosing reports whether the TUI is exiting because the user quit the
// daemon from the tray.
func (m *Model) DaemonClosing() bool { return m.daemonClosing }

// Updating reports whether the daemon closed to install an update.
func (m *Model) Updating() bool { return m.updating }

func formDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	if seconds%60 == 0 {
		return strconv.Itoa(seconds / 60)
	}
	return duration.Format(time.Duration(seconds) * time.Second)
}

func dropLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func isDigitText(s string) bool {
	if len([]rune(s)) != 1 {
		return false
	}
	return isDigitRune([]rune(s)[0])
}

func isDigitRune(r rune) bool { return r >= '0' && r <= '9' }

func soundIndex(name string) int {
	for i, s := range config.Sounds {
		if s == name {
			return i
		}
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
